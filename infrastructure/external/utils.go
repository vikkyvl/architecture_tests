package external

import (
	"fmt"
	"io"
	"os"

	c "github.com/archguard/project/shared/constants"
)

func buildEnv(envMap map[string]string) []string {
	base := os.Environ()
	for k, v := range envMap {
		base = append(base, k+"="+os.ExpandEnv(v))
	}
	return base
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func subprocessStderr(tail io.Writer) io.Writer {
	if c.IsEnvTrue(envMCPStderrPassthrough) {
		return io.MultiWriter(os.Stderr, tail)
	}
	return tail
}

func (c *MCPClient) withStderrTail(base string) string {
	tail := c.stderr.String()
	if tail == "" {
		return base
	}
	return base + fmt.Sprintf(mcpStderrTailSuffix, tail)
}
