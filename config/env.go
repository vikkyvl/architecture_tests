package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	envCommentPrefix = "#"
	envKeyValueSep   = "="
	envKeyValueParts = 2
	warnEnvOpen      = "warning: could not open %s: %v\n"
	warnEnvScan      = "warning: error reading %s: %v\n"
)

func LoadEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, warnEnvOpen, path, err)
		}
		return
	}
	defer file.Close()

	s := bufio.NewScanner(file)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, envCommentPrefix) {
			continue
		}
		parts := strings.SplitN(line, envKeyValueSep, envKeyValueParts)
		if len(parts) != envKeyValueParts {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := trimQuotePair(strings.TrimSpace(parts[1]))
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
	if err := s.Err(); err != nil {
		fmt.Fprintf(os.Stderr, warnEnvScan, path, err)
	}
}

func trimQuotePair(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
