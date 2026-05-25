package output

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/archguard/project/infrastructure/detector"
	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

const (
	uiWidth             = 76
	labelWidth          = 15
	progressWidth       = 32
	headerPad           = 1
	terminalClearLine   = "\r\x1b[2K"
	newline             = "\n"
	doubleNewline       = "\n\n"
	spaceSeparator      = " "
	rateLimitTick       = 120 * time.Millisecond
	rateLimitTimeSlice  = time.Second
	linkMarker          = "↗"
	labelError          = "Error"
	labelValidation     = "Validation error"
	labelNotFound       = "Not found"
	labelPermission     = "Permission denied"
	labelExternal       = "External service error"
	labelRateLimited    = "Rate limited"
	labelInternal       = "Internal error"
	labelProject        = "Project"
	labelRules          = "Rules"
	labelLayers         = "Layers"
	labelLLM            = "LLM"
	labelLimits         = "Limits"
	labelDetected       = "Detected"
	labelJSONReport     = "JSON report"
	labelMarkdown       = "Markdown"
	labelStatus         = "Status"
	labelViolations     = "Violations"
	labelToolCalls      = "Tool calls"
	labelDuration       = "Duration"
	panelRunOverview    = "Run overview"
	panelDetection      = "Language detection"
	panelReports        = "Report files"
	panelResults        = "Results"
	badgeInfo           = "INFO"
	BadgeSkip           = "SKIP"
	badgeDenied         = "DENIED"
	badgeFail           = "FAIL"
	BadgeOK             = "OK"
	badgeRetry          = "RETRY"
	badgeRateLimit      = "RATE LIMIT"
	badgeProactivePause = "PROACTIVE PAUSE"
	statusComplete      = "complete"
	statusIncomplete    = "incomplete"
	msgNoViolations     = "No architectural violations found."
	fileURLScheme       = "file"
)

const (
	formatErrorFallback      = "%s %s"
	formatNameWithContext    = "%s (%s)"
	formatRulesSummary       = "%d dependency, %d domain"
	formatLimitsSummary      = "%d tool calls, %s timeout"
	formatFileCount          = "%d files"
	formatFileConfidence     = "%d files (confidence %.0f%%)"
	formatToolIndex          = "#%02d"
	formatToolResult         = "%s %s %s"
	formatToolResultWithMeta = "%s %s %s %s"
	formatRetryDetail        = "%s request failed, retry %d/%d in %s"
	formatErrorDetail        = "%s: %s"
	formatRateLimitFrame     = "%s %s %s"
	formatRateLimitWaiting   = "waiting %s before retry"
	formatRateLimitComplete  = "rate limit wait complete after %s"
	formatProactiveWaiting   = "pausing %s ahead of token-budget exhaustion"
	formatProactiveComplete  = "proactive pause complete after %s"
	formatHyperlink          = "\x1b]8;;%s\a%s\x1b]8;;\a"
	formatStyledLink         = "\x1b[1;4;38;5;51m%s\x1b[0m"
	formatSeverityRow        = "%s %d"
	formatNotice             = "%s %s"
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
	ValueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	WarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	linkStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Underline(true)

	SuccessBadgeStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("16")).
				Background(lipgloss.Color("42")).
				Padding(0, 1)
	WarnBadgeStyle = lipgloss.NewStyle().
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
)

func RenderHeader(title string) string {
	return titleStyle.Render(title)
}

func RenderError(err error) string {
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		return errorStyle.Render(labelError) + spaceSeparator + ValueStyle.Render(publicErrorMessage(err))
	}

	title := labelError
	switch appErr.Kind {
	case apperrors.KindValidation:
		title = labelValidation
	case apperrors.KindNotFound:
		title = labelNotFound
	case apperrors.KindPermission:
		title = labelPermission
	case apperrors.KindExternalService:
		title = labelExternal
	case apperrors.KindRateLimited:
		title = labelRateLimited
	case apperrors.KindInternal:
		title = labelInternal
	}

	return RenderNotice(errorBadgeStyle.Render(title), publicErrorMessage(appErr))
}

func RenderRunOverview(cfgName, language string, layers, depRules, domainRules int, providerName, providerModel string, maxTools int, timeout string) string {
	rows := []string{
		RenderKV(labelProject, fmt.Sprintf(formatNameWithContext, cfgName, language)),
		RenderKV(labelRules, fmt.Sprintf(formatRulesSummary, depRules, domainRules)),
		RenderKV(labelLayers, fmt.Sprintf(c.FormatInt, layers)),
		RenderKV(labelLLM, fmt.Sprintf(formatNameWithContext, providerName, providerModel)),
		RenderKV(labelLimits, fmt.Sprintf(formatLimitsSummary, maxTools, timeout)),
	}

	return RenderPanel(panelRunOverview, rows)
}

func RenderDetection(r detector.DetectResult) string {
	var rows []string
	rows = append(rows, RenderKV(labelDetected, r.PrimaryLanguage))

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
		value := fmt.Sprintf(formatFileCount, count)
		if conf, ok := r.Confidence[lang]; ok {
			value = fmt.Sprintf(formatFileConfidence, count, conf*100)
		}
		rows = append(rows, RenderKV(lang, value))
	}
	return RenderPanel(panelDetection, rows)
}

func RenderKV(label, value string) string {
	return labelStyle.Render(label) + spaceSeparator + ValueStyle.Render(value)
}

func renderKVRaw(label, value string) string {
	return labelStyle.Render(label) + spaceSeparator + value
}

func RenderStep(text string) string {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	return infoBadgeStyle.Render(badgeInfo) + spaceSeparator + infoStyle.Render(s.View()) + spaceSeparator + ValueStyle.Render(text)
}

func RenderLLMText(text string) string {
	return RenderSubtlePanel(labelLLM, []string{ValueStyle.Render(text)})
}

func RenderPanel(title string, rows []string) string {
	body := strings.Join(rows, newline)
	if title == "" {
		return panelStyle.Render(body)
	}
	content := sectionStyle.Render(title) + doubleNewline + body
	return panelStyle.Render(content)
}

func RenderSubtlePanel(title string, rows []string) string {
	body := strings.Join(rows, newline)
	if title == "" {
		return subtlePanelStyle.Render(body)
	}
	content := mutedStyle.Bold(true).Render(title) + doubleNewline + body
	return subtlePanelStyle.Render(content)
}

func RenderNotice(prefix, text string) string {
	return fmt.Sprintf(formatNotice, prefix, ValueStyle.Render(text))
}
