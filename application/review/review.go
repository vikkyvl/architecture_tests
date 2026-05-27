package review

import (
	"time"

	"github.com/archguard/project/infrastructure/llm"
	"github.com/archguard/project/shared/models"
)

type ToolRunner interface {
	ListTools() []models.ToolDefinition
	ExecuteTool(name string, args map[string]interface{}, budget int) (string, error)
	GetViolations() []models.Violation
	ListSourceFiles() []string
}

type AuditReader interface {
	Entries() []models.AuditEntry
}

type LLMTurnRecorder interface {
	RecordLLMTurn(input, cached, cacheWrite, pruned int)
}

type Observer interface {
	LimitReached(message string)
	Retry(provider string, attempt, maxAttempts int, wait time.Duration, err error)
	LLMText(text string)
	ToolResult(callNumber int, toolName string, resultSize int, err error)
	AnalysisComplete()
}

type Options struct {
	SystemPrompt    string
	InitialContext  string
	MaxToolCalls    int
	Timeout         time.Duration
	MaxRetries      int
	RetryWaits      []time.Duration
	TextPreviewSize int
	PruneAfterTurns int
	KeepRecentTurns int
}

type RunResult struct {
	ToolCalls       int
	Incomplete      bool
	Violations      []models.Violation
	AuditEntries    []models.AuditEntry
	AnalyzedModules []string
	SkippedModules  []string
}

type Engine struct {
	provider llm.Provider
	tools    ToolRunner
	audit    AuditReader
	recorder LLMTurnRecorder
	observer Observer
}

type noopObserver struct{}

func (noopObserver) LimitReached(string)                          {}
func (noopObserver) Retry(string, int, int, time.Duration, error) {}
func (noopObserver) LLMText(string)                               {}
func (noopObserver) ToolResult(int, string, int, error)           {}
func (noopObserver) AnalysisComplete()                            {}

func NewEngine(provider llm.Provider, tools ToolRunner, audit AuditReader, observer Observer) *Engine {
	if observer == nil {
		observer = new(noopObserver)
	}
	recorder, _ := audit.(LLMTurnRecorder)
	return &Engine{
		provider: provider,
		tools:    tools,
		audit:    audit,
		recorder: recorder,
		observer: observer,
	}
}
