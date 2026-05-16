package review

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/archguard/project/api/llm"
	"github.com/archguard/project/client/mcp"
	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/models"
)

const (
	defaultMaxRetries = 3
	initialUserPrompt = "Analyze this project for architectural violations. Check every source file."
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

func (noopObserver) LimitReached(string)                          {}
func (noopObserver) Retry(string, int, int, time.Duration, error) {}
func (noopObserver) LLMText(string)                               {}
func (noopObserver) ToolResult(int, string, int, error)           {}
func (noopObserver) AnalysisComplete()                            {}

func NewEngine(provider llm.Provider, tools ToolRunner, audit AuditReader, observer Observer) *Engine {
	if observer == nil {
		observer = noopObserver{}
	}
	return &Engine{provider: provider, tools: tools, audit: audit, observer: observer}
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
		initialText += "\n\n" + opts.InitialContext
	}
	messages := []llm.Message{
		{Role: c.RoleUser, Content: []llm.ContentBlock{llm.NewTextBlock(initialText)}},
	}
	apiTools := toAPITools(nonExternalTools(e.tools.ListTools()))

	toolCalls := 0
	deadline := time.Now().Add(opts.Timeout)
	incomplete := false

	for {
		if toolCalls >= opts.MaxToolCalls {
			e.observer.LimitReached("max tool calls")
			incomplete = true
			break
		}
		if time.Now().After(deadline) {
			e.observer.LimitReached("timeout")
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

		messages = append(messages, llm.Message{Role: c.RoleAssistant, Content: resp.Content})
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
		messages = append(messages, llm.Message{Role: c.RoleUser, Content: results})
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

func (e *Engine) sendMessageWithRetry(req llm.Request, maxRetries int, waits []time.Duration) (*llm.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := e.provider.SendMessage(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if attempt >= maxRetries || !isRetryableLLMError(err) {
			break
		}
		wait := retryWait(attempt, waits)
		e.observer.Retry(e.provider.Name(), attempt+1, maxRetries, wait, err)
		time.Sleep(wait)
	}
	return nil, lastErr
}

func decodeToolArgs(block llm.ContentBlock) map[string]interface{} {
	args := make(map[string]interface{})
	if len(block.Input) == 0 {
		return args
	}
	if err := json.Unmarshal(block.Input, &args); err != nil {
		return map[string]interface{}{}
	}
	return args
}

func isRetryableLLMError(err error) bool {
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		return false
	}
	if appErr.Kind == apperrors.KindRateLimited || appErr.Kind == apperrors.KindExternalService {
		return true
	}
	msg := strings.ToLower(appErr.Error())
	return strings.Contains(msg, "overloaded") || strings.Contains(msg, "temporarily unavailable")
}

func retryWait(attempt int, waits []time.Duration) time.Duration {
	if attempt < len(waits) {
		return waits[attempt]
	}
	return waits[len(waits)-1]
}

func toAPITools(defs []mcp.ToolDefinition) []llm.Tool {
	tools := make([]llm.Tool, 0, len(defs))
	for _, d := range defs {
		tools = append(tools, llm.Tool{Name: d.Name, Description: d.Description, InputSchema: d.InputSchema})
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

func extractAnalyzedModules(entries []models.AuditEntry) []string {
	seen := make(map[string]bool)
	var result []string
	for _, e := range entries {
		if e.ToolName != c.ToolReadFile || e.Error != "" {
			continue
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(e.Arguments), &args); err != nil {
			continue
		}
		path, ok := args[c.ArgPath].(string)
		if ok && path != "" && !seen[path] {
			seen[path] = true
			result = append(result, path)
		}
	}
	return result
}

func skippedModules(incomplete bool, analyzed []string, sourceFiles []string) []string {
	if !incomplete {
		return nil
	}
	analyzedSet := make(map[string]bool, len(analyzed))
	for _, f := range analyzed {
		analyzedSet[f] = true
	}
	var skipped []string
	for _, f := range sourceFiles {
		if !analyzedSet[f] {
			skipped = append(skipped, f)
		}
	}
	return skipped
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + c.TruncationSuffix
}
