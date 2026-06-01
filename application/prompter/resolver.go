package contextresolver

import (
	"fmt"
	"strings"
)

const (
	systemPromptIntro               = "You are an expert software architecture reviewer.\n"
	systemPromptGoal                = "Analyze the codebase and find architectural violations.\n\n"
	systemPromptProcessHeader       = "Process:\n"
	systemPromptProcess             = "1. Call get_architecture_rules to load all rules (structural + domain).\n2. Call get_project_structure to get the COMPLETE file list.\n3. Process files in a SINGLE COMBINED PASS — do NOT do a full structural sweep first and domain sweep second:\n   a. For every file: call get_class_dependencies to check layer dependencies.\n   b. If the file name or path suggests it implements logic covered by any domain rule (e.g. payment processing, audit logging, error handling, data isolation) — call read_file on it IMMEDIATELY in the same pass, before moving to the next file.\n4. Call report_violation IMMEDIATELY for each violation found — do not batch.\n5. Stop only after every file has been processed in this combined pass.\n\n"
	systemPromptExternalHeader      = "Configured external systems (optional; do not query unless explicitly requested):\n"
	systemPromptExternalLine        = "- %s: external context may be available through system=\"%s\"\n"
	systemPromptChecksHeader        = "What to check:\n"
	systemPromptChecks              = "- Structural: verify each class only depends on allowed layers (use get_class_dependencies).\n- Domain: check domain-specific requirements by reading actual file content (use read_file) — imports alone cannot reveal missing audit calls, wrong error handling, or business rule violations.\n\n"
	systemPromptProjectLine         = "Project: %s\n"
	systemPromptLanguageLine        = "Language: %s\n"
	systemPromptDomainLine          = "Domain: %s\n"
	systemPromptContextLine         = "Context: %s\n"
	systemPromptLayersHeader        = "\nLayers:\n"
	systemPromptLayerLine           = "- %s (paths: %s)\n"
	systemPromptRulesHeader         = "\nDependency rules:\n"
	systemPromptRuleAllowed         = "allowed"
	systemPromptRuleForbidden       = "FORBIDDEN"
	systemPromptRuleLine            = "- %s to %s: %s\n"
	systemPromptDomainHeader        = "\nDomain rules:\n"
	systemPromptDomainRuleLine      = "- %s [%s]: %s\n"
	systemPromptDesignHeader        = "\nDesign principles to check:\n"
	systemPromptDesignRuleLine      = "- %s [%s]: %s\n"
	systemPromptDesignRuleWithLayer = "- %s [%s] (layers: %s): %s\n"
	systemPromptArchStyle           = "\nArchitecture: %s\n"
	systemPromptArchDesc            = "Style context: %s\n"
	systemPromptArchHeader          = "\nArchitecture invariants:\n"
	systemPromptArchInvariantLine   = "- %s [%s]: %s\n"
	systemPromptArchInvWithLayer    = "- %s [%s] (layers: %s): %s\n"
	systemPromptGuidelines          = "\nCoverage requirement — CRITICAL:\n- get_class_dependencies MUST be called on every source file, no exceptions.\n- read_file MUST be called for every file that is plausibly relevant to a domain rule, design principle, or architecture invariant — do not wait until structural analysis is complete; do it in the same pass.\n- get_class_dependencies shows only imports; it cannot detect SRP violations, missing abstractions, framework leakage into domain, or architecture style violations. Those require read_file.\n- Never exhaust your call budget on get_class_dependencies alone. Prioritise relevant files for read_file early in the pass.\n- Report exact file paths and line numbers for each violation.\n- Provide a fix suggestion for each violation.\n"
	pathJoinSeparator               = ", "
)

func (r *Resolver) BuildSystemPrompt() string {
	var sb strings.Builder

	sb.WriteString(systemPromptIntro)
	sb.WriteString(systemPromptGoal)
	sb.WriteString(systemPromptProcessHeader)

	sb.WriteString(systemPromptProcess)
	if len(r.external) > 0 {
		sb.WriteString(systemPromptExternalHeader)
		for _, ext := range r.external {
			sb.WriteString(fmt.Sprintf(systemPromptExternalLine, ext.System, ext.System))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(systemPromptChecksHeader)
	sb.WriteString(systemPromptChecks)

	sb.WriteString(fmt.Sprintf(systemPromptProjectLine, r.project.Name))
	if r.project.Language != "" {
		sb.WriteString(fmt.Sprintf(systemPromptLanguageLine, r.project.Language))
	}
	if r.domainCtx.Domain != "" {
		sb.WriteString(fmt.Sprintf(systemPromptDomainLine, r.domainCtx.Domain))
		if r.domainCtx.Description != "" {
			sb.WriteString(fmt.Sprintf(systemPromptContextLine, r.domainCtx.Description))
		}
	}

	sb.WriteString(systemPromptLayersHeader)
	for _, l := range r.layers {
		sb.WriteString(fmt.Sprintf(systemPromptLayerLine, l.Name, strings.Join(l.Paths, pathJoinSeparator)))
	}

	sb.WriteString(systemPromptRulesHeader)
	for _, rule := range r.rules {
		action := systemPromptRuleForbidden
		if rule.Allow {
			action = systemPromptRuleAllowed
		}
		sb.WriteString(fmt.Sprintf(systemPromptRuleLine, rule.From, rule.To, action))
	}

	if len(r.domainCtx.Rules) > 0 {
		sb.WriteString(systemPromptDomainHeader)
		for _, dr := range r.domainCtx.Rules {
			sb.WriteString(fmt.Sprintf(systemPromptDomainRuleLine, dr.ID, dr.Severity, dr.Description))
		}
	}

	if len(r.principles) > 0 {
		sb.WriteString(systemPromptDesignHeader)
		for _, dp := range r.principles {
			if len(dp.AppliesTo) > 0 {
				sb.WriteString(fmt.Sprintf(systemPromptDesignRuleWithLayer,
					dp.ID, dp.Severity, strings.Join(dp.AppliesTo, pathJoinSeparator), dp.Description))
			} else {
				sb.WriteString(fmt.Sprintf(systemPromptDesignRuleLine, dp.ID, dp.Severity, dp.Description))
			}
		}
	}

	if r.architecture.Style != "" {
		sb.WriteString(fmt.Sprintf(systemPromptArchStyle, r.architecture.Style))
		if r.architecture.Description != "" {
			sb.WriteString(fmt.Sprintf(systemPromptArchDesc, r.architecture.Description))
		}
		if len(r.architecture.Invariants) > 0 {
			sb.WriteString(systemPromptArchHeader)
			for _, inv := range r.architecture.Invariants {
				if len(inv.AppliesTo) > 0 {
					sb.WriteString(fmt.Sprintf(systemPromptArchInvWithLayer,
						inv.ID, inv.Severity, strings.Join(inv.AppliesTo, pathJoinSeparator), inv.Description))
				} else {
					sb.WriteString(fmt.Sprintf(systemPromptArchInvariantLine, inv.ID, inv.Severity, inv.Description))
				}
			}
		}
	}

	sb.WriteString(systemPromptGuidelines)

	return sb.String()
}
