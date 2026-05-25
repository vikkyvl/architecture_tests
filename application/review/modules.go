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
		if e.ToolName != c.ToolReadFile || e.Error != "" {
			continue
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(e.Arguments), &args); err != nil {
			continue
		}
		path, ok := args[c.ArgPath].(string)
		if ok && path != "" && !seen[path] {
			seen[path] = true
			result = append(result, path)
		}
	}
	return result
}

func skippedModules(incomplete bool, analyzed []string, sourceFiles []string) []string {
	if !incomplete {
		return nil
	}
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
