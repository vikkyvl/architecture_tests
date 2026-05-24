package cli

import (
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/archguard/project/shared/apperrors"
)

const (
	testJSONReportPath           = "report.json"
	testMarkdownReportPath       = "report.md"
	testRawRateLimitFallback     = "[rate limit]"
	testExpectedRateLimitBadge   = badgeRateLimit
	testExpectedRetryBadge       = badgeRetry
	testExpectedRateLimitMessage = "expected rate limit badge"
	testRawFallbackMessage       = "rate limit output should not use raw fallback text"
	testTerminalRepaintMessage   = "expected animated rate limit output to repaint the same terminal line"
	testAnimatedRateMessage      = "expected animated rate limit output to show rate limit state"
	testAnimatedRetryMessage     = "expected animated rate limit output to finish with retry state"
	testFileLinkFormat           = "\x1b]8;;%s\a%s\x1b]8;;\a"
	testAbsPathFormat            = "abs path: %v"
	testClickableLinkFormat      = "expected clickable link for %q in output"
)

func TestRenderReportPathsUsesClickableFileLinks(t *testing.T) {
	output := renderReportPaths(testJSONReportPath, testMarkdownReportPath)

	assertFileLink(t, output, testJSONReportPath)
	assertFileLink(t, output, testMarkdownReportPath)
}

func TestRenderRateLimitWaitUsesUIBadge(t *testing.T) {
	output := renderRateLimitWait(15 * time.Second)

	if !strings.Contains(output, testExpectedRateLimitBadge) {
		t.Fatal(testExpectedRateLimitMessage)
	}
	if strings.Contains(output, testRawRateLimitFallback) {
		t.Fatal(testRawFallbackMessage)
	}
}

func TestAnimateRateLimitWaitRendersSpinnerAndResume(t *testing.T) {
	var out bytes.Buffer

	animateRateLimitWait(&out, time.Millisecond)

	output := out.String()
	if !strings.Contains(output, terminalClearLine) {
		t.Fatal(testTerminalRepaintMessage)
	}
	if !strings.Contains(output, testExpectedRateLimitBadge) {
		t.Fatal(testAnimatedRateMessage)
	}
	if !strings.Contains(output, testExpectedRetryBadge) {
		t.Fatal(testAnimatedRetryMessage)
	}
}

func TestRenderRetryHidesProviderPayload(t *testing.T) {
	err := apperrors.ExternalService(geminiUnavailablePayload())

	output := renderRetry("gemini", 1, 3, 2*time.Second, err)

	assertNoProviderPayload(t, output)
	if !strings.Contains(output, "gemini is temporarily unavailable") {
		t.Fatalf("expected sanitized provider message, got %q", output)
	}
}

func TestRenderErrorHidesProviderPayload(t *testing.T) {
	err := apperrors.ExternalService(geminiUnavailablePayload())

	output := RenderError(err)

	assertNoProviderPayload(t, output)
	if !strings.Contains(output, "gemini is temporarily unavailable") {
		t.Fatalf("expected sanitized provider message, got %q", output)
	}
}

func assertNoProviderPayload(t *testing.T, output string) {
	t.Helper()

	for _, raw := range []string{`"error"`, `"code"`, "high demand", "UNAVAILABLE"} {
		if strings.Contains(output, raw) {
			t.Fatalf("output leaked provider payload %q in %q", raw, output)
		}
	}
}

func geminiUnavailablePayload() string {
	return `gemini request failed: {
  "error": {
    "code": 503,
    "message": "This model is currently experiencing high demand. Spikes in demand are usually temporary. Please try again later.",
    "status": "UNAVAILABLE"
  }
}`
}

func assertFileLink(t *testing.T, output, path string) {
	t.Helper()

	uri := testFileURI(t, path)
	want := fmt.Sprintf(testFileLinkFormat, uri, styledFileLinkText(path))
	if !strings.Contains(output, want) {
		t.Fatalf(testClickableLinkFormat, path)
	}
}

func testFileURI(t *testing.T, path string) string {
	t.Helper()

	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf(testAbsPathFormat, err)
	}
	return (&url.URL{
		Scheme: fileURLScheme,
		Path:   abs,
	}).String()
}
