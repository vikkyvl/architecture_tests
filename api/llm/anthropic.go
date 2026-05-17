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
	errParseAnthropic = "failed to parse anthropic response"
)

type AnthropicClient struct {
	apiKey string
	model  string
	http   *httpclient.Client
}

func NewAnthropicClient(apiKey, model string) (*AnthropicClient, error) {
	return NewAnthropicClientWithRateLimitHook(apiKey, model, nil)
}

func NewAnthropicClientWithRateLimitHook(apiKey, model string, rateLimitHook func(time.Duration)) (*AnthropicClient, error) {
	cfg, err := newProviderClientConfig(
		apiKey, model, c.EnvAnthropicKey, c.AnthropicModel,
		c.AnthropicTimeout, c.AnthropicMaxRetries, c.AnthropicRetryWait,
		rateLimitHook,
	)
	if err != nil {
		return nil, err
	}
	return &AnthropicClient{
		apiKey: cfg.apiKey,
		model:  cfg.model,
		http:   cfg.http,
	}, nil
}

func (a *AnthropicClient) Name() string {
	return c.ProviderAnthropic
}

func (a *AnthropicClient) Model() string {
	return a.model
}

func (a *AnthropicClient) SendMessage(req Request) (*Response, error) {
	req.Model = a.model
	if req.MaxTokens == 0 {
		req.MaxTokens = c.DefaultMaxTokens
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.KindInternal, operationAnthropicRequest, errMarshalRequest, err)
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
		return nil, apperrors.Wrap(apperrors.KindExternalService, operationAnthropicResp, errParseAnthropic, err)
	}
	return &out, nil
}
