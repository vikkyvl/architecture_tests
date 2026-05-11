package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/archguard/project/api/contextresolver"
	"github.com/archguard/project/api/llm"
	"github.com/archguard/project/api/result"
	"github.com/archguard/project/client/audit"
	"github.com/archguard/project/client/consent"
	"github.com/archguard/project/client/detector"
	"github.com/archguard/project/client/mcp"
	"github.com/archguard/project/shared/apperrors"
	"github.com/archguard/project/shared/config"
	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/models"
)

const (
	errConfig         = "config error: %w"
	errLLMRequest     = "LLM request failed: %w"
	initialUserPrompt = "Analyze this project for architectural violations. Check every source file."
	llmMaxRetries     = 3
)

var llmRetryWaits = []time.Duration{
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
}

type ReviewOptions struct {
	ProjectPath  string
	RulesPath    string
	DocsPath     string
	OutputJSON   string
	OutputMD     string
	Provider     string
	APIKey       string
	Model        string
	MaxToolCalls int
	Timeout      time.Duration
	Interactive  bool
}

func RunReview(opts ReviewOptions) error {
	startTime := time.Now()

	cfg, err := config.LoadConfig(opts.RulesPath)
	if err != nil {
		return fmt.Errorf(errConfig, err)
	}

	if cfg.Project.Language == "" {
		det := detector.Detect(opts.ProjectPath)
		cfg.Project.Language = det.PrimaryLanguage
		fmt.Fprintln(os.Stderr, renderHeader("Language detection"))
		fmt.Fprintln(os.Stderr, renderDetection(det))
	}

	exts := detector.Extensions(cfg.Project.Language)

	consentMgr := consent.NewManager(opts.Interactive, opts.ProjectPath)
	auditLog := audit.NewLog()
	mcpResolver := mcp.NewResolver(cfg, opts.ProjectPath, opts.DocsPath, exts, consentMgr, auditLog)

	provider, err := llm.NewProvider(opts.Provider, opts.APIKey, opts.Model)
	if err != nil {
		return err
	}

	ctxResolver := contextresolver.NewResolver(cfg)
	reviewer := result.NewReviewer()

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, renderHeader("ArchGuard Analyzer"))
	fmt.Fprintln(os.Stderr, renderRunOverview(
		cfg.Project.Name,
		cfg.Project.Language,
		len(cfg.Layers),
		len(cfg.Rules),
		len(cfg.DomainContext.Rules),
		provider.Name(),
		provider.Model(),
		opts.MaxToolCalls,
		opts.Timeout.String(),
	))
	fmt.Fprintln(os.Stderr)

	systemPrompt := ctxResolver.BuildSystemPrompt()
	apiTools := toAPITools(mcpResolver.ListTools())

	messages := []llm.Message{
		{Role: c.RoleUser, Content: []llm.ContentBlock{
			llm.NewTextBlock(initialUserPrompt),
		}},
	}

	toolCalls := 0
	deadline := time.Now().Add(opts.Timeout)
	incomplete := false

	for {
		if toolCalls >= opts.MaxToolCalls {
			fmt.Fprintln(os.Stderr, warnStyle.Render(fmt.Sprintf("Limit reached: max %d tool calls", opts.MaxToolCalls)))
			incomplete = true
			break
		}
		if time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, warnStyle.Render(fmt.Sprintf("Limit reached: timeout %s", opts.Timeout)))
			incomplete = true
			break
		}

		resp, err := sendMessageWithRetry(provider, llm.Request{
			System: systemPrompt, MaxTokens: c.DefaultMaxTokens,
			Messages: messages, Tools: apiTools,
		})
		if err != nil {
			return fmt.Errorf(errLLMRequest, err)
		}

		messages = append(messages, llm.Message{Role: c.RoleAssistant, Content: resp.Content})

		for _, b := range resp.Content {
			if b.Type == c.ContentTypeText && b.Text != "" {
				fmt.Fprintln(os.Stderr, renderLLMText(truncate(b.Text, c.TruncateLength)))
			}
		}

		if resp.StopReason == c.StopReasonEndTurn {
			fmt.Fprintln(os.Stderr, renderStep("Analysis complete"))
			break
		}

		if resp.StopReason == c.StopReasonToolUse {
			var results []llm.ContentBlock
			for _, b := range resp.Content {
				if b.Type != c.ContentTypeToolUse {
					continue
				}
				toolCalls++
				remaining := opts.MaxToolCalls - toolCalls

				var args map[string]interface{}
				if len(b.Input) > 0 {
					json.Unmarshal(b.Input, &args)
				}
				if args == nil {
					args = make(map[string]interface{})
				}

				res, execErr := mcpResolver.ExecuteTool(b.Name, args, remaining)
				if execErr != nil {
					fmt.Fprintln(os.Stderr, renderToolResult(toolCalls, b.Name, 0, execErr))
					content := execErr.Error()
					if res != "" {
						content = res
					}
					results = append(results, llm.NewToolResult(b.ID, content, true))
				} else {
					fmt.Fprintln(os.Stderr, renderToolResult(toolCalls, b.Name, len(res), nil))
					results = append(results, llm.NewToolResult(b.ID, res, false))
				}
			}
			messages = append(messages, llm.Message{Role: c.RoleUser, Content: results})
		}
	}

	violations := mcpResolver.GetViolations()
	duration := time.Since(startTime)
	auditEntries := auditLog.Entries()

	analyzedModules := extractAnalyzedModules(auditEntries)
	var skippedModules []string
	if incomplete {
		analyzedSet := make(map[string]bool)
		for _, f := range analyzedModules {
			analyzedSet[f] = true
		}
		for _, f := range mcpResolver.ListSourceFiles() {
			if !analyzedSet[f] {
				skippedModules = append(skippedModules, f)
			}
		}
	}

	ar := &models.AnalysisResult{
		ProjectName: cfg.Project.Name, Language: cfg.Project.Language,
		LLMProvider: provider.Name(), LLMModel: provider.Model(),
		AnalyzedAt: startTime, Duration: duration.Round(time.Second).String(),
		FilesScanned: countFiles(mcpResolver), ToolCalls: toolCalls,
		Violations: violations, AuditLog: auditEntries,
		Incomplete:      incomplete,
		AnalyzedModules: analyzedModules,
		SkippedModules:  skippedModules,
	}

	reviewer.Process(ar)

	if err := reviewer.RenderJSON(ar, opts.OutputJSON); err != nil {
		return err
	}

	if err := reviewer.RenderMarkdown(ar, opts.OutputMD); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, renderHeader("Reports"))
	fmt.Fprintln(os.Stderr, renderReportPaths(opts.OutputJSON, opts.OutputMD))

	fmt.Println(renderHeader("Results"))
	fmt.Println(renderResults(ar))
	return nil
}

func sendMessageWithRetry(provider llm.Provider, req llm.Request) (*llm.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= llmMaxRetries; attempt++ {
		resp, err := provider.SendMessage(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if attempt >= llmMaxRetries || !isRetryableLLMError(err) {
			break
		}
		wait := retryWait(attempt)
		fmt.Fprintln(os.Stderr, renderRetry(provider.Name(), attempt+1, llmMaxRetries, wait, err))
		time.Sleep(wait)
	}
	return nil, lastErr
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

func retryWait(attempt int) time.Duration {
	if attempt < len(llmRetryWaits) {
		return llmRetryWaits[attempt]
	}
	return llmRetryWaits[len(llmRetryWaits)-1]
}

func toAPITools(defs []mcp.ToolDefinition) []llm.Tool {
	var tools []llm.Tool
	for _, d := range defs {
		tools = append(tools, llm.Tool{Name: d.Name, Description: d.Description, InputSchema: d.InputSchema})
	}
	return tools
}

func countFiles(r *mcp.Resolver) int {
	res, _ := r.ExecuteTool(c.ToolGetProjectStructure, nil, c.FileCountBudget)
	var p struct{ Total int }
	json.Unmarshal([]byte(res), &p)
	return p.Total
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

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + c.TruncationSuffix
}
