package cli

import (
	"github.com/archguard/project/bootstrap"
	"github.com/archguard/project/delivery/output"
)

type EstimateOptions struct {
	ProjectPath  string
	RulesPath    string
	MaxToolCalls int
}

func RunEstimate(opts EstimateOptions) error {
	out := output.NewOutputTransport()
	result, err := bootstrap.Estimate(bootstrap.EstimateOptions{
		ProjectPath:  opts.ProjectPath,
		RulesPath:    opts.RulesPath,
		MaxToolCalls: opts.MaxToolCalls,
	})
	if err != nil {
		out.WriteProgress(output.RenderError(err))
		return err
	}

	out.WriteResult(output.RenderEstimate(result))
	return nil
}
