package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
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
	readFilePrevRunStubFmt        = "[previously analyzed: %s was read in a prior run; its violations are already pre-loaded — do not re-analyze this file]"
)

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
