package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/archguard/project/shared/models"
)

const (
	resumeContextHeader     = "=== Resume: continuing a previous analysis run ===\n"
	resumePreviousRunFmt    = "Previous run: %s, %d tool calls, %d violations found%s\n"
	resumeIncompleteSuffix  = " (incomplete)"
	resumeCompleteSuffix    = " (complete)"
	resumeViolationsHeader  = "\nAlready-recorded violations (pre-loaded — do NOT re-report these):\n"
	resumeViolationLineFmt  = "  %s [%s] %s — %s:%d\n"
	resumeAnalyzedFmt       = "\nAlready analyzed: %d files (return cached stubs if re-requested)\n"
	resumeRemainingFmt      = "\nRemaining files to analyze — focus EXCLUSIVELY on these (%d files):\n"
	resumeRemainingFileFmt  = "  %s\n"
	resumeRemainingTruncFmt = "  ... and %d more (use get_project_structure to see all)\n"
	resumeInstruction       = "\nContinue the analysis. Focus ONLY on the remaining files above. " +
		"Do NOT re-report violations already listed above."
	resumeMaxRemainingShown = 60
	errLoadResume           = "failed to load resume report %q: %w"
)

func loadResumeReport(path string) (*models.AnalysisResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(errLoadResume, path, err)
	}
	var r models.AnalysisResult
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf(errLoadResume, path, err)
	}
	return &r, nil
}

func buildResumeContext(prev *models.AnalysisResult) string {
	var sb strings.Builder
	sb.WriteString(resumeContextHeader)

	status := resumeCompleteSuffix
	if prev.Incomplete {
		status = resumeIncompleteSuffix
	}
	sb.WriteString(fmt.Sprintf(resumePreviousRunFmt, prev.Duration, prev.ToolCalls, len(prev.Violations), status))

	if len(prev.Violations) > 0 {
		sb.WriteString(resumeViolationsHeader)
		for _, v := range prev.Violations {
			sb.WriteString(fmt.Sprintf(resumeViolationLineFmt, v.ID, v.Severity, v.Rule, v.File, v.Line))
		}
	}

	if len(prev.AnalyzedModules) > 0 {
		sb.WriteString(fmt.Sprintf(resumeAnalyzedFmt, len(prev.AnalyzedModules)))
	}

	if len(prev.SkippedModules) > 0 {
		sb.WriteString(fmt.Sprintf(resumeRemainingFmt, len(prev.SkippedModules)))
		shown := prev.SkippedModules
		truncated := 0
		if len(shown) > resumeMaxRemainingShown {
			truncated = len(shown) - resumeMaxRemainingShown
			shown = shown[:resumeMaxRemainingShown]
		}
		for _, f := range shown {
			sb.WriteString(fmt.Sprintf(resumeRemainingFileFmt, f))
		}
		if truncated > 0 {
			sb.WriteString(fmt.Sprintf(resumeRemainingTruncFmt, truncated))
		}
	}

	sb.WriteString(resumeInstruction)
	return sb.String()
}

func mergeAnalyzedModules(prev, current []string) []string {
	seen := make(map[string]bool, len(prev)+len(current))
	out := make([]string, 0, len(prev)+len(current))
	for _, m := range append(prev, current...) {
		if seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

func diffSkippedModules(analyzed []string, all []string) []string {
	a := make(map[string]bool, len(analyzed))
	for _, f := range analyzed {
		a[f] = true
	}
	var out []string
	for _, f := range all {
		if !a[f] {
			out = append(out, f)
		}
	}
	return out
}
