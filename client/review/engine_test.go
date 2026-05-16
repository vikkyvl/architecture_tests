package review

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/archguard/project/api/llm"
	"github.com/archguard/project/client/mcp"
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

func (f *fakeProvider) Model() string { return "fake-model" }
func (f *fakeProvider) Name() string  { return "fake" }

type fakeTools struct {
	calls       []string
	sourceFiles []string
	violations  []models.Violation
}

func (f *fakeTools) ListTools() []mcp.ToolDefinition {
	return []mcp.ToolDefinition{
		{Name: c.ToolReadFile, InputSchema: json.RawMessage(`{"type":"object"}`)},
		{Name: c.ToolGetExternalContext, InputSchema: json.RawMessage(`{"type":"object"}`)},
	}
}

func (f *fakeTools) ExecuteTool(name string, args map[string]interface{}, budget int) (string, error) {
	f.calls = append(f.calls, name)
	return "file contents", nil
}

func (f *fakeTools) GetViolations() []models.Violation { return f.violations }
func (f *fakeTools) ListSourceFiles() []string         { return f.sourceFiles }

type fakeAudit struct {
	entries []models.AuditEntry
}

func (f fakeAudit) Entries() []models.AuditEntry { return f.entries }

type captureObserver struct {
	toolResults int
	completed   bool
}

func (captureObserver) LimitReached(string)                          {}
func (captureObserver) Retry(string, int, int, time.Duration, error) {}
func (captureObserver) LLMText(string)                               {}
func (o *captureObserver) ToolResult(int, string, int, error) {
	o.toolResults++
}
func (o *captureObserver) AnalysisComplete() {
	o.completed = true
}

func TestEngineRunsToolUseLoopAndBuildsResult(t *testing.T) {
	toolInput := json.RawMessage(`{"path":"src/App.php"}`)
	provider := &fakeProvider{responses: []*llm.Response{
		{
			StopReason: c.StopReasonToolUse,
			Content: []llm.ContentBlock{{
				Type: c.ContentTypeToolUse, ID: "tool-1",
				Name: c.ToolReadFile, Input: toolInput,
			}},
		},
		{StopReason: c.StopReasonEndTurn, Content: []llm.ContentBlock{llm.NewTextBlock("done")}},
	}}
	tools := &fakeTools{sourceFiles: []string{"src/App.php", "src/Other.php"}}
	audit := fakeAudit{entries: []models.AuditEntry{{
		ToolName:  c.ToolReadFile,
		Arguments: `{"path":"src/App.php"}`,
	}}}
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

func TestEngineDoesNotExposeExternalContextAsLLMTool(t *testing.T) {
	provider := &fakeProvider{responses: []*llm.Response{{StopReason: c.StopReasonEndTurn}}}
	tools := &fakeTools{}

	_, err := NewEngine(provider, tools, fakeAudit{}, nil).Run(Options{
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
