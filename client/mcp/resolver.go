package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

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
	emptyObjectSchema               = `{"type":"object","properties":` + c.EmptyJSONObject + `}`
)

var schemaEmpty = json.RawMessage(emptyObjectSchema)

const (
	readFileSchema                = `{"type":"object","properties":{"path":{"type":"string","description":"Relative file path"}},"required":["path"]}`
	classDepsSchema               = `{"type":"object","properties":{"class":{"type":"string","description":"Class name or file path"}},"required":["class"]}`
	externalContextSchema         = `{"type":"object","properties":{"query":{"type":"string","description":"Read-only context query"},"system":{"type":"string","description":"External system name as configured in archguard.yaml (e.g. notion, youtrack, jira)"}},"required":["query"]}`
	reportViolationSchema         = `{"type":"object","properties":{"file":{"type":"string"},"line":{"type":"number"},"severity":{"type":"string"},"category":{"type":"string"},"rule":{"type":"string"},"description":{"type":"string"},"suggestion":{"type":"string"}},"required":["file","severity","category","rule","description"]}`
	msgToolDenied                 = "Tool call denied (%s)"
	errUnknownTool                = "unknown tool: %s"
	errPathRequired               = "path is required"
	errRelativePathRequired       = "invalid path: must be relative"
	errPathTraversal              = "path traversal not allowed"
	errReadFile                   = "failed to read %s: %v"
	errFileNotFoundDisplay        = "file not found: %s"
	errFileTooLarge               = "file too large (%d bytes); max %d bytes"
	errSensitivePath              = "blocked sensitive path: %s"
	lineNumberFormat              = "%4d | %s\n"
	lineSeparator                 = "\n"
	warnSkipInaccessiblePath      = "warning: skipping inaccessible path: %v\n"
	warnExtractDepsRecovered      = "extractDeps recovered from panic in lang=%s: %v\n"
	errClassRequired              = "class is required"
	errFileNotFound               = "file not found for %q"
	msgNoDocumentation            = "No project documentation was provided."
	errExternalQueryRequired      = "query is required"
	errExternalSystemNotAllowed   = "external system not allowed: %s"
	msgExternalContextUnavailable = "External context lookup for %q in %s was allowed but no connector is configured."
	errViolationRequired          = "required: file, severity, rule, description"
	violationIDFormat             = "v%03d"
	msgViolationRecorded          = "Violation %s recorded: [%s] %s in %s:%d"
	toolResultProjectStructure    = "project structure"
	toolResultArchRules           = "arch rules"
	toolResultClassDeps           = "class deps"
	toolResultMarshalError        = "failed to marshal result"
	operationReadFile             = "read file"
	namespaceWildcardSuffix       = "\\*"
	pathTraversalPrefix           = ".."
	hiddenPathPrefix              = "."
	phpNamespaceSeparator         = "\\"
	pathSeparator                 = "/"
	maxReadFileBytes              = 10 * 1024 * 1024
)

type ConsentChecker interface {
	Check(toolName string, args map[string]interface{}, budget int) string
}

type AuditLogger interface {
	Record(toolName, args, decision string, size int, err error)
}

type ExternalSearcher interface {
	Search(query string) (string, error)
}

type Resolver struct {
	cfg         *config.Config
	projectPath string
	docsPath    string
	extensions  []string
	consent     ConsentChecker
	auditLog    AuditLogger
	externals   map[string]ExternalSearcher
	mu          sync.Mutex
	violations  []models.Violation
	nextID      int
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

func NewResolver(cfg *config.Config, projectPath, docsPath string, exts []string, cm ConsentChecker, al AuditLogger, externals map[string]ExternalSearcher) *Resolver {
	return &Resolver{
		cfg:         cfg,
		projectPath: projectPath,
		docsPath:    docsPath,
		extensions:  exts,
		consent:     cm,
		auditLog:    al,
		externals:   externals,
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

func fmtArgs(args map[string]interface{}) string {
	if args == nil {
		return c.EmptyJSONObject
	}
	data, err := json.Marshal(args)
	if err != nil {
		return c.EmptyJSONObject
	}
	return string(data)
}
