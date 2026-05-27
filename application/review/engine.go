package review

import (
	"sort"
	"strings"
	"time"

	"github.com/archguard/project/infrastructure/llm"
	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/models"
)

const (
	defaultMaxRetries        = 3
	initialUserPrompt        = "Analyze this project for architectural violations in small, targeted batches. First map modules and dependency boundaries, then request only the source files needed to validate likely rule violations. Avoid rereading files and keep tool usage focused."
	contextSeparator         = "\n\n"
	retryableOverloaded      = "overloaded"
	retryableTempUnavailable = "temporarily unavailable"
	pruneAfterTurns          = 20
	keepRecentTurns          = 8
	prunedPlaceholder        = "<elided to conserve tokens; full content in audit log>"
	charsPerTokenEstimate    = 4
	smallRequestSpacing      = 2 * time.Second
	mediumRequestSpacing     = 6 * time.Second
	largeRequestSpacing      = 15 * time.Second
	veryLargeRequestSpacing  = 30 * time.Second
	mediumRequestTokens      = 8000
	largeRequestTokens       = 16000
	veryLargeRequestTokens   = 24000
	maxElisionsPerCall       = 2
	defaultSoftLimitTokens   = 20000
)

var defaultRetryWaits = []time.Duration{
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
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
	if opts.PruneAfterTurns == 0 {
		opts.PruneAfterTurns = pruneAfterTurns
	}
	if opts.KeepRecentTurns == 0 {
		opts.KeepRecentTurns = keepRecentTurns
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
	systemBlocks := []llm.SystemBlock{llm.NewSystemBlock(opts.SystemPrompt, false)}

	toolCalls := 0
	turn := 0
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

		projected := estimateRequestTokens(llm.Request{
			System: systemBlocks, Messages: messages, Tools: apiTools,
		})
		softLimit := providerSoftLimit(e.provider.Name())
		pruned := prunePressureAware(messages, projected, softLimit, opts.KeepRecentTurns)

		req := llm.Request{
			System: systemBlocks, MaxTokens: providerMaxOutputTokens(e.provider.Name()),
			Messages: messages, Tools: apiTools,
		}
		if turn > 0 {
			wait := requestPaceWait(estimateRequestTokens(req), e.provider.Name())
			if time.Now().Add(wait).After(deadline) {
				e.observer.LimitReached(c.LimitReasonTimeout)
				incomplete = true
				break
			}
			time.Sleep(wait)
		}
		resp, err := e.sendMessageWithRetry(req, opts.MaxRetries, opts.RetryWaits)
		turn++
		if err != nil {
			auditEntries := e.audit.Entries()
			analyzedModules := extractAnalyzedModules(auditEntries)
			return &RunResult{
				ToolCalls:       toolCalls,
				Incomplete:      true,
				Violations:      e.tools.GetViolations(),
				AuditEntries:    auditEntries,
				AnalyzedModules: analyzedModules,
				SkippedModules:  skippedModules(true, analyzedModules, e.tools.ListSourceFiles()),
			}, err
		}
		if e.recorder != nil {
			e.recorder.RecordLLMTurn(resp.Usage.InputTokens, resp.Usage.CacheReadTokens, resp.Usage.CacheCreationTokens, pruned)
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

func toAPITools(defs []models.ToolDefinition) []llm.Tool {
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

type pruneCandidate struct {
	msg   int
	block int
	size  int
}

func prunePressureAware(messages []llm.Message, projectedTokens, softLimit, keepRecent int) int {
	if softLimit <= 0 || projectedTokens < softLimit {
		return 0
	}
	keepFromIdx := len(messages) - keepRecent*2
	if keepFromIdx <= 0 {
		return 0
	}
	candidates := make([]pruneCandidate, 0)
	for i := 0; i < keepFromIdx; i++ {
		for j := range messages[i].Content {
			b := &messages[i].Content[j]
			if b.Type != c.ContentTypeToolResult {
				continue
			}
			if b.Content == prunedPlaceholder {
				continue
			}
			if b.CacheControl != nil {
				continue
			}
			candidates = append(candidates, pruneCandidate{msg: i, block: j, size: len(b.Content)})
		}
	}
	if len(candidates) == 0 {
		return 0
	}
	sort.Slice(candidates, func(a, b int) bool {
		if candidates[a].size != candidates[b].size {
			return candidates[a].size > candidates[b].size
		}
		if candidates[a].msg != candidates[b].msg {
			return candidates[a].msg < candidates[b].msg
		}
		return candidates[a].block < candidates[b].block
	})
	limit := maxElisionsPerCall
	if limit > len(candidates) {
		limit = len(candidates)
	}
	for k := 0; k < limit; k++ {
		c := candidates[k]
		messages[c.msg].Content[c.block].Content = prunedPlaceholder
	}
	return limit
}

func providerSoftLimit(name string) int {
	switch name {
	case c.ProviderAnthropic:
		return 2 * c.AnthropicInputTPM / 3
	case c.ProviderGemini:
		return 2 * c.GeminiInputTPM / 3
	case c.ProviderOpenAI:
		return 2 * c.OpenAIInputTPM / 3
	default:
		return defaultSoftLimitTokens
	}
}

func providerMinInterval(name string) time.Duration {
	switch name {
	case c.ProviderAnthropic:
		return c.AnthropicMinInterval
	case c.ProviderGemini:
		return c.GeminiMinInterval
	case c.ProviderOpenAI:
		return c.OpenAIMinInterval
	default:
		return smallRequestSpacing
	}
}

func providerMaxOutputTokens(name string) int {
	switch name {
	case c.ProviderAnthropic:
		return c.AnthropicMaxOutputTokens
	case c.ProviderGemini:
		return c.GeminiMaxOutputTokens
	case c.ProviderOpenAI:
		return c.OpenAIMaxOutputTokens
	default:
		return c.AnalyzerMaxTokens
	}
}

func nonExternalTools(defs []models.ToolDefinition) []models.ToolDefinition {
	tools := make([]models.ToolDefinition, 0, len(defs))
	for _, d := range defs {
		if d.Name == c.ToolGetExternalContext {
			continue
		}
		tools = append(tools, d)
	}
	return tools
}

func requestPaceWait(projectedTokens int, providerName string) time.Duration {
	var d time.Duration
	switch {
	case projectedTokens >= veryLargeRequestTokens:
		d = veryLargeRequestSpacing
	case projectedTokens >= largeRequestTokens:
		d = largeRequestSpacing
	case projectedTokens >= mediumRequestTokens:
		d = mediumRequestSpacing
	default:
		d = smallRequestSpacing
	}
	if min := providerMinInterval(providerName); d < min {
		return min
	}
	return d
}

func estimateRequestTokens(req llm.Request) int {
	total := len(llm.SystemText(req.System))
	for _, msg := range req.Messages {
		total += len(msg.Role)
		for _, block := range msg.Content {
			total += len(block.Type)
			total += len(block.Text)
			total += len(block.ID)
			total += len(block.Name)
			total += len(block.Input)
			total += len(block.ToolUseID)
			total += len(block.Content)
		}
	}
	for _, tool := range req.Tools {
		total += len(tool.Name)
		total += len(tool.Description)
		total += len(tool.InputSchema)
	}
	return total / charsPerTokenEstimate
}
