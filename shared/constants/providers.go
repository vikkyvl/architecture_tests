package constants

import (
	"strings"
	"time"
)

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

	AnthropicRPM             = 50
	AnthropicMinInterval     = 1200 * time.Millisecond
	AnthropicInputTPM        = 40000
	AnthropicMaxOutputTokens = 8192
)

const (
	GeminiBaseURL    = "https://generativelanguage.googleapis.com/v1beta/models"
	GeminiModel      = "gemini-2.5-flash"
	GeminiTimeout    = 180 * time.Second
	GeminiMaxRetries = 5
	GeminiRetryWait  = 30 * time.Second

	GeminiRPM             = 15
	GeminiMinInterval     = 4 * time.Second
	GeminiInputTPM        = 250000
	GeminiMaxOutputTokens = 8192
)

const (
	OpenAIBaseURL    = "https://api.openai.com/v1/chat/completions"
	OpenAIModel      = "gpt-4o"
	OpenAITimeout    = 120 * time.Second
	OpenAIMaxRetries = 5
	OpenAIRetryWait  = 15 * time.Second

	OpenAIRPM             = 500
	OpenAIMinInterval     = 120 * time.Millisecond
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
)

const (
	EnvAnthropicKey = "ANTHROPIC_API_KEY"
	EnvGeminiKey    = "GEMINI_API_KEY"
	EnvOpenAIKey    = "OPENAI_API_KEY"
)

var OpenAIReasoningPrefixes = []string{"o1", "o3", "o4", "gpt-5"}

func IsOpenAIReasoningModel(name string) bool {
	for _, prefix := range OpenAIReasoningPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

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
