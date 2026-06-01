package review

import (
	"encoding/json"

	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/models"
)

func extractAnalyzedModules(entries []models.AuditEntry) []string {
	seen := make(map[string]bool)
	var result []string
	for _, e := range entries {
		if e.Error != "" {
			continue
		}
		var argKey string
		switch e.ToolName {
		case c.ToolReadFile:
			argKey = c.ArgPath
		case c.ToolGetClassDependencies:
			argKey = c.ArgClass
		default:
			continue
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(e.Arguments), &args); err != nil {
			continue
		}
		val, ok := args[argKey].(string)
		if ok && val != "" && !seen[val] {
			seen[val] = true
			result = append(result, val)
		}
	}
	return result
}

func skippedModules(_ bool, analyzed []string, sourceFiles []string) []string {
	analyzedSet := make(map[string]bool, len(analyzed))
	for _, f := range analyzed {
		analyzedSet[f] = true
	}
	var skipped []string
	for _, f := range sourceFiles {
		if !analyzedSet[f] {
			skipped = append(skipped, f)
		}
	}
	return skipped
}
