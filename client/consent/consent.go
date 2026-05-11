package consent

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	c "github.com/archguard/project/shared/constants"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

const (
	msgBlockedTool        = "BLOCKED %s %s\n"
	msgConsentTool        = "Tool"
	msgConsentBudget      = "Budget"
	msgConsentPattern     = "Pattern [%s]: "
	choiceAllowOnce       = "a"
	choiceAllowForSession = "s"
	choiceAlwaysAllow     = "p"
	choiceDeny            = "d"
	recursiveGlobSuffix   = "/**"
	projectConsentPath    = ".aat/consent.yaml"
	userConsentPath       = ".config/aat/consent.yaml"
	consentFilePermission = 0o600
	consentDirPermission  = 0o700
	patternAll            = "all"
	patternGlobPrefix     = "glob:"
	patternSystemPrefix   = "system:"
	defaultExternalSystem = "notion"
	msgNonInteractiveDeny = "DENIED %s requires consent; run interactively or add a whitelist rule\n"
	consentPanelWidth     = 76
	consentLabelWidth     = 12
	consentCursor         = ">"
	consentNoCursor       = " "
)

type Decision = string

type Manager struct {
	sessionAllowed []Rule
	projectAllowed []Rule
	userAllowed    []Rule
	projectPath    string
	interactive    bool
}

type Rule struct {
	Tool    string `yaml:"tool"`
	Pattern string `yaml:"pattern"`
}

type consentFile struct {
	Allowed []Rule `yaml:"allowed"`
}

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
	rows    []string
	options []consentOption
	cursor  int
	choice  string
}

var _ tea.Model = consentChoiceModel{}

func NewManager(interactive bool, projectPath string) *Manager {
	m := &Manager{interactive: interactive, projectPath: projectPath}
	m.projectAllowed = loadRules(filepath.Join(projectPath, projectConsentPath))
	if home, err := os.UserHomeDir(); err == nil {
		m.userAllowed = loadRules(filepath.Join(home, userConsentPath))
	}
	return m
}

func (m *Manager) Check(toolName string, args map[string]interface{}, budget int) Decision {
	if blocked, value := isBlockedCall(toolName, args); blocked {
		fmt.Fprint(os.Stderr, renderStatus(consentDangerStyle.Render(fmt.Sprintf(msgBlockedTool, toolName, value))))
		return c.DecisionBlocked
	}

	if c.AutoApprovedTools[toolName] {
		return c.DecisionAutoApproved
	}

	if allowedBy(m.projectAllowed, toolName, args) {
		return c.DecisionProjectAllow
	}

	if allowedBy(m.userAllowed, toolName, args) {
		return c.DecisionGlobalAllow
	}

	if allowedBy(m.sessionAllowed, toolName, args) {
		return c.DecisionSessionAllow
	}

	if !m.interactive {
		fmt.Fprint(os.Stderr, renderStatus(consentDangerStyle.Render(fmt.Sprintf(msgNonInteractiveDeny, toolName))))
		return c.DecisionUserDenied
	}

	return m.prompt(toolName, args, budget)
}

func (m *Manager) prompt(toolName string, args map[string]interface{}, budget int) Decision {
	rows := []string{
		consentTitleStyle.Render("Consent required"),
		"",
		renderConsentKV(msgConsentTool, toolName),
	}
	for k, v := range args {
		rows = append(rows, renderConsentKV(k, fmt.Sprintf("%v", v)))
	}
	rows = append(rows, renderConsentKV(msgConsentBudget, fmt.Sprintf("%d calls remaining", budget)))

	fmt.Fprintln(os.Stderr)
	choice := runConsentMenu(rows)

	switch choice {
	case choiceAllowForSession:
		m.sessionAllowed = append(m.sessionAllowed, Rule{Tool: toolName, Pattern: patternAll})
		return c.DecisionUserAllowed
	case choiceAlwaysAllow:
		pattern := m.readPattern(toolName, args)
		rule := Rule{Tool: toolName, Pattern: pattern}
		if m.promptSaveScope() == "u" {
			m.userAllowed = append(m.userAllowed, rule)
			if home, err := os.UserHomeDir(); err == nil {
				_ = saveRules(filepath.Join(home, userConsentPath), m.userAllowed)
			}
			return c.DecisionGlobalAllow
		}
		m.projectAllowed = append(m.projectAllowed, rule)
		_ = saveRules(filepath.Join(m.projectPath, projectConsentPath), m.projectAllowed)
		return c.DecisionProjectAllow
	case choiceDeny:
		return c.DecisionUserDenied
	default:
		return c.DecisionUserAllowed
	}
}

func runConsentMenu(rows []string) string {
	model := consentChoiceModel{
		rows: rows,
		options: []consentOption{
			{key: choiceAllowOnce, label: "Allow once", description: "Only this tool call"},
			{key: choiceAllowForSession, label: "Allow session", description: "Same pattern until review ends"},
			{key: choiceAlwaysAllow, label: "Always allow", description: "Save pattern to project .aat/consent.yaml"},
			{key: choiceDeny, label: "Deny", description: "Return denied tool result"},
		},
	}
	program := tea.NewProgram(model, tea.WithInput(os.Stdin), tea.WithOutput(os.Stderr))
	out, err := program.Run()
	if err != nil {
		return fallbackConsentChoice(rows)
	}
	final, ok := out.(consentChoiceModel)
	if !ok || final.choice == "" {
		return choiceAllowOnce
	}
	return final.choice
}

func (m consentChoiceModel) Init() tea.Cmd {
	return nil
}

func (m consentChoiceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.choice = choiceDeny
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter":
			m.choice = m.options[m.cursor].key
			return m, tea.Quit
		case choiceAllowOnce, choiceAllowForSession, choiceAlwaysAllow, choiceDeny:
			m.choice = msg.String()
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m consentChoiceModel) View() string {
	var lines []string
	lines = append(lines, m.rows...)
	lines = append(lines, "")
	for i, option := range m.options {
		cursor := consentNoCursor
		label := fmt.Sprintf("[%s] %s", option.key, option.label)
		if i == m.cursor {
			cursor = consentCursor
			label = consentSelectedStyle.Render(label)
		} else {
			label = consentChoiceStyle.Render(label)
		}
		lines = append(lines, fmt.Sprintf("%s %s  %s", cursor, label, consentHelpStyle.Render(option.description)))
	}
	lines = append(lines, "", consentHelpStyle.Render("Use Up/Down and Enter, or press a/s/p/d. Esc denies."))
	return consentPanelStyle.Render(strings.Join(lines, "\n"))
}

func fallbackConsentChoice(rows []string) string {
	fmt.Fprintln(os.Stderr, consentPanelStyle.Render(strings.Join(rows, "\n")))
	fmt.Fprint(os.Stderr, consentChoiceStyle.Render("[a] Allow once  [s] Allow session  [p] Always allow  [d] Deny: "))
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(strings.ToLower(input))
	switch choice {
	case choiceAllowForSession, choiceAlwaysAllow, choiceDeny:
		return choice
	default:
		return choiceAllowOnce
	}
}

func (m *Manager) readPattern(toolName string, args map[string]interface{}) string {
	defaultValue := defaultPattern(toolName, args)
	fmt.Fprintf(os.Stderr, consentChoiceStyle.Render(msgConsentPattern), defaultValue)
	input, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	pattern := strings.TrimSpace(input)
	if pattern == "" {
		return defaultValue
	}
	return pattern
}

func (m *Manager) promptSaveScope() string {
	fmt.Fprint(os.Stderr, consentChoiceStyle.Render("Save to [p]roject / [u]ser (default: project): "))
	input, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if strings.TrimSpace(strings.ToLower(input)) == "u" {
		return "u"
	}
	return "p"
}

func renderConsentKV(label, value string) string {
	return consentLabelStyle.Render(label) + " " + consentValueStyle.Render(value)
}

func renderStatus(text string) string {
	return consentPanelStyle.Render(text) + "\n"
}

func isBlockedCall(toolName string, args map[string]interface{}) (bool, string) {
	if toolName == c.ToolReadFile {
		path, _ := args[c.ArgPath].(string)
		if path == "" || isPathBlocked(path) {
			return true, path
		}
	}

	if toolName == c.ToolGetExternalContext {
		system := externalSystem(args)
		if !c.AllowedExternalSystems[system] {
			return true, system
		}
	}

	return false, ""
}

func isPathBlocked(path string) bool {
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return true
	}
	return c.IsSensitivePath(path)
}

func allowedBy(rules []Rule, toolName string, args map[string]interface{}) bool {
	for _, rule := range rules {
		if rule.Tool == toolName && patternMatches(rule.Pattern, toolName, args) {
			return true
		}
	}
	return false
}

func patternMatches(pattern, toolName string, args map[string]interface{}) bool {
	if pattern == "" || pattern == patternAll {
		return true
	}
	if toolName == c.ToolReadFile && strings.HasPrefix(pattern, patternGlobPrefix) {
		path, _ := args[c.ArgPath].(string)
		return globMatches(strings.TrimPrefix(pattern, patternGlobPrefix), filepath.ToSlash(path))
	}
	if toolName == c.ToolGetExternalContext && strings.HasPrefix(pattern, patternSystemPrefix) {
		return externalSystem(args) == strings.TrimPrefix(pattern, patternSystemPrefix)
	}
	return pattern == defaultPattern(toolName, args)
}

func globMatches(pattern, path string) bool {
	pattern = filepath.ToSlash(pattern)
	if strings.HasSuffix(pattern, recursiveGlobSuffix) {
		prefix := strings.TrimSuffix(pattern, recursiveGlobSuffix)
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	ok, err := filepath.Match(pattern, path)
	return err == nil && ok
}

func defaultPattern(toolName string, args map[string]interface{}) string {
	switch toolName {
	case c.ToolReadFile:
		path, _ := args[c.ArgPath].(string)
		return patternGlobPrefix + filepath.ToSlash(path)
	case c.ToolGetExternalContext:
		return patternSystemPrefix + externalSystem(args)
	default:
		return patternAll
	}
}

func externalSystem(args map[string]interface{}) string {
	system, _ := args[c.ArgSystem].(string)
	system = strings.ToLower(strings.TrimSpace(system))
	if system == "" {
		return defaultExternalSystem
	}
	return system
}

func loadRules(path string) []Rule {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var file consentFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil
	}
	return file.Allowed
}

func saveRules(path string, rules []Rule) error {
	if err := os.MkdirAll(filepath.Dir(path), consentDirPermission); err != nil {
		return err
	}
	data, err := yaml.Marshal(consentFile{Allowed: rules})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, consentFilePermission)
}
