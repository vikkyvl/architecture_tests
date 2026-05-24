package llm

import "time"

type RuntimeOptions struct {
	MaxRetries              int
	RetryWait               time.Duration
	AnthropicCacheTail      bool
	RespectRateLimitHeaders bool
	RateLimitObserver       func(kind string)
	PreemptiveSleepObserver func(wait time.Duration)
}

func (o RuntimeOptions) retries(fallback int) int {
	if o.MaxRetries > 0 {
		return o.MaxRetries
	}
	return fallback
}

func (o RuntimeOptions) retryWait(fallback time.Duration) time.Duration {
	if o.RetryWait > 0 {
		return o.RetryWait
	}
	return fallback
}
