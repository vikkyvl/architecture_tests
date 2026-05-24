package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/archguard/project/shared/apperrors"
	"github.com/archguard/project/shared/config"
	c "github.com/archguard/project/shared/constants"
)

const minimalProjectYAML = `project:
  name: test
layers:
  - name: a
    paths: ["a/"]
`

func writeConfigFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "archguard.yaml")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("write tmp config: %v", err)
	}
	return path
}

func TestRuntimeOmittedAppliesAutoProfile(t *testing.T) {
	cfg, err := config.LoadConfig(writeConfigFile(t, minimalProjectYAML))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Runtime.Profile != c.RuntimeProfileAuto {
		t.Errorf("Profile = %q, want %q", cfg.Runtime.Profile, c.RuntimeProfileAuto)
	}
	if cfg.Runtime.PruneAfterTurns != 10 {
		t.Errorf("PruneAfterTurns = %d, want 10", cfg.Runtime.PruneAfterTurns)
	}
	if cfg.Runtime.KeepRecentTurns != 4 {
		t.Errorf("KeepRecentTurns = %d, want 4", cfg.Runtime.KeepRecentTurns)
	}
	if cfg.Runtime.ProviderMaxRetries != 5 {
		t.Errorf("ProviderMaxRetries = %d, want 5", cfg.Runtime.ProviderMaxRetries)
	}
	if cfg.Runtime.ProviderRetryWait != 20*time.Second {
		t.Errorf("ProviderRetryWait = %v, want 20s", cfg.Runtime.ProviderRetryWait)
	}
	if cfg.Runtime.MaxFileBytes != c.DefaultMaxReadFileBytes {
		t.Errorf("MaxFileBytes = %d, want %d", cfg.Runtime.MaxFileBytes, c.DefaultMaxReadFileBytes)
	}
	if cfg.Runtime.RespectRateLimitHeaders == nil || !*cfg.Runtime.RespectRateLimitHeaders {
		t.Errorf("RespectRateLimitHeaders should be true under auto profile")
	}
	if cfg.Runtime.AnthropicCacheTail == nil || !*cfg.Runtime.AnthropicCacheTail {
		t.Errorf("AnthropicCacheTail should be true under auto profile")
	}
}

func TestRuntimeConservativeProfileExpansion(t *testing.T) {
	body := minimalProjectYAML + `runtime:
  profile: conservative
`
	cfg, err := config.LoadConfig(writeConfigFile(t, body))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Runtime.PruneAfterTurns != 6 {
		t.Errorf("PruneAfterTurns = %d, want 6", cfg.Runtime.PruneAfterTurns)
	}
	if cfg.Runtime.ProviderMaxRetries != 7 {
		t.Errorf("ProviderMaxRetries = %d, want 7", cfg.Runtime.ProviderMaxRetries)
	}
	if cfg.Runtime.ProviderRetryWait != 30*time.Second {
		t.Errorf("ProviderRetryWait = %v, want 30s", cfg.Runtime.ProviderRetryWait)
	}
	if cfg.Runtime.RespectRateLimitHeaders == nil || !*cfg.Runtime.RespectRateLimitHeaders {
		t.Errorf("conservative profile must keep RespectRateLimitHeaders=true")
	}
}

func TestRuntimeAggressiveDisablesRateLimitHeaders(t *testing.T) {
	body := minimalProjectYAML + `runtime:
  profile: aggressive
`
	cfg, err := config.LoadConfig(writeConfigFile(t, body))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Runtime.ProviderMaxRetries != 3 {
		t.Errorf("ProviderMaxRetries = %d, want 3", cfg.Runtime.ProviderMaxRetries)
	}
	if cfg.Runtime.ProviderRetryWait != 10*time.Second {
		t.Errorf("ProviderRetryWait = %v, want 10s", cfg.Runtime.ProviderRetryWait)
	}
	if cfg.Runtime.RespectRateLimitHeaders == nil {
		t.Fatal("RespectRateLimitHeaders should not be nil after profile expansion")
	}
	if *cfg.Runtime.RespectRateLimitHeaders {
		t.Errorf("aggressive profile should set RespectRateLimitHeaders=false")
	}
}

func TestRuntimeFieldLevelOverrideWinsOverPreset(t *testing.T) {
	body := minimalProjectYAML + `runtime:
  profile: conservative
  provider_retry_wait: 60s
`
	cfg, err := config.LoadConfig(writeConfigFile(t, body))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Runtime.ProviderRetryWait != 60*time.Second {
		t.Errorf("ProviderRetryWait = %v, want 60s (override should win)", cfg.Runtime.ProviderRetryWait)
	}
	if cfg.Runtime.ProviderMaxRetries != 7 {
		t.Errorf("ProviderMaxRetries = %d, want 7 (preset must survive non-overridden field)", cfg.Runtime.ProviderMaxRetries)
	}
}

func TestRuntimeBooleanPointerOverrides(t *testing.T) {
	body := minimalProjectYAML + `runtime:
  profile: auto
  anthropic_cache_tail: false
`
	cfg, err := config.LoadConfig(writeConfigFile(t, body))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Runtime.AnthropicCacheTail == nil {
		t.Fatal("AnthropicCacheTail should not be nil after profile expansion")
	}
	if *cfg.Runtime.AnthropicCacheTail {
		t.Errorf("AnthropicCacheTail = true, want false (explicit override)")
	}
	if cfg.Runtime.RespectRateLimitHeaders == nil || !*cfg.Runtime.RespectRateLimitHeaders {
		t.Errorf("RespectRateLimitHeaders should remain true (from auto preset)")
	}
}

func TestRuntimeProfileUnknownRejected(t *testing.T) {
	body := minimalProjectYAML + `runtime:
  profile: yolo
`
	_, err := config.LoadConfig(writeConfigFile(t, body))
	if err == nil {
		t.Fatal("expected validation error for unknown profile, got nil")
	}
	if !apperrors.IsKind(err, apperrors.KindValidation) {
		t.Errorf("expected KindValidation, got %v", err)
	}
}

func TestRuntimeNegativeRetriesRejected(t *testing.T) {
	body := minimalProjectYAML + `runtime:
  provider_max_retries: -1
`
	_, err := config.LoadConfig(writeConfigFile(t, body))
	if err == nil {
		t.Fatal("expected validation error for negative retries, got nil")
	}
	if !apperrors.IsKind(err, apperrors.KindValidation) {
		t.Errorf("expected KindValidation, got %v", err)
	}
}

func TestRuntimeNegativeRetryWaitRejected(t *testing.T) {
	body := minimalProjectYAML + `runtime:
  provider_retry_wait: -5s
`
	_, err := config.LoadConfig(writeConfigFile(t, body))
	if err == nil {
		t.Fatal("expected validation error for negative retry_wait, got nil")
	}
	if !apperrors.IsKind(err, apperrors.KindValidation) {
		t.Errorf("expected KindValidation, got %v", err)
	}
}

func TestRuntimePartialFieldOverridesUseAutoProfile(t *testing.T) {
	body := minimalProjectYAML + `runtime:
  max_file_bytes: 8192
`
	cfg, err := config.LoadConfig(writeConfigFile(t, body))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Runtime.Profile != c.RuntimeProfileAuto {
		t.Errorf("Profile = %q, want auto (default when omitted)", cfg.Runtime.Profile)
	}
	if cfg.Runtime.MaxFileBytes != 8192 {
		t.Errorf("MaxFileBytes = %d, want 8192", cfg.Runtime.MaxFileBytes)
	}
	if cfg.Runtime.ProviderMaxRetries != 5 {
		t.Errorf("ProviderMaxRetries = %d, want 5 (auto preset)", cfg.Runtime.ProviderMaxRetries)
	}
}
