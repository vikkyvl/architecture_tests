package output

import (
	"fmt"
	"strings"

	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/models"
)

const (
	panelEstimate        = "Estimate"
	panelRunPlan         = "Run plan"
	panelResumeHint      = "Analysis incomplete — resume hint"
	fmtResumeHintLine1   = "Limit reached before all files were covered."
	fmtResumeHintLine2   = "Re-run with --resume to continue from where it stopped:"
	fmtCmdMaxToolCalls   = " --max-tool-calls=%d"
	labelFiles           = "Files"
	labelEstToolCalls    = "Est. tool calls"
	labelRunsNeeded      = "Runs needed"
	labelSuggestedBudget = "Suggested"
	labelByLayer         = "By layer"
	labelUnknown         = "Unknown"
	fmtLayerRow          = "  %-20s %4d files  ~%d calls"
	fmtRunsOK            = "%d  (fits in one run with --max-tool-calls=%d)"
	fmtRunsMany          = "%d  (use --resume between runs; --max-tool-calls=%d each)"
	fmtSuggestedBudget   = "%d (set --max-tool-calls=%d to finish in one run)"
	fmtRunHeader         = "Run %d of %d:"
	fmtCmdBase           = "  ./%s analyze -p %s -r %s --max-tool-calls=%d"
	fmtCmdOutputJSON     = " --output-json=%s"
	fmtCmdResume         = " --resume=%s"
	fmtCmdProvider       = " --provider=%s"
	fmtCmdModel          = " --model=%s"
	fmtCmdTimeout        = " --timeout=%s"
	fmtRepeatNote        = "  (repeat run 2 command %d more time(s) until coverage is complete)"
	labelOptionSingleRun = "Option A — single run (increase budget to %d):"
	labelOptionSplit     = "Option B — split across %d runs (keep --max-tool-calls=%d):"
)

type RunPlanOpts struct {
	ProjectPath string
	RulesPath   string
	OutputJSON  string
	Provider    string
	Model       string
	Timeout     string
}

func RenderEstimate(r *models.EstimateResult) string {
	rows := []string{
		RenderKV(labelProject, fmt.Sprintf("%s (%s)", r.ProjectName, r.Language)),
		RenderKV(labelFiles, fmt.Sprintf("%d", r.TotalFiles)),
		RenderKV(labelEstToolCalls, fmt.Sprintf("%d", r.EstimatedToolCalls)),
	}

	layerLines := make([]string, 0, len(r.FilesByLayer)+1)
	for _, l := range r.FilesByLayer {
		layerLines = append(layerLines, fmt.Sprintf(fmtLayerRow, l.Name, l.FileCount, l.ToolCalls))
	}
	if r.UnknownFiles > 0 {
		layerLines = append(layerLines, fmt.Sprintf(fmtLayerRow, labelUnknown, r.UnknownFiles, 0))
	}
	if len(layerLines) > 0 {
		rows = append(rows, renderKVRaw(labelByLayer, newline+strings.Join(layerLines, newline)))
	}

	if r.RunsNeeded == 1 {
		rows = append(rows, RenderKV(labelRunsNeeded, fmt.Sprintf(fmtRunsOK, r.RunsNeeded, r.MaxToolCalls)))
	} else {
		rows = append(rows, RenderKV(labelRunsNeeded, WarnStyle.Render(fmt.Sprintf(fmtRunsMany, r.RunsNeeded, r.MaxToolCalls))))
		rows = append(rows, RenderKV(labelSuggestedBudget, fmt.Sprintf(fmtSuggestedBudget, r.SuggestedMaxToolCalls, r.SuggestedMaxToolCalls)))
	}

	return RenderPanel(panelEstimate, rows)
}

func RenderResumeHint(opts RunPlanOpts, maxToolCalls int) string {
	outputJSON := opts.OutputJSON
	if outputJSON == "" {
		outputJSON = "report.json"
	}

	cmd := fmt.Sprintf(fmtCmdBase, c.AppName, opts.ProjectPath, opts.RulesPath, maxToolCalls)
	cmd += fmt.Sprintf(fmtCmdOutputJSON, outputJSON)
	cmd += fmt.Sprintf(fmtCmdResume, outputJSON)
	if opts.Provider != "" {
		cmd += fmt.Sprintf(fmtCmdProvider, opts.Provider)
	}
	if opts.Model != "" {
		cmd += fmt.Sprintf(fmtCmdModel, opts.Model)
	}
	if opts.Timeout != "" {
		cmd += fmt.Sprintf(fmtCmdTimeout, opts.Timeout)
	}

	rows := []string{
		WarnStyle.Render(fmtResumeHintLine1),
		fmtResumeHintLine2,
		"",
		mutedStyle.Render(cmd),
	}
	return RenderPanel(panelResumeHint, rows)
}

func RenderEstimateRunPlan(r *models.EstimateResult, opts RunPlanOpts) string {
	outputJSON := opts.OutputJSON
	if outputJSON == "" {
		outputJSON = "report.json"
	}

	buildCmd := func(maxCalls int) string {
		cmd := fmt.Sprintf(fmtCmdBase, c.AppName, opts.ProjectPath, opts.RulesPath, maxCalls)
		cmd += fmt.Sprintf(fmtCmdOutputJSON, outputJSON)
		if opts.Provider != "" {
			cmd += fmt.Sprintf(fmtCmdProvider, opts.Provider)
		}
		if opts.Model != "" {
			cmd += fmt.Sprintf(fmtCmdModel, opts.Model)
		}
		if opts.Timeout != "" {
			cmd += fmt.Sprintf(fmtCmdTimeout, opts.Timeout)
		}
		return cmd
	}

	var rows []string

	if r.RunsNeeded > 1 {
		singleCmd := buildCmd(r.SuggestedMaxToolCalls)
		rows = append(rows, sectionStyle.Render(fmt.Sprintf(labelOptionSingleRun, r.SuggestedMaxToolCalls)))
		rows = append(rows, mutedStyle.Render(singleCmd))
		rows = append(rows, "")

		baseCmd := buildCmd(r.MaxToolCalls)
		resumeCmd := baseCmd + fmt.Sprintf(fmtCmdResume, outputJSON)
		rows = append(rows, sectionStyle.Render(fmt.Sprintf(labelOptionSplit, r.RunsNeeded, r.MaxToolCalls)))
		rows = append(rows, sectionStyle.Render(fmt.Sprintf(fmtRunHeader, 1, r.RunsNeeded)))
		rows = append(rows, mutedStyle.Render(baseCmd))
		rows = append(rows, "")
		rows = append(rows, sectionStyle.Render(fmt.Sprintf(fmtRunHeader, 2, r.RunsNeeded)))
		rows = append(rows, mutedStyle.Render(resumeCmd))
		if r.RunsNeeded > 2 {
			rows = append(rows, "")
			rows = append(rows, mutedStyle.Render(fmt.Sprintf(fmtRepeatNote, r.RunsNeeded-2)))
		}
	} else {
		rows = append(rows, sectionStyle.Render(fmt.Sprintf(fmtRunHeader, 1, 1)))
		rows = append(rows, mutedStyle.Render(buildCmd(r.MaxToolCalls)))
	}

	return RenderPanel(panelRunPlan, rows)
}
