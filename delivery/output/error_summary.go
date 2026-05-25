package output

import (
	"errors"
	"fmt"
	"strings"

	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
)

const (
	msgProviderRateLimited   = "%s is rate limited; waiting before retry"
	msgProviderUnavailable   = "%s is temporarily unavailable; retrying shortly"
	msgExternalUnavailable   = "upstream service is temporarily unavailable"
	msgRateLimited           = "provider rate limit reached; waiting before retry"
	geminiUnavailableStatus  = `"status": "UNAVAILABLE"`
	geminiUnavailableCompact = `"status":"UNAVAILABLE"`
)

func publicErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		return err.Error()
	}
	switch appErr.Kind {
	case apperrors.KindExternalService:
		return publicProviderMessage(appErr.Message, msgExternalUnavailable, msgProviderUnavailable)
	case apperrors.KindRateLimited:
		return publicProviderMessage(appErr.Message, msgRateLimited, msgProviderRateLimited)
	default:
		return appErr.Error()
	}
}

func publicProviderMessage(message, fallback, format string) string {
	if provider := providerFromMessage(message); provider != "" {
		return fmt.Sprintf(format, provider)
	}
	if strings.Contains(message, geminiUnavailableStatus) || strings.Contains(message, geminiUnavailableCompact) {
		return fmt.Sprintf(format, c.ProviderGemini)
	}
	return fallback
}

func providerFromMessage(message string) string {
	for _, provider := range []string{c.ProviderAnthropic, c.ProviderGemini, c.ProviderOpenAI} {
		if strings.HasPrefix(message, provider+" ") {
			return provider
		}
	}
	return ""
}
