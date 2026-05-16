package consent

import (
	"strings"
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

func TestCheckAutoApprovesConfiguredExternalSystem(t *testing.T) {
	m := &Manager{
		interactive:    false,
		allowedSystems: map[string]bool{"notion": true},
	}

	decision := m.Check(c.ToolGetExternalContext, map[string]interface{}{
		c.ArgQuery:  "audit trail",
		c.ArgSystem: "notion",
	}, 10)

	if decision != c.DecisionAutoApproved {
		t.Fatalf("expected %q, got %q", c.DecisionAutoApproved, decision)
	}
}

func TestConsentViewLeavesTrailingLineForBubbleTeaShutdown(t *testing.T) {
	model := consentChoiceModel{
		rows: []string{
			consentTitleStyle.Render("Consent required"),
			renderConsentKV(msgConsentTool, c.ToolReadFile),
		},
		options: []consentOption{
			{key: choiceAllowOnce, label: "Allow once", description: "Only this tool call"},
		},
	}

	view := model.View()
	if !strings.HasSuffix(view, "\n") {
		t.Fatal("consent view must end with a newline so Bubble Tea does not erase the bottom border on shutdown")
	}
}
