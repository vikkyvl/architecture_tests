package llm

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/httpclient"
)

const (
	errParseOpenAI       = "failed to parse openai response"
	openAIFinishToolCall = "tool_calls"
	openAIRoleTool       = "tool"
	openAITypeFunction   = "function"
)

type oRequest struct {
	Model     string     `json:"model"`
	MaxTokens int        `json:"max_tokens,omitempty"`
	Messages  []oMessage `json:"messages"`
	Tools     []oTool    `json:"tools,omitempty"`
}

type oMessage struct {
	Role       string      `json:"role"`
	Content    string      `json:"content,omitempty"`
	ToolCalls  []oToolCall `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

type oToolCall struct {
	ID       string    `json:"id"`
	Type     string    `json:"type"`
	Function oFunction `json:"function"`
}

type oFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type oTool struct {
	Type     string    `json:"type"`
	Function oFuncDecl `json:"function"`
}

type oFuncDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type oResponse struct {
	Choices []struct {
		Message struct {
			Role      string      `json:"role"`
			Content   *string     `json:"content"`
			ToolCalls []oToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

type OpenAIClient struct {
	apiKey string
	model  string
	http   *httpclient.Client
}

func NewOpenAIClient(apiKey, model string) (*OpenAIClient, error) {
	return NewOpenAIClientWithRateLimitHook(apiKey, model, nil)
}

func NewOpenAIClientWithRateLimitHook(apiKey, model string, rateLimitHook func(time.Duration)) (*OpenAIClient, error) {
	cfg, err := newProviderClientConfig(
		apiKey, model, c.EnvOpenAIKey, c.OpenAIModel,
		c.OpenAITimeout, c.OpenAIMaxRetries, c.OpenAIRetryWait,
		rateLimitHook,
	)
	if err != nil {
		return nil, err
	}
	return &OpenAIClient{
		apiKey: cfg.apiKey,
		model:  cfg.model,
		http:   cfg.http,
	}, nil
}

func (o *OpenAIClient) Name() string {
	return c.ProviderOpenAI
}

func (o *OpenAIClient) Model() string {
	return o.model
}

func (o *OpenAIClient) SendMessage(req Request) (*Response, error) {
	or := o.toOpenAI(req)
	body, err := json.Marshal(or)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.KindInternal, operationOpenAIRequest, errMarshalRequest, err)
	}

	resp, err := o.http.Post(httpclient.RequestConfig{
		URL:  c.OpenAIBaseURL,
		Body: body,
		Headers: map[string]string{
			c.HTTPHeaderContentType:   c.HTTPContentTypeJSON,
			c.HTTPHeaderAuthorization: c.HTTPAuthBearer + o.apiKey,
		},
	})
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, providerStatusError(c.ProviderOpenAI, resp.StatusCode, resp.Body)
	}

	var out oResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, apperrors.Wrap(apperrors.KindExternalService, operationOpenAIResp, errParseOpenAI, err)
	}
	return o.fromOpenAI(out), nil
}

func (o *OpenAIClient) toOpenAI(req Request) oRequest {
	or := oRequest{
		Model:     o.model,
		MaxTokens: req.MaxTokens,
	}

	if req.System != "" {
		or.Messages = append(or.Messages, oMessage{
			Role:    c.RoleSystem,
			Content: req.System,
		})
	}

	for _, t := range req.Tools {
		or.Tools = append(or.Tools, oTool{
			Type: openAITypeFunction,
			Function: oFuncDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	for _, msg := range req.Messages {
		switch msg.Role {
		case c.RoleAssistant:
			or.Messages = append(or.Messages, o.assistantMessage(msg))
		default:
			or.Messages = append(or.Messages, o.userMessages(msg)...)
		}
	}

	return or
}

func (o *OpenAIClient) assistantMessage(msg Message) oMessage {
	m := oMessage{
		Role: c.RoleAssistant,
	}
	for _, b := range msg.Content {
		switch b.Type {
		case c.ContentTypeText:
			m.Content = b.Text
		case c.ContentTypeToolUse:
			args := string(b.Input)
			if args == "" {
				args = c.EmptyJSONObject
			}
			m.ToolCalls = append(m.ToolCalls, oToolCall{
				ID:   b.ID,
				Type: openAITypeFunction,
				Function: oFunction{
					Name:      b.Name,
					Arguments: args,
				},
			})
		}
	}
	return m
}

func (o *OpenAIClient) userMessages(msg Message) []oMessage {
	var out []oMessage
	var textParts string

	for _, b := range msg.Content {
		switch b.Type {
		case c.ContentTypeToolResult:
			if textParts != "" {
				out = append(out, oMessage{
					Role:    c.RoleUser,
					Content: textParts,
				})
				textParts = ""
			}
			out = append(out, oMessage{
				Role:       openAIRoleTool,
				ToolCallID: b.ToolUseID,
				Content:    b.Content,
			})
		case c.ContentTypeText:
			textParts += b.Text
		}
	}

	if textParts != "" {
		out = append(out, oMessage{
			Role:    c.RoleUser,
			Content: textParts,
		})
	}
	return out
}

func (o *OpenAIClient) fromOpenAI(or oResponse) *Response {
	resp := &Response{
		Role:       c.RoleAssistant,
		StopReason: c.StopReasonEndTurn,
	}
	if len(or.Choices) == 0 {
		return resp
	}

	choice := or.Choices[0]
	msg := choice.Message

	if msg.Content != nil && *msg.Content != "" {
		resp.Content = append(resp.Content, ContentBlock{
			Type: c.ContentTypeText,
			Text: *msg.Content,
		})
	}

	for _, tc := range msg.ToolCalls {
		args := json.RawMessage(tc.Function.Arguments)
		if len(args) == 0 {
			args = json.RawMessage(c.EmptyJSONObject)
		}
		resp.Content = append(resp.Content, ContentBlock{
			Type:  c.ContentTypeToolUse,
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: args,
		})
	}

	if choice.FinishReason == openAIFinishToolCall {
		resp.StopReason = c.StopReasonToolUse
	}

	return resp
}
