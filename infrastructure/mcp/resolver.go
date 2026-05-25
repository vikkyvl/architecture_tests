package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/archguard/project/config"
	"github.com/archguard/project/shared/apperrors"
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
	maxFileBytesSuffixFmt         = "\n...\n[file truncated after %d bytes; %d bytes remain — request specific line ranges if you need the rest]"
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
	readFileDedupStubFmt          = "[dedup: file %s (%d bytes) already returned earlier in this run; refer to the prior tool_result for content]"
)

type ConsentChecker interface {
	Check(toolName string, args map[string]interface{}, budget int) string
}

type AuditLogger interface {
	Record(toolName, args, decision string, size int, err error, dedup bool)
}

type ExternalSearcher interface {
	Search(query string) (string, error)
}

type Resolver struct {
	projectPath     string
	docsPath        string
	extensions      []string
	maxFileBytes    int
	srcRoot         string
	language        string
	layers          []config.LayerConfig
	rules           []config.RuleConfig
	domainContext   config.DomainContextConfig
	consent         ConsentChecker
	auditLog        AuditLogger
	externals       map[string]ExternalSearcher
	mu              sync.Mutex
	violations      []models.Violation
	nextID          int
	readFiles       map[string]readFileEntry
	readFilesMu     sync.Mutex
	readTurn        int
	keepWindowTurns int
}

type readFileEntry struct {
	Turn int
	Size int
}

type ResolverConfig struct {
	ProjectPath     string
	DocsPath        string
	Extensions      []string
	MaxFileBytes    int
	SrcRoot         string
	Language        string
	Layers          []config.LayerConfig
	Rules           []config.RuleConfig
	DomainContext   config.DomainContextConfig
	KeepWindowTurns int
}

func NewResolver(cfg ResolverConfig, cm ConsentChecker, al AuditLogger, externals map[string]ExternalSearcher) *Resolver {
	return &Resolver{
		projectPath:     cfg.ProjectPath,
		docsPath:        cfg.DocsPath,
		extensions:      cfg.Extensions,
		maxFileBytes:    cfg.MaxFileBytes,
		srcRoot:         cfg.SrcRoot,
		language:        cfg.Language,
		layers:          cfg.Layers,
		rules:           cfg.Rules,
		domainContext:   cfg.DomainContext,
		consent:         cm,
		auditLog:        al,
		externals:       externals,
		readFiles:       make(map[string]readFileEntry),
		keepWindowTurns: cfg.KeepWindowTurns,
	}
}

func (r *Resolver) ListTools() []models.ToolDefinition {
	return []models.ToolDefinition{
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
		r.auditLog.Record(toolName, fmtArgs(args), decision, 0, nil, false)
		msg := fmt.Sprintf(msgToolDenied, decision)
		return msg, apperrors.PermissionDenied(msg)
	}

	result, dedup, err := r.dispatch(toolName, args)
	r.auditLog.Record(toolName, fmtArgs(args), decision, len(result), err, dedup)
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

func (r *Resolver) dispatch(tool string, args map[string]interface{}) (string, bool, error) {
	switch tool {
	case c.ToolGetProjectStructure:
		res, err := r.projectStructure()
		return res, false, err
	case c.ToolReadFile:
		p, _ := args[c.ArgPath].(string)
		return r.readFile(p)
	case c.ToolGetArchitectureRules:
		res, err := r.archRules()
		return res, false, err
	case c.ToolGetClassDependencies:
		cl, _ := args[c.ArgClass].(string)
		res, err := r.classDeps(cl)
		return res, false, err
	case c.ToolGetDocumentation:
		res, err := r.documentation()
		return res, false, err
	case c.ToolGetExternalContext:
		query, _ := args[c.ArgQuery].(string)
		system, _ := args[c.ArgSystem].(string)
		res, err := r.externalContext(query, system)
		return res, false, err
	case c.ToolReportViolation:
		res, err := r.reportViolation(args)
		return res, false, err
	default:
		return "", false, apperrors.Validation(fmt.Sprintf(errUnknownTool, tool))
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
