package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/archguard/project/config"
	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/models"
)

var schemaEmpty = json.RawMessage(emptyObjectSchema)

type ConsentChecker interface {
	Check(toolName string, args map[string]interface{}, budget int) string
}

type AuditLogger interface {
	Record(toolName, args, decision string, size int, err error, dedup bool)
}

type ExternalSearcher interface {
	Search(query string) (string, error)
}

type ResolverConfig struct {
	ProjectPath        string
	DocsPath           string
	Extensions         []string
	MaxFileBytes       int
	SrcRoot            string
	Language           string
	Layers             []config.LayerConfig
	Rules              []config.RuleConfig
	DomainContext      config.DomainContextConfig
	KeepWindowTurns    int
	InitialViolations  []models.Violation // pre-loaded from a previous run
	PreviouslyAnalyzed []string           // files read in a previous run; return stub instead of content
}

type readFileEntry struct {
	Turn int
	Size int
}

type Observer interface {
	LimitReached(message string)
	Retry(provider string, attempt, maxAttempts int, wait time.Duration, err error)
	LLMText(text string)
	ToolResult(callNumber int, toolName string, resultSize int, err error)
	AnalysisComplete()
}

type Resolver struct {
	projectPath        string
	docsPath           string
	extensions         []string
	maxFileBytes       int
	srcRoot            string
	language           string
	layers             []config.LayerConfig
	rules              []config.RuleConfig
	domainContext      config.DomainContextConfig
	consent            ConsentChecker
	auditLog           AuditLogger
	externals          map[string]ExternalSearcher
	mu                 sync.Mutex
	violations         []models.Violation
	nextID             int
	readFiles          map[string]readFileEntry
	readFilesMu        sync.Mutex
	readTurn           int
	keepWindowTurns    int
	previouslyAnalyzed map[string]bool // files from a prior run; immutable after construction
	observer           Observer
}

func NewResolver(cfg ResolverConfig, cm ConsentChecker, al AuditLogger, externals map[string]ExternalSearcher, observer Observer) *Resolver {
	prevViolations := make([]models.Violation, len(cfg.InitialViolations))
	copy(prevViolations, cfg.InitialViolations)

	prevAnalyzed := make(map[string]bool, len(cfg.PreviouslyAnalyzed))
	for _, p := range cfg.PreviouslyAnalyzed {
		prevAnalyzed[filepath.Clean(p)] = true
	}

	return &Resolver{
		projectPath:        cfg.ProjectPath,
		docsPath:           cfg.DocsPath,
		extensions:         cfg.Extensions,
		maxFileBytes:       cfg.MaxFileBytes,
		srcRoot:            cfg.SrcRoot,
		language:           cfg.Language,
		layers:             cfg.Layers,
		rules:              cfg.Rules,
		domainContext:      cfg.DomainContext,
		consent:            cm,
		auditLog:           al,
		externals:          externals,
		violations:         prevViolations,
		nextID:             len(prevViolations),
		readFiles:          make(map[string]readFileEntry),
		keepWindowTurns:    cfg.KeepWindowTurns,
		previouslyAnalyzed: prevAnalyzed,
		observer:           observer,
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
