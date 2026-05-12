package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/httpclient"
)

const (
	errParseAnthropic = "failed to parse anthropic response"
)

type AnthropicClient struct {
	apiKey string
	model  string
	http   *httpclient.Client
}

func NewAnthropicClient(apiKey, model string) (*AnthropicClient, error) {
	if apiKey == "" {
		apiKey = os.Getenv(c.EnvAnthropicKey)
	}
	if apiKey == "" {
		return nil, apperrors.Validation(fmt.Sprintf(errAPIKeyRequired, c.EnvAnthropicKey))
	}
	if model == "" {
		model = c.AnthropicModel
	}
	return &AnthropicClient{
		apiKey: apiKey, model: model,
		http: httpclient.New(c.AnthropicTimeout, c.AnthropicMaxRetries, c.AnthropicRetryWait),
	}, nil
}

func (a *AnthropicClient) Name() string  { return c.ProviderAnthropic }
func (a *AnthropicClient) Model() string { return a.model }

func (a *AnthropicClient) SendMessage(req Request) (*Response, error) {
	req.Model = a.model
	if req.MaxTokens == 0 {
		req.MaxTokens = c.DefaultMaxTokens
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.KindInternal, "anthropic request", "failed to marshal request", err)
	}

	resp, err := a.http.Post(httpclient.RequestConfig{
		URL:  c.AnthropicBaseURL,
		Body: body,
		Headers: map[string]string{
			c.HTTPHeaderContentType:      c.HTTPContentTypeJSON,
			c.HTTPHeaderAnthropicAPIKey:  a.apiKey,
			c.HTTPHeaderAnthropicVersion: c.AnthropicVersion,
		},
	})
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, providerStatusError(c.ProviderAnthropic, resp.StatusCode, resp.Body)
	}

	var out Response
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, apperrors.Wrap(apperrors.KindExternalService, "anthropic response", errParseAnthropic, err)
	}
	return &out, nil
}
