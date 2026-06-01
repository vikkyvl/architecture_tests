package external

import (
	"testing"
)

func TestBuildQueryNoTemplate(t *testing.T) {
	if got := BuildQuery("audit", "", ""); got != "audit" {
		t.Fatalf("got %q, want %q", got, "audit")
	}
}

func TestBuildQueryWithFilter(t *testing.T) {
	if got := BuildQuery("audit", "", "PAYMENT"); got != "PAYMENT audit" {
		t.Fatalf("got %q, want %q", got, "PAYMENT audit")
	}
}

func TestBuildQuerySingleTemplate(t *testing.T) {
	want := `text ~ "audit"`
	if got := BuildQuery("audit", `text ~ "%s"`, "PAYMENT"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildQueryDoubleTemplate(t *testing.T) {
	want := `text ~ "audit" AND project = PAYMENT`
	if got := BuildQuery("audit", `text ~ "%s" AND project = %s`, "PAYMENT"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildQueryZeroPlaceholderTemplate(t *testing.T) {
	if got := BuildQuery("audit", "SELECT *", "PAYMENT"); got != "SELECT *" {
		t.Fatalf("got %q, want %q", got, "SELECT *")
	}
}

func TestFilterBlocksNoPatterns(t *testing.T) {
	blocks := []contentBlock{
		{Type: "text", Text: "PAYMENT-001: audit trail"},
		{Type: "text", Text: "PAYMENT-002: data isolation"},
	}
	out := filterBlocks(blocks, nil, nil)
	if len(out) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(out))
	}
}

func TestFilterBlocksInclude(t *testing.T) {
	blocks := []contentBlock{
		{Type: "text", Text: "PAYMENT-001: audit trail required"},
		{Type: "text", Text: "PAYMENT-002: performance improvement"},
		{Type: "text", Text: "PAYMENT-003: security review needed"},
	}
	out := filterBlocks(blocks, []string{"audit", "security"}, nil)
	if len(out) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(out))
	}
	if out[0].Text != blocks[0].Text || out[1].Text != blocks[2].Text {
		t.Fatalf("unexpected blocks: %v", out)
	}
}

func TestFilterBlocksExclude(t *testing.T) {
	blocks := []contentBlock{
		{Type: "text", Text: "PAYMENT-001: audit trail"},
		{Type: "text", Text: "PAYMENT-9999: legacy WONTFIX"},
		{Type: "text", Text: "PAYMENT-003: security"},
	}
	out := filterBlocks(blocks, nil, []string{"PAYMENT-9999", "WONTFIX"})
	if len(out) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(out))
	}
}

func TestFilterBlocksIncludeAndExclude(t *testing.T) {
	blocks := []contentBlock{
		{Type: "text", Text: "PAYMENT-001: audit trail"},
		{Type: "text", Text: "PAYMENT-002: audit WONTFIX"},
		{Type: "text", Text: "PAYMENT-003: performance"},
	}
	// include "audit", exclude "WONTFIX"
	out := filterBlocks(blocks, []string{"audit"}, []string{"WONTFIX"})
	if len(out) != 1 {
		t.Fatalf("expected 1 block, got %d", len(out))
	}
	if out[0].Text != blocks[0].Text {
		t.Fatalf("wrong block: %q", out[0].Text)
	}
}

func TestFilterBlocksNonTextSkipped(t *testing.T) {
	blocks := []contentBlock{
		{Type: "image", Text: "data"},
		{Type: "text", Text: "audit trail"},
	}
	out := filterBlocks(blocks, []string{"audit"}, nil)
	if len(out) != 1 || out[0].Type != "text" {
		t.Fatalf("non-text blocks must be skipped, got %v", out)
	}
}

func TestMatchesAnySubstring(t *testing.T) {
	if !matchesAny("PAYMENT-123: audit required", []string{"audit"}) {
		t.Fatal("expected match for substring 'audit'")
	}
}

func TestMatchesAnyGlob(t *testing.T) {
	if !matchesAny("PAYMENT-123: audit required", []string{"PAYMENT-*"}) {
		t.Fatal("expected glob match for 'PAYMENT-*' in single-line text")
	}
}

func TestMatchesAnyGlobMultiLine(t *testing.T) {
	text := "Issue summary:\nPAYMENT-456 needs audit\nStatus: open"
	if !matchesAny(text, []string{"PAYMENT-*"}) {
		t.Fatal("expected glob match per-line in multi-line text")
	}
}

func TestMatchesAnyNoMatch(t *testing.T) {
	if matchesAny("performance optimization", []string{"audit", "security"}) {
		t.Fatal("expected no match")
	}
}
