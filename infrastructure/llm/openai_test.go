package llm

import (
	"testing"

	c "github.com/archguard/project/shared/constants"
)

func TestOpenAIFlattensSystemBlocks(t *testing.T) {
	client := &OpenAIClient{model: "gpt-test"}
	req := Request{
		System: []SystemBlock{
			NewSystemBlock("alpha ", false),
			NewSystemBlock("beta", true),
		},
		Messages: []Message{
			{Role: c.RoleUser, Content: []ContentBlock{NewTextBlock("hi")}},
		},
	}
	or := client.toOpenAI(req)
	if len(or.Messages) == 0 || or.Messages[0].Role != c.RoleSystem {
		t.Fatalf("expected first message to be a system message, got %#v", or.Messages)
	}
	if or.Messages[0].Content != "alpha beta" {
		t.Fatalf("system content = %q, want concatenated %q", or.Messages[0].Content, "alpha beta")
	}
}

func TestOpenAIDecodesCachedTokens(t *testing.T) {
	cachedContent := "ok"
	resp := oResponse{
		Choices: []struct {
			Message struct {
				Role      string      `json:"role"`
				Content   *string     `json:"content"`
				ToolCalls []oToolCall `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			{
				Message: struct {
					Role      string      `json:"role"`
					Content   *string     `json:"content"`
					ToolCalls []oToolCall `json:"tool_calls"`
				}{Role: c.RoleAssistant, Content: &cachedContent},
				FinishReason: "stop",
			},
		},
		Usage: &oUsage{
			PromptTokens:        500,
			CompletionTokens:    50,
			PromptTokensDetails: &oPromptTokenDtls{CachedTokens: 300},
		},
	}
	client := &OpenAIClient{model: "gpt-test"}
	out := client.fromOpenAI(resp)
	if out.Usage.InputTokens != 500 {
		t.Fatalf("InputTokens = %d, want 500", out.Usage.InputTokens)
	}
	if out.Usage.CacheReadTokens != 300 {
		t.Fatalf("CacheReadTokens = %d, want 300", out.Usage.CacheReadTokens)
	}
}
