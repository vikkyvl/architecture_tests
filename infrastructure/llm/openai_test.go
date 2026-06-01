package llm

import (
	"encoding/json"
	"strings"
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

func TestAssistantMessageConcatenatesMultipleTextBlocks(t *testing.T) {
	client := &OpenAIClient{model: "gpt-4o"}
	msg := Message{
		Role: c.RoleAssistant,
		Content: []ContentBlock{
			{Type: c.ContentTypeText, Text: "a"},
			{Type: c.ContentTypeText, Text: "b"},
		},
	}
	out := client.assistantMessage(msg)
	if out.Content != "ab" {
		t.Fatalf("assistant content = %q, want %q", out.Content, "ab")
	}
}

func TestToOpenAIUsesMaxCompletionTokensForReasoningModels(t *testing.T) {
	client := &OpenAIClient{model: "o3-mini"}
	or := client.toOpenAI(Request{MaxTokens: 2048})
	if or.MaxCompletionTokens != 2048 {
		t.Fatalf("MaxCompletionTokens = %d, want 2048", or.MaxCompletionTokens)
	}
	if or.MaxTokens != 0 {
		t.Fatalf("MaxTokens must be 0 for reasoning model, got %d", or.MaxTokens)
	}
}

func TestToOpenAIUsesMaxTokensForChatModels(t *testing.T) {
	client := &OpenAIClient{model: "gpt-4o"}
	or := client.toOpenAI(Request{MaxTokens: 2048})
	if or.MaxTokens != 2048 {
		t.Fatalf("MaxTokens = %d, want 2048", or.MaxTokens)
	}
	if or.MaxCompletionTokens != 0 {
		t.Fatalf("MaxCompletionTokens must be 0 for chat model, got %d", or.MaxCompletionTokens)
	}
}

func TestToOpenAIAlwaysSetsStoreFalse(t *testing.T) {
	for _, model := range []string{"gpt-4o", "o3-mini", "gpt-5"} {
		t.Run(model, func(t *testing.T) {
			client := &OpenAIClient{model: model}
			or := client.toOpenAI(Request{MaxTokens: 1024})
			if or.Store == nil {
				t.Fatalf("Store must not be nil")
			}
			if *or.Store {
				t.Fatalf("Store must be false")
			}
			body, err := json.Marshal(or)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !strings.Contains(string(body), `"store":false`) {
				t.Errorf("wire body must contain \"store\":false, got %s", body)
			}
		})
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
