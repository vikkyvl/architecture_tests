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

const (
	mcpProtocolVersion      = "2024-11-05"
	mcpClientName           = "archguard"
	mcpClientVersion        = "0.1.0"
	mcpContentTypeText      = "text"
	mcpJSONRPCVersion       = "2.0"
	mcpOperation            = "mcp"
	mcpParamProtocolVersion = "protocolVersion"
	mcpParamCapabilities    = "capabilities"
	mcpParamClientInfo      = "clientInfo"
	mcpParamName            = "name"
	mcpParamVersion         = "version"
	mcpParamArguments       = "arguments"
	mcpMethodInitialize     = "initialize"
	mcpMethodInitialized    = "notifications/initialized"
	mcpMethodToolsCall      = "tools/call"
	mcpMethodToolsList      = "tools/list"
	mcpScannerMaxBytes      = 16 * 1024 * 1024
	mcpStderrTailBytes      = 4 * 1024
	envMCPStderrPassthrough = "ARCHGUARD_MCP_STDERR"
	errMCPNoCommand         = "mcp server %q: command is required"
	errMCPStart             = "failed to start mcp server %q"
	errMCPInit              = "failed to initialize mcp server %q"
	errMCPWrite             = "failed to write to mcp server"
	errMCPRead              = "failed to read from mcp server"
	errMCPClosed            = "mcp server closed connection"
	errMCPParseResponse     = "failed to parse mcp response"
	errMCPToolCall          = "mcp tool call failed"
	errMCPMarshalRequest    = "failed to marshal request"
	errMCPMarshalNotify     = "failed to marshal notification"
	errMCPToolMissing       = "mcp tool %q not found; available tools: %s"
	errMCPErrorFormat       = "mcp error %d: %s"
	mcpStderrTailSuffix     = " (server stderr tail: %s)"
	mcpToolListSeparator    = ", "
	mcpWriteLineFormat      = "%s\n"
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
	return &MCPClient{
		cfg: cfg,
	}
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

func (c *MCPClient) ensureStarted() error {
	c.once.Do(func() {
		c.startErr = c.start()
	})
	return c.startErr
}

func (c *MCPClient) start() error {
	if len(c.cfg.Command) == 0 {
		return apperrors.Validation(fmt.Sprintf(errMCPNoCommand, c.cfg.System))
	}
	cmd := exec.Command(c.cfg.Command[0], c.cfg.Command[1:]...)
	cmd.Env = buildEnv(c.cfg.Env)
	c.stderr = newStderrTail(mcpStderrTailBytes)
	cmd.Stderr = subprocessStderr(c.stderr)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return apperrors.Wrap(apperrors.KindInternal, mcpOperation, fmt.Sprintf(errMCPStart, c.cfg.System), err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return apperrors.Wrap(apperrors.KindInternal, mcpOperation, fmt.Sprintf(errMCPStart, c.cfg.System), err)
	}
	if err := cmd.Start(); err != nil {
		return apperrors.Wrap(apperrors.KindExternalService, mcpOperation, fmt.Sprintf(errMCPStart, c.cfg.System), err)
	}
	c.cmd = cmd
	c.stdin = stdin
	c.scanner = bufio.NewScanner(stdout)
	c.scanner.Buffer(make([]byte, 0, 64*1024), mcpScannerMaxBytes)

	if err := c.initialize(); err != nil {
		c.stdin.Close()
		c.cmd.Process.Kill()
		return apperrors.Wrap(apperrors.KindExternalService, mcpOperation, c.withStderrTail(fmt.Sprintf(errMCPInit, c.cfg.System)), err)
	}
	if tools, err := c.listTools(); err == nil {
		c.tools = tools
	}
	return nil
}
