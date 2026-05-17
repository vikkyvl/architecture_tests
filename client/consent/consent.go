package consent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	c "github.com/archguard/project/shared/constants"
	"gopkg.in/yaml.v3"
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
)

type Decision = string

type Manager struct {
	sessionAllowed []Rule
	projectAllowed []Rule
	userAllowed    []Rule
	projectPath    string
	interactive    bool
	allowedSystems map[string]bool
}

type Rule struct {
	Tool    string `yaml:"tool"`
	Pattern string `yaml:"pattern"`
}

type consentFile struct {
	Allowed []Rule `yaml:"allowed"`
}

func NewManager(interactive bool, projectPath string, allowedSystems map[string]bool) *Manager {
	m := &Manager{
		interactive:    interactive,
		projectPath:    projectPath,
		allowedSystems: allowedSystems,
	}
	m.projectAllowed = loadRules(filepath.Join(projectPath, projectConsentPath))
	if home, err := os.UserHomeDir(); err == nil {
		m.userAllowed = loadRules(filepath.Join(home, userConsentPath))
	}
	return m
}

func (m *Manager) Check(toolName string, args map[string]interface{}, budget int) Decision {
	if blocked, value := m.isBlockedCall(toolName, args); blocked {
		fmt.Fprint(os.Stderr, renderStatus(consentDangerStyle.Render(fmt.Sprintf(msgBlockedTool, toolName, value))))
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
		fmt.Fprint(os.Stderr, renderStatus(consentDangerStyle.Render(fmt.Sprintf(msgNonInteractiveDeny, toolName))))
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

	fmt.Fprintln(os.Stderr)
	choice := runConsentMenu(rows)

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
		_ = saveRules(filepath.Join(m.projectPath, projectConsentPath), m.projectAllowed)
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
	fmt.Fprintln(os.Stderr)
	return runPatternConfirmMenu(rows) == choicePatternYes
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
	data, err := yaml.Marshal(consentFile{
		Allowed: rules,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, consentFilePermission)
}
