package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/archguard/project/client/external"
	"github.com/archguard/project/client/mcp"
	"github.com/archguard/project/client/review"
	"github.com/archguard/project/shared/config"
	c "github.com/archguard/project/shared/constants"
)

const (
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

func promptExternalContext(systems []config.ExternalConfig, cfg *config.Config, resolver review.ToolRunner, interactive bool) string {
	var results []string

	for _, sys := range systems {
		if ready, reason := externalSystemReadiness(sys); !ready {
			fmt.Fprintln(os.Stderr, renderNotice(warnBadgeStyle.Render(badgeSkip), fmt.Sprintf(externalFormatSkipped, systemLabel(sys), reason)))
			continue
		}
		defaultQuery := defaultExternalQuery(cfg, sys)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, renderPanel(externalPanelTitle, []string{
			renderKV(externalLabelSystem, sys.System),
			renderKV(externalLabelMode, externalModeForceInclude),
			renderKV(externalLabelDefaultQuery, defaultQuery),
		}))
		if interactive {
			confirmRows := []string{
				renderKV(externalLabelSystem, sys.System),
				renderKV(externalLabelDefaultQuery, defaultQuery),
			}
			if !askYesNo(
				fmt.Sprintf(externalFormatAsk, sys.System),
				confirmRows,
				externalAskYesLabel,
				externalAskNoLabel,
			) {
				fmt.Fprintln(os.Stderr, renderNotice(warnBadgeStyle.Render(badgeSkip), fmt.Sprintf(externalFormatSkippedByUser, sys.System)))
				continue
			}
		}
		result, err := resolver.ExecuteTool(c.ToolGetExternalContext, map[string]interface{}{
			c.ArgQuery:  defaultQuery,
			c.ArgSystem: sys.System,
		}, c.FileCountBudget)
		if err != nil {
			fmt.Fprintln(os.Stderr, renderNotice(warnBadgeStyle.Render(badgeSkip), fmt.Sprintf(externalFormatUnavailable, sys.System, err)))
			continue
		}
		fmt.Fprintln(os.Stderr, renderNotice(successBadgeStyle.Render(badgeOK), fmt.Sprintf(externalFormatLoaded, sys.System, len(result))))
		if debugExternalsEnabled() {
			fmt.Fprintln(os.Stderr, renderSubtlePanel(fmt.Sprintf(externalFormatDebugPanel, sys.System), []string{
				valueStyle.Render(result),
			}))
		}
		results = append(results, fmt.Sprintf(externalFormatContextBlock, sys.System, result))
	}

	return strings.Join(results, externalResultSeparator)
}

func defaultExternalQuery(cfg *config.Config, sys config.ExternalConfig) string {
	if strings.TrimSpace(sys.DefaultQuery) != "" {
		return sys.DefaultQuery
	}
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
		return sys.System
	}
	return strings.Join(parts, externalQuerySeparator)
}

func debugExternalsEnabled() bool {
	return c.IsEnvTrue(envDebugExternals)
}

func externalSystemReadiness(sys config.ExternalConfig) (bool, string) {
	if sys.System == "" || len(sys.Command) == 0 || sys.SearchTool == "" || sys.SearchArg == "" {
		return false, externalIncompleteReason
	}
	missing := missingEnvReferences(sys.Env)
	if len(missing) > 0 {
		return false, externalMissingEnvPrefix + strings.Join(missing, externalEnvSeparator)
	}
	return true, ""
}

func missingEnvReferences(env map[string]string) []string {
	seen := make(map[string]bool)
	var missing []string
	for _, value := range env {
		os.Expand(value, func(name string) string {
			v := os.Getenv(name)
			if v == "" && !seen[name] {
				seen[name] = true
				missing = append(missing, name)
			}
			return v
		})
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

func buildExternals(cfgs []config.ExternalConfig) (map[string]mcp.ExternalSearcher, map[string]bool, []*external.MCPClient) {
	searchers := make(map[string]mcp.ExternalSearcher, len(cfgs))
	allowed := make(map[string]bool, len(cfgs))
	var closers []*external.MCPClient
	for _, ec := range cfgs {
		cl := external.NewMCPClient(ec)
		searchers[ec.System] = cl
		allowed[ec.System] = true
		closers = append(closers, cl)
	}
	return searchers, allowed, closers
}
