package bootstrap

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/archguard/project/application/review"
	"github.com/archguard/project/config"
	"github.com/archguard/project/delivery/output"
	extpkg "github.com/archguard/project/infrastructure/external"
	"github.com/archguard/project/infrastructure/mcp"
	c "github.com/archguard/project/shared/constants"
)

const (
	envDebugExternals           = "ARCHGUARD_DEBUG_EXTERNALS"
	externalPanelTitle          = "External context"
	externalLabelSystem         = "System"
	externalLabelMode           = "Mode"
	externalLabelDefaultQuery   = "Default query"
	externalModeForceInclude    = "Force include before analysis"
	externalAskYesLabel         = "Yes - use default query"
	externalAskNoLabel          = "No - skip this system"
	externalUnnamedSystem       = "<unnamed>"
	externalIncompleteReason    = "external: entry is incomplete"
	externalMissingEnvPrefix    = "env vars not set: "
	externalFormatSkipped       = "%s context skipped: %s"
	externalFormatAsk           = "Pre-load %s context?"
	externalFormatSkippedByUser = "%s context skipped by user"
	externalFormatUnavailable   = "%s context unavailable: %v"
	externalFormatLoaded        = "%s context loaded (%d chars)"
	externalFormatDebugPanel    = "External context: %s"
	externalFormatContextBlock  = "[%s]\n%s"
	externalResultSeparator     = "\n\n"
	externalQuerySeparator      = " "
	externalEnvSeparator        = ", "
)

func promptExternalContext(systems []config.ExternalConfig, cfg *config.Config, resolver review.ToolRunner, interactive bool, out *output.OutputTransport) string {
	var results []string

	for _, sys := range systems {
		if ready, reason := externalSystemReadiness(sys); !ready {
			out.WriteProgress(output.RenderNotice(output.WarnBadgeStyle.Render(output.BadgeSkip), fmt.Sprintf(externalFormatSkipped, systemLabel(sys), reason)))
			continue
		}
		defaultQuery := defaultExternalQuery(cfg, sys)
		out.WriteProgressBlank()
		out.WriteProgress(output.RenderPanel(externalPanelTitle, []string{
			output.RenderKV(externalLabelSystem, sys.System),
			output.RenderKV(externalLabelMode, externalModeForceInclude),
			output.RenderKV(externalLabelDefaultQuery, defaultQuery),
		}))
		if interactive {
			confirmRows := []string{
				output.RenderKV(externalLabelSystem, sys.System),
				output.RenderKV(externalLabelDefaultQuery, defaultQuery),
			}
			if !output.AskYesNo(
				fmt.Sprintf(externalFormatAsk, sys.System),
				confirmRows,
				externalAskYesLabel,
				externalAskNoLabel,
			) {
				out.WriteProgress(output.RenderNotice(output.WarnBadgeStyle.Render(output.BadgeSkip), fmt.Sprintf(externalFormatSkippedByUser, sys.System)))
				continue
			}
		}
		result, err := resolver.ExecuteTool(c.ToolGetExternalContext, map[string]interface{}{
			c.ArgQuery:  defaultQuery,
			c.ArgSystem: sys.System,
		}, c.FileCountBudget)
		if err != nil {
			out.WriteProgress(output.RenderNotice(output.WarnBadgeStyle.Render(output.BadgeSkip), fmt.Sprintf(externalFormatUnavailable, sys.System, err)))
			continue
		}
		out.WriteProgress(output.RenderNotice(output.SuccessBadgeStyle.Render(output.BadgeOK), fmt.Sprintf(externalFormatLoaded, sys.System, len(result))))
		if debugExternalsEnabled() {
			out.WriteProgress(output.RenderSubtlePanel(fmt.Sprintf(externalFormatDebugPanel, sys.System), []string{
				output.ValueStyle.Render(result),
			}))
		}
		results = append(results, fmt.Sprintf(externalFormatContextBlock, sys.System, result))
	}

	return strings.Join(results, externalResultSeparator)
}

func defaultExternalQuery(cfg *config.Config, sys config.ExternalConfig) string {
	var baseQuery string
	if strings.TrimSpace(sys.DefaultQuery) != "" {
		baseQuery = os.ExpandEnv(sys.DefaultQuery)
	} else {
		var parts []string
		if cfg.Project.Name != "" {
			parts = append(parts, cfg.Project.Name)
		}
		if cfg.DomainContext.Domain != "" {
			parts = append(parts, cfg.DomainContext.Domain)
		}
		if cfg.DomainContext.Description != "" {
			parts = append(parts, cfg.DomainContext.Description)
		}
		if len(parts) == 0 {
			baseQuery = sys.System
		} else {
			baseQuery = strings.Join(parts, externalQuerySeparator)
		}
	}
	filter := os.ExpandEnv(sys.ProjectFilter)
	return extpkg.BuildQuery(baseQuery, sys.QueryTemplate, filter)
}

func debugExternalsEnabled() bool {
	return c.IsEnvTrue(envDebugExternals)
}

func externalSystemReadiness(sys config.ExternalConfig) (bool, string) {
	if sys.System == "" || len(sys.Command) == 0 || sys.SearchTool == "" || sys.SearchArg == "" {
		return false, externalIncompleteReason
	}
	missing := missingEnvReferences(sys.Env, sys.ProjectFilter)
	if len(missing) > 0 {
		return false, externalMissingEnvPrefix + strings.Join(missing, externalEnvSeparator)
	}
	return true, ""
}

func missingEnvReferences(env map[string]string, extraValues ...string) []string {
	seen := make(map[string]bool)
	var missing []string
	scanValue := func(value string) {
		os.Expand(value, func(name string) string {
			v := os.Getenv(name)
			if v == "" && !seen[name] {
				seen[name] = true
				missing = append(missing, name)
			}
			return v
		})
	}
	for _, value := range env {
		scanValue(value)
	}
	for _, value := range extraValues {
		scanValue(value)
	}
	sort.Strings(missing)
	return missing
}

func systemLabel(sys config.ExternalConfig) string {
	if sys.System != "" {
		return sys.System
	}
	return externalUnnamedSystem
}

func buildExternals(cfgs []config.ExternalConfig) (map[string]mcp.ExternalSearcher, map[string]bool, []*extpkg.MCPClient) {
	searchers := make(map[string]mcp.ExternalSearcher, len(cfgs))
	allowed := make(map[string]bool, len(cfgs))
	var closers []*extpkg.MCPClient
	for _, ec := range cfgs {
		cl := extpkg.NewMCPClient(ec)
		searchers[ec.System] = cl
		allowed[ec.System] = true
		closers = append(closers, cl)
	}
	return searchers, allowed, closers
}
