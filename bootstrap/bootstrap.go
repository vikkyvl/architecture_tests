package bootstrap

import (
	"fmt"
	"time"

	contextresolver "github.com/archguard/project/application/prompter"
	result "github.com/archguard/project/application/reporter"
	"github.com/archguard/project/application/review"
	"github.com/archguard/project/config"
	"github.com/archguard/project/delivery/output"
	"github.com/archguard/project/infrastructure/audit"
	"github.com/archguard/project/infrastructure/consent"
	"github.com/archguard/project/infrastructure/detector"
	"github.com/archguard/project/infrastructure/llm"
	"github.com/archguard/project/infrastructure/mcp"
	"github.com/archguard/project/shared/models"
)

const (
	errConfig               = "config error: %w"
	errLLMRequest           = "LLM request failed: %w"
	headerLanguageDetection = "Language detection"
	headerAnalyzer          = "ArchGuard Analyzer"
	headerReports           = "Reports"
	headerResults           = "Results"
)

type ContextBuilder interface {
	BuildSystemPrompt() string
}

type ResultProcessor interface {
	Process(res *models.AnalysisResult)
	RenderJSON(res *models.AnalysisResult, path string) error
	RenderMarkdown(res *models.AnalysisResult, path string) error
}

type Options struct {
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

type App struct {
	out         *output.OutputTransport
	opts        Options
	cfg         *config.Config
	provider    llm.Provider
	mcpResolver review.ToolRunner
	ctxResolver ContextBuilder
	reviewer    ResultProcessor
	auditLog    *audit.Log
	closers     []func()
	engine      *review.Engine
	startTime   time.Time
}

func NewApp(opts Options, out *output.OutputTransport, obs review.Observer) (*App, error) {
	startTime := time.Now()

	cfg, err := config.LoadConfig(opts.RulesPath)
	if err != nil {
		return nil, fmt.Errorf(errConfig, err)
	}

	if cfg.Project.Language == "" {
		det := detector.Detect(opts.ProjectPath)
		cfg.Project.Language = det.PrimaryLanguage
		out.WriteProgress(output.RenderHeader(headerLanguageDetection))
		out.WriteProgress(output.RenderDetection(det))
	}
	exts := detector.Extensions(cfg.Project.Language)

	externals, allowedSystems, rawClosers := buildExternals(cfg.External)
	closers := make([]func(), 0, len(rawClosers))
	for _, cl := range rawClosers {
		closers = append(closers, cl.Close)
	}

	consentMgr := consent.NewManager(opts.Interactive, opts.ProjectPath, allowedSystems)
	auditLog := audit.NewLog()

	var mcpResolver review.ToolRunner = mcp.NewResolver(
		mcp.ResolverConfig{
			ProjectPath:     opts.ProjectPath,
			DocsPath:        opts.DocsPath,
			Extensions:      exts,
			MaxFileBytes:    cfg.Runtime.MaxFileBytes,
			SrcRoot:         cfg.Project.SrcRoot,
			Language:        cfg.Project.Language,
			Layers:          cfg.Layers,
			Rules:           cfg.Rules,
			DomainContext:   cfg.DomainContext,
			KeepWindowTurns: cfg.Runtime.KeepRecentTurns,
		},
		consentMgr, auditLog, externals,
	)

	llmOpts := llm.RuntimeOptions{
		MaxRetries:              cfg.Runtime.ProviderMaxRetries,
		RetryWait:               cfg.Runtime.ProviderRetryWait,
		AnthropicCacheTail:      derefBool(cfg.Runtime.AnthropicCacheTail),
		RespectRateLimitHeaders: derefBool(cfg.Runtime.RespectRateLimitHeaders),
		RateLimitObserver:       auditLog.RecordRateLimit,
		PreemptiveSleepObserver: func(wait time.Duration) {
			auditLog.RecordPreemptiveSleep(wait)
			output.AnimateProactivePauseWait(out.Progress, wait)
		},
	}
	provider, err := llm.NewProviderWithOptions(opts.Provider, opts.APIKey, opts.Model, llmOpts, func(wait time.Duration) {
		output.AnimateRateLimitWait(out.Progress, wait)
	})
	if err != nil {
		for _, cl := range closers {
			cl()
		}
		return nil, err
	}

	var ctxResolver ContextBuilder = contextresolver.NewResolver(
		cfg.Project, cfg.Layers, cfg.Rules, cfg.DomainContext, cfg.External,
	)
	var reviewer ResultProcessor = result.NewReviewer()
	out.WriteProgressBlank()
	out.WriteProgress(output.RenderHeader(headerAnalyzer))
	out.WriteProgress(output.RenderRunOverview(
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
	out.WriteProgressBlank()

	engine := review.NewEngine(provider, mcpResolver, auditLog, obs)

	return &App{
		out:         out,
		opts:        opts,
		cfg:         cfg,
		provider:    provider,
		mcpResolver: mcpResolver,
		ctxResolver: ctxResolver,
		reviewer:    reviewer,
		auditLog:    auditLog,
		closers:     closers,
		engine:      engine,
		startTime:   startTime,
	}, nil
}

func (a *App) Close() {
	for _, cl := range a.closers {
		cl()
	}
}

func (a *App) Run() error {
	systemPrompt := a.ctxResolver.BuildSystemPrompt()

	run, err := a.engine.Run(review.Options{
		SystemPrompt:    systemPrompt,
		InitialContext:  promptExternalContext(a.cfg.External, a.cfg, a.mcpResolver, a.opts.Interactive, a.out),
		MaxToolCalls:    a.opts.MaxToolCalls,
		Timeout:         a.opts.Timeout,
		PruneAfterTurns: a.cfg.Runtime.PruneAfterTurns,
		KeepRecentTurns: a.cfg.Runtime.KeepRecentTurns,
	})
	if err != nil {
		return fmt.Errorf(errLLMRequest, err)
	}

	duration := time.Since(a.startTime)
	filesScanned, err := countFiles(a.mcpResolver)
	if err != nil {
		a.out.WriteProgress(output.RenderNotice(output.WarnBadgeStyle.Render(output.BadgeSkip), fmt.Sprintf("could not count files: %v", err)))
	}
	ar := &models.AnalysisResult{
		ProjectName:     a.cfg.Project.Name,
		Language:        a.cfg.Project.Language,
		LLMProvider:     a.provider.Name(),
		LLMModel:        a.provider.Model(),
		AnalyzedAt:      a.startTime,
		Duration:        duration.Round(time.Second).String(),
		FilesScanned:    filesScanned,
		ToolCalls:       run.ToolCalls,
		Violations:      run.Violations,
		AuditLog:        run.AuditEntries,
		Incomplete:      run.Incomplete,
		AnalyzedModules: run.AnalyzedModules,
		SkippedModules:  run.SkippedModules,
	}

	a.reviewer.Process(ar)

	if err := a.reviewer.RenderJSON(ar, a.opts.OutputJSON); err != nil {
		return err
	}
	if err := a.reviewer.RenderMarkdown(ar, a.opts.OutputMD); err != nil {
		return err
	}

	a.out.WriteProgress(output.RenderHeader(headerReports))
	a.out.WriteProgress(output.RenderReportPaths(a.opts.OutputJSON, a.opts.OutputMD))

	a.out.WriteResult(output.RenderHeader(headerResults))
	a.out.WriteResult(output.RenderResults(ar))
	return nil
}

func derefBool(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}
