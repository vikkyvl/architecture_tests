package cli

import (
	"fmt"
	"strings"

	"github.com/archguard/project/shared/models"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	c "github.com/archguard/project/shared/constants"
)

var progressGradient = progress.WithGradient("#5fd7ff", "#7d5fff")

type reviewSummaryModel struct {
	content string
}

var _ tea.Model = (*reviewSummaryModel)(nil)

func (m reviewSummaryModel) Init() tea.Cmd {
	return nil
}

func (m reviewSummaryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m reviewSummaryModel) View() string {
	return m.content
}

func renderResults(ar *models.AnalysisResult) string {
	bar := progress.New(progressGradient, progress.WithWidth(progressWidth))
	total := max(ar.Metrics.TotalViolations, 1)
	severityRows := make([]string, 0, len(c.SeverityList))
	for _, sev := range c.SeverityList {
		count := ar.Metrics.BySeverity[sev]
		if count == 0 {
			continue
		}
		severityRows = append(severityRows, renderKV(titleCase(sev), fmt.Sprintf(formatSeverityRow, bar.ViewAs(float64(count)/float64(total)), count)))
	}
	if len(severityRows) == 0 {
		severityRows = append(severityRows, successStyle.Render(msgNoViolations))
	}

	status := successStyle.Render(statusComplete)
	if ar.Incomplete {
		status = warnStyle.Render(statusIncomplete)
	}

	rows := []string{
		renderKV(labelStatus, status),
		renderKV(labelViolations, fmt.Sprintf(c.FormatInt, ar.Metrics.TotalViolations)),
	}
	rows = append(rows, severityRows...)
	rows = append(rows,
		renderKV(labelToolCalls, fmt.Sprintf(c.FormatInt, ar.ToolCalls)),
		renderKV(labelDuration, ar.Duration),
	)

	model := reviewSummaryModel{
		content: renderPanel(panelResults, rows),
	}
	return model.View()
}

func titleCase(s string) string {
	result := s
	if result != "" {
		result = strings.ToUpper(result[:1]) + result[1:]
	}
	return result
}
