package tui

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
)

func TestBlockedDisplayShowsDurableCorrectionAndBudget(t *testing.T) {
	m := NewTestModel(nil)
	m.width = 120
	m.status = cycle.StatusView{CycleNumber: 17, BlockedStages: []cycle.StatusBlockedStage{{
		Name: "Browser UI Validation", RemainingBudgetMS: 500000,
		Blockers: []cycle.StatusStageBlocker{{ID: "block-admin", Reason: "account_unusable",
			AffectedCoverageIDs: []string{"screen-02", "screen-03"}, AffectedProfileIDs: []string{"admin"},
			Uncertainty: "admin screens not inspected", NextAction: "Configure administrator in Test users"}},
	}}}
	var b strings.Builder
	m.appendStatusHandoffSections(&b)
	view := stripANSI(b.String())
	for _, want := range []string{"Browser UI Validation blocked", "account_unusable", "screen-02, screen-03", "admin", "Configure administrator", "8m20s", "/hero-continue"} {
		if !strings.Contains(view, want) {
			t.Fatalf("blocked display missing %q", want)
		}
	}
}

func TestBlockedDisplayNeutralizesTerminalControls(t *testing.T) {
	m := NewTestModel(nil)
	m.width = 40
	m.status.BlockedStages = []cycle.StatusBlockedStage{{Name: "Browser UI", Blockers: []cycle.StatusStageBlocker{{NextAction: "correct\x1b]52;c;payload\x07"}}}}
	var b strings.Builder
	m.writeStatusBlockedStages(&b)
	if strings.Contains(stripANSI(b.String()), "\x07") || strings.Contains(b.String(), "\x1b]52") {
		t.Fatal("terminal control sequence survived blocked display")
	}
}

func TestBudgetDisplayPreservesInterruptedElapsedAndRemaining(t *testing.T) {
	m := NewTestModel(nil)
	m.width = 120
	m.status.StageBudgets = []cycle.StatusStageBudget{{Name: "Implementation", State: "interrupted", ConsumedMS: 60000, RemainingMS: 120000, InterruptionReason: "restart"}}
	var b strings.Builder
	m.writeStatusStageBudgets(&b)
	view := stripANSI(b.String())
	for _, want := range []string{"Implementation", "interrupted", "elapsed 1m0s", "remaining 2m0s", "restart", "evidence preserved", "/hero-continue"} {
		if !strings.Contains(view, want) {
			t.Fatalf("budget display missing %q", want)
		}
	}
}
