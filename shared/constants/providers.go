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
)

const (
	GeminiBaseURL    = "https://generativelanguage.googleapis.com/v1beta/models"
	GeminiModel      = "gemini-2.5-flash"
	GeminiTimeout    = 120 * time.Second
	GeminiMaxRetries = 5
	GeminiRetryWait  = 15 * time.Second
)

const (
	OpenAIBaseURL    = "https://api.openai.com/v1/chat/completions"
	OpenAIModel      = "gpt-4o"
	OpenAITimeout    = 120 * time.Second
	OpenAIMaxRetries = 5
	OpenAIRetryWait  = 15 * time.Second
)

const (
	CacheControlEphemeral = "ephemeral"
)

const (
	AnthropicInputTPM = 30000
	GeminiInputTPM    = 60000
	OpenAIInputTPM    = 30000
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
