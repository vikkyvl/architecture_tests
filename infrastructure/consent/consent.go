package consent

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	c "github.com/archguard/project/shared/constants"
)

const (
	msgBlockedTool        = "BLOCKED %s %s\n"
	msgConsentTool        = "Tool"
	msgConsentBudget      = "Budget"
	msgConsentPattern     = "Pattern"
	msgConsentRequired    = "Consent required"
	msgCallsRemaining     = "%d calls remaining"
	formatArgValue        = "%v"
	choiceAllowOnce       = "a"
	choiceAllowForSession = "s"
	choiceAlwaysAllow     = "p"
	choiceDeny            = "d"
	choicePatternYes      = "y"
	choicePatternNo       = "n"
	choiceInterrupt       = "ctrl+c"
	recursiveGlobSuffix   = "/**"
	emptyConsentLine      = ""
	projectConsentPath    = ".archguard/consent.yaml"
	userConsentPath       = ".config/archguard/consent.yaml"
	consentFilePermission = 0o600
	consentDirPermission  = 0o700
	patternAll            = "all"
	patternGlobPrefix     = "glob:"
	patternSystemPrefix   = "system:"
	defaultExternalSystem = "notion"
	msgNonInteractiveDeny = "DENIED %s requires consent; run interactively or add a whitelist rule\n"
	msgConsentSaveFailed  = "warning: failed to persist consent rule: %v\n"
)

type Decision = string

type Manager struct {
	sessionAllowed []Rule
	projectAllowed []Rule
	userAllowed    []Rule
	projectPath    string
	interactive    bool
	allowedSystems map[string]bool
	out            io.Writer
}

type Rule struct {
	Tool    string `yaml:"tool"`
	Pattern string `yaml:"pattern"`
}

func NewManager(interactive bool, projectPath string, allowedSystems map[string]bool, out io.Writer) *Manager {
	m := &Manager{
		interactive:    interactive,
		projectPath:    projectPath,
		allowedSystems: allowedSystems,
		out:            out,
	}
	m.projectAllowed = loadRules(filepath.Join(projectPath, projectConsentPath))
	if home, err := os.UserHomeDir(); err == nil {
		m.userAllowed = loadRules(filepath.Join(home, userConsentPath))
	}
	return m
}

func (m *Manager) writer() io.Writer {
	if m.out != nil {
		return m.out
	}
	return os.Stderr
}

func (m *Manager) Check(toolName string, args map[string]interface{}, budget int) Decision {
	if blocked, value := m.isBlockedCall(toolName, args); blocked {
		fmt.Fprint(m.writer(), renderStatus(consentDangerStyle.Render(fmt.Sprintf(msgBlockedTool, toolName, value))))
		return c.DecisionBlocked
	}

	if c.AutoApprovedTools[toolName] {
		return c.DecisionAutoApproved
	}

	if toolName == c.ToolGetExternalContext && m.allowedSystems[externalSystem(args)] {
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
		fmt.Fprint(m.writer(), renderStatus(consentDangerStyle.Render(fmt.Sprintf(msgNonInteractiveDeny, toolName))))
		return c.DecisionUserDenied
	}

	return m.prompt(toolName, args, budget)
}

func (m *Manager) prompt(toolName string, args map[string]interface{}, budget int) Decision {
	rows := []string{
		consentTitleStyle.Render(msgConsentRequired),
		emptyConsentLine,
		renderConsentKV(msgConsentTool, toolName),
	}
	argKeys := make([]string, 0, len(args))
	for k := range args {
		argKeys = append(argKeys, k)
	}
	sort.Strings(argKeys)
	for _, k := range argKeys {
		rows = append(rows, renderConsentKV(k, fmt.Sprintf(formatArgValue, args[k])))
	}
	rows = append(rows, renderConsentKV(msgConsentBudget, fmt.Sprintf(msgCallsRemaining, budget)))

	fmt.Fprintln(m.writer())
	choice := runConsentMenu(rows, m.writer())

	switch choice {
	case choiceAllowForSession:
		m.sessionAllowed = append(m.sessionAllowed, Rule{
			Tool:    toolName,
			Pattern: patternAll,
		})
		return c.DecisionUserAllowed
	case choiceAlwaysAllow:
		if !m.confirmDefaultPattern(toolName, args) {
			return c.DecisionUserAllowed
		}
		rule := Rule{
			Tool:    toolName,
			Pattern: defaultPattern(toolName, args),
		}
		m.projectAllowed = append(m.projectAllowed, rule)
		if err := saveRules(filepath.Join(m.projectPath, projectConsentPath), m.projectAllowed); err != nil {
			fmt.Fprintf(m.writer(), msgConsentSaveFailed, err)
		}
		return c.DecisionProjectAllow
	case choiceDeny:
		return c.DecisionUserDenied
	default:
		return c.DecisionUserAllowed
	}
}

func (m *Manager) confirmDefaultPattern(toolName string, args map[string]interface{}) bool {
	rows := []string{
		consentTitleStyle.Render(patternMenuTitle),
		"",
		renderConsentKV(msgConsentTool, toolName),
		renderConsentKV(msgConsentPattern, defaultPattern(toolName, args)),
		"",
		consentHelpStyle.Render(patternMenuFooterHint),
	}
	fmt.Fprintln(m.writer())
	return runPatternConfirmMenu(rows, m.writer()) == choicePatternYes
}

func (m *Manager) isBlockedCall(toolName string, args map[string]interface{}) (bool, string) {
	if toolName == c.ToolReadFile {
		path, _ := args[c.ArgPath].(string)
		if path == "" || isPathBlocked(path) {
			return true, path
		}
	}

	if toolName == c.ToolGetExternalContext {
		system := externalSystem(args)
		if !m.allowedSystems[system] {
			return true, system
		}
	}

	return false, ""
}
