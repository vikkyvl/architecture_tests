package external

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/archguard/project/config"
	"github.com/archguard/project/shared/apperrors"
)

type MCPClient struct {
	cfg      config.ExternalConfig
	once     sync.Once
	startErr error
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	scanner  *bufio.Scanner
	stderr   *stderrTail
	mu       sync.Mutex
	nextID   int64
	tools    []string
}

func NewMCPClient(cfg config.ExternalConfig) *MCPClient {
	return &MCPClient{cfg: cfg}
}

func (c *MCPClient) Search(query string) (string, error) {
	if err := c.ensureStarted(); err != nil {
		return "", err
	}
	if len(c.tools) > 0 && !containsString(c.tools, c.cfg.SearchTool) {
		return "", apperrors.ExternalService(fmt.Sprintf(errMCPToolMissing, c.cfg.SearchTool, strings.Join(c.tools, mcpToolListSeparator)))
	}
	result, err := c.callTool(c.cfg.SearchTool, map[string]interface{}{
		c.cfg.SearchArg: query,
	})
	if err != nil {
		return "", err
	}
	var toolResult struct {
		Content []contentBlock `json:"content"`
	}
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return "", apperrors.Wrap(apperrors.KindExternalService, mcpOperation, errMCPToolCall, err)
	}
	var sb strings.Builder
	for _, block := range filterBlocks(toolResult.Content, c.cfg.Include, c.cfg.Exclude) {
		sb.WriteString(block.Text)
	}
	return sb.String(), nil
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func BuildQuery(query, template, filter string) string {
	switch {
	case template == "" && filter == "":
		return query
	case template == "":
		return filter + " " + query
	default:
		n := strings.Count(template, "%s")
		switch n {
		case 0:
			return template
		case 1:
			return fmt.Sprintf(template, query)
		default:
			return fmt.Sprintf(template, query, filter)
		}
	}
}

func filterBlocks(blocks []contentBlock, include, exclude []string) []contentBlock {
	if len(include) == 0 && len(exclude) == 0 {
		return blocks
	}
	out := make([]contentBlock, 0, len(blocks))
	for _, b := range blocks {
		if b.Type != mcpContentTypeText {
			continue
		}
		if len(include) > 0 && !matchesAny(b.Text, include) {
			continue
		}
		if matchesAny(b.Text, exclude) {
			continue
		}
		out = append(out, b)
	}
	return out
}

func matchesAny(text string, patterns []string) bool {
	for _, p := range patterns {
		if strings.ContainsAny(p, "*?[") {
			if ok, _ := filepath.Match(p, text); ok {
				return true
			}
			for _, line := range strings.Split(text, "\n") {
				if ok, _ := filepath.Match(p, strings.TrimSpace(line)); ok {
					return true
				}
			}
		} else {
			if strings.Contains(text, p) {
				return true
			}
		}
	}
	return false
}

func (c *MCPClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Wait()
	}
}
