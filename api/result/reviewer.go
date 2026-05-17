package result

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/models"
)

const (
	markdownReportTitle       = "# Architecture Analysis Report: %s\n\n"
	markdownLanguageLine      = "- Language: %s\n"
	markdownLLMLine           = "- LLM: %s (%s)\n"
	markdownAnalyzedAtLine    = "- Analyzed at: %s\n"
	markdownDurationLine      = "- Duration: %s\n"
	markdownFilesScannedLine  = "- Files scanned: %d\n"
	markdownToolCallsLine     = "- Tool calls: %d\n"
	markdownViolationsLine    = "- Violations: %d\n"
	markdownIncompleteWarning = "- Warning: analysis incomplete (limit reached)\n"
	markdownSeverityTable     = "\n## By Severity\n\n| Severity | Count |\n|----------|-------|\n"
	markdownCategoryTable     = "\n## By Category\n\n| Category | Count |\n|----------|-------|\n"
	markdownTableRow          = "| %s | %d |\n"
	markdownViolationsHeader  = "\n## Violations\n\n"
	markdownSeverityHeader    = "### %s\n\n"
	markdownViolationTitle    = "**%d. [%s] %s**\n\n"
	markdownFileLine          = "- File: `%s`"
	markdownLineSuffix        = " (line %d)"
	markdownDescriptionLine   = "- Description: %s\n"
	markdownSuggestionLine    = "- Suggestion: %s\n"
	markdownAuditTable        = "## Audit Log\n\n| # | Tool | Decision | Size | Error |\n|---|------|----------|------|-------|\n"
	markdownAuditRow          = "| %d | %s | %s | %d | %s |\n"
	markdownModuleHeader      = "\n## %s\n\n"
	markdownModuleLine        = "- `%s`\n"
	markdownAnalyzedModules   = "Analyzed Modules"
	markdownSkippedModules    = "Skipped Modules (limit reached)"
	dedupKeyFormat            = "%s:%d:%s"
	operationResult           = "result"
	errMarshalResult          = "failed to marshal result"
)

type Reviewer struct{}

func NewReviewer() *Reviewer {
	return new(Reviewer)
}

func (r *Reviewer) Process(res *models.AnalysisResult) {
	res.Violations = r.dedup(res.Violations)
	res.Violations = r.sortViolations(res.Violations)
	res.Metrics = r.metrics(res.Violations)
}

func (r *Reviewer) RenderJSON(res *models.AnalysisResult, path string) error {
	data, err := json.MarshalIndent(res, "", c.JSONIndent)
	if err != nil {
		return apperrors.Wrap(apperrors.KindInternal, operationResult, errMarshalResult, err)
	}
	return os.WriteFile(path, data, c.FilePermission)
}

func (r *Reviewer) RenderMarkdown(res *models.AnalysisResult, path string) error {
	var sb strings.Builder

	r.writeMarkdownSummary(&sb, res)
	r.writeMetricTables(&sb, res.Metrics)
	r.writeViolations(&sb, res.Violations)
	r.writeModuleLists(&sb, res)
	r.writeAuditLog(&sb, res.AuditLog)

	return os.WriteFile(path, []byte(sb.String()), c.FilePermission)
}

func (r *Reviewer) writeMarkdownSummary(sb *strings.Builder, res *models.AnalysisResult) {
	sb.WriteString(fmt.Sprintf(markdownReportTitle, res.ProjectName))
	sb.WriteString(fmt.Sprintf(markdownLanguageLine, res.Language))
	sb.WriteString(fmt.Sprintf(markdownLLMLine, res.LLMProvider, res.LLMModel))
	sb.WriteString(fmt.Sprintf(markdownAnalyzedAtLine, res.AnalyzedAt.Format(c.ReportTimeLayout)))
	sb.WriteString(fmt.Sprintf(markdownDurationLine, res.Duration))
	sb.WriteString(fmt.Sprintf(markdownFilesScannedLine, res.FilesScanned))
	sb.WriteString(fmt.Sprintf(markdownToolCallsLine, res.ToolCalls))
	sb.WriteString(fmt.Sprintf(markdownViolationsLine, res.Metrics.TotalViolations))
	if res.Incomplete {
		sb.WriteString(markdownIncompleteWarning)
	}
}

func (r *Reviewer) writeMetricTables(sb *strings.Builder, metrics models.Metrics) {
	sb.WriteString(markdownSeverityTable)
	for _, sev := range c.SeverityList {
		if n := metrics.BySeverity[sev]; n > 0 {
			sb.WriteString(fmt.Sprintf(markdownTableRow, sev, n))
		}
	}

	sb.WriteString(markdownCategoryTable)
	for cat, n := range metrics.ByCategory {
		sb.WriteString(fmt.Sprintf(markdownTableRow, cat, n))
	}
}

func (r *Reviewer) writeViolations(sb *strings.Builder, violations []models.Violation) {
	if len(violations) == 0 {
		return
	}
	sb.WriteString(markdownViolationsHeader)
	curSev := ""
	for i, v := range violations {
		if v.Severity != curSev {
			curSev = v.Severity
			sb.WriteString(fmt.Sprintf(markdownSeverityHeader, strings.ToUpper(curSev)))
		}
		sb.WriteString(fmt.Sprintf(markdownViolationTitle, i+1, v.Category, v.Rule))
		sb.WriteString(fmt.Sprintf(markdownFileLine, v.File))
		if v.Line > 0 {
			sb.WriteString(fmt.Sprintf(markdownLineSuffix, v.Line))
		}
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(markdownDescriptionLine, v.Description))
		if v.Suggestion != "" {
			sb.WriteString(fmt.Sprintf(markdownSuggestionLine, v.Suggestion))
		}
		sb.WriteString("\n")
	}
}

func (r *Reviewer) writeModuleLists(sb *strings.Builder, res *models.AnalysisResult) {
	writeModuleList(sb, markdownAnalyzedModules, res.AnalyzedModules)
	writeModuleList(sb, markdownSkippedModules, res.SkippedModules)
}

func writeModuleList(sb *strings.Builder, title string, modules []string) {
	if len(modules) == 0 {
		return
	}
	sb.WriteString(fmt.Sprintf(markdownModuleHeader, title))
	for _, m := range modules {
		sb.WriteString(fmt.Sprintf(markdownModuleLine, m))
	}
}

func (r *Reviewer) writeAuditLog(sb *strings.Builder, entries []models.AuditEntry) {
	if len(entries) == 0 {
		return
	}
	sb.WriteString(markdownAuditTable)
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf(markdownAuditRow, i+1, e.ToolName, e.Decision, e.ResultSize, e.Error))
	}
}

func (r *Reviewer) dedup(vs []models.Violation) []models.Violation {
	seen := make(map[string]bool)
	var out []models.Violation
	for _, v := range vs {
		key := fmt.Sprintf(dedupKeyFormat, v.File, v.Line, v.Rule)
		if !seen[key] {
			seen[key] = true
			out = append(out, v)
		}
	}
	return out
}

func (r *Reviewer) sortViolations(vs []models.Violation) []models.Violation {
	sort.Slice(vs, func(i, j int) bool {
		si, sj := c.SeverityOrder[vs[i].Severity], c.SeverityOrder[vs[j].Severity]
		if si != sj {
			return si < sj
		}
		return vs[i].File < vs[j].File
	})
	return vs
}

func (r *Reviewer) metrics(vs []models.Violation) models.Metrics {
	m := models.Metrics{
		TotalViolations: len(vs),
		BySeverity:      make(map[string]int),
		ByCategory:      make(map[string]int),
	}
	for _, v := range vs {
		m.BySeverity[v.Severity]++
		m.ByCategory[v.Category]++
	}
	return m
}
