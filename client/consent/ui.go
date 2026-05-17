package consent

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	mainMenuHelpHint      = "Use Up/Down and Enter, or press a/s/p/d. Esc denies."
	patternMenuHelpHint   = "Use Up/Down and Enter, or press y/n. Esc declines."
	patternMenuTitle      = "Save as project consent rule?"
	patternMenuFooterHint = "To narrow this rule, edit .archguard/consent.yaml after saving."
	consentPanelWidth     = 76
	consentLabelWidth     = 12
	consentCursor         = ">"
	consentNoCursor       = " "
	consentLineBreak      = "\n"
	consentEmptyLine      = ""
	consentSpace          = " "
	consentOptionGap      = "  "
	consentPromptSuffix   = ": "
	consentFormatChoice   = "[%s] %s"
	consentFormatRow      = "%s %s  %s"
	labelAllowOnce        = "Allow once"
	labelAllowSession     = "Allow session"
	labelAlwaysAllow      = "Always allow"
	labelDeny             = "Deny"
	labelPatternYes       = "Yes — save default pattern"
	labelPatternNo        = "No — allow once only"
	descAllowOnce         = "Only this tool call"
	descAllowSession      = "Same tool until review ends"
	descAlwaysAllow       = "Remember this pattern"
	descDeny              = "Return denied tool result"
	descPatternYes        = "Persist rule to project consent"
	descPatternNo         = "Don't save a rule"
)

var (
	consentPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("214")).
				Padding(1, 2).
				Width(consentPanelWidth)
	consentTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("214"))
	consentLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Width(consentLabelWidth)
	consentValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))
	consentDangerStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("203"))
	consentChoiceStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("81"))
	consentSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("16")).
				Background(lipgloss.Color("81")).
				Padding(0, 1)
	consentHelpStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))
)

type consentOption struct {
	key         string
	label       string
	description string
}

type consentChoiceModel struct {
	rows      []string
	options   []consentOption
	cursor    int
	choice    string
	cancelKey string
	helpHint  string
}

var _ tea.Model = (*consentChoiceModel)(nil)

var mainConsentOptions = []consentOption{
	{
		key:         choiceAllowOnce,
		label:       labelAllowOnce,
		description: descAllowOnce,
	},
	{
		key:         choiceAllowForSession,
		label:       labelAllowSession,
		description: descAllowSession,
	},
	{
		key:         choiceAlwaysAllow,
		label:       labelAlwaysAllow,
		description: descAlwaysAllow,
	},
	{
		key:         choiceDeny,
		label:       labelDeny,
		description: descDeny,
	},
}

var patternConfirmOptions = []consentOption{
	{
		key:         choicePatternYes,
		label:       labelPatternYes,
		description: descPatternYes,
	},
	{
		key:         choicePatternNo,
		label:       labelPatternNo,
		description: descPatternNo,
	},
}

func runConsentMenu(rows []string) string {
	return runMenu(rows, mainConsentOptions, choiceDeny, mainMenuHelpHint)
}

func runPatternConfirmMenu(rows []string) string {
	return runMenu(rows, patternConfirmOptions, choicePatternNo, patternMenuHelpHint)
}

func runMenu(rows []string, options []consentOption, cancelKey, helpHint string) string {
	model := consentChoiceModel{
		rows:      rows,
		options:   options,
		cancelKey: cancelKey,
		helpHint:  helpHint,
	}
	program := tea.NewProgram(model, tea.WithInput(os.Stdin), tea.WithOutput(os.Stderr))
	out, err := program.Run()
	if err != nil {
		return fallbackMenuChoice(rows, options, cancelKey)
	}
	final, ok := out.(consentChoiceModel)
	if !ok || final.choice == "" {
		return options[0].key
	}
	return final.choice
}

func (m consentChoiceModel) Init() tea.Cmd {
	return nil
}

func (m consentChoiceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch key {
		case "ctrl+c", "esc":
			m.choice = m.cancelKey
			return m, tea.Quit
		case "up", "k", "left", "h":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j", "right", "l":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter":
			m.choice = m.options[m.cursor].key
			return m, tea.Quit
		default:
			for _, opt := range m.options {
				if key == opt.key {
					m.choice = key
					return m, tea.Quit
				}
			}
		}
	}
	return m, nil
}

func (m consentChoiceModel) View() string {
	var lines []string
	lines = append(lines, m.rows...)
	lines = append(lines, consentEmptyLine)
	for i, option := range m.options {
		cursor := consentNoCursor
		label := fmt.Sprintf(consentFormatChoice, option.key, option.label)
		if i == m.cursor {
			cursor = consentCursor
			label = consentSelectedStyle.Render(label)
		} else {
			label = consentChoiceStyle.Render(label)
		}
		lines = append(lines, fmt.Sprintf(consentFormatRow, cursor, label, consentHelpStyle.Render(option.description)))
	}
	hint := m.helpHint
	if hint == "" {
		hint = mainMenuHelpHint
	}
	lines = append(lines, consentEmptyLine, consentHelpStyle.Render(hint))
	return consentPanelStyle.Render(strings.Join(lines, consentLineBreak)) + consentLineBreak
}

func fallbackMenuChoice(rows []string, options []consentOption, defaultKey string) string {
	fmt.Fprintln(os.Stderr, consentPanelStyle.Render(strings.Join(rows, consentLineBreak)))
	var keys []string
	for _, opt := range options {
		keys = append(keys, fmt.Sprintf(consentFormatChoice, opt.key, opt.label))
	}
	fmt.Fprint(os.Stderr, consentChoiceStyle.Render(strings.Join(keys, consentOptionGap)+consentPromptSuffix))
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(strings.ToLower(input))
	for _, opt := range options {
		if choice == opt.key {
			return opt.key
		}
	}
	return defaultKey
}

func renderConsentKV(label, value string) string {
	return consentLabelStyle.Render(label) + consentSpace + consentValueStyle.Render(value)
}

func renderStatus(text string) string {
	return consentPanelStyle.Render(text) + consentLineBreak
}
