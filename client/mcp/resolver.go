package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/archguard/project/client/audit"
	"github.com/archguard/project/client/consent"
	"github.com/archguard/project/shared/apperrors"
	"github.com/archguard/project/shared/config"
	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/models"
)

const (
	toolProjectStructureDescription = "Returns the project file tree."
	toolReadFileDescription         = "Reads source file content with line numbers."
	toolArchRulesDescription        = "Returns layer definitions, dependency rules, and domain rules."
	toolClassDepsDescription        = "Analyzes imports of a class/file with layer mapping."
	toolDocumentationDescription    = "Returns project documentation."
	toolExternalContextDescription  = "Fetches read-only context from allowed external systems."
	toolReportViolationDescription  = "Records an architectural violation. Call IMMEDIATELY when found."
	emptyObjectSchema               = `{"type":"object","properties":{}}`
)

var schemaEmpty = json.RawMessage(emptyObjectSchema)

const (
	readFileSchema                  = `{"type":"object","properties":{"path":{"type":"string","description":"Relative file path"}},"required":["path"]}`
	classDepsSchema                 = `{"type":"object","properties":{"class":{"type":"string","description":"Class name or file path"}},"required":["class"]}`
	externalContextSchema           = `{"type":"object","properties":{"query":{"type":"string","description":"Read-only context query"},"system":{"type":"string","description":"External system: notion or youtrack"}},"required":["query"]}`
	reportViolationSchema           = `{"type":"object","properties":{"file":{"type":"string"},"line":{"type":"number"},"severity":{"type":"string"},"category":{"type":"string"},"rule":{"type":"string"},"description":{"type":"string"},"suggestion":{"type":"string"}},"required":["file","severity","category","rule","description"]}`
	msgToolDenied                   = "Tool call denied (%s)"
	errUnknownTool                  = "unknown tool: %s"
	errPathRequired                 = "path is required"
	errRelativePathRequired         = "invalid path: must be relative"
	errPathTraversal                = "path traversal not allowed"
	errReadFile                     = "failed to read %s: %v"
	errSensitivePath                = "blocked sensitive path: %s"
	lineNumberFormat                = "%4d | %s\n"
	errClassRequired                = "class is required"
	errFileNotFound                 = "file not found for %q"
	msgNoDocumentation              = "No project documentation was provided."
	errExternalQueryRequired        = "query is required"
	errExternalSystemNotAllowed     = "external system not allowed: %s"
	msgExternalContextUnavailable   = "External context lookup for %q in %s was allowed but no connector is configured."
	errViolationRequired            = "required: file, severity, rule, description"
	violationIDFormat               = "v%03d"
	msgViolationRecorded            = "Violation %s recorded: [%s] %s in %s:%d"
	namespaceWildcardSuffix         = "\\*"
	pathTraversalPrefix             = ".."
	hiddenPathPrefix                = "."
	phpNamespaceSeparator           = "\\"
	pathSeparator                   = "/"
	emptyArgsJSON                   = "{}"
)

type Resolver struct {
	cfg         *config.Config
	projectPath string
	docsPath    string
	extensions  []string
	consent     *consent.Manager
	auditLog    *audit.Log
	mu          sync.Mutex
	violations  []models.Violation
	nextID      int
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

func NewResolver(cfg *config.Config, projectPath, docsPath string, exts []string, cm *consent.Manager, al *audit.Log) *Resolver {
	return &Resolver{
		cfg: cfg, projectPath: projectPath, docsPath: docsPath,
		extensions: exts, consent: cm, auditLog: al,
	}
}

func (r *Resolver) ListTools() []ToolDefinition {
	return []ToolDefinition{
		{Name: c.ToolGetProjectStructure, Description: toolProjectStructureDescription, InputSchema: schemaEmpty},
		{Name: c.ToolReadFile, Description: toolReadFileDescription, InputSchema: json.RawMessage(readFileSchema)},
		{Name: c.ToolGetArchitectureRules, Description: toolArchRulesDescription, InputSchema: schemaEmpty},
		{Name: c.ToolGetClassDependencies, Description: toolClassDepsDescription, InputSchema: json.RawMessage(classDepsSchema)},
		{Name: c.ToolGetDocumentation, Description: toolDocumentationDescription, InputSchema: schemaEmpty},
		{Name: c.ToolGetExternalContext, Description: toolExternalContextDescription, InputSchema: json.RawMessage(externalContextSchema)},
		{Name: c.ToolReportViolation, Description: toolReportViolationDescription, InputSchema: json.RawMessage(reportViolationSchema)},
	}
}

func (r *Resolver) ExecuteTool(toolName string, args map[string]interface{}, budget int) (string, error) {
	decision := r.consent.Check(toolName, args, budget)
	if decision == c.DecisionBlocked || decision == c.DecisionUserDenied {
		r.auditLog.Record(toolName, fmtArgs(args), decision, 0, nil)
		msg := fmt.Sprintf(msgToolDenied, decision)
		return msg, apperrors.PermissionDenied(msg)
	}

	result, err := r.dispatch(toolName, args)
	r.auditLog.Record(toolName, fmtArgs(args), decision, len(result), err)
	return result, err
}

func (r *Resolver) ListSourceFiles() []string {
	var files []string
	r.walkProjectFiles(func(rel string, _ os.FileInfo) {
		files = append(files, rel)
	})
	return files
}

func (r *Resolver) GetViolations() []models.Violation {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.Violation, len(r.violations))
	copy(out, r.violations)
	return out
}

func (r *Resolver) dispatch(tool string, args map[string]interface{}) (string, error) {
	switch tool {
	case c.ToolGetProjectStructure:
		return r.projectStructure()
	case c.ToolReadFile:
		p, _ := args[c.ArgPath].(string)
		return r.readFile(p)
	case c.ToolGetArchitectureRules:
		return r.archRules()
	case c.ToolGetClassDependencies:
		cl, _ := args[c.ArgClass].(string)
		return r.classDeps(cl)
	case c.ToolGetDocumentation:
		return r.documentation()
	case c.ToolGetExternalContext:
		query, _ := args[c.ArgQuery].(string)
		system, _ := args[c.ArgSystem].(string)
		return r.externalContext(query, system)
	case c.ToolReportViolation:
		return r.reportViolation(args)
	default:
		return "", apperrors.Validation(fmt.Sprintf(errUnknownTool, tool))
	}
}

func (r *Resolver) walkProjectFiles(fn func(rel string, info os.FileInfo)) {
	filepath.Walk(r.projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping inaccessible path: %v\n", err)
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), hiddenPathPrefix) {
				return filepath.SkipDir
			}
			for _, d := range c.SkipDirs {
				if info.Name() == d {
					return filepath.SkipDir
				}
			}
			return nil
		}
		rel, _ := filepath.Rel(r.projectPath, path)
		if c.IsSensitivePath(rel) {
			return nil
		}
		ext := strings.TrimPrefix(filepath.Ext(path), hiddenPathPrefix)
		if len(r.extensions) > 0 && !slices.Contains(r.extensions, ext) {
			return nil
		}
		fn(rel, info)
		return nil
	})
}

func (r *Resolver) projectStructure() (string, error) {
	var files []map[string]interface{}
	r.walkProjectFiles(func(rel string, info os.FileInfo) {
		ext := strings.TrimPrefix(filepath.Ext(info.Name()), hiddenPathPrefix)
		files = append(files, map[string]interface{}{
			c.JSONKeyPath: rel, c.JSONKeyName: info.Name(), c.JSONKeyExtension: ext, c.JSONKeySize: info.Size(),
		})
	})
	data, _ := json.MarshalIndent(map[string]interface{}{
		c.JSONKeyRoot: r.projectPath, c.JSONKeyFiles: files, c.JSONKeyTotal: len(files),
	}, "", c.JSONIndent)
	return string(data), nil
}

func readSourceFile(fullPath, displayPath string) ([]byte, error) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apperrors.NotFound(fmt.Sprintf("file not found: %s", displayPath))
		}
		return nil, apperrors.Wrap(apperrors.KindInternal, "read file", fmt.Sprintf(errReadFile, displayPath, err), err)
	}
	return data, nil
}

func (r *Resolver) readFile(relPath string) (string, error) {
	if relPath == "" {
		return "", apperrors.Validation(errPathRequired)
	}
	clean := filepath.Clean(relPath)
	if strings.HasPrefix(clean, pathTraversalPrefix) || filepath.IsAbs(clean) {
		return "", apperrors.PermissionDenied(errRelativePathRequired)
	}
	if c.IsSensitivePath(clean) {
		return "", apperrors.PermissionDenied(fmt.Sprintf(errSensitivePath, relPath))
	}
	full := filepath.Join(r.projectPath, clean)
	absProj, _ := filepath.Abs(r.projectPath)
	absFile, _ := filepath.Abs(full)
	rel, err := filepath.Rel(absProj, absFile)
	if err != nil || strings.HasPrefix(rel, pathTraversalPrefix) || filepath.IsAbs(rel) {
		return "", apperrors.PermissionDenied(errPathTraversal)
	}
	data, err := readSourceFile(full, relPath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	var sb strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&sb, lineNumberFormat, i+1, line)
	}
	return sb.String(), nil
}

func (r *Resolver) archRules() (string, error) {
	data, _ := json.MarshalIndent(map[string]interface{}{
		c.JSONKeyLayers: r.cfg.Layers, c.JSONKeyRules: r.cfg.Rules,
		c.JSONKeyDomain: r.cfg.DomainContext.Domain, c.JSONKeyDomainDesc: r.cfg.DomainContext.Description,
		c.JSONKeyDomainRules: r.cfg.DomainContext.Rules,
	}, "", c.JSONIndent)
	return string(data), nil
}

func (r *Resolver) classDeps(classOrPath string) (string, error) {
	if classOrPath == "" {
		return "", apperrors.Validation(errClassRequired)
	}
	fp, err := r.resolveFile(classOrPath)
	if err != nil {
		return "", err
	}
	content, err := readSourceFile(filepath.Join(r.projectPath, fp), fp)
	if err != nil {
		return "", err
	}
	data, _ := json.MarshalIndent(map[string]interface{}{
		c.JSONKeyClass: classOrPath, c.JSONKeyFile: fp,
		c.JSONKeyLayer: r.resolveLayer(classOrPath), c.JSONKeyDependencies: r.extractDeps(string(content)),
	}, "", c.JSONIndent)
	return string(data), nil
}

func (r *Resolver) documentation() (string, error) {
	if r.docsPath == "" {
		return msgNoDocumentation, nil
	}
	data, err := readSourceFile(r.docsPath, r.docsPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *Resolver) externalContext(query, system string) (string, error) {
	if query == "" {
		return "", apperrors.Validation(errExternalQueryRequired)
	}
	system = strings.ToLower(strings.TrimSpace(system))
	if system == "" {
		system = "notion"
	}
	if !c.AllowedExternalSystems[system] {
		return "", apperrors.PermissionDenied(fmt.Sprintf(errExternalSystemNotAllowed, system))
	}
	return fmt.Sprintf(msgExternalContextUnavailable, query, system), nil
}

func (r *Resolver) reportViolation(args map[string]interface{}) (string, error) {
	file, _ := args[c.ArgFile].(string)
	sev, _ := args[c.ArgSeverity].(string)
	cat, _ := args[c.ArgCategory].(string)
	rule, _ := args[c.ArgRule].(string)
	desc, _ := args[c.ArgDescription].(string)
	sug, _ := args[c.ArgSuggestion].(string)
	var line int
	if v, ok := args[c.ArgLine].(float64); ok {
		line = int(v)
	}
	if file == "" || sev == "" || rule == "" || desc == "" {
		return "", apperrors.Validation(errViolationRequired)
	}

	r.mu.Lock()
	r.nextID++
	id := fmt.Sprintf(violationIDFormat, r.nextID)
	r.violations = append(r.violations, models.Violation{
		ID: id, File: file, Line: line, Severity: sev,
		Category: cat, Rule: rule, Description: desc, Suggestion: sug,
	})
	r.mu.Unlock()

	return fmt.Sprintf(msgViolationRecorded, id, sev, rule, file, line), nil
}

func (r *Resolver) resolveFile(classOrPath string) (string, error) {
	if strings.Contains(classOrPath, pathSeparator) {
		clean := filepath.Clean(classOrPath)
		if strings.HasPrefix(clean, pathTraversalPrefix) || filepath.IsAbs(clean) || c.IsSensitivePath(clean) {
			return "", apperrors.PermissionDenied(fmt.Sprintf(errSensitivePath, classOrPath))
		}
		if _, err := os.Stat(filepath.Join(r.projectPath, clean)); err == nil {
			return clean, nil
		}
	}
	parts := strings.Split(classOrPath, phpNamespaceSeparator)
	if len(parts) > 1 {
		srcRoot := r.cfg.Project.SrcRoot
		if srcRoot == "" {
			srcRoot = c.DefaultSrcRoot
		}
		ext := c.LangFileExtensions[r.cfg.Project.Language]
		if ext == "" {
			ext = c.DefaultPHPFile
		}
		rel := filepath.Join(srcRoot, filepath.Join(parts[1:]...)) + ext
		if c.IsSensitivePath(rel) {
			return "", apperrors.PermissionDenied(fmt.Sprintf(errSensitivePath, rel))
		}
		if _, err := os.Stat(filepath.Join(r.projectPath, rel)); err == nil {
			return rel, nil
		}
	}
	return "", apperrors.NotFound(fmt.Sprintf(errFileNotFound, classOrPath))
}

func (r *Resolver) resolveLayer(name string) string {
	for _, layer := range r.cfg.Layers {
		for _, ns := range layer.Namespaces {
			pattern := strings.TrimSuffix(strings.TrimSuffix(ns, namespaceWildcardSuffix), "*")
			if strings.HasPrefix(name, pattern) {
				return layer.Name
			}
		}
	}
	return c.UnknownValue
}

var depPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^use\s+(.+?)\s*;`),
	regexp.MustCompile(`(?m)^from\s+(\S+)\s+import`),
	regexp.MustCompile(`(?m)^import\s+(?:\{[^}]*\}|\w+)\s+from\s+['"](.+?)['"]`),
}

func (r *Resolver) extractDeps(source string) []map[string]string {
	var deps []map[string]string
	for _, re := range depPatterns {
		for _, m := range re.FindAllStringSubmatch(source, -1) {
			if len(m) > 1 {
				deps = append(deps, map[string]string{
					c.JSONKeyClass: m[1], c.JSONKeyLayer: r.resolveLayer(m[1]), c.JSONKeyType: c.JSONValueImport,
				})
			}
		}
	}
	return deps
}

func fmtArgs(args map[string]interface{}) string {
	if args == nil {
		return emptyArgsJSON
	}
	data, _ := json.Marshal(args)
	return string(data)
}

