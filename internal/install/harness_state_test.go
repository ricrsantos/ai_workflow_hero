package install_test

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

func TestToolsFlagRemovedMessage(t *testing.T) {
	if !strings.Contains(install.ToolsFlagRemovedMsg, "--tools is not supported") {
		t.Fatalf("msg=%q", install.ToolsFlagRemovedMsg)
	}
	if !strings.Contains(install.ToolsFlagRemovedSuggestion, "/hero-harness") {
		t.Fatalf("suggestion=%q", install.ToolsFlagRemovedSuggestion)
	}
}

func TestMigrateHarnessStateFromLegacyTools(t *testing.T) {
	hero := install.HeroJSON{
		CLI: install.CLIInfo{Tools: []string{"cursor"}},
	}
	if !install.MigrateHarnessState(&hero) {
		t.Fatal("expected migration")
	}
	if !install.IsHarnessEnabled(hero, "cursor") {
		t.Fatal("cursor should be enabled")
	}
	if install.IsHarnessEnabled(hero, "opencode") {
		t.Fatal("opencode should stay disabled on 1.x upgrade")
	}
}

func TestListEnabledHarnesses(t *testing.T) {
	hero := install.HeroJSON{
		Harnesses: install.HarnessesFromSelection([]string{"cursor", "opencode"}),
	}
	got := install.ListEnabledHarnesses(hero)
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}
