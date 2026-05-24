package result_test

import (
	"math"
	"testing"

	"github.com/archguard/project/api/result"
	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/models"
)

func TestReviewerComputesCacheHitRatio(t *testing.T) {
	r := result.NewReviewer()
	input := &models.AnalysisResult{
		Violations: []models.Violation{
			{File: "a.go", Line: 1, Rule: "r", Severity: "high", Category: "x"},
		},
		AuditLog: []models.AuditEntry{
			{ToolName: c.EventLLMTurn, InputTokens: 1000, CachedInputTokens: 0},
			{ToolName: c.EventLLMTurn, InputTokens: 200, CachedInputTokens: 800},
			{ToolName: c.EventLLMTurn, InputTokens: 100, CachedInputTokens: 900},
			{ToolName: c.ToolReadFile, ResultSize: 500},
		},
	}
	r.Process(input)
	want := 1700.0 / 3000.0
	if math.Abs(input.Metrics.CacheHitRatio-want) > 1e-9 {
		t.Errorf("CacheHitRatio = %f, want %f", input.Metrics.CacheHitRatio, want)
	}
}

func TestReviewerCacheHitRatioZeroWhenNoLLMTurns(t *testing.T) {
	r := result.NewReviewer()
	input := &models.AnalysisResult{
		AuditLog: []models.AuditEntry{
			{ToolName: c.ToolReadFile, ResultSize: 500},
		},
	}
	r.Process(input)
	if input.Metrics.CacheHitRatio != 0 {
		t.Errorf("CacheHitRatio = %f, want 0 when no llm_turn entries", input.Metrics.CacheHitRatio)
	}
}

func TestReviewerCountsRateLimitHits(t *testing.T) {
	r := result.NewReviewer()
	input := &models.AnalysisResult{
		AuditLog: []models.AuditEntry{
			{ToolName: c.EventRateLimit, Arguments: c.RateLimitKind429},
			{ToolName: c.EventLLMTurn, InputTokens: 1000},
			{ToolName: c.EventRateLimit, Arguments: c.RateLimitKindOverloaded},
			{ToolName: c.EventRateLimit, Arguments: c.RateLimitKind429},
		},
	}
	r.Process(input)
	if input.Metrics.RateLimitHits != 3 {
		t.Errorf("RateLimitHits = %d, want 3", input.Metrics.RateLimitHits)
	}
}

func TestReviewerRateLimitHitsZeroByDefault(t *testing.T) {
	r := result.NewReviewer()
	input := &models.AnalysisResult{}
	r.Process(input)
	if input.Metrics.RateLimitHits != 0 {
		t.Errorf("RateLimitHits = %d, want 0 on empty audit log", input.Metrics.RateLimitHits)
	}
}

func TestCountPreemptiveSleeps(t *testing.T) {
	r := result.NewReviewer()
	input := &models.AnalysisResult{
		AuditLog: []models.AuditEntry{
			{ToolName: c.EventPreemptiveSleep, Arguments: "1s"},
			{ToolName: c.EventRateLimit, Arguments: c.RateLimitKind429},
			{ToolName: c.EventPreemptiveSleep, Arguments: "2s"},
			{ToolName: c.EventLLMTurn, InputTokens: 100},
			{ToolName: c.EventPreemptiveSleep, Arguments: "3s"},
		},
	}
	r.Process(input)
	if input.Metrics.PreemptiveSleeps != 3 {
		t.Errorf("PreemptiveSleeps = %d, want 3", input.Metrics.PreemptiveSleeps)
	}
	if input.Metrics.RateLimitHits != 1 {
		t.Errorf("RateLimitHits = %d, want 1 (must not double-count preemptive)", input.Metrics.RateLimitHits)
	}
}

func TestReviewerPreemptiveSleepsZeroByDefault(t *testing.T) {
	r := result.NewReviewer()
	input := &models.AnalysisResult{}
	r.Process(input)
	if input.Metrics.PreemptiveSleeps != 0 {
		t.Errorf("PreemptiveSleeps = %d, want 0 on empty audit log", input.Metrics.PreemptiveSleeps)
	}
}

func TestReviewerDedup(t *testing.T) {
	r := result.NewReviewer()
	input := &models.AnalysisResult{
		Violations: []models.Violation{
			{File: "a.php", Line: 10, Rule: "no-domain-in-infra", Severity: "high"},
			{File: "a.php", Line: 10, Rule: "no-domain-in-infra", Severity: "high"},
			{File: "b.php", Line: 5, Rule: "no-domain-in-infra", Severity: "critical"},
		},
	}
	r.Process(input)
	if len(input.Violations) != 2 {
		t.Errorf("expected 2 violations after dedup, got %d", len(input.Violations))
	}
}

func TestReviewerMetrics(t *testing.T) {
	r := result.NewReviewer()
	input := &models.AnalysisResult{
		Violations: []models.Violation{
			{File: "a.php", Line: 1, Rule: "r1", Severity: "critical", Category: "layer_dependency"},
			{File: "b.php", Line: 2, Rule: "r2", Severity: "high", Category: "layer_dependency"},
			{File: "c.php", Line: 3, Rule: "r3", Severity: "high", Category: "domain_rule"},
		},
	}
	r.Process(input)

	if input.Metrics.TotalViolations != 3 {
		t.Errorf("expected TotalViolations=3, got %d", input.Metrics.TotalViolations)
	}
	if input.Metrics.BySeverity["critical"] != 1 {
		t.Errorf("expected 1 critical, got %d", input.Metrics.BySeverity["critical"])
	}
	if input.Metrics.BySeverity["high"] != 2 {
		t.Errorf("expected 2 high, got %d", input.Metrics.BySeverity["high"])
	}
	if input.Metrics.ByCategory["layer_dependency"] != 2 {
		t.Errorf("expected 2 layer_dependency, got %d", input.Metrics.ByCategory["layer_dependency"])
	}
}

func TestReviewerSortsBySeverity(t *testing.T) {
	r := result.NewReviewer()
	input := &models.AnalysisResult{
		Violations: []models.Violation{
			{File: "a.php", Severity: "low"},
			{File: "b.php", Severity: "critical"},
			{File: "c.php", Severity: "high"},
		},
	}
	r.Process(input)

	order := []string{"critical", "high", "low"}
	for i, want := range order {
		if input.Violations[i].Severity != want {
			t.Errorf("position %d: expected severity %q, got %q", i, want, input.Violations[i].Severity)
		}
	}
}
