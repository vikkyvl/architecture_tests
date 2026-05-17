package llm

import (
	"fmt"
	"os"
	"time"

	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
)

const (
	errNoAPIKeyFound       = "no API key found: configure %s, %s or %s in .env"
	errUnsupportedProvider = "unsupported provider: %s"
)

type Provider interface {
	SendMessage(req Request) (*Response, error)
	Model() string
	Name() string
}

func NewProvider(provider, apiKey, model string) (Provider, error) {
	return NewProviderWithRateLimitHook(provider, apiKey, model, nil)
}

func NewProviderWithRateLimitHook(provider, apiKey, model string, rateLimitHook func(time.Duration)) (Provider, error) {
	switch provider {
	case c.ProviderAnthropic:
		return NewAnthropicClientWithRateLimitHook(apiKey, model, rateLimitHook)
	case c.ProviderGemini:
		return NewGeminiClientWithRateLimitHook(apiKey, model, rateLimitHook)
	case c.ProviderOpenAI:
		return NewOpenAIClientWithRateLimitHook(apiKey, model, rateLimitHook)
	case "":
		if apiKey != "" || os.Getenv(c.EnvAnthropicKey) != "" {
			return NewAnthropicClientWithRateLimitHook(apiKey, model, rateLimitHook)
		}
		if os.Getenv(c.EnvGeminiKey) != "" {
			return NewGeminiClientWithRateLimitHook("", model, rateLimitHook)
		}
		if os.Getenv(c.EnvOpenAIKey) != "" {
			return NewOpenAIClientWithRateLimitHook("", model, rateLimitHook)
		}
		return nil, apperrors.Validation(fmt.Sprintf(errNoAPIKeyFound, c.EnvAnthropicKey, c.EnvGeminiKey, c.EnvOpenAIKey))
	default:
		return nil, apperrors.Validation(fmt.Sprintf(errUnsupportedProvider, provider))
	}
}
