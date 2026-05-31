package config

import (
	"strings"
	"testing"
)

func minimalConfig() *Config {
	return &Config{
		Project: ProjectConfig{Name: "test"},
		Layers: []LayerConfig{
			{Name: "Domain", Paths: []string{"src/domain/"}},
		},
	}
}

func TestValidateArchMissingStyle(t *testing.T) {
	cfg := minimalConfig()
	cfg.Architecture = Architecture{
		Invariants: []ArchitectureInvariant{{ID: "p", Description: "d", Severity: "high"}},
	}
	if err := validate(cfg); err == nil {
		t.Fatal("expected error when invariants present but style missing")
	}
}

func TestValidateArchInvariantInvalidSeverity(t *testing.T) {
	cfg := minimalConfig()
	cfg.Architecture = Architecture{
		Style:      "hexagonal",
		Invariants: []ArchitectureInvariant{{ID: "p", Description: "d", Severity: "extreme"}},
	}
	if err := validate(cfg); err == nil {
		t.Fatal("expected error for invalid severity in architecture invariant")
	}
}

func TestValidateArchInvariantUnknownLayer(t *testing.T) {
	cfg := minimalConfig()
	cfg.Architecture = Architecture{
		Style:      "hexagonal",
		Invariants: []ArchitectureInvariant{{ID: "p", Description: "d", Severity: "high", AppliesTo: []string{"Ghost"}}},
	}
	if err := validate(cfg); err == nil {
		t.Fatal("expected error for unknown layer in architecture invariant")
	}
}

func TestValidateArchValid(t *testing.T) {
	cfg := minimalConfig()
	cfg.Architecture = Architecture{
		Style:       "hexagonal",
		Description: "Ports and Adapters",
		Invariants: []ArchitectureInvariant{
			{ID: "ports_in_domain", Description: "ports must live in domain", Severity: "critical", AppliesTo: []string{"Domain"}},
		},
	}
	if err := validate(cfg); err != nil {
		t.Fatalf("valid architecture config failed: %v", err)
	}
}

func TestValidateExternalIncludeInvalidGlob(t *testing.T) {
	cfg := minimalConfig()
	cfg.External = []ExternalConfig{
		{
			System:  "jira",
			Include: []string{"[invalid"},
		},
	}
	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for invalid include pattern")
	}
	if !strings.Contains(err.Error(), "include") {
		t.Fatalf("error should mention 'include', got: %v", err)
	}
}

func TestValidateExternalExcludeInvalidGlob(t *testing.T) {
	cfg := minimalConfig()
	cfg.External = []ExternalConfig{
		{
			System:  "jira",
			Exclude: []string{"[bad"},
		},
	}
	err := validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for invalid exclude pattern")
	}
	if !strings.Contains(err.Error(), "exclude") {
		t.Fatalf("error should mention 'exclude', got: %v", err)
	}
}

func TestValidateDesignPrincipleMissingID(t *testing.T) {
	cfg := minimalConfig()
	cfg.DesignPrinciples = []DesignPrinciple{{Description: "desc", Severity: "high"}}
	if err := validate(cfg); err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestValidateDesignPrincipleInvalidSeverity(t *testing.T) {
	cfg := minimalConfig()
	cfg.DesignPrinciples = []DesignPrinciple{{ID: "srp", Description: "desc", Severity: "extreme"}}
	if err := validate(cfg); err == nil {
		t.Fatal("expected error for invalid severity")
	}
}

func TestValidateDesignPrincipleUnknownLayer(t *testing.T) {
	cfg := minimalConfig()
	cfg.DesignPrinciples = []DesignPrinciple{{ID: "srp", Description: "desc", Severity: "high", AppliesTo: []string{"Unknown"}}}
	if err := validate(cfg); err == nil {
		t.Fatal("expected error for unknown layer in applies_to")
	}
}

func TestValidateDesignPrincipleValid(t *testing.T) {
	cfg := minimalConfig()
	cfg.DesignPrinciples = []DesignPrinciple{
		{ID: "srp", Description: "single responsibility", Severity: "high", AppliesTo: []string{"Domain"}},
		{ID: "dry", Description: "don't repeat yourself", Severity: "medium"},
	}
	if err := validate(cfg); err != nil {
		t.Fatalf("valid design principles failed validation: %v", err)
	}
}

func TestValidateExternalValidPatternsPass(t *testing.T) {
	cfg := minimalConfig()
	cfg.External = []ExternalConfig{
		{
			System:  "jira",
			Include: []string{"PAYMENT-*", "audit"},
			Exclude: []string{"PAYMENT-9999", "WONTFIX"},
		},
	}
	if err := validate(cfg); err != nil {
		t.Fatalf("valid patterns should not fail validation: %v", err)
	}
}
