package llm

import (
	"fmt"
	"os"

	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
)

const (
	errNoAPIKeyFound       = "no API key found: configure %s or %s in .env"
	errUnsupportedProvider = "unsupported provider: %s"
)

type Provider interface {
	SendMessage(req Request) (*Response, error)
	Model() string
	Name() string
}

func NewProvider(provider, apiKey, model string) (Provider, error) {
	switch provider {
	case c.ProviderAnthropic:
		return NewAnthropicClient(apiKey, model)
	case c.ProviderGemini:
		return NewGeminiClient(apiKey, model)
	case "":
		if apiKey != "" || os.Getenv(c.EnvAnthropicKey) != "" {
			return NewAnthropicClient(apiKey, model)
		}
		if os.Getenv(c.EnvGeminiKey) != "" {
			return NewGeminiClient("", model)
		}
		return nil, apperrors.Validation(fmt.Sprintf(errNoAPIKeyFound, c.EnvAnthropicKey, c.EnvGeminiKey))
	default:
		return nil, apperrors.Validation(fmt.Sprintf(errUnsupportedProvider, provider))
	}
}
