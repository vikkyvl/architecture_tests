package review

import (
	"strings"
	"time"

	"github.com/archguard/project/api/llm"
	"github.com/archguard/project/client/mcp"
	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/models"
)

const (
	defaultMaxRetries        = 3
	initialUserPrompt        = "Analyze this project for architectural violations. Check every source file."
	contextSeparator         = "\n\n"
	retryableOverloaded      = "overloaded"
	retryableTempUnavailable = "temporarily unavailable"
)

var defaultRetryWaits = []time.Duration{
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
}

type ToolRunner interface {
	ListTools() []mcp.ToolDefinition
	ExecuteTool(name string, args map[string]interface{}, budget int) (string, error)
	GetViolations() []models.Violation
	ListSourceFiles() []string
}

type AuditReader interface {
	Entries() []models.AuditEntry
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
}

type Engine struct {
	provider llm.Provider
	tools    ToolRunner
	audit    AuditReader
	observer Observer
}

type RunResult struct {
	ToolCalls       int
	Incomplete      bool
	Violations      []models.Violation
	AuditEntries    []models.AuditEntry
	AnalyzedModules []string
	SkippedModules  []string
}

type noopObserver struct{}

func (noopObserver) LimitReached(string) {
	return
}

func (noopObserver) Retry(string, int, int, time.Duration, error) {
	return
}

func (noopObserver) LLMText(string) {
	return
}

func (noopObserver) ToolResult(int, string, int, error) {
	return
}

func (noopObserver) AnalysisComplete() {
	return
}

func NewEngine(provider llm.Provider, tools ToolRunner, audit AuditReader, observer Observer) *Engine {
	if observer == nil {
		observer = new(noopObserver)
	}
	return &Engine{
		provider: provider,
		tools:    tools,
		audit:    audit,
		observer: observer,
	}
}

func (e *Engine) Run(opts Options) (*RunResult, error) {
	if opts.MaxRetries == 0 {
		opts.MaxRetries = defaultMaxRetries
	}
	if len(opts.RetryWaits) == 0 {
		opts.RetryWaits = defaultRetryWaits
	}
	if opts.TextPreviewSize == 0 {
		opts.TextPreviewSize = c.TruncateLength
	}

	initialText := initialUserPrompt
	if strings.TrimSpace(opts.InitialContext) != "" {
		initialText += contextSeparator + opts.InitialContext
	}
	messages := []llm.Message{
		{
			Role: c.RoleUser,
			Content: []llm.ContentBlock{
				llm.NewTextBlock(initialText),
			},
		},
	}
	apiTools := toAPITools(nonExternalTools(e.tools.ListTools()))

	toolCalls := 0
	deadline := time.Now().Add(opts.Timeout)
	incomplete := false

	for {
		if toolCalls >= opts.MaxToolCalls {
			e.observer.LimitReached(c.LimitReasonMaxToolCalls)
			incomplete = true
			break
		}
		if time.Now().After(deadline) {
			e.observer.LimitReached(c.LimitReasonTimeout)
			incomplete = true
			break
		}

		resp, err := e.sendMessageWithRetry(llm.Request{
			System: opts.SystemPrompt, MaxTokens: c.DefaultMaxTokens,
			Messages: messages, Tools: apiTools,
		}, opts.MaxRetries, opts.RetryWaits)
		if err != nil {
			return nil, err
		}

		messages = append(messages, llm.Message{
			Role:    c.RoleAssistant,
			Content: resp.Content,
		})
		for _, b := range resp.Content {
			if b.Type == c.ContentTypeText && b.Text != "" {
				e.observer.LLMText(truncate(b.Text, opts.TextPreviewSize))
			}
		}

		if resp.StopReason == c.StopReasonEndTurn {
			e.observer.AnalysisComplete()
			break
		}
		if resp.StopReason != c.StopReasonToolUse {
			continue
		}

		results := make([]llm.ContentBlock, 0)
		for _, b := range resp.Content {
			if b.Type != c.ContentTypeToolUse {
				continue
			}
			toolCalls++
			remaining := opts.MaxToolCalls - toolCalls
			args := decodeToolArgs(b)
			res, execErr := e.tools.ExecuteTool(b.Name, args, remaining)
			if execErr != nil {
				e.observer.ToolResult(toolCalls, b.Name, 0, execErr)
				content := execErr.Error()
				if res != "" {
					content = res
				}
				results = append(results, llm.NewToolResult(b.ID, content, true))
				continue
			}
			e.observer.ToolResult(toolCalls, b.Name, len(res), nil)
			results = append(results, llm.NewToolResult(b.ID, res, false))
		}
		messages = append(messages, llm.Message{
			Role:    c.RoleUser,
			Content: results,
		})
	}

	auditEntries := e.audit.Entries()
	analyzedModules := extractAnalyzedModules(auditEntries)
	return &RunResult{
		ToolCalls:       toolCalls,
		Incomplete:      incomplete,
		Violations:      e.tools.GetViolations(),
		AuditEntries:    auditEntries,
		AnalyzedModules: analyzedModules,
		SkippedModules:  skippedModules(incomplete, analyzedModules, e.tools.ListSourceFiles()),
	}, nil
}

func toAPITools(defs []mcp.ToolDefinition) []llm.Tool {
	tools := make([]llm.Tool, 0, len(defs))
	for _, d := range defs {
		tools = append(tools, llm.Tool{
			Name:        d.Name,
			Description: d.Description,
			InputSchema: d.InputSchema,
		})
	}
	return tools
}

func nonExternalTools(defs []mcp.ToolDefinition) []mcp.ToolDefinition {
	tools := make([]mcp.ToolDefinition, 0, len(defs))
	for _, d := range defs {
		if d.Name == c.ToolGetExternalContext {
			continue
		}
		tools = append(tools, d)
	}
	return tools
}
