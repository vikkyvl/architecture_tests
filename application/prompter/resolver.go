package contextresolver

import (
	"fmt"
	"strings"
)

const (
	systemPromptIntro          = "You are an expert software architecture reviewer.\n"
	systemPromptGoal           = "Analyze the codebase and find architectural violations.\n\n"
	systemPromptProcessHeader  = "Process:\n"
	systemPromptProcess        = "1. Call get_architecture_rules to understand constraints.\n2. Call get_project_structure to see all source files.\n3. Read files with read_file and check dependencies with get_class_dependencies.\n4. Use get_documentation when domain context is needed.\n5. Call report_violation IMMEDIATELY when you find a violation.\n6. When done, stop and summarize.\n\n"
	systemPromptExternalHeader = "Configured external systems (optional; do not query unless explicitly requested):\n"
	systemPromptExternalLine   = "- %s: external context may be available through system=\"%s\"\n"
	systemPromptChecksHeader   = "What to check:\n"
	systemPromptChecks         = "- Structural: verify each class only depends on allowed layers.\n- Semantic: analyze business logic, not just imports.\n- Domain: check domain-specific requirements.\n\n"
	systemPromptProjectLine    = "Project: %s\n"
	systemPromptLanguageLine   = "Language: %s\n"
	systemPromptDomainLine     = "Domain: %s\n"
	systemPromptContextLine    = "Context: %s\n"
	systemPromptLayersHeader   = "\nLayers:\n"
	systemPromptLayerLine      = "- %s (paths: %s)\n"
	systemPromptRulesHeader    = "\nDependency rules:\n"
	systemPromptRuleAllowed    = "allowed"
	systemPromptRuleForbidden  = "FORBIDDEN"
	systemPromptRuleLine       = "- %s to %s: %s\n"
	systemPromptDomainHeader   = "\nDomain rules:\n"
	systemPromptDomainRuleLine = "- %s [%s]: %s\n"
	systemPromptGuidelines     = "\nGuidelines:\n- Check every source file.\n- Report exact file paths and line numbers.\n- Call report_violation immediately per violation.\n- Provide fix suggestions.\n- When done, stop and summarize.\n"
	pathJoinSeparator          = ", "
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

	sb.WriteString(systemPromptGuidelines)

	return sb.String()
}
