package contextresolver

import "github.com/archguard/project/config"

type Resolver struct {
	project      config.ProjectConfig
	layers       []config.LayerConfig
	rules        []config.RuleConfig
	domainCtx    config.DomainContextConfig
	principles   []config.DesignPrinciple
	architecture config.Architecture
	external     []config.ExternalConfig
}

func NewResolver(
	project config.ProjectConfig,
	layers []config.LayerConfig,
	rules []config.RuleConfig,
	domainCtx config.DomainContextConfig,
	principles []config.DesignPrinciple,
	architecture config.Architecture,
	external []config.ExternalConfig,
) *Resolver {
	return &Resolver{
		project:      project,
		layers:       layers,
		rules:        rules,
		domainCtx:    domainCtx,
		principles:   principles,
		architecture: architecture,
		external:     external,
	}
}
