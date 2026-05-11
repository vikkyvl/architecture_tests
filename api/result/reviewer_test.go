package result_test

import (
	"testing"

	"github.com/archguard/project/api/result"
	"github.com/archguard/project/shared/models"
)

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
