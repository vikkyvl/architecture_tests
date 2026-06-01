package llm

import (
	"testing"
)

func TestGeminiFlattensSystemBlocks(t *testing.T) {
	client := &GeminiClient{model: "gemini-test"}
	req := Request{
		System: []SystemBlock{
			NewSystemBlock("you are ", false),
			NewSystemBlock("an expert", true),
		},
	}
	gr := client.toGemini(req)
	if gr.SystemInstruction == nil {
		t.Fatalf("expected system instruction set")
	}
	if len(gr.SystemInstruction.Parts) != 1 {
		t.Fatalf("expected single flattened part, got %d", len(gr.SystemInstruction.Parts))
	}
	if got := gr.SystemInstruction.Parts[0].Text; got != "you are an expert" {
		t.Fatalf("flattened text = %q, want %q", got, "you are an expert")
	}
}

func TestGeminiDecodesCachedContentTokens(t *testing.T) {
	client := &GeminiClient{model: "gemini-test"}
	gr := gResponse{
		UsageMetadata: &gUsageMetadata{
			PromptTokenCount:        800,
			CandidatesTokenCount:    60,
			CachedContentTokenCount: 600,
		},
	}
	out := client.fromGemini(gr)
	if out.Usage.InputTokens != 800 {
		t.Fatalf("InputTokens = %d, want 800", out.Usage.InputTokens)
	}
	if out.Usage.CacheReadTokens != 600 {
		t.Fatalf("CacheReadTokens = %d, want 600", out.Usage.CacheReadTokens)
	}
}
