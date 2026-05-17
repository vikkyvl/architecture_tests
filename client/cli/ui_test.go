package cli

import (
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
