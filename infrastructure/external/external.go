package external

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
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
	value := query
	if c.cfg.QueryTemplate != "" {
		value = fmt.Sprintf(c.cfg.QueryTemplate, query)
	}
	result, err := c.callTool(c.cfg.SearchTool, map[string]interface{}{
		c.cfg.SearchArg: value,
	})
	if err != nil {
		return "", err
	}
	var toolResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return "", apperrors.Wrap(apperrors.KindExternalService, mcpOperation, errMCPToolCall, err)
	}
	var sb strings.Builder
	for _, block := range toolResult.Content {
		if block.Type == mcpContentTypeText {
			sb.WriteString(block.Text)
		}
	}
	return sb.String(), nil
}

func (c *MCPClient) Close() {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Wait()
	}
}
