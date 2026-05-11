package config

import (
	"bufio"
	"os"
	"strings"
)

const (
	envCommentPrefix  = "#"
	envKeyValueSep    = "="
	envKeyValueParts  = 2
	envQuoteTrimChars = `"'`
)

func LoadEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
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
		val := strings.Trim(strings.TrimSpace(parts[1]), envQuoteTrimChars)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
