package config

import (
	"fmt"
	"time"

	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
)

const (
	errUnknownProfile       = "runtime.profile must be one of: auto, conservative, aggressive (got %q)"
	errNegativeMaxFileBytes = "runtime.max_file_bytes must be >= 0"
	errNegativePruneAfter   = "runtime.prune_after_turns must be >= 0"
	errNegativeKeepRecent   = "runtime.keep_recent_turns must be >= 0"
	errNegativeMaxRetries   = "runtime.provider_max_retries must be >= 0"
	errNegativeRetryWait    = "runtime.provider_retry_wait must be >= 0"
	defaultProfile          = c.RuntimeProfileAuto
)

type RuntimeConfig struct {
	Profile                 string        `yaml:"profile"`
	RespectRateLimitHeaders *bool         `yaml:"respect_rate_limit_headers"`
	AnthropicCacheTail      *bool         `yaml:"anthropic_cache_tail"`
	MaxFileBytes            int           `yaml:"max_file_bytes"`
	PruneAfterTurns         int           `yaml:"prune_after_turns"`
	KeepRecentTurns         int           `yaml:"keep_recent_turns"`
	ProviderMaxRetries      int           `yaml:"provider_max_retries"`
	ProviderRetryWait       time.Duration `yaml:"provider_retry_wait"`
}

type runtimePreset struct {
	pruneAfter     int
	keepRecent     int
	maxRetries     int
	retryWait      time.Duration
	cacheTail      bool
	respectHeaders bool
}

var runtimePresets = map[string]runtimePreset{
	c.RuntimeProfileAggressive: {
		pruneAfter:     10,
		keepRecent:     4,
		maxRetries:     3,
		retryWait:      10 * time.Second,
		cacheTail:      true,
		respectHeaders: false,
	},
	c.RuntimeProfileAuto: {
		pruneAfter:     10,
		keepRecent:     4,
		maxRetries:     5,
		retryWait:      20 * time.Second,
		cacheTail:      true,
		respectHeaders: true,
	},
	c.RuntimeProfileConservative: {
		pruneAfter:     6,
		keepRecent:     4,
		maxRetries:     7,
		retryWait:      30 * time.Second,
		cacheTail:      true,
		respectHeaders: true,
	},
}

func applyRuntimeProfile(rc *RuntimeConfig) error {
	if rc.Profile == "" {
		rc.Profile = defaultProfile
	}
	preset, ok := runtimePresets[rc.Profile]
	if !ok {
		return apperrors.Validation(fmt.Sprintf(errUnknownProfile, rc.Profile))
	}
	if rc.PruneAfterTurns == 0 {
		rc.PruneAfterTurns = preset.pruneAfter
	}
	if rc.KeepRecentTurns == 0 {
		rc.KeepRecentTurns = preset.keepRecent
	}
	if rc.ProviderMaxRetries == 0 {
		rc.ProviderMaxRetries = preset.maxRetries
	}
	if rc.ProviderRetryWait == 0 {
		rc.ProviderRetryWait = preset.retryWait
	}
	if rc.MaxFileBytes == 0 {
		rc.MaxFileBytes = c.DefaultMaxReadFileBytes
	}
	if rc.AnthropicCacheTail == nil {
		v := preset.cacheTail
		rc.AnthropicCacheTail = &v
	}
	if rc.RespectRateLimitHeaders == nil {
		v := preset.respectHeaders
		rc.RespectRateLimitHeaders = &v
	}
	return nil
}

func validateRuntime(rc *RuntimeConfig) error {
	if rc.MaxFileBytes < 0 {
		return apperrors.Validation(errNegativeMaxFileBytes)
	}
	if rc.PruneAfterTurns < 0 {
		return apperrors.Validation(errNegativePruneAfter)
	}
	if rc.KeepRecentTurns < 0 {
		return apperrors.Validation(errNegativeKeepRecent)
	}
	if rc.ProviderMaxRetries < 0 {
		return apperrors.Validation(errNegativeMaxRetries)
	}
	if rc.ProviderRetryWait < 0 {
		return apperrors.Validation(errNegativeRetryWait)
	}
	return nil
}
