package review

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/archguard/project/api/llm"
	"github.com/archguard/project/shared/apperrors"
	"github.com/archguard/project/shared/constants"
)

func (e *Engine) sendMessageWithRetry(req llm.Request, maxRetries int, waits []time.Duration) (*llm.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := e.provider.SendMessage(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if attempt >= maxRetries || !isRetryableLLMError(err) {
			break
		}
		wait := retryWait(attempt, waits)
		e.observer.Retry(e.provider.Name(), attempt+1, maxRetries, wait, err)
		time.Sleep(wait)
	}
	return nil, lastErr
}

func decodeToolArgs(block llm.ContentBlock) map[string]interface{} {
	args := make(map[string]interface{})
	if len(block.Input) == 0 {
		return args
	}
	if err := json.Unmarshal(block.Input, &args); err != nil {
		return make(map[string]interface{})
	}
	return args
}

func isRetryableLLMError(err error) bool {
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		return false
	}
	if appErr.Kind == apperrors.KindRateLimited {
		return false
	}
	if appErr.Kind == apperrors.KindExternalService {
		return true
	}
	msg := strings.ToLower(appErr.Error())
	return strings.Contains(msg, retryableOverloaded) || strings.Contains(msg, retryableTempUnavailable)
}

func retryWait(attempt int, waits []time.Duration) time.Duration {
	if attempt < len(waits) {
		return waits[attempt]
	}
	return waits[len(waits)-1]
}

func truncate(s string, max int) string {
	result := s
	if len(result) > max {
		result = result[:max] + constants.TruncationSuffix
	}
	return result
}
