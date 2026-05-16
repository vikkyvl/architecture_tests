package cli

import (
	"bytes"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRenderReportPathsUsesClickableFileLinks(t *testing.T) {
	output := renderReportPaths("report.json", "report.md")

	assertFileLink(t, output, "report.json")
	assertFileLink(t, output, "report.md")
}

func TestRenderRateLimitWaitUsesUIBadge(t *testing.T) {
	output := renderRateLimitWait(15 * time.Second)

	if !strings.Contains(output, "RATE LIMIT") {
		t.Fatal("expected rate limit badge")
	}
	if strings.Contains(output, "[rate limit]") {
		t.Fatal("rate limit output should not use raw fallback text")
	}
}

func TestAnimateRateLimitWaitRendersSpinnerAndResume(t *testing.T) {
	var out bytes.Buffer

	animateRateLimitWait(&out, time.Millisecond)

	output := out.String()
	if !strings.Contains(output, terminalClearLine) {
		t.Fatal("expected animated rate limit output to repaint the same terminal line")
	}
	if !strings.Contains(output, "RATE LIMIT") {
		t.Fatal("expected animated rate limit output to show rate limit state")
	}
	if !strings.Contains(output, "RETRY") {
		t.Fatal("expected animated rate limit output to finish with retry state")
	}
}

func assertFileLink(t *testing.T, output, path string) {
	t.Helper()

	uri := testFileURI(t, path)
	want := "\x1b]8;;" + uri + "\a" + styledFileLinkText(path) + "\x1b]8;;\a"
	if !strings.Contains(output, want) {
		t.Fatalf("expected clickable link for %q in output", path)
	}
}

func testFileURI(t *testing.T, path string) string {
	t.Helper()

	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	return (&url.URL{Scheme: "file", Path: abs}).String()
}
