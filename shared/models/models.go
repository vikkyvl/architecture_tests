package models

import (
	"encoding/json"
	"time"
)

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type Violation struct {
	ID          string `json:"id"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Rule        string `json:"rule"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

type AnalysisResult struct {
	ProjectName     string       `json:"project_name"`
	Language        string       `json:"language"`
	LLMProvider     string       `json:"llm_provider"`
	LLMModel        string       `json:"llm_model"`
	AnalyzedAt      time.Time    `json:"analyzed_at"`
	Duration        string       `json:"duration"`
	FilesScanned    int          `json:"files_scanned"`
	ToolCalls       int          `json:"tool_calls"`
	Violations      []Violation  `json:"violations"`
	Metrics         Metrics      `json:"metrics"`
	AuditLog        []AuditEntry `json:"audit_log,omitempty"`
	Incomplete      bool         `json:"incomplete,omitempty"`
	AnalyzedModules []string     `json:"analyzed_modules,omitempty"`
	SkippedModules  []string     `json:"skipped_modules,omitempty"`
}

type Metrics struct {
	TotalViolations  int            `json:"total_violations"`
	BySeverity       map[string]int `json:"by_severity"`
	ByCategory       map[string]int `json:"by_category"`
	CacheHitRatio    float64        `json:"cache_hit_ratio,omitempty"`
	RateLimitHits    int            `json:"rate_limit_hits,omitempty"`
	PreemptiveSleeps int            `json:"preemptive_sleeps,omitempty"`
}

type AuditEntry struct {
	Timestamp             time.Time `json:"timestamp"`
	ToolName              string    `json:"tool_name"`
	Arguments             string    `json:"arguments,omitempty"`
	Decision              string    `json:"decision"`
	ResultSize            int       `json:"result_size"`
	Error                 string    `json:"error,omitempty"`
	InputTokens           int       `json:"input_tokens,omitempty"`
	CachedInputTokens     int       `json:"cached_input_tokens,omitempty"`
	CacheWriteInputTokens int       `json:"cache_write_input_tokens,omitempty"`
	PrunedTurns           int       `json:"pruned_turns,omitempty"`
	Dedup                 bool      `json:"dedup,omitempty"`
}
