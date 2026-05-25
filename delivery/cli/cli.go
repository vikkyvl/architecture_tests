package cli

import (
	"time"

	"github.com/archguard/project/bootstrap"
	"github.com/archguard/project/delivery/output"
)

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
	out := output.NewOutputTransport()
	obs := cliReviewObserver{
		out:          out,
		maxToolCalls: opts.MaxToolCalls,
		timeout:      opts.Timeout,
	}
	app, err := bootstrap.NewApp(bootstrap.Options{
		ProjectPath:  opts.ProjectPath,
		RulesPath:    opts.RulesPath,
		DocsPath:     opts.DocsPath,
		OutputJSON:   opts.OutputJSON,
		OutputMD:     opts.OutputMD,
		Provider:     opts.Provider,
		APIKey:       opts.APIKey,
		Model:        opts.Model,
		MaxToolCalls: opts.MaxToolCalls,
		Timeout:      opts.Timeout,
		Interactive:  opts.Interactive,
	}, out, obs)
	if err != nil {
		return err
	}
	defer app.Close()
	return app.Run()
}
