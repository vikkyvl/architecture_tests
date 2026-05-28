package cli

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/archguard/project/bootstrap"
	"github.com/archguard/project/delivery/output"
)

const exitCodeInterrupted = 130

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
	ResumePath   string
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
		ResumePath:   opts.ResumePath,
	}, out, obs)
	if err != nil {
		return err
	}
	defer app.Close()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			signal.Stop(sigCh)
			out.WriteProgress(output.RenderInterrupted() + "\n")
			app.Close()
			os.Exit(exitCodeInterrupted)
		case <-done:
		}
	}()

	err = app.Run()
	close(done)
	return err
}
