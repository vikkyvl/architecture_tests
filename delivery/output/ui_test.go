package output

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
	out := RenderReportPaths(testJSONReportPath, testMarkdownReportPath)

	assertFileLink(t, out, testJSONReportPath)
	assertFileLink(t, out, testMarkdownReportPath)
}

func TestRenderReportPathsHyphenatedNamesAreClickable(t *testing.T) {
	out := RenderReportPaths("report-test.json", "report-test.md")

	assertFileLink(t, out, "report-test.json")
	assertFileLink(t, out, "report-test.md")
}

func TestRenderRateLimitWaitUsesUIBadge(t *testing.T) {
	out := renderRateLimitWait(15 * time.Second)

	if !strings.Contains(out, testExpectedRateLimitBadge) {
		t.Fatal(testExpectedRateLimitMessage)
	}
	if strings.Contains(out, testRawRateLimitFallback) {
		t.Fatal(testRawFallbackMessage)
	}
}

func TestAnimateRateLimitWaitRendersSpinnerAndResume(t *testing.T) {
	var buf bytes.Buffer

	AnimateRateLimitWait(&buf, time.Millisecond)

	out := buf.String()
	if !strings.Contains(out, terminalClearLine) {
		t.Fatal(testTerminalRepaintMessage)
	}
	if !strings.Contains(out, testExpectedRateLimitBadge) {
		t.Fatal(testAnimatedRateMessage)
	}
	if !strings.Contains(out, testExpectedRetryBadge) {
		t.Fatal(testAnimatedRetryMessage)
	}
}

func TestRenderRetryHidesProviderPayload(t *testing.T) {
	err := apperrors.ExternalService(geminiUnavailablePayload())

	out := RenderRetry("gemini", 1, 3, 2*time.Second, err)

	assertNoProviderPayload(t, out)
	if !strings.Contains(out, "gemini is temporarily unavailable") {
		t.Fatalf("expected sanitized provider message, got %q", out)
	}
}

func TestRenderErrorHidesProviderPayload(t *testing.T) {
	err := apperrors.ExternalService(geminiUnavailablePayload())

	out := RenderError(err)

	assertNoProviderPayload(t, out)
	if !strings.Contains(out, "gemini is temporarily unavailable") {
		t.Fatalf("expected sanitized provider message, got %q", out)
	}
}

func assertNoProviderPayload(t *testing.T, out string) {
	t.Helper()

	for _, raw := range []string{`"error"`, `"code"`, "high demand", "UNAVAILABLE"} {
		if strings.Contains(out, raw) {
			t.Fatalf("output leaked provider payload %q in %q", raw, out)
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

func assertFileLink(t *testing.T, out, path string) {
	t.Helper()

	uri := testFileURI(t, path)
	// New format: ANSI styling wraps the OSC 8 sequence, plain text inside.
	osc8 := fmt.Sprintf(testFileLinkFormat, uri, path)
	want := fmt.Sprintf(formatStyledLink, osc8)
	if !strings.Contains(out, want) {
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
