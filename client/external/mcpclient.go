package external

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/archguard/project/shared/apperrors"
	"github.com/archguard/project/shared/config"
)

const (
	mcpProtocolVersion  = "2024-11-05"
	mcpClientName       = "archguard"
	mcpClientVersion    = "0.1.0"
	mcpContentTypeText  = "text"
	errMCPNoCommand     = "mcp server %q: command is required"
	errMCPStart         = "failed to start mcp server %q"
	errMCPInit          = "failed to initialize mcp server %q"
	errMCPWrite         = "failed to write to mcp server"
	errMCPRead          = "failed to read from mcp server"
	errMCPClosed        = "mcp server closed connection"
	errMCPParseResponse = "failed to parse mcp response"
	errMCPToolCall      = "mcp tool call failed"
)

type mcpRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      *int64      `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type MCPClient struct {
	cfg      config.ExternalConfig
	once     sync.Once
	startErr error
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	scanner  *bufio.Scanner
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
		return "", apperrors.ExternalService(fmt.Sprintf("mcp tool %q not found; available tools: %s", c.cfg.SearchTool, strings.Join(c.tools, ", ")))
	}
	result, err := c.callTool(c.cfg.SearchTool, map[string]interface{}{
		c.cfg.SearchArg: query,
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
		return "", apperrors.Wrap(apperrors.KindExternalService, "mcp", errMCPToolCall, err)
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
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return apperrors.Wrap(apperrors.KindInternal, "mcp", fmt.Sprintf(errMCPStart, c.cfg.System), err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return apperrors.Wrap(apperrors.KindInternal, "mcp", fmt.Sprintf(errMCPStart, c.cfg.System), err)
	}
	if err := cmd.Start(); err != nil {
		return apperrors.Wrap(apperrors.KindExternalService, "mcp", fmt.Sprintf(errMCPStart, c.cfg.System), err)
	}
	c.cmd = cmd
	c.stdin = stdin
	c.scanner = bufio.NewScanner(stdout)

	if err := c.initialize(); err != nil {
		c.stdin.Close()
		c.cmd.Process.Kill()
		return apperrors.Wrap(apperrors.KindExternalService, "mcp", fmt.Sprintf(errMCPInit, c.cfg.System), err)
	}
	if tools, err := c.listTools(); err == nil {
		c.tools = tools
	}
	return nil
}

func (c *MCPClient) initialize() error {
	_, err := c.rpc("initialize", map[string]interface{}{
		"protocolVersion": mcpProtocolVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    mcpClientName,
			"version": mcpClientVersion,
		},
	})
	if err != nil {
		return err
	}
	return c.notify("notifications/initialized", nil)
}

func (c *MCPClient) callTool(tool string, arguments map[string]interface{}) (json.RawMessage, error) {
	return c.rpc("tools/call", map[string]interface{}{
		"name":      tool,
		"arguments": arguments,
	})
}

func (c *MCPClient) listTools() ([]string, error) {
	result, err := c.rpc("tools/list", nil)
	if err != nil {
		return nil, err
	}
	var list struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &list); err != nil {
		return nil, apperrors.Wrap(apperrors.KindExternalService, "mcp", errMCPParseResponse, err)
	}
	tools := make([]string, 0, len(list.Tools))
	for _, tool := range list.Tools {
		if tool.Name != "" {
			tools = append(tools, tool.Name)
		}
	}
	return tools, nil
}

func (c *MCPClient) rpc(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nextID++
	id := c.nextID
	req := mcpRequest{JSONRPC: "2.0", ID: &id, Method: method, Params: params}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.KindInternal, "mcp", "failed to marshal request", err)
	}
	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		return nil, apperrors.Wrap(apperrors.KindExternalService, "mcp", errMCPWrite, err)
	}
	if !c.scanner.Scan() {
		if scanErr := c.scanner.Err(); scanErr != nil {
			return nil, apperrors.Wrap(apperrors.KindExternalService, "mcp", errMCPRead, scanErr)
		}
		return nil, apperrors.ExternalService(errMCPClosed)
	}
	var resp mcpResponse
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		return nil, apperrors.Wrap(apperrors.KindExternalService, "mcp", errMCPParseResponse, err)
	}
	if resp.Error != nil {
		return nil, apperrors.ExternalService(fmt.Sprintf("mcp error %d: %s", resp.Error.Code, resp.Error.Message))
	}
	return resp.Result, nil
}

func (c *MCPClient) notify(method string, params interface{}) error {
	n := mcpRequest{JSONRPC: "2.0", Method: method, Params: params}
	data, err := json.Marshal(n)
	if err != nil {
		return apperrors.Wrap(apperrors.KindInternal, "mcp", "failed to marshal notification", err)
	}
	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		return apperrors.Wrap(apperrors.KindExternalService, "mcp", errMCPWrite, err)
	}
	return nil
}

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
