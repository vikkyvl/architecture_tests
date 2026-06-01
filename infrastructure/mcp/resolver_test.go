package mcp_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/archguard/project/config"
	"github.com/archguard/project/infrastructure/audit"
	"github.com/archguard/project/infrastructure/consent"
	"github.com/archguard/project/infrastructure/mcp"
	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
)

// newTestResolver returns a minimal resolver; read_file is NOT consented.
func newTestResolver(t *testing.T) (*mcp.Resolver, string) {
	t.Helper()
	dir := t.TempDir()
	cm := consent.NewManager(false, dir, nil, io.Discard)
	al := audit.NewLog()
	return mcp.NewResolver(mcp.ResolverConfig{ProjectPath: dir, Language: "go"}, cm, al, nil, nil), dir
}

// setupReadFileConsent writes a project consent.yaml that auto-approves read_file.
func setupReadFileConsent(t *testing.T, dir string) {
	t.Helper()
	consentDir := filepath.Join(dir, ".archguard")
	_ = os.MkdirAll(consentDir, 0700)
	_ = os.WriteFile(filepath.Join(consentDir, "consent.yaml"),
		[]byte("allowed:\n  - tool: read_file\n    pattern: all\n"), 0600)
}

func TestReadFileBlocksTraversal(t *testing.T) {
	r, _ := newTestResolver(t)
	_, err := r.ExecuteTool(c.ToolReadFile, map[string]interface{}{c.ArgPath: "../outside.go"}, 10)
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
	if !apperrors.IsKind(err, apperrors.KindPermission) {
		t.Errorf("expected permission error, got %v", err)
	}
}

func TestReadFileBlocksSensitivePath(t *testing.T) {
	r, dir := newTestResolver(t)
	_ = os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=1"), 0600)
	_, err := r.ExecuteTool(c.ToolReadFile, map[string]interface{}{c.ArgPath: ".env"}, 10)
	if err == nil {
		t.Fatal("expected error for sensitive path, got nil")
	}
}

func TestReadFileSuccess(t *testing.T) {
	dir := t.TempDir()
	setupReadFileConsent(t, dir)
	r := mcp.NewResolver(mcp.ResolverConfig{ProjectPath: dir, Language: "go"},
		consent.NewManager(false, dir, nil, io.Discard), audit.NewLog(), nil, nil)

	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	got, err := r.ExecuteTool(c.ToolReadFile, map[string]interface{}{c.ArgPath: "main.go"}, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Error("expected non-empty result")
	}
}

func TestReadFileTruncatesAtSoftCap(t *testing.T) {
	dir := t.TempDir()
	setupReadFileConsent(t, dir)
	const softCap = 100
	const originalSize = 500
	r := mcp.NewResolver(mcp.ResolverConfig{ProjectPath: dir, Language: "go", MaxFileBytes: softCap},
		consent.NewManager(false, dir, nil, io.Discard), audit.NewLog(), nil, nil)

	payload := make([]byte, originalSize)
	for i := range payload {
		payload[i] = 'a'
	}
	_ = os.WriteFile(filepath.Join(dir, "big.go"), payload, 0644)

	got, err := r.ExecuteTool(c.ToolReadFile, map[string]interface{}{c.ArgPath: "big.go"}, 10)
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	if !contains(got, "file truncated after 100 bytes; 400 bytes remain") {
		t.Errorf("expected truncation suffix mentioning 100/400 bytes, got tail: %q", tail(got, 120))
	}
}

func TestReadFileSkipsSoftCapWhenZero(t *testing.T) {
	dir := t.TempDir()
	setupReadFileConsent(t, dir)
	r := mcp.NewResolver(mcp.ResolverConfig{ProjectPath: dir, Language: "go"},
		consent.NewManager(false, dir, nil, io.Discard), audit.NewLog(), nil, nil)

	payload := make([]byte, 500)
	for i := range payload {
		payload[i] = 'a'
	}
	_ = os.WriteFile(filepath.Join(dir, "big.go"), payload, 0644)

	got, err := r.ExecuteTool(c.ToolReadFile, map[string]interface{}{c.ArgPath: "big.go"}, 10)
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	if contains(got, "file truncated") {
		t.Errorf("MaxFileBytes=0 must disable truncation, got suffix: %q", tail(got, 120))
	}
}

func TestReadFileDoesNotTruncateBelowSoftCap(t *testing.T) {
	dir := t.TempDir()
	setupReadFileConsent(t, dir)
	r := mcp.NewResolver(mcp.ResolverConfig{ProjectPath: dir, Language: "go", MaxFileBytes: 1000},
		consent.NewManager(false, dir, nil, io.Discard), audit.NewLog(), nil, nil)

	_ = os.WriteFile(filepath.Join(dir, "small.go"), []byte("package main"), 0644)

	got, err := r.ExecuteTool(c.ToolReadFile, map[string]interface{}{c.ArgPath: "small.go"}, 10)
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	if contains(got, "file truncated") {
		t.Errorf("file smaller than cap must not be truncated, got: %q", tail(got, 120))
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func TestReadFileDedupReturnsStubOnRepeat(t *testing.T) {
	dir := t.TempDir()
	setupReadFileConsent(t, dir)
	al := audit.NewLog()
	r := mcp.NewResolver(mcp.ResolverConfig{ProjectPath: dir, Language: "go", KeepWindowTurns: 8},
		consent.NewManager(false, dir, nil, io.Discard), al, nil, nil)

	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	first, err := r.ExecuteTool(c.ToolReadFile, map[string]interface{}{c.ArgPath: "main.go"}, 10)
	if err != nil {
		t.Fatalf("first read: %v", err)
	}
	if strings.Contains(first, "dedup:") {
		t.Fatalf("first read must return content, not dedup stub: %q", first)
	}

	second, err := r.ExecuteTool(c.ToolReadFile, map[string]interface{}{c.ArgPath: "main.go"}, 10)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if !strings.Contains(second, "dedup:") {
		t.Fatalf("repeat read within window must return dedup stub, got: %q", second)
	}
	if !strings.Contains(second, "main.go") {
		t.Errorf("stub must reference the path: %q", second)
	}

	entries := al.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(entries))
	}
	if entries[0].Dedup {
		t.Errorf("first entry must have Dedup=false")
	}
	if !entries[1].Dedup {
		t.Errorf("second entry must have Dedup=true")
	}
}

func TestReadFileDedupAfterPruneReServes(t *testing.T) {
	dir := t.TempDir()
	setupReadFileConsent(t, dir)
	al := audit.NewLog()
	r := mcp.NewResolver(mcp.ResolverConfig{ProjectPath: dir, Language: "go", KeepWindowTurns: 2},
		consent.NewManager(false, dir, nil, io.Discard), al, nil, nil)

	for _, name := range []string{"a.go", "b.go", "c.go", "d.go"} {
		_ = os.WriteFile(filepath.Join(dir, name), []byte("package x\n"), 0644)
	}

	if _, err := r.ExecuteTool(c.ToolReadFile, map[string]interface{}{c.ArgPath: "a.go"}, 10); err != nil {
		t.Fatalf("read a.go: %v", err)
	}
	for _, p := range []string{"b.go", "c.go", "d.go"} {
		if _, err := r.ExecuteTool(c.ToolReadFile, map[string]interface{}{c.ArgPath: p}, 10); err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
	}

	got, err := r.ExecuteTool(c.ToolReadFile, map[string]interface{}{c.ArgPath: "a.go"}, 10)
	if err != nil {
		t.Fatalf("re-read a.go after window expiry: %v", err)
	}
	if strings.Contains(got, "dedup:") {
		t.Fatalf("a.go fell outside keepWindowTurns=2 — must re-serve content, got stub: %q", got)
	}

	entries := al.Entries()
	last := entries[len(entries)-1]
	if last.Dedup {
		t.Errorf("re-served entry must have Dedup=false, got true")
	}
}

func TestReadFileDedupDisabledWhenKeepWindowZero(t *testing.T) {
	dir := t.TempDir()
	setupReadFileConsent(t, dir)
	r := mcp.NewResolver(mcp.ResolverConfig{ProjectPath: dir, Language: "go"},
		consent.NewManager(false, dir, nil, io.Discard), audit.NewLog(), nil, nil)

	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	for i := 0; i < 3; i++ {
		got, err := r.ExecuteTool(c.ToolReadFile, map[string]interface{}{c.ArgPath: "main.go"}, 10)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		if strings.Contains(got, "dedup:") {
			t.Fatalf("iter %d: keepWindowTurns=0 must disable dedup, got stub", i)
		}
	}
}

func TestReportViolationValidation(t *testing.T) {
	r, _ := newTestResolver(t)
	_, err := r.ExecuteTool(c.ToolReportViolation, map[string]interface{}{
		c.ArgFile: "a.php",
	}, 10)
	if err == nil {
		t.Fatal("expected validation error for missing fields")
	}
}

func TestReportViolationSuccess(t *testing.T) {
	r, _ := newTestResolver(t)
	_, err := r.ExecuteTool(c.ToolReportViolation, map[string]interface{}{
		c.ArgFile:        "a.php",
		c.ArgSeverity:    c.SeverityHigh,
		c.ArgCategory:    c.CategoryLayerDep,
		c.ArgRule:        "no-infra-in-domain",
		c.ArgDescription: "infrastructure class used in domain layer",
	}, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	violations := r.GetViolations()
	if len(violations) != 1 {
		t.Errorf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Rule != "no-infra-in-domain" {
		t.Errorf("unexpected rule: %s", violations[0].Rule)
	}
}

func TestArchRulesContainsConfiguredData(t *testing.T) {
	dir := t.TempDir()
	r := mcp.NewResolver(mcp.ResolverConfig{
		ProjectPath: dir,
		Language:    "go",
		Layers: []config.LayerConfig{
			{Name: "domain", Namespaces: []string{"src/domain"}},
			{Name: "infrastructure", Namespaces: []string{"src/infra"}},
		},
		Rules: []config.RuleConfig{
			{From: "infrastructure", To: "domain", Allow: false},
		},
		DomainContext: config.DomainContextConfig{
			Domain:      "fintech",
			Description: "payment processing",
			Rules: []config.DomainRule{
				{ID: "dr-001", Severity: "critical", Description: "no direct DB access"},
			},
		},
	}, consent.NewManager(false, dir, nil, io.Discard), audit.NewLog(), nil, nil)

	got, err := r.ExecuteTool(c.ToolGetArchitectureRules, nil, 10)
	if err != nil {
		t.Fatalf("get_architecture_rules: %v", err)
	}
	for _, want := range []string{"domain", "infrastructure", "fintech", "payment processing", "dr-001", "no direct DB access"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in arch rules output; got:\n%s", want, got)
		}
	}
}

func TestClassDepsResolvesLayerFromNamespace(t *testing.T) {
	dir := t.TempDir()
	r := mcp.NewResolver(mcp.ResolverConfig{
		ProjectPath: dir,
		Language:    "go",
		Layers: []config.LayerConfig{
			{Name: "domain", Namespaces: []string{"src/domain"}},
		},
	}, consent.NewManager(false, dir, nil, io.Discard), audit.NewLog(), nil, nil)

	_ = os.MkdirAll(filepath.Join(dir, "src/domain"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "src/domain/service.go"), []byte("package domain\n"), 0644)

	got, err := r.ExecuteTool(c.ToolGetClassDependencies, map[string]interface{}{
		c.ArgClass: "src/domain/service.go",
	}, 10)
	if err != nil {
		t.Fatalf("get_class_dependencies: %v", err)
	}
	if !strings.Contains(got, `"domain"`) {
		t.Errorf("expected layer=domain in output; got:\n%s", got)
	}
}
