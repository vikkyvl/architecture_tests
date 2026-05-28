package cli

import (
	"fmt"

	"github.com/archguard/project/bootstrap"
	"github.com/archguard/project/delivery/output"
)

const (
	estimateMsgProceed   = "Proceed with analysis?"
	estimateMsgStartRun  = "Start run 1 of %d now?"
	estimateLabelYes     = "Yes, analyze now"
	estimateLabelYesRun1 = "Yes, start run 1"
	estimateLabelNo      = "No, cancel"
	estimateRowSingleRun = "Estimated: %d tool calls  •  budget: %d"
	estimateRowMultiRun  = "Estimated: %d tool calls across %d runs  •  budget: %d/run"
)

func RunEstimate(opts ReviewOptions) error {
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

	out.WriteProgress(output.RenderEstimate(result))

	planOpts := output.RunPlanOpts{
		ProjectPath: opts.ProjectPath,
		RulesPath:   opts.RulesPath,
		OutputJSON:  opts.OutputJSON,
		Provider:    opts.Provider,
		Model:       opts.Model,
	}

	if !opts.Interactive {
		if result.RunsNeeded > 1 {
			out.WriteProgress(output.RenderEstimateRunPlan(result, planOpts))
		}
		return nil
	}

	if result.RunsNeeded == 1 {
		rows := []string{fmt.Sprintf(estimateRowSingleRun, result.EstimatedToolCalls, result.MaxToolCalls)}
		if output.AskYesNo(estimateMsgProceed, rows, estimateLabelYes, estimateLabelNo) {
			return RunReview(opts)
		}
		return nil
	}

	out.WriteProgress(output.RenderEstimateRunPlan(result, planOpts))

	rows := []string{fmt.Sprintf(estimateRowMultiRun, result.EstimatedToolCalls, result.RunsNeeded, result.MaxToolCalls)}
	if output.AskYesNo(fmt.Sprintf(estimateMsgStartRun, result.RunsNeeded), rows, estimateLabelYesRun1, estimateLabelNo) {
		return RunReview(opts)
	}
	return nil
}
