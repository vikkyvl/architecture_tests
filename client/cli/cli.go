package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/archguard/project/api/contextresolver"
	"github.com/archguard/project/api/llm"
	"github.com/archguard/project/api/result"
	"github.com/archguard/project/client/audit"
	"github.com/archguard/project/client/consent"
	"github.com/archguard/project/client/detector"
	"github.com/archguard/project/client/mcp"
	"github.com/archguard/project/client/review"
	"github.com/archguard/project/shared/config"
	"github.com/archguard/project/shared/models"
)

const (
	errConfig               = "config error: %w"
	errLLMRequest           = "LLM request failed: %w"
	envDebugExternals       = "ARCHGUARD_DEBUG_EXTERNALS"
	headerLanguageDetection = "Language detection"
	headerAnalyzer          = "ArchGuard Analyzer"
	headerReports           = "Reports"
	headerResults           = "Results"
	rateLimitFrames         = "|/-\\"
)

type ResultProcessor interface {
	Process(res *models.AnalysisResult)
	RenderJSON(res *models.AnalysisResult, path string) error
	RenderMarkdown(res *models.AnalysisResult, path string) error
}

type ContextBuilder interface {
	BuildSystemPrompt() string
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
		fmt.Fprintln(os.Stderr, renderHeader(headerLanguageDetection))
		fmt.Fprintln(os.Stderr, renderDetection(det))
	}

	exts := detector.Extensions(cfg.Project.Language)

	externals, allowedSystems, closers := buildExternals(cfg.External)
	defer func() {
		for _, cl := range closers {
			cl.Close()
		}
	}()

	consentMgr := consent.NewManager(opts.Interactive, opts.ProjectPath, allowedSystems)
	auditLog := audit.NewLog()

	var mcpResolver review.ToolRunner = mcp.NewResolver(cfg, opts.ProjectPath, opts.DocsPath, exts, consentMgr, auditLog, externals)

	provider, err := llm.NewProviderWithRateLimitHook(opts.Provider, opts.APIKey, opts.Model, func(wait time.Duration) {
		animateRateLimitWait(os.Stderr, wait)
	})
	if err != nil {
		return err
	}

	var ctxResolver ContextBuilder = contextresolver.NewResolver(cfg)
	var reviewer ResultProcessor = result.NewReviewer()

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, renderHeader(headerAnalyzer))
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

	engine := review.NewEngine(provider, mcpResolver, auditLog, cliReviewObserver{
		maxToolCalls: opts.MaxToolCalls,
		timeout:      opts.Timeout,
	})
	run, err := engine.Run(review.Options{
		SystemPrompt:   systemPrompt,
		InitialContext: promptExternalContext(cfg.External, cfg, mcpResolver, opts.Interactive),
		MaxToolCalls:   opts.MaxToolCalls,
		Timeout:        opts.Timeout,
	})
	if err != nil {
		return fmt.Errorf(errLLMRequest, err)
	}

	duration := time.Since(startTime)
	ar := &models.AnalysisResult{
		ProjectName: cfg.Project.Name, Language: cfg.Project.Language,
		LLMProvider: provider.Name(), LLMModel: provider.Model(),
		AnalyzedAt: startTime, Duration: duration.Round(time.Second).String(),
		FilesScanned: countFiles(mcpResolver), ToolCalls: run.ToolCalls,
		Violations: run.Violations, AuditLog: run.AuditEntries,
		Incomplete:      run.Incomplete,
		AnalyzedModules: run.AnalyzedModules,
		SkippedModules:  run.SkippedModules,
	}

	reviewer.Process(ar)

	if err := reviewer.RenderJSON(ar, opts.OutputJSON); err != nil {
		return err
	}

	if err := reviewer.RenderMarkdown(ar, opts.OutputMD); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, renderHeader(headerReports))
	fmt.Fprintln(os.Stderr, renderReportPaths(opts.OutputJSON, opts.OutputMD))

	fmt.Println(renderHeader(headerResults))
	fmt.Println(renderResults(ar))
	return nil
}

func animateRateLimitWait(out io.Writer, wait time.Duration) {
	if wait <= 0 {
		fmt.Fprintln(out, renderRateLimitResume(wait))
		return
	}

	deadline := time.Now().Add(wait)
	timer := time.NewTimer(wait)
	defer timer.Stop()
	ticker := time.NewTicker(rateLimitTick)
	defer ticker.Stop()

	for frame := 0; ; frame++ {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		fmt.Fprint(out, terminalClearLine)
		fmt.Fprint(out, renderRateLimitWaitFrame(remaining, string(rateLimitFrames[frame%len(rateLimitFrames)])))
		select {
		case <-timer.C:
			fmt.Fprint(out, terminalClearLine)
			fmt.Fprintln(out, renderRateLimitResume(wait))
			return
		case <-ticker.C:
		}
	}

	fmt.Fprint(out, terminalClearLine)
	fmt.Fprintln(out, renderRateLimitResume(wait))
}
