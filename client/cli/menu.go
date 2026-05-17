package cli

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	yesNoChoiceYes   = "y"
	yesNoChoiceNo    = "n"
	yesNoCursor      = ">"
	yesNoNoCursor    = " "
	yesNoMenuHint    = "Arrows or y/n to choose, Enter to confirm, Esc cancels."
	yesNoMenuPadding = 76
	yesNoKeyUpperY   = "Y"
	yesNoKeyUpperN   = "N"
	yesNoKeyCtrlC    = "ctrl+c"
	yesNoKeyEsc      = "esc"
	yesNoKeyUp       = "up"
	yesNoKeyDown     = "down"
	yesNoKeyLeft     = "left"
	yesNoKeyRight    = "right"
	yesNoKeyK        = "k"
	yesNoKeyJ        = "j"
	yesNoKeyH        = "h"
	yesNoKeyL        = "l"
	yesNoKeyEnter    = "enter"
	yesNoEmptyLine   = ""
	yesNoLineBreak   = "\n"
	yesNoHintGap     = "  "
	yesNoFormatLabel = "[%s] %s"
	yesNoFormatRow   = "%s %s%s"
)

var (
	yesNoPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("81")).
			Padding(1, 2).
			Width(yesNoMenuPadding)
	yesNoSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("16")).
				Background(lipgloss.Color("81")).
				Padding(0, 1)
	yesNoChoiceStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("81"))
	yesNoHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	yesNoTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("81"))
)

type yesNoOption struct {
	key   string
	label string
	hint  string
}

type yesNoModel struct {
	title   string
	rows    []string
	options []yesNoOption
	cursor  int
	choice  string
}

var _ tea.Model = (*yesNoModel)(nil)

func (m yesNoModel) Init() tea.Cmd {
	return nil
}

func (m yesNoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case yesNoKeyCtrlC, yesNoKeyEsc:
			m.choice = yesNoChoiceNo
			return m, tea.Quit
		case yesNoKeyUp, yesNoKeyK, yesNoKeyLeft, yesNoKeyH:
			if m.cursor > 0 {
				m.cursor--
			}
		case yesNoKeyDown, yesNoKeyJ, yesNoKeyRight, yesNoKeyL:
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case yesNoKeyEnter:
			m.choice = m.options[m.cursor].key
			return m, tea.Quit
		case yesNoChoiceYes, yesNoKeyUpperY:
			m.choice = yesNoChoiceYes
			return m, tea.Quit
		case yesNoChoiceNo, yesNoKeyUpperN:
			m.choice = yesNoChoiceNo
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m yesNoModel) View() string {
	var lines []string
	lines = append(lines, yesNoTitleStyle.Render(m.title))
	if len(m.rows) > 0 {
		lines = append(lines, yesNoEmptyLine)
		lines = append(lines, m.rows...)
	}
	lines = append(lines, yesNoEmptyLine)
	for i, opt := range m.options {
		cursor := yesNoNoCursor
		label := fmt.Sprintf(yesNoFormatLabel, opt.key, opt.label)
		if i == m.cursor {
			cursor = yesNoCursor
			label = yesNoSelectedStyle.Render(label)
		} else {
			label = yesNoChoiceStyle.Render(label)
		}
		hint := yesNoEmptyLine
		if opt.hint != "" {
			hint = yesNoHintGap + yesNoHelpStyle.Render(opt.hint)
		}
		lines = append(lines, fmt.Sprintf(yesNoFormatRow, cursor, label, hint))
	}
	lines = append(lines, yesNoEmptyLine, yesNoHelpStyle.Render(yesNoMenuHint))
	return yesNoPanelStyle.Render(strings.Join(lines, yesNoLineBreak)) + yesNoLineBreak
}

func askYesNo(title string, rows []string, yesLabel, noLabel string) bool {
	model := yesNoModel{
		title: title,
		rows:  rows,
		options: []yesNoOption{
			{
				key:   yesNoChoiceYes,
				label: yesLabel,
			},
			{
				key:   yesNoChoiceNo,
				label: noLabel,
			},
		},
	}
	program := tea.NewProgram(model, tea.WithInput(os.Stdin), tea.WithOutput(os.Stderr))
	out, err := program.Run()
	if err != nil {
		return false
	}
	final, ok := out.(yesNoModel)
	if !ok || final.choice == "" {
		return false
	}
	return final.choice == yesNoChoiceYes
}
