package constants

import (
	"path/filepath"
	"strings"
)

var SkipDirs = []string{
	"node_modules", "vendor", ".git", "__pycache__",
	"build", "dist", "var", "cache", ".idea", ".vscode",
}

var BlacklistPaths = []string{
	".env", "*.pem", "*.key", "secrets/**",
}

func IsSensitivePath(path string) bool {
	normalized := filepath.ToSlash(filepath.Clean(path))
	if strings.HasPrefix(filepath.Base(normalized), ".") {
		return true
	}
	for _, pattern := range BlacklistPaths {
		trimmed := strings.TrimSuffix(filepath.ToSlash(pattern), "/**")
		if strings.Contains(normalized, trimmed) {
			return true
		}
		if strings.HasPrefix(pattern, "*.") {
			if strings.HasSuffix(normalized, strings.TrimPrefix(pattern, "*")) {
				return true
			}
		}
	}
	return false
}
