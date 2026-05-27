package consent

import (
	"os"
	"path/filepath"
	"strings"

	c "github.com/archguard/project/shared/constants"
	"gopkg.in/yaml.v3"
)

type consentFile struct {
	Allowed []Rule `yaml:"allowed"`
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

func isPathBlocked(path string) bool {
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return true
	}
	return c.IsSensitivePath(path)
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
