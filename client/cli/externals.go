package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/archguard/project/client/external"
	"github.com/archguard/project/client/mcp"
	"github.com/archguard/project/client/review"
	"github.com/archguard/project/shared/config"
	c "github.com/archguard/project/shared/constants"
)

func promptExternalContext(systems []config.ExternalConfig, cfg *config.Config, resolver review.ToolRunner, interactive bool) string {
	var results []string

	for _, sys := range systems {
		if !externalSystemReady(sys) {
			continue
		}
		defaultQuery := defaultExternalQuery(cfg, sys)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, renderPanel("External context", []string{
			renderKV("System", sys.System),
			renderKV("Mode", "Force include before analysis"),
			renderKV("Default query", defaultQuery),
		}))
		if interactive {
			confirmRows := []string{
				renderKV("System", sys.System),
				renderKV("Default query", defaultQuery),
			}
			if !askYesNo(
				fmt.Sprintf("Pre-load %s context?", sys.System),
				confirmRows,
				"Yes - use default query",
				"No - skip this system",
			) {
				fmt.Fprintln(os.Stderr, renderNotice(warnBadgeStyle.Render("SKIP"), fmt.Sprintf("%s context skipped by user", sys.System)))
				continue
			}
		}
		result, err := resolver.ExecuteTool(c.ToolGetExternalContext, map[string]interface{}{
			c.ArgQuery:  defaultQuery,
			c.ArgSystem: sys.System,
		}, c.FileCountBudget)
		if err != nil {
			fmt.Fprintln(os.Stderr, renderNotice(warnBadgeStyle.Render("SKIP"), fmt.Sprintf("%s context unavailable: %v", sys.System, err)))
			continue
		}
		fmt.Fprintln(os.Stderr, renderNotice(successBadgeStyle.Render("OK"), fmt.Sprintf("%s context loaded (%d chars)", sys.System, len(result))))
		if debugExternalsEnabled() {
			fmt.Fprintln(os.Stderr, renderSubtlePanel(fmt.Sprintf("External context: %s", sys.System), []string{
				valueStyle.Render(result),
			}))
		}
		results = append(results, fmt.Sprintf("[%s]\n%s", sys.System, result))
	}

	return strings.Join(results, "\n\n")
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
	return strings.Join(parts, " ")
}

// debugExternalsEnabled returns true when the user wants the raw content of
// each external system's response printed to stderr alongside the OK status
// line. Off by default to keep the analyzer's main UI clean.
func debugExternalsEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(envDebugExternals))) {
	case envDebugExternalsOn, envDebugExternalsTrue:
		return true
	default:
		return false
	}
}

func externalSystemReady(sys config.ExternalConfig) bool {
	if sys.System == "" || len(sys.Command) == 0 || sys.SearchTool == "" || sys.SearchArg == "" {
		return false
	}
	for _, value := range sys.Env {
		if !expandedEnvValuePresent(value) {
			return false
		}
	}
	return true
}

func expandedEnvValuePresent(value string) bool {
	missing := false
	expanded := os.Expand(value, func(name string) string {
		env := os.Getenv(name)
		if env == "" {
			missing = true
		}
		return env
	})
	return !missing && strings.TrimSpace(expanded) != ""
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
