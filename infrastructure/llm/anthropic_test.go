package llm

import (
	"encoding/json"
	"strings"
	"testing"

	c "github.com/archguard/project/shared/constants"
)

func buildTwoTurnRequest() Request {
	return Request{
		Messages: []Message{
			{
				Role: c.RoleUser,
				Content: []ContentBlock{
					{Type: c.ContentTypeText, Text: "hi"},
				},
			},
			{
				Role: c.RoleAssistant,
				Content: []ContentBlock{
					{Type: c.ContentTypeToolUse, ID: "tu1", Name: c.ToolReadFile, Input: json.RawMessage(`{}`)},
				},
			},
			{
				Role: c.RoleUser,
				Content: []ContentBlock{
					{Type: c.ContentTypeToolResult, ToolUseID: "tu1", Content: "file contents"},
				},
			},
		},
	}
}

func TestMarkCacheBreakpointsSetsLastSystemAndTool(t *testing.T) {
	req := Request{
		System: []SystemBlock{
			NewSystemBlock("first", false),
			NewSystemBlock("second", false),
		},
		Tools: []Tool{
			{Name: "a", InputSchema: json.RawMessage(`{}`)},
			{Name: "b", InputSchema: json.RawMessage(`{}`)},
		},
	}
	markCacheBreakpoints(&req)
	if req.System[0].CacheControl != nil {
		t.Fatalf("first system block must not be cached")
	}
	if req.System[1].CacheControl == nil || req.System[1].CacheControl.Type != c.CacheControlEphemeral {
		t.Fatalf("last system block must carry ephemeral cache marker, got %#v", req.System[1].CacheControl)
	}
	if req.Tools[0].CacheControl != nil {
		t.Fatalf("first tool must not be cached")
	}
	if req.Tools[1].CacheControl == nil || req.Tools[1].CacheControl.Type != c.CacheControlEphemeral {
		t.Fatalf("last tool must carry ephemeral cache marker, got %#v", req.Tools[1].CacheControl)
	}
}

func TestMarkTailCacheBreakpointSetsLastToolResult(t *testing.T) {
	req := buildTwoTurnRequest()
	markTailCacheBreakpoint(&req)

	lastUserMsg := req.Messages[len(req.Messages)-1]
	last := lastUserMsg.Content[len(lastUserMsg.Content)-1]
	if last.CacheControl == nil || last.CacheControl.Type != c.CacheControlEphemeral {
		t.Fatalf("expected ephemeral cache marker on tail tool_result, got %#v", last.CacheControl)
	}

	for i := 0; i < len(req.Messages)-1; i++ {
		for _, b := range req.Messages[i].Content {
			if b.CacheControl != nil {
				t.Errorf("only the tail tool_result should be marked, got marker on msg[%d] block type=%q", i, b.Type)
			}
		}
	}
}

func TestMarkTailCacheBreakpointSkipsOnFirstTurn(t *testing.T) {
	req := Request{
		Messages: []Message{
			{
				Role: c.RoleUser,
				Content: []ContentBlock{
					{Type: c.ContentTypeText, Text: "analyze this"},
				},
			},
		},
	}
	markTailCacheBreakpoint(&req)
	for _, b := range req.Messages[0].Content {
		if b.CacheControl != nil {
			t.Errorf("first-turn message must not carry a cache marker, got %#v", b.CacheControl)
		}
	}
}

func TestMarkTailCacheBreakpointSkipsWhenNoToolResult(t *testing.T) {
	req := Request{
		Messages: []Message{
			{Role: c.RoleUser, Content: []ContentBlock{{Type: c.ContentTypeText, Text: "hi"}}},
			{Role: c.RoleAssistant, Content: []ContentBlock{{Type: c.ContentTypeText, Text: "ok"}}},
		},
	}
	markTailCacheBreakpoint(&req)
	for i, msg := range req.Messages {
		for _, b := range msg.Content {
			if b.CacheControl != nil {
				t.Errorf("msg[%d] must not carry cache marker (no tool_result in conversation)", i)
			}
		}
	}
}

func buildMultiTurnRequestWithStaleMarkers(turns int) Request {
	req := Request{
		Messages: []Message{
			{Role: c.RoleUser, Content: []ContentBlock{{Type: c.ContentTypeText, Text: "go"}}},
		},
	}
	for i := 0; i < turns; i++ {
		toolID := "tu" + string(rune('A'+i))
		req.Messages = append(req.Messages,
			Message{Role: c.RoleAssistant, Content: []ContentBlock{
				{Type: c.ContentTypeToolUse, ID: toolID, Name: c.ToolReadFile, Input: json.RawMessage(`{}`)},
			}},
			Message{Role: c.RoleUser, Content: []ContentBlock{
				{Type: c.ContentTypeToolResult, ToolUseID: toolID, Content: "payload",
					CacheControl: &CacheControl{Type: c.CacheControlEphemeral}},
			}},
		)
	}
	return req
}

func TestMarkTailCacheBreakpointClearsPriorMarkers(t *testing.T) {
	req := buildMultiTurnRequestWithStaleMarkers(5)
	markTailCacheBreakpoint(&req)

	marked := 0
	for _, msg := range req.Messages {
		for _, b := range msg.Content {
			if b.Type == c.ContentTypeToolResult && b.CacheControl != nil {
				marked++
			}
		}
	}
	if marked != 1 {
		t.Fatalf("expected exactly 1 tool_result with cache_control after marking, got %d", marked)
	}

	lastUserMsg := req.Messages[len(req.Messages)-1]
	last := lastUserMsg.Content[len(lastUserMsg.Content)-1]
	if last.CacheControl == nil || last.CacheControl.Type != c.CacheControlEphemeral {
		t.Fatalf("tail tool_result must be the one carrying the marker, got %#v", last.CacheControl)
	}
}

func TestMarkMidPrefixCacheBreakpoint(t *testing.T) {
	req := Request{
		Messages: []Message{
			{Role: c.RoleUser, Content: []ContentBlock{{Type: c.ContentTypeText, Text: "hi"}}},
			{Role: c.RoleAssistant, Content: []ContentBlock{
				{Type: c.ContentTypeToolUse, ID: "tu1", Name: c.ToolReadFile, Input: json.RawMessage(`{}`)},
			}},
			{Role: c.RoleUser, Content: []ContentBlock{
				{Type: c.ContentTypeToolResult, ToolUseID: "tu1", Content: "x"},
				{Type: c.ContentTypeToolResult, ToolUseID: "tu2", Content: "y"},
			}},
			{Role: c.RoleAssistant, Content: []ContentBlock{
				{Type: c.ContentTypeToolUse, ID: "tu3", Name: c.ToolReadFile, Input: json.RawMessage(`{}`)},
			}},
			{Role: c.RoleUser, Content: []ContentBlock{
				{Type: c.ContentTypeToolResult, ToolUseID: "tu3", Content: "z"},
			}},
		},
	}
	markMidPrefixCacheBreakpoint(&req)

	first := req.Messages[2].Content[0]
	if first.CacheControl == nil || first.CacheControl.Type != c.CacheControlEphemeral {
		t.Fatalf("first tool_result of first post-assistant user message must be marked, got %#v", first.CacheControl)
	}
	if req.Messages[2].Content[1].CacheControl != nil {
		t.Errorf("only the first tool_result of the mid-prefix turn should be marked")
	}
	if req.Messages[4].Content[0].CacheControl != nil {
		t.Errorf("later user turns must not be touched by mid-prefix marker, got %#v",
			req.Messages[4].Content[0].CacheControl)
	}
}

func TestMarkMidPrefixCacheBreakpointShortConversation(t *testing.T) {
	req := Request{Messages: []Message{
		{Role: c.RoleUser, Content: []ContentBlock{{Type: c.ContentTypeText, Text: "hi"}}},
		{Role: c.RoleAssistant, Content: []ContentBlock{{Type: c.ContentTypeText, Text: "ok"}}},
	}}
	markMidPrefixCacheBreakpoint(&req)
	for i, msg := range req.Messages {
		for _, b := range msg.Content {
			if b.CacheControl != nil {
				t.Errorf("msg[%d] must not be marked on short conversation", i)
			}
		}
	}
}

func TestCacheMarkersFitFourBudget(t *testing.T) {
	req := buildMultiTurnRequestWithStaleMarkers(10)
	req.System = []SystemBlock{NewSystemBlock("sys", false)}
	req.Tools = []Tool{{Name: "t", InputSchema: json.RawMessage(`{}`)}}

	markCacheBreakpoints(&req)
	markMidPrefixCacheBreakpoint(&req)
	markTailCacheBreakpoint(&req)

	total := 0
	for _, sb := range req.System {
		if sb.CacheControl != nil {
			total++
		}
	}
	for _, tool := range req.Tools {
		if tool.CacheControl != nil {
			total++
		}
	}
	for _, msg := range req.Messages {
		for _, b := range msg.Content {
			if b.CacheControl != nil {
				total++
			}
		}
	}
	if total > 4 {
		t.Fatalf("total cache_control markers must fit within the 4-block API budget, got %d", total)
	}
}

func TestContentBlockMarshalIncludesCacheControl(t *testing.T) {
	block := ContentBlock{
		Type:         c.ContentTypeToolResult,
		ToolUseID:    "tu1",
		Content:      "x",
		CacheControl: &CacheControl{Type: c.CacheControlEphemeral},
	}
	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"cache_control"`) {
		t.Errorf("missing cache_control key in %s", data)
	}
	if !strings.Contains(string(data), c.CacheControlEphemeral) {
		t.Errorf("missing %q in %s", c.CacheControlEphemeral, data)
	}
}

func TestContentBlockMarshalOmitsCacheControlWhenNil(t *testing.T) {
	block := ContentBlock{
		Type:      c.ContentTypeToolResult,
		ToolUseID: "tu1",
		Content:   "x",
	}
	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), `"cache_control"`) {
		t.Errorf("cache_control must be omitted when nil, got %s", data)
	}
}

func TestAnthropicCacheTailGatedByOption(t *testing.T) {
	enabled, err := NewAnthropicClientWithOptions("k", "m", RuntimeOptions{AnthropicCacheTail: true}, nil)
	if err != nil {
		t.Fatalf("NewAnthropicClientWithOptions: %v", err)
	}
	disabled, err := NewAnthropicClientWithOptions("k", "m", RuntimeOptions{}, nil)
	if err != nil {
		t.Fatalf("NewAnthropicClientWithOptions: %v", err)
	}
	if !enabled.cacheTail {
		t.Errorf("opt-in client should have cacheTail=true")
	}
	if disabled.cacheTail {
		t.Errorf("default client should have cacheTail=false (back-compat)")
	}
}

func TestAnthropicUsageDecodesCacheTokens(t *testing.T) {
	payload := []byte(`{
		"id": "msg_1",
		"role": "assistant",
		"content": [],
		"stop_reason": "end_turn",
		"usage": {
			"input_tokens": 100,
			"output_tokens": 25,
			"cache_creation_input_tokens": 4000,
			"cache_read_input_tokens": 1500
		}
	}`)
	var resp Response
	if err := json.Unmarshal(payload, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Usage.CacheCreationTokens != 4000 {
		t.Fatalf("CacheCreationTokens = %d, want 4000", resp.Usage.CacheCreationTokens)
	}
	if resp.Usage.CacheReadTokens != 1500 {
		t.Fatalf("CacheReadTokens = %d, want 1500", resp.Usage.CacheReadTokens)
	}
}
