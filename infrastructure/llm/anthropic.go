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
	apiKey                  string
	model                   string
	http                    *httpclient.Client
	cacheTail               bool
	respectRateLimitHeaders bool
}

func NewAnthropicClient(apiKey, model string) (*AnthropicClient, error) {
	return NewAnthropicClientWithRateLimitHook(apiKey, model, nil)
}

func NewAnthropicClientWithRateLimitHook(apiKey, model string, rateLimitHook func(time.Duration)) (*AnthropicClient, error) {
	return NewAnthropicClientWithOptions(apiKey, model, RuntimeOptions{}, rateLimitHook)
}

var anthropicRateLimitHeaders = httpclient.RateLimitHeaders{
	RequestsRemaining: c.HeaderAnthropicRequestsRemaining,
	TokensRemaining:   c.HeaderAnthropicTokensRemaining,
	RequestsReset:     c.HeaderAnthropicRequestsReset,
	TokensReset:       c.HeaderAnthropicTokensReset,
	TokenThreshold:    c.TailTokenThreshold,
}

func NewAnthropicClientWithOptions(apiKey, model string, opts RuntimeOptions, rateLimitHook func(time.Duration)) (*AnthropicClient, error) {
	cfg, err := newProviderClientConfig(
		apiKey, model, c.EnvAnthropicKey, c.AnthropicModel,
		c.AnthropicTimeout,
		opts.retries(c.AnthropicMaxRetries),
		opts.retryWait(c.AnthropicRetryWait),
		rateLimitHook,
	)
	if err != nil {
		return nil, err
	}
	if opts.RespectRateLimitHeaders {
		cfg.http.EnablePreemptiveSleep(anthropicRateLimitHeaders)
	}
	if opts.RateLimitObserver != nil {
		cfg.http.WithRateLimitObserver(opts.RateLimitObserver)
	}
	if opts.PreemptiveSleepObserver != nil {
		cfg.http.WithPreemptiveSleepObserver(opts.PreemptiveSleepObserver)
	}
	return &AnthropicClient{
		apiKey:                  cfg.apiKey,
		model:                   cfg.model,
		http:                    cfg.http,
		cacheTail:               opts.AnthropicCacheTail,
		respectRateLimitHeaders: opts.RespectRateLimitHeaders,
	}, nil
}

func markCacheBreakpoints(req *Request) {
	if n := len(req.System); n > 0 {
		req.System[n-1].CacheControl = &CacheControl{Type: c.CacheControlEphemeral}
	}
	if n := len(req.Tools); n > 0 {
		req.Tools[n-1].CacheControl = &CacheControl{Type: c.CacheControlEphemeral}
	}
}

func markMidPrefixCacheBreakpoint(req *Request) {
	if len(req.Messages) < 3 {
		return
	}
	for i := 1; i < len(req.Messages); i++ {
		if req.Messages[i].Role != c.RoleUser {
			continue
		}
		if req.Messages[i-1].Role != c.RoleAssistant {
			continue
		}
		for j := range req.Messages[i].Content {
			if req.Messages[i].Content[j].Type == c.ContentTypeToolResult {
				req.Messages[i].Content[j].CacheControl = &CacheControl{Type: c.CacheControlEphemeral}
				return
			}
		}
		return
	}
}

func markTailCacheBreakpoint(req *Request) {
	if len(req.Messages) < 2 {
		return
	}
	for i := range req.Messages {
		for j := range req.Messages[i].Content {
			if req.Messages[i].Content[j].Type == c.ContentTypeToolResult {
				req.Messages[i].Content[j].CacheControl = nil
			}
		}
	}
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role != c.RoleUser {
			continue
		}
		for j := len(req.Messages[i].Content) - 1; j >= 0; j-- {
			if req.Messages[i].Content[j].Type == c.ContentTypeToolResult {
				req.Messages[i].Content[j].CacheControl = &CacheControl{Type: c.CacheControlEphemeral}
				return
			}
		}
		return
	}
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
	markCacheBreakpoints(&req)
	if a.cacheTail {
		markTailCacheBreakpoint(&req)
		markMidPrefixCacheBreakpoint(&req)
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
