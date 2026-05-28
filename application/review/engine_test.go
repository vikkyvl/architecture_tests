package review

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/archguard/project/infrastructure/llm"
	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/models"
)

type fakeProvider struct {
	requests  []llm.Request
	responses []*llm.Response
}

func (f *fakeProvider) SendMessage(req llm.Request) (*llm.Response, error) {
	f.requests = append(f.requests, req)
	if len(f.responses) == 0 {
		return &llm.Response{StopReason: c.StopReasonEndTurn}, nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func (f *fakeProvider) Model() string {
	return "fake-model"
}

func (f *fakeProvider) Name() string {
	return "fake"
}

type fakeTools struct {
	calls       []string
	sourceFiles []string
	violations  []models.Violation
}

func (f *fakeTools) ListTools() []models.ToolDefinition {
	return []models.ToolDefinition{
		{Name: c.ToolReadFile, InputSchema: json.RawMessage(`{"type":"object"}`)},
		{Name: c.ToolGetExternalContext, InputSchema: json.RawMessage(`{"type":"object"}`)},
	}
}

func (f *fakeTools) ExecuteTool(name string, args map[string]interface{}, budget int) (string, error) {
	f.calls = append(f.calls, name)
	return "file contents", nil
}

func (f *fakeTools) GetViolations() []models.Violation {
	return f.violations
}

func (f *fakeTools) ListSourceFiles() []string {
	return f.sourceFiles
}

type fakeAudit struct {
	entries []models.AuditEntry
}

func (f fakeAudit) Entries() []models.AuditEntry {
	return f.entries
}

type captureObserver struct {
	toolResults int
	completed   bool
}

func (captureObserver) LimitReached(string) {
	return
}

func (captureObserver) Retry(string, int, int, time.Duration, error) {
	return
}

func (captureObserver) LLMText(string) {
	return
}
func (o *captureObserver) ToolResult(int, string, int, error) {
	o.toolResults++
}
func (o *captureObserver) AnalysisComplete() {
	o.completed = true
}

func TestEngineRunsToolUseLoopAndBuildsResult(t *testing.T) {
	toolInput := json.RawMessage(`{"path":"src/App.php"}`)
	provider := &fakeProvider{
		responses: []*llm.Response{
			{
				StopReason: c.StopReasonToolUse,
				Content: []llm.ContentBlock{
					{
						Type:  c.ContentTypeToolUse,
						ID:    "tool-1",
						Name:  c.ToolReadFile,
						Input: toolInput,
					},
				},
			},
			{
				StopReason: c.StopReasonEndTurn,
				Content: []llm.ContentBlock{
					llm.NewTextBlock("done"),
				},
			},
		},
	}
	tools := &fakeTools{
		sourceFiles: []string{
			"src/App.php",
			"src/Other.php",
		},
	}
	audit := fakeAudit{
		entries: []models.AuditEntry{
			{
				ToolName:  c.ToolReadFile,
				Arguments: `{"path":"src/App.php"}`,
			},
		},
	}
	observer := &captureObserver{}

	result, err := NewEngine(provider, tools, audit, observer).Run(Options{
		SystemPrompt: "system", MaxToolCalls: 5, Timeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if result.ToolCalls != 1 {
		t.Fatalf("expected one tool call, got %d", result.ToolCalls)
	}
	if len(tools.calls) != 1 || tools.calls[0] != c.ToolReadFile {
		t.Fatalf("expected read_file execution, got %v", tools.calls)
	}
	if len(result.AnalyzedModules) != 1 || result.AnalyzedModules[0] != "src/App.php" {
		t.Fatalf("expected analyzed module from audit log, got %v", result.AnalyzedModules)
	}
	if observer.toolResults != 1 || !observer.completed {
		t.Fatalf("expected tool result and completion events, got results=%d completed=%v", observer.toolResults, observer.completed)
	}
}

type recordingAudit struct {
	turns []models.AuditEntry
}

func (r *recordingAudit) Entries() []models.AuditEntry {
	return r.turns
}

func (r *recordingAudit) RecordLLMTurn(input, cached, cacheWrite, pruned int) {
	r.turns = append(r.turns, models.AuditEntry{
		ToolName:              c.EventLLMTurn,
		InputTokens:           input,
		CachedInputTokens:     cached,
		CacheWriteInputTokens: cacheWrite,
		PrunedTurns:           pruned,
	})
}

func TestPrunePressureAwareSkipsBelowSoftLimit(t *testing.T) {
	messages := buildSyntheticConversation(30)
	pruned := prunePressureAware(messages, 100, 1000, keepRecentTurns)
	if pruned != 0 {
		t.Fatalf("expected no pruning below soft limit, got %d", pruned)
	}
	for _, m := range messages {
		for _, b := range m.Content {
			if b.Content == prunedPlaceholder {
				t.Fatal("no message should be mutated when projected < soft limit")
			}
		}
	}
}

func TestPrunePressureAwareElidesLargestFirst(t *testing.T) {
	messages := buildSyntheticConversation(30)
	keepFromIdx := len(messages) - keepRecentTurns*2
	messages[2].Content[0].Content = "small"
	messages[4].Content[0].Content = strings.Repeat("HUGE", 2000)
	messages[6].Content[0].Content = strings.Repeat("MID", 500)

	pruned := prunePressureAware(messages, 100000, 1000, keepRecentTurns)
	if pruned != maxElisionsPerCall {
		t.Fatalf("expected exactly %d elisions per call, got %d", maxElisionsPerCall, pruned)
	}
	if messages[4].Content[0].Content != prunedPlaceholder {
		t.Errorf("largest tool_result (msg[4]) must be elided, got %q",
			head(messages[4].Content[0].Content, 60))
	}
	if messages[6].Content[0].Content != prunedPlaceholder {
		t.Errorf("second-largest tool_result (msg[6]) must be elided, got %q",
			head(messages[6].Content[0].Content, 60))
	}
	if messages[2].Content[0].Content == prunedPlaceholder {
		t.Errorf("smallest tool_result must survive elision pass")
	}
	if keepFromIdx <= 6 {
		t.Errorf("synthetic conversation too short to validate keep window: keepFromIdx=%d", keepFromIdx)
	}
}

func TestPrunePressureAwareSkipsCacheMarked(t *testing.T) {
	messages := buildSyntheticConversation(30)
	for i := 1; i < len(messages); i++ {
		for j := range messages[i].Content {
			if messages[i].Content[j].Type == c.ContentTypeToolResult {
				messages[i].Content[j].CacheControl = &llm.CacheControl{Type: c.CacheControlEphemeral}
			}
		}
	}
	pruned := prunePressureAware(messages, 100000, 1000, keepRecentTurns)
	if pruned != 0 {
		t.Fatalf("all blocks cache-marked → must not prune, got %d", pruned)
	}
}

func TestPrunePressureAwareCapsAtTwoPerTurn(t *testing.T) {
	messages := buildSyntheticConversation(30)
	for i := 1; i < len(messages)-keepRecentTurns*2; i++ {
		for j := range messages[i].Content {
			if messages[i].Content[j].Type == c.ContentTypeToolResult {
				messages[i].Content[j].Content = strings.Repeat("X", 5000+i)
			}
		}
	}
	pruned := prunePressureAware(messages, 100000, 1000, keepRecentTurns)
	if pruned > maxElisionsPerCall {
		t.Fatalf("must elide at most %d per call, got %d", maxElisionsPerCall, pruned)
	}
}

func TestPrunePressureAwareKeepsRecentWindow(t *testing.T) {
	messages := buildSyntheticConversation(30)
	keepFromIdx := len(messages) - keepRecentTurns*2
	for i := keepFromIdx; i < len(messages); i++ {
		for j := range messages[i].Content {
			if messages[i].Content[j].Type == c.ContentTypeToolResult {
				messages[i].Content[j].Content = strings.Repeat("Z", 10000)
			}
		}
	}
	_ = prunePressureAware(messages, 100000, 1000, keepRecentTurns)
	for i := keepFromIdx; i < len(messages); i++ {
		for _, b := range messages[i].Content {
			if b.Type == c.ContentTypeToolResult && b.Content == prunedPlaceholder {
				t.Fatalf("msg[%d] in keep window unexpectedly pruned", i)
			}
		}
	}
}

func TestProviderSoftLimitDefaultsByName(t *testing.T) {
	cases := []struct {
		name string
		want int
	}{
		{c.ProviderAnthropic, 2 * c.AnthropicInputTPM / 3},
		{c.ProviderGemini, 2 * c.GeminiInputTPM / 3},
		{c.ProviderOpenAI, 2 * c.OpenAIInputTPM / 3},
		{"unknown", defaultSoftLimitTokens},
	}
	for _, tc := range cases {
		if got := providerSoftLimit(tc.name); got != tc.want {
			t.Errorf("providerSoftLimit(%q) = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func TestEngineEmitsLLMTurnAuditEntries(t *testing.T) {
	provider := &fakeProvider{
		responses: []*llm.Response{
			{
				StopReason: c.StopReasonEndTurn,
				Content:    []llm.ContentBlock{llm.NewTextBlock("done")},
				Usage:      llm.Usage{InputTokens: 1200, CacheReadTokens: 900},
			},
		},
	}
	tools := new(fakeTools)
	audit := &recordingAudit{}

	_, err := NewEngine(provider, tools, audit, nil).Run(Options{
		SystemPrompt: "system", MaxToolCalls: 5, Timeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(audit.turns) != 1 {
		t.Fatalf("expected 1 llm-turn audit entry, got %d", len(audit.turns))
	}
	entry := audit.turns[0]
	if entry.ToolName != c.EventLLMTurn {
		t.Fatalf("turn entry ToolName = %q, want %q", entry.ToolName, c.EventLLMTurn)
	}
	if entry.InputTokens != 1200 || entry.CachedInputTokens != 900 {
		t.Fatalf("turn entry tokens = %d/%d, want 1200/900", entry.InputTokens, entry.CachedInputTokens)
	}
}

func TestEngineSendsSystemAsBlockSlice(t *testing.T) {
	provider := &fakeProvider{
		responses: []*llm.Response{
			{StopReason: c.StopReasonEndTurn},
		},
	}
	tools := new(fakeTools)

	_, err := NewEngine(provider, tools, fakeAudit{}, nil).Run(Options{
		SystemPrompt: "system-prompt-content", MaxToolCalls: 5, Timeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(provider.requests))
	}
	sys := provider.requests[0].System
	if len(sys) != 1 || sys[0].Text != "system-prompt-content" {
		t.Fatalf("system blocks = %#v, want single text block", sys)
	}
}

func TestEngineUsesAnalyzerMaxTokens(t *testing.T) {
	provider := &fakeProvider{
		responses: []*llm.Response{{StopReason: c.StopReasonEndTurn}},
	}
	_, err := NewEngine(provider, new(fakeTools), fakeAudit{}, nil).Run(Options{
		SystemPrompt: "system", MaxToolCalls: 5, Timeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(provider.requests) == 0 {
		t.Fatal("expected at least one provider request")
	}
	got := provider.requests[0].MaxTokens
	if got != c.AnalyzerMaxTokens {
		t.Fatalf("analyzer must request MaxTokens=%d (AnalyzerMaxTokens), got %d", c.AnalyzerMaxTokens, got)
	}
	if got == c.DefaultMaxTokens {
		t.Fatalf("analyzer must not fall back to DefaultMaxTokens=%d", c.DefaultMaxTokens)
	}
}

func TestEngineInitialPromptRequiresExhaustiveCoverage(t *testing.T) {
	provider := &fakeProvider{
		responses: []*llm.Response{
			{StopReason: c.StopReasonEndTurn},
		},
	}
	tools := new(fakeTools)

	_, err := NewEngine(provider, tools, fakeAudit{}, nil).Run(Options{
		SystemPrompt: "system", MaxToolCalls: 5, Timeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(provider.requests))
	}
	text := provider.requests[0].Messages[0].Content[0].Text
	if strings.Contains(text, "small, targeted batches") {
		t.Fatalf("initial prompt must not encourage sampling: %q", text)
	}
	if !strings.Contains(text, "EVERY") {
		t.Fatalf("initial prompt must require exhaustive coverage: %q", text)
	}
}

func TestRequestPaceWaitScalesWithProjectedTokens(t *testing.T) {
	tests := []struct {
		name   string
		tokens int
		want   time.Duration
	}{
		{name: "small", tokens: mediumRequestTokens - 1, want: smallRequestSpacing},
		{name: "medium", tokens: mediumRequestTokens, want: mediumRequestSpacing},
		{name: "large", tokens: largeRequestTokens, want: largeRequestSpacing},
		{name: "very large", tokens: veryLargeRequestTokens, want: veryLargeRequestSpacing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := requestPaceWait(tt.tokens, "fake"); got != tt.want {
				t.Fatalf("requestPaceWait(%d, fake) = %s, want %s", tt.tokens, got, tt.want)
			}
		})
	}
}

func buildSyntheticConversation(turns int) []llm.Message {
	messages := []llm.Message{
		{
			Role:    c.RoleUser,
			Content: []llm.ContentBlock{llm.NewTextBlock("init")},
		},
	}
	for i := 0; i < turns; i++ {
		id := "tool-" + itoa(i)
		messages = append(messages, llm.Message{
			Role: c.RoleAssistant,
			Content: []llm.ContentBlock{
				{Type: c.ContentTypeToolUse, ID: id, Name: c.ToolReadFile, Input: json.RawMessage(`{}`)},
			},
		})
		messages = append(messages, llm.Message{
			Role: c.RoleUser,
			Content: []llm.ContentBlock{
				llm.NewToolResult(id, "long file contents that consume tokens", false),
			},
		})
	}
	return messages
}

func collectToolUseIDs(messages []llm.Message) map[string]bool {
	ids := make(map[string]bool)
	for _, m := range messages {
		for _, b := range m.Content {
			if b.Type == c.ContentTypeToolUse || b.Type == c.ContentTypeToolResult {
				if b.ID != "" {
					ids[b.ID] = true
				}
				if b.ToolUseID != "" {
					ids[b.ToolUseID] = true
				}
			}
		}
	}
	return ids
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func TestEngineDoesNotExposeExternalContextAsLLMTool(t *testing.T) {
	provider := &fakeProvider{
		responses: []*llm.Response{
			{
				StopReason: c.StopReasonEndTurn,
			},
		},
	}
	tools := new(fakeTools)

	var audit fakeAudit

	_, err := NewEngine(provider, tools, audit, nil).Run(Options{
		SystemPrompt: "system", MaxToolCalls: 5, Timeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("expected one provider request, got %d", len(provider.requests))
	}
	for _, tool := range provider.requests[0].Tools {
		if tool.Name == c.ToolGetExternalContext {
			t.Fatalf("external context tool should be preloaded by CLI, not exposed to LLM")
		}
	}
}
