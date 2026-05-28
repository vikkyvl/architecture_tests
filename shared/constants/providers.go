package constants

import "time"

const (
	ProviderAnthropic = "anthropic"
	ProviderGemini    = "gemini"
	ProviderOpenAI    = "openai"
)

const (
	AnthropicBaseURL    = "https://api.anthropic.com/v1/messages"
	AnthropicVersion    = "2023-06-01"
	AnthropicModel      = "claude-opus-4-7"
	AnthropicTimeout    = 120 * time.Second
	AnthropicMaxRetries = 5
	AnthropicRetryWait  = 20 * time.Second

	// Tier-1 Build: 50 RPM, 200 K TPM (claude-opus-4+).
	// TPM is used as the context-pruning soft-limit denominator; RPM drives the
	// minimum inter-request interval so we never burst past the quota ceiling.
	AnthropicRPM             = 50
	AnthropicMinInterval     = 1200 * time.Millisecond // 60 s / 50 RPM
	AnthropicInputTPM        = 40000
	AnthropicMaxOutputTokens = 8192
)

const (
	GeminiBaseURL    = "https://generativelanguage.googleapis.com/v1beta/models"
	GeminiModel      = "gemini-2.5-flash"
	GeminiTimeout    = 180 * time.Second // Flash thinking can be slow
	GeminiMaxRetries = 5
	GeminiRetryWait  = 30 * time.Second // Gemini quota resets are typically 60 s windows

	// Free tier: 15 RPM, 1 M TPM.  Paid: 2 000 RPM, 4 M TPM.
	// We use the free-tier RPM as a conservative floor so the app works out-of-the-box
	// on any key.  Gemini does not expose remaining-quota response headers, so the
	// RPM-based interval is the only proactive pacing signal we have.
	GeminiRPM             = 15
	GeminiMinInterval     = 4 * time.Second // 60 s / 15 RPM
	GeminiInputTPM        = 250000          // paid tier; avoids over-pruning large contexts
	GeminiMaxOutputTokens = 8192
)

const (
	OpenAIBaseURL    = "https://api.openai.com/v1/chat/completions"
	OpenAIModel      = "gpt-4o"
	OpenAITimeout    = 120 * time.Second
	OpenAIMaxRetries = 5
	OpenAIRetryWait  = 15 * time.Second

	// Tier-1: 500 RPM, 200 K TPM (gpt-4o).
	// InputTPM is intentionally set to 30 K (not the full 200 K quota) so that
	// the 2/3 soft-limit (~20 K) triggers context pruning at a reasonable size.
	// Without this cap, gpt-4o accumulates 100 K+ contexts that cost full price
	// on every turn (no automatic caching below the 1 K prefix threshold).
	OpenAIRPM             = 500
	OpenAIMinInterval     = 120 * time.Millisecond // 60 s / 500 RPM
	OpenAIInputTPM        = 30000
	OpenAIMaxOutputTokens = 4096
)

const (
	CacheControlEphemeral = "ephemeral"
)

const (
	NotionBaseURL       = "https://api.notion.com/v1"
	NotionVersion       = "2022-06-28"
	NotionVersionHeader = "Notion-Version"
	NotionTimeout       = 30 * time.Second
	NotionMaxResults    = 3
	NotionMaxContentLen = 3000
)

const (
	EnvAnthropicKey = "ANTHROPIC_API_KEY"
	EnvGeminiKey    = "GEMINI_API_KEY"
	EnvOpenAIKey    = "OPENAI_API_KEY"
)

const (
	HeaderAnthropicRequestsRemaining = "anthropic-ratelimit-requests-remaining"
	HeaderAnthropicTokensRemaining   = "anthropic-ratelimit-input-tokens-remaining"
	HeaderAnthropicRequestsReset     = "anthropic-ratelimit-requests-reset"
	HeaderAnthropicTokensReset       = "anthropic-ratelimit-input-tokens-reset"
	HeaderOpenAIRequestsRemaining    = "x-ratelimit-remaining-requests"
	HeaderOpenAITokensRemaining      = "x-ratelimit-remaining-tokens"
	HeaderOpenAIRequestsReset        = "x-ratelimit-reset-requests"
	HeaderOpenAITokensReset          = "x-ratelimit-reset-tokens"
)
