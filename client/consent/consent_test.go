package consent

import (
	"testing"

	c "github.com/archguard/project/shared/constants"
)

func TestCheckBlocksSensitiveReadFile(t *testing.T) {
	m := &Manager{interactive: true}

	decision := m.Check(c.ToolReadFile, map[string]interface{}{c.ArgPath: ".env"}, 10)

	if decision != c.DecisionBlocked {
		t.Fatalf("expected %q, got %q", c.DecisionBlocked, decision)
	}
}

func TestCheckAllowsProjectWhitelistBeforePrompt(t *testing.T) {
	m := &Manager{
		interactive:    true,
		projectAllowed: []Rule{{Tool: c.ToolReadFile, Pattern: "glob:src/**"}},
	}

	decision := m.Check(c.ToolReadFile, map[string]interface{}{c.ArgPath: "src/App/Service.php"}, 10)

	if decision != c.DecisionProjectAllow {
		t.Fatalf("expected %q, got %q", c.DecisionProjectAllow, decision)
	}
}

func TestCheckDeniesConsentRequiredToolWhenNonInteractive(t *testing.T) {
	m := &Manager{interactive: false}

	decision := m.Check(c.ToolGetDocumentation, map[string]interface{}{}, 10)

	if decision != c.DecisionUserDenied {
		t.Fatalf("expected %q, got %q", c.DecisionUserDenied, decision)
	}
}

func TestCheckBlocksUnknownExternalSystem(t *testing.T) {
	m := &Manager{interactive: true}

	decision := m.Check(c.ToolGetExternalContext, map[string]interface{}{
		c.ArgQuery:  "ADR auth",
		c.ArgSystem: "jira",
	}, 10)

	if decision != c.DecisionBlocked {
		t.Fatalf("expected %q, got %q", c.DecisionBlocked, decision)
	}
}
