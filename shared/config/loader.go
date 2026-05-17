package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/archguard/project/shared/apperrors"
	"gopkg.in/yaml.v3"
)

const (
	errReadConfig          = "failed to read config %s: %v"
	errConfigNotFound      = "config file not found: %s"
	errParseYAML           = "failed to parse YAML %s: %v"
	operationLoadConfig    = "load config"
	operationParseConfig   = "parse config"
	errProjectNameRequired = "project.name is required"
	errLayerRequired       = "at least one layer is required"
	errLayerNameRequired   = "each layer must have a name"
	errLayerPathRequired   = "layer %q must have namespaces or paths"
	errUnknownFromLayer    = "rule references unknown layer: from=%q"
	errUnknownToLayer      = "rule references unknown layer: to=%q"
)

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apperrors.NotFound(fmt.Sprintf(errConfigNotFound, path))
		}
		return nil, apperrors.Wrap(apperrors.KindInternal, operationLoadConfig, fmt.Sprintf(errReadConfig, path, err), err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, apperrors.Wrap(apperrors.KindValidation, operationParseConfig, fmt.Sprintf(errParseYAML, path, err), err)
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	if cfg.Project.Name == "" {
		return apperrors.Validation(errProjectNameRequired)
	}
	if len(cfg.Layers) == 0 {
		return apperrors.Validation(errLayerRequired)
	}

	layers := make(map[string]bool)
	for _, l := range cfg.Layers {
		if l.Name == "" {
			return apperrors.Validation(errLayerNameRequired)
		}
		if len(l.Namespaces) == 0 && len(l.Paths) == 0 {
			return apperrors.Validation(fmt.Sprintf(errLayerPathRequired, l.Name))
		}
		layers[l.Name] = true
	}

	for _, r := range cfg.Rules {
		if !layers[r.From] {
			return apperrors.Validation(fmt.Sprintf(errUnknownFromLayer, r.From))
		}
		if !layers[r.To] {
			return apperrors.Validation(fmt.Sprintf(errUnknownToLayer, r.To))
		}
	}

	return nil
}
