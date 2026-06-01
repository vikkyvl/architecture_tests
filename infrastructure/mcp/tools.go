package mcp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/models"
)

func (r *Resolver) archRules() (string, error) {
	return marshalToolResult(toolResultArchRules, map[string]interface{}{
		c.JSONKeyLayers:      r.layers,
		c.JSONKeyRules:       r.rules,
		c.JSONKeyDomain:      r.domainContext.Domain,
		c.JSONKeyDomainDesc:  r.domainContext.Description,
		c.JSONKeyDomainRules: r.domainContext.Rules,
	})
}

func (r *Resolver) classDeps(classOrPath string) (string, error) {
	if classOrPath == "" {
		return "", apperrors.Validation(errClassRequired)
	}
	fp, err := r.resolveFile(classOrPath)
	if err != nil {
		return "", err
	}
	content, err := readSourceFile(filepath.Join(r.projectPath, fp), fp)
	if err != nil {
		return "", err
	}
	lang := c.ExtToLang[strings.TrimPrefix(filepath.Ext(fp), ".")]
	return marshalToolResult(toolResultClassDeps, map[string]interface{}{
		c.JSONKeyClass:        classOrPath,
		c.JSONKeyFile:         fp,
		c.JSONKeyLayer:        r.resolveLayer(classOrPath),
		c.JSONKeyDependencies: r.extractDeps(string(content), lang),
	})
}

func (r *Resolver) documentation() (string, error) {
	if r.docsPath == "" {
		return msgNoDocumentation, nil
	}
	data, err := readSourceFile(r.docsPath, r.docsPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *Resolver) externalContext(query, system string) (string, error) {
	if query == "" {
		return "", apperrors.Validation(errExternalQueryRequired)
	}
	system = strings.ToLower(strings.TrimSpace(system))
	if system == "" && len(r.externals) > 0 {
		for k := range r.externals {
			system = k
			break
		}
	}
	searcher, ok := r.externals[system]
	if !ok {
		return "", apperrors.PermissionDenied(fmt.Sprintf(errExternalSystemNotAllowed, system))
	}
	return searcher.Search(query)
}

func (r *Resolver) reportViolation(args map[string]interface{}) (string, error) {
	violation, err := violationFromArgs(args)
	if err != nil {
		return "", err
	}

	r.mu.Lock()
	r.nextID++
	id := fmt.Sprintf(violationIDFormat, r.nextID)
	violation.ID = id
	r.violations = append(r.violations, violation)
	r.mu.Unlock()

	return fmt.Sprintf(msgViolationRecorded, id, violation.Severity, violation.Rule, violation.File, violation.Line), nil
}

func (r *Resolver) resolveLayer(name string) string {
	for _, layer := range r.layers {
		for _, ns := range layer.Namespaces {
			pattern := strings.TrimSuffix(strings.TrimSuffix(ns, namespaceWildcardSuffix), "*")
			if strings.HasPrefix(name, pattern) {
				return layer.Name
			}
		}
	}
	return c.UnknownValue
}

func marshalToolResult(scope string, value interface{}) (string, error) {
	data, err := json.MarshalIndent(value, "", c.JSONIndent)
	if err != nil {
		return "", apperrors.Wrap(apperrors.KindInternal, scope, toolResultMarshalError, err)
	}
	return string(data), nil
}

func violationFromArgs(args map[string]interface{}) (models.Violation, error) {
	file, _ := args[c.ArgFile].(string)
	sev, _ := args[c.ArgSeverity].(string)
	rule, _ := args[c.ArgRule].(string)
	desc, _ := args[c.ArgDescription].(string)
	if file == "" || sev == "" || rule == "" || desc == "" {
		var empty models.Violation

		return empty, apperrors.Validation(errViolationRequired)
	}
	line := 0
	if v, ok := args[c.ArgLine].(float64); ok {
		line = int(v)
	}
	cat, _ := args[c.ArgCategory].(string)
	sug, _ := args[c.ArgSuggestion].(string)
	return models.Violation{
		File:        file,
		Line:        line,
		Severity:    sev,
		Category:    cat,
		Rule:        rule,
		Description: desc,
		Suggestion:  sug,
	}, nil
}
