package cli

import (
	"fmt"

	"github.com/archguard/project/bootstrap"
	"github.com/archguard/project/delivery/output"
)

const (
	estimateMsgProceed = "Proceed with analysis?"
	estimateMsgChoose  = "How would you like to proceed?"

	estimateLabelYes       = "Yes, analyze now"
	estimateLabelSingleRun = "Single run  (--max-tool-calls=%d)"
	estimateLabelSplitRun  = "Split into %d runs  (--max-tool-calls=%d each, use --resume)"
	estimateLabelNo        = "Show me the commands"
	estimateLabelExit      = "Exit"

	estimateKeySingleRun = "a"
	estimateKeySplitRun  = "s"
	estimateKeyNo        = "n"
	estimateKeyExit      = "q"

	estimateRowSingleRun = "Estimated: %d tool calls  •  budget: %d"
	estimateRowMultiRun  = "Estimated: %d tool calls  •  suggested budget: %d (single run) or %d/run (split)"
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
		out.WriteProgress(output.RenderEstimateRunPlan(result, planOpts))
		return nil
	}

	if result.RunsNeeded == 1 {
		rows := []string{fmt.Sprintf(estimateRowSingleRun, result.EstimatedToolCalls, result.MaxToolCalls)}
		if output.AskYesNo(estimateMsgProceed, rows, estimateLabelYes, estimateLabelNo) {
			return RunReview(opts)
		}
		out.WriteProgress(output.RenderEstimateRunPlan(result, planOpts))
		return nil
	}

	rows := []string{fmt.Sprintf(estimateRowMultiRun, result.EstimatedToolCalls, result.SuggestedMaxToolCalls, result.MaxToolCalls)}
	choice := output.AskChoice(estimateMsgChoose, rows, []output.MenuOption{
		{Key: estimateKeySingleRun, Label: fmt.Sprintf(estimateLabelSingleRun, result.SuggestedMaxToolCalls), Hint: "one run, larger budget"},
		{Key: estimateKeySplitRun, Label: fmt.Sprintf(estimateLabelSplitRun, result.RunsNeeded, result.MaxToolCalls), Hint: "resume between runs"},
		{Key: estimateKeyNo, Label: estimateLabelNo, Hint: "print commands to copy", Default: true},
		{Key: estimateKeyExit, Label: estimateLabelExit},
	})

	switch choice {
	case estimateKeySingleRun:
		opts.MaxToolCalls = result.SuggestedMaxToolCalls
		return RunReview(opts)
	case estimateKeySplitRun:
		return RunReview(opts)
	case estimateKeyNo:
		out.WriteProgress(output.RenderEstimateRunPlan(result, planOpts))
	}
	return nil
}
