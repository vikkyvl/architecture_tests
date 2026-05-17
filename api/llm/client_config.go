package llm

import (
	"fmt"
	"os"
	"time"

	"github.com/archguard/project/shared/apperrors"
	"github.com/archguard/project/shared/httpclient"
)

type providerClientConfig struct {
	apiKey string
	model  string
	http   *httpclient.Client
}

func newProviderClientConfig(
	apiKey string,
	model string,
	envKey string,
	defaultModel string,
	timeout time.Duration,
	maxRetries int,
	retryWait time.Duration,
	rateLimitHook func(time.Duration),
) (*providerClientConfig, error) {
	if apiKey == "" {
		apiKey = os.Getenv(envKey)
	}
	if apiKey == "" {
		return nil, apperrors.Validation(fmt.Sprintf(errAPIKeyRequired, envKey))
	}
	if model == "" {
		model = defaultModel
	}
	return &providerClientConfig{
		apiKey: apiKey,
		model:  model,
		http:   httpclient.NewWithRateLimitHook(timeout, maxRetries, retryWait, rateLimitHook),
	}, nil
}
