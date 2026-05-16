package mcp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/archguard/project/client/audit"
	"github.com/archguard/project/client/consent"
	"github.com/archguard/project/client/mcp"
	"github.com/archguard/project/shared/apperrors"
	"github.com/archguard/project/shared/config"
)

func newTestResolver(t *testing.T) (*mcp.Resolver, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		Project: config.ProjectConfig{Name: "test", Language: "go"},
	}
	cm := consent.NewManager(false, dir, nil)
	al := audit.NewLog()
	return mcp.NewResolver(cfg, dir, "", nil, cm, al, nil), dir
}

func TestReadFileBlocksTraversal(t *testing.T) {
	r, _ := newTestResolver(t)
	_, err := r.ExecuteTool("read_file", map[string]interface{}{"path": "../outside.go"}, 10)
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
	_, err := r.ExecuteTool("read_file", map[string]interface{}{"path": ".env"}, 10)
	if err == nil {
		t.Fatal("expected error for sensitive path, got nil")
	}
}

func TestReadFileSuccess(t *testing.T) {
	dir := t.TempDir()
	consentDir := filepath.Join(dir, ".archguard")
	_ = os.MkdirAll(consentDir, 0700)
	_ = os.WriteFile(filepath.Join(consentDir, "consent.yaml"),
		[]byte("allowed:\n  - tool: read_file\n    pattern: all\n"), 0600)

	cfg := &config.Config{Project: config.ProjectConfig{Name: "test", Language: "go"}}
	cm := consent.NewManager(false, dir, nil)
	al := audit.NewLog()
	r := mcp.NewResolver(cfg, dir, "", nil, cm, al, nil)

	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	got, err := r.ExecuteTool("read_file", map[string]interface{}{"path": "main.go"}, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Error("expected non-empty result")
	}
}

func TestReportViolationValidation(t *testing.T) {
	r, _ := newTestResolver(t)
	_, err := r.ExecuteTool("report_violation", map[string]interface{}{
		"file": "a.php",
	}, 10)
	if err == nil {
		t.Fatal("expected validation error for missing fields")
	}
}

func TestReportViolationSuccess(t *testing.T) {
	r, _ := newTestResolver(t)
	_, err := r.ExecuteTool("report_violation", map[string]interface{}{
		"file":        "a.php",
		"severity":    "high",
		"category":    "layer_dependency",
		"rule":        "no-infra-in-domain",
		"description": "infrastructure class used in domain layer",
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
