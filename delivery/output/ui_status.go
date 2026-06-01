package output

import (
	"fmt"
	"time"

	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
)

const (
	badgeInterrupted = "INTERRUPTED"
	msgInterrupted   = "Session forcefully terminated — all subprocesses have been shut down."
	msgInterruptHint = "Press Ctrl+C at any time to interrupt and exit cleanly."
)

func RenderInterrupted() string {
	return fmt.Sprintf(formatNotice, WarnBadgeStyle.Render(badgeInterrupted), mutedStyle.Render(msgInterrupted))
}

func RenderInterruptHint() string {
	return fmt.Sprintf(formatNotice, infoBadgeStyle.Render(badgeInfo), mutedStyle.Render(msgInterruptHint))
}

func RenderToolResult(index int, name string, size int, err error) string {
	prefix := mutedStyle.Render(fmt.Sprintf(formatToolIndex, index))
	if err != nil {
		if apperrors.IsKind(err, apperrors.KindPermission) {
			return fmt.Sprintf(formatToolResult, prefix, WarnBadgeStyle.Render(badgeDenied), ValueStyle.Render(fmt.Sprintf(formatErrorDetail, name, err.Error())))
		}
		return fmt.Sprintf(formatToolResult, prefix, errorBadgeStyle.Render(badgeFail), ValueStyle.Render(fmt.Sprintf(formatErrorDetail, name, err.Error())))
	}
	meta := mutedStyle.Render(fmt.Sprintf(c.FormatChars, size))
	return fmt.Sprintf(formatToolResultWithMeta, prefix, SuccessBadgeStyle.Render(BadgeOK), ValueStyle.Render(name), meta)
}

func RenderRetry(provider string, attempt, maxAttempts int, wait time.Duration, err error) string {
	detail := fmt.Sprintf(formatRetryDetail, provider, attempt, maxAttempts, wait)
	if err != nil {
		detail = fmt.Sprintf(formatErrorDetail, detail, publicErrorMessage(err))
	}
	return fmt.Sprintf(formatNotice, WarnBadgeStyle.Render(badgeRetry), mutedStyle.Render(detail))
}

func renderRateLimitWait(wait time.Duration) string {
	return renderRateLimitWaitFrame(wait, "|")
}

func renderRateLimitWaitFrame(remaining time.Duration, frame string) string {
	return fmt.Sprintf(
		formatRateLimitFrame,
		WarnBadgeStyle.Render(badgeRateLimit),
		infoStyle.Render(frame),
		mutedStyle.Render(fmt.Sprintf(formatRateLimitWaiting, roundUpDuration(remaining, rateLimitTimeSlice))),
	)
}

func renderRateLimitResume(wait time.Duration) string {
	return fmt.Sprintf(formatNotice, WarnBadgeStyle.Render(badgeRetry), mutedStyle.Render(fmt.Sprintf(formatRateLimitComplete, wait)))
}

func renderProactivePauseWaitFrame(remaining time.Duration, frame string) string {
	return fmt.Sprintf(
		formatRateLimitFrame,
		infoBadgeStyle.Render(badgeProactivePause),
		infoStyle.Render(frame),
		mutedStyle.Render(fmt.Sprintf(formatProactiveWaiting, roundUpDuration(remaining, rateLimitTimeSlice))),
	)
}

func renderProactivePauseResume(wait time.Duration) string {
	return fmt.Sprintf(formatNotice, infoBadgeStyle.Render(badgeInfo), mutedStyle.Render(fmt.Sprintf(formatProactiveComplete, wait)))
}

func roundUpDuration(d, unit time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	if unit <= 0 || d%unit == 0 {
		return d
	}
	return (d/unit + 1) * unit
}
