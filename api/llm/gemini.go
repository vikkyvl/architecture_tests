package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/httpclient"
)

const (
	geminiGenerateURLFormat = "%s/%s:generateContent?key=%s"
	geminiToolResultKey     = "result"
	geminiCallIDFormat      = "call_%s_%d"
	errParseGemini          = "failed to parse gemini response"
)

type GeminiClient struct {
	apiKey string
	model  string
	http   *httpclient.Client
}

func NewGeminiClient(apiKey, model string) (*GeminiClient, error) {
	return NewGeminiClientWithRateLimitHook(apiKey, model, nil)
}

func NewGeminiClientWithRateLimitHook(apiKey, model string, rateLimitHook func(time.Duration)) (*GeminiClient, error) {
	if apiKey == "" {
		apiKey = os.Getenv(c.EnvGeminiKey)
	}
	if apiKey == "" {
		return nil, apperrors.Validation(fmt.Sprintf(errAPIKeyRequired, c.EnvGeminiKey))
	}
	if model == "" {
		model = c.GeminiModel
	}
	return &GeminiClient{
		apiKey: apiKey, model: model,
		http: httpclient.NewWithRateLimitHook(c.GeminiTimeout, c.GeminiMaxRetries, c.GeminiRetryWait, rateLimitHook),
	}, nil
}

func (g *GeminiClient) Name() string  { return c.ProviderGemini }
func (g *GeminiClient) Model() string { return g.model }

func (g *GeminiClient) SendMessage(req Request) (*Response, error) {
	gr := g.toGemini(req)
	body, err := json.Marshal(gr)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.KindInternal, "gemini request", "failed to marshal request", err)
	}
	url := fmt.Sprintf(geminiGenerateURLFormat, c.GeminiBaseURL, g.model, g.apiKey)

	resp, err := g.http.Post(httpclient.RequestConfig{
		URL:     url,
		Body:    body,
		Headers: map[string]string{c.HTTPHeaderContentType: c.HTTPContentTypeJSON},
	})
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, providerStatusError(c.ProviderGemini, resp.StatusCode, resp.Body)
	}

	var out gResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, apperrors.Wrap(apperrors.KindExternalService, "gemini response", errParseGemini, err)
	}
	return g.fromGemini(out), nil
}

type gRequest struct {
	Contents          []gContent  `json:"contents"`
	Tools             []gToolSet  `json:"tools,omitempty"`
	SystemInstruction *gContent   `json:"systemInstruction,omitempty"`
	GenerationConfig  *gGenConfig `json:"generationConfig,omitempty"`
}
type gContent struct {
	Role  string  `json:"role,omitempty"`
	Parts []gPart `json:"parts"`
}
type gPart struct {
	Text     string    `json:"text,omitempty"`
	FuncCall *gFCall   `json:"functionCall,omitempty"`
	FuncResp *gFResult `json:"functionResponse,omitempty"`
}
type gFCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}
type gFResult struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}
type gToolSet struct {
	FunctionDeclarations []gFDecl `json:"functionDeclarations"`
}
type gFDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}
type gGenConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens,omitempty"`
}
type gResponse struct {
	Candidates []struct {
		Content gContent `json:"content"`
	} `json:"candidates"`
}

func (g *GeminiClient) toGemini(req Request) gRequest {
	gr := gRequest{GenerationConfig: &gGenConfig{MaxOutputTokens: req.MaxTokens}}
	if req.System != "" {
		gr.SystemInstruction = &gContent{Parts: []gPart{{Text: req.System}}}
	}
	if len(req.Tools) > 0 {
		var decls []gFDecl
		for _, t := range req.Tools {
			decls = append(decls, gFDecl{Name: t.Name, Description: t.Description, Parameters: t.InputSchema})
		}
		gr.Tools = []gToolSet{{FunctionDeclarations: decls}}
	}
	for _, msg := range req.Messages {
		role := c.RoleUser
		if msg.Role == c.RoleAssistant {
			role = c.RoleModel
		}
		var parts []gPart
		for _, b := range msg.Content {
			switch b.Type {
			case c.ContentTypeText:
				if b.Text != "" {
					parts = append(parts, gPart{Text: b.Text})
				}
			case c.ContentTypeToolUse:
				var args map[string]interface{}
				if len(b.Input) > 0 {
					if err := json.Unmarshal(b.Input, &args); err != nil {
						args = make(map[string]interface{})
					}
				}
				parts = append(parts, gPart{FuncCall: &gFCall{Name: b.Name, Args: args}})
			case c.ContentTypeToolResult:
				name := findToolName(req.Messages, b.ToolUseID)
				parts = append(parts, gPart{FuncResp: &gFResult{
					Name: name, Response: map[string]interface{}{geminiToolResultKey: b.Content},
				}})
			}
		}
		if len(parts) > 0 {
			gr.Contents = append(gr.Contents, gContent{Role: role, Parts: parts})
		}
	}
	return gr
}

func (g *GeminiClient) fromGemini(gr gResponse) *Response {
	resp := &Response{Role: c.RoleAssistant, StopReason: c.StopReasonEndTurn}
	if len(gr.Candidates) == 0 {
		return resp
	}
	hasCalls := false
	for _, p := range gr.Candidates[0].Content.Parts {
		if p.Text != "" {
			resp.Content = append(resp.Content, ContentBlock{Type: c.ContentTypeText, Text: p.Text})
		}
		if p.FuncCall != nil {
			hasCalls = true
			argBytes, err := json.Marshal(p.FuncCall.Args)
			if err != nil {
				argBytes = []byte("{}")
			}
			resp.Content = append(resp.Content, ContentBlock{
				Type: c.ContentTypeToolUse, ID: fmt.Sprintf(geminiCallIDFormat, p.FuncCall.Name, len(resp.Content)),
				Name: p.FuncCall.Name, Input: argBytes,
			})
		}
	}
	if hasCalls {
		resp.StopReason = c.StopReasonToolUse
	}
	return resp
}

func findToolName(msgs []Message, id string) string {
	for _, msg := range msgs {
		for _, b := range msg.Content {
			if b.Type == c.ContentTypeToolUse && b.ID == id {
				return b.Name
			}
		}
	}
	return c.UnknownValue
}
