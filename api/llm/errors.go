package llm

import (
	"fmt"
	"net/http"

	"github.com/archguard/project/shared/apperrors"
)

const (
	errAPIKeyRequired         = "%s is required"
	providerNotFoundFormat    = "%s resource not found: %s"
	providerPermissionFormat  = "%s permission denied: %s"
	providerRateLimitFormat   = "%s rate limit exceeded: %s"
	providerUnavailableFormat = "%s request failed: %s"
)

func providerStatusError(provider string, statusCode int, body []byte) error {
	switch statusCode {
	case http.StatusNotFound:
		return apperrors.NotFound(fmt.Sprintf(providerNotFoundFormat, provider, string(body)))
	case http.StatusUnauthorized, http.StatusForbidden:
		return apperrors.PermissionDenied(fmt.Sprintf(providerPermissionFormat, provider, string(body)))
	case http.StatusTooManyRequests:
		return apperrors.RateLimited(fmt.Sprintf(providerRateLimitFormat, provider, string(body)))
	default:
		return apperrors.ExternalService(fmt.Sprintf(providerUnavailableFormat, provider, string(body)))
	}
}
