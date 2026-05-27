package output

import (
	"fmt"
	"strings"

	"github.com/archguard/project/shared/models"
)

const (
	panelEstimate        = "Estimate"
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
)

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
