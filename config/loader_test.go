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
