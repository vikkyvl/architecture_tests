package cli

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/archguard/project/client/detector"
	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/models"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	uiWidth            = 76
	labelWidth         = 15
	progressWidth      = 32
	headerPad          = 1
	terminalClearLine  = "\r\x1b[2K"
	rateLimitTick      = 120 * time.Millisecond
	rateLimitTimeSlice = time.Second
	linkMarker         = "↗"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("63")).
			Padding(0, headerPad)

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63"))

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("60")).
			Padding(1, 2).
			Width(uiWidth)

	subtlePanelStyle = panelStyle.Copy().
				BorderForeground(lipgloss.Color("238"))

	mutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(labelWidth)
	valueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	linkStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Underline(true)

	successBadgeStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("16")).
				Background(lipgloss.Color("42")).
				Padding(0, 1)
	warnBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("16")).
			Background(lipgloss.Color("214")).
			Padding(0, 1)
	errorBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("203")).
			Padding(0, 1)
	infoBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("16")).
			Background(lipgloss.Color("81")).
			Padding(0, 1)

	progressGradient = progress.WithGradient("#5fd7ff", "#7d5fff")
)

type reviewSummaryModel struct {
	content string
}

var _ tea.Model = reviewSummaryModel{}

func (m reviewSummaryModel) Init() tea.Cmd {
	return nil
}

func (m reviewSummaryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m reviewSummaryModel) View() string {
	return m.content
}

func renderHeader(title string) string {
	return titleStyle.Render(title)
}

func RenderError(err error) string {
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		return errorStyle.Render("Error") + " " + valueStyle.Render(err.Error())
	}

	title := "Error"
	switch appErr.Kind {
	case apperrors.KindValidation:
		title = "Validation error"
	case apperrors.KindNotFound:
		title = "Not found"
	case apperrors.KindPermission:
		title = "Permission denied"
	case apperrors.KindExternalService:
		title = "External service error"
	case apperrors.KindRateLimited:
		title = "Rate limited"
	case apperrors.KindInternal:
		title = "Internal error"
	}

	return renderNotice(errorBadgeStyle.Render(title), appErr.Error())
}

func renderRunOverview(cfgName, language string, layers, depRules, domainRules int, providerName, providerModel string, maxTools int, timeout string) string {
	rows := []string{
		renderKV("Project", fmt.Sprintf("%s (%s)", cfgName, language)),
		renderKV("Rules", fmt.Sprintf("%d dependency, %d domain", depRules, domainRules)),
		renderKV("Layers", fmt.Sprintf("%d", layers)),
		renderKV("LLM", fmt.Sprintf("%s (%s)", providerName, providerModel)),
		renderKV("Limits", fmt.Sprintf("%d tool calls, %s timeout", maxTools, timeout)),
	}

	return renderPanel("Run overview", rows)
}

func renderDetection(r detector.DetectResult) string {
	var rows []string
	rows = append(rows, renderKV("Detected", r.PrimaryLanguage))

	langs := make([]string, 0, len(r.FileCounts))
	for lang := range r.FileCounts {
		langs = append(langs, lang)
	}
	sort.Slice(langs, func(i, j int) bool {
		left := r.FileCounts[langs[i]]
		right := r.FileCounts[langs[j]]
		if left == right {
			return langs[i] < langs[j]
		}
		return left > right
	})

	for _, lang := range langs {
		count := r.FileCounts[lang]
		value := fmt.Sprintf("%d files", count)
		if conf, ok := r.Confidence[lang]; ok {
			value = fmt.Sprintf("%d files (confidence %.0f%%)", count, conf*100)
		}
		rows = append(rows, renderKV(lang, value))
	}
	return renderPanel("Language detection", rows)
}

func renderKV(label, value string) string {
	return labelStyle.Render(label) + " " + valueStyle.Render(value)
}

func renderKVRaw(label, value string) string {
	return labelStyle.Render(label) + " " + value
}

func renderStep(text string) string {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	return infoBadgeStyle.Render("INFO") + " " + infoStyle.Render(s.View()) + " " + valueStyle.Render(text)
}

func renderLLMText(text string) string {
	return renderSubtlePanel("LLM", []string{valueStyle.Render(text)})
}

func renderToolResult(index int, name string, size int, err error) string {
	prefix := mutedStyle.Render(fmt.Sprintf("#%02d", index))
	if err != nil {
		if apperrors.IsKind(err, apperrors.KindPermission) {
			return fmt.Sprintf("%s %s %s", prefix, warnBadgeStyle.Render("DENIED"), valueStyle.Render(name+": "+err.Error()))
		}
		return fmt.Sprintf("%s %s %s", prefix, errorBadgeStyle.Render("FAIL"), valueStyle.Render(name+": "+err.Error()))
	}
	meta := mutedStyle.Render(fmt.Sprintf("%d chars", size))
	return fmt.Sprintf("%s %s %s %s", prefix, successBadgeStyle.Render("OK"), valueStyle.Render(name), meta)
}

func renderRetry(provider string, attempt, maxAttempts int, wait time.Duration, err error) string {
	detail := fmt.Sprintf("%s request failed, retry %d/%d in %s", provider, attempt, maxAttempts, wait)
	if err != nil {
		detail = fmt.Sprintf("%s: %s", detail, err.Error())
	}
	return fmt.Sprintf("%s %s", warnBadgeStyle.Render("RETRY"), mutedStyle.Render(detail))
}

func renderRateLimitWait(wait time.Duration) string {
	return renderRateLimitWaitFrame(wait, "|")
}

func renderRateLimitWaitFrame(remaining time.Duration, frame string) string {
	return fmt.Sprintf(
		"%s %s %s",
		warnBadgeStyle.Render("RATE LIMIT"),
		infoStyle.Render(frame),
		mutedStyle.Render(fmt.Sprintf("waiting %s before retry", roundUpDuration(remaining, rateLimitTimeSlice))),
	)
}

func renderRateLimitResume(wait time.Duration) string {
	return fmt.Sprintf("%s %s", warnBadgeStyle.Render("RETRY"), mutedStyle.Render(fmt.Sprintf("rate limit wait complete after %s", wait)))
}

func roundUpDuration(d, unit time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	if unit <= 0 || d%unit == 0 {
		return d
	}
	return (d/unit + 1) * unit
}

func renderReportPaths(jsonPath, mdPath string) string {
	arrow := mutedStyle.Render(linkMarker)
	rows := []string{
		renderKVRaw("JSON report", arrow+" "+jsonPath),
		renderKVRaw("Markdown", arrow+" "+mdPath),
	}
	panel := renderPanel("Report files", rows)
	panel = insertFileLink(panel, jsonPath)
	panel = insertFileLink(panel, mdPath)
	return panel
}

func insertFileLink(panel, path string) string {
	uri := fileURI(path)
	if uri == "" {
		return panel
	}
	styled := styledFileLinkText(path)
	linked := fmt.Sprintf("\x1b]8;;%s\a%s\x1b]8;;\a", uri, styled)
	return strings.Replace(panel, path, linked, 1)
}

func styledFileLinkText(path string) string {
	return fmt.Sprintf("\x1b[1;4;38;5;51m%s\x1b[0m", path)
}

func fileURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return (&url.URL{Scheme: "file", Path: abs}).String()
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
		severityRows = append(severityRows, renderKV(titleCase(sev), fmt.Sprintf("%s %d", bar.ViewAs(float64(count)/float64(total)), count)))
	}
	if len(severityRows) == 0 {
		severityRows = append(severityRows, successStyle.Render("No architectural violations found."))
	}

	status := successStyle.Render("complete")
	if ar.Incomplete {
		status = warnStyle.Render("incomplete")
	}

	rows := []string{
		renderKV("Status", status),
		renderKV("Violations", fmt.Sprintf("%d", ar.Metrics.TotalViolations)),
	}
	rows = append(rows, severityRows...)
	rows = append(rows,
		renderKV("Tool calls", fmt.Sprintf("%d", ar.ToolCalls)),
		renderKV("Duration", ar.Duration),
	)

	model := reviewSummaryModel{content: renderPanel("Results", rows)}
	return model.View()
}

func renderPanel(title string, rows []string) string {
	body := strings.Join(rows, "\n")
	if title == "" {
		return panelStyle.Render(body)
	}
	content := sectionStyle.Render(title) + "\n\n" + body
	return panelStyle.Render(content)
}

func renderSubtlePanel(title string, rows []string) string {
	body := strings.Join(rows, "\n")
	if title == "" {
		return subtlePanelStyle.Render(body)
	}
	content := mutedStyle.Bold(true).Render(title) + "\n\n" + body
	return subtlePanelStyle.Render(content)
}

func renderNotice(prefix, text string) string {
	return fmt.Sprintf("%s %s", prefix, valueStyle.Render(text))
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
