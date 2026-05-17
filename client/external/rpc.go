package external

import (
	"encoding/json"
	"fmt"

	"github.com/archguard/project/shared/apperrors"
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

func (c *MCPClient) initialize() error {
	capabilities := make(map[string]interface{})
	_, err := c.rpc(mcpMethodInitialize, map[string]interface{}{
		mcpParamProtocolVersion: mcpProtocolVersion,
		mcpParamCapabilities:    capabilities,
		mcpParamClientInfo: map[string]interface{}{
			mcpParamName:    mcpClientName,
			mcpParamVersion: mcpClientVersion,
		},
	})
	if err != nil {
		return err
	}
	return c.notify(mcpMethodInitialized, nil)
}

func (c *MCPClient) callTool(tool string, arguments map[string]interface{}) (json.RawMessage, error) {
	return c.rpc(mcpMethodToolsCall, map[string]interface{}{
		mcpParamName:      tool,
		mcpParamArguments: arguments,
	})
}

func (c *MCPClient) listTools() ([]string, error) {
	result, err := c.rpc(mcpMethodToolsList, nil)
	if err != nil {
		return nil, err
	}
	var list struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &list); err != nil {
		return nil, apperrors.Wrap(apperrors.KindExternalService, mcpOperation, errMCPParseResponse, err)
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
	req := mcpRequest{
		JSONRPC: mcpJSONRPCVersion,
		ID:      &id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.KindInternal, mcpOperation, errMCPMarshalRequest, err)
	}
	if _, err := fmt.Fprintf(c.stdin, mcpWriteLineFormat, data); err != nil {
		return nil, apperrors.Wrap(apperrors.KindExternalService, mcpOperation, errMCPWrite, err)
	}
	if !c.scanner.Scan() {
		if scanErr := c.scanner.Err(); scanErr != nil {
			return nil, apperrors.Wrap(apperrors.KindExternalService, mcpOperation, c.withStderrTail(errMCPRead), scanErr)
		}
		return nil, apperrors.ExternalService(c.withStderrTail(errMCPClosed))
	}
	var resp mcpResponse
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		return nil, apperrors.Wrap(apperrors.KindExternalService, mcpOperation, errMCPParseResponse, err)
	}
	if resp.Error != nil {
		return nil, apperrors.ExternalService(fmt.Sprintf(errMCPErrorFormat, resp.Error.Code, resp.Error.Message))
	}
	return resp.Result, nil
}

func (c *MCPClient) notify(method string, params interface{}) error {
	n := mcpRequest{
		JSONRPC: mcpJSONRPCVersion,
		Method:  method,
		Params:  params,
	}
	data, err := json.Marshal(n)
	if err != nil {
		return apperrors.Wrap(apperrors.KindInternal, mcpOperation, errMCPMarshalNotify, err)
	}
	if _, err := fmt.Fprintf(c.stdin, mcpWriteLineFormat, data); err != nil {
		return apperrors.Wrap(apperrors.KindExternalService, mcpOperation, errMCPWrite, err)
	}
	return nil
}
