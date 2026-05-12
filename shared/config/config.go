package config

type Config struct {
	Project       ProjectConfig       `yaml:"project"`
	Layers        []LayerConfig       `yaml:"layers"`
	Rules         []RuleConfig        `yaml:"rules"`
	DomainContext DomainContextConfig `yaml:"domain_context"`
	External      []ExternalConfig    `yaml:"external,omitempty"`
}

type ProjectConfig struct {
	Name     string   `yaml:"name"`
	Language string   `yaml:"language"`
	SrcRoot  string   `yaml:"src_root"`
	Exclude  []string `yaml:"exclude"`
}

type LayerConfig struct {
	Name       string   `yaml:"name"`
	Namespaces []string `yaml:"namespaces"`
	Paths      []string `yaml:"paths"`
}

type RuleConfig struct {
	From        string `yaml:"from"`
	To          string `yaml:"to"`
	Allow       bool   `yaml:"allow"`
	Description string `yaml:"description"`
}

type DomainContextConfig struct {
	Domain      string       `yaml:"domain"`
	Description string       `yaml:"description"`
	Rules       []DomainRule `yaml:"domain_rules"`
}

type DomainRule struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Severity    string `yaml:"severity"`
}

type ExternalConfig struct {
	System     string            `yaml:"system"`
	Command    []string          `yaml:"command"`
	Env        map[string]string `yaml:"env,omitempty"`
	SearchTool string            `yaml:"search_tool"`
	SearchArg  string            `yaml:"search_arg"`
}
