package tui

import (
	"strings"
	"testing"
)

func TestCycleWelcomeDialogRendersGuidanceAndButtons(t *testing.T) {
	m := NewTestModel(nil)
	m.width = 120
	m.height = 42
	m.cycleWelcomeDialog = true

	view := stripANSI(ViewForTest(m))
	for _, want := range []string{
		"New development cycle created",
		"Thank you for starting a new development cycle with Hero!",
		"authenticated with every Harness",
		"Equalize my Skills across the configured Harnesses",
		"docs/idea/",
		".workflow-hero/cycles/current/workflow-config.yml",
		"[ Go to Config ]",
		"[ Close ]",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("welcome dialog missing %q:\n%s", want, view)
		}
	}
}

func TestCycleWelcomeDialogCloseAndGoToConfig(t *testing.T) {
	m := NewTestModel(nil)
	m.status.CycleNumber = 1
	m.cycleWelcomeDialog = true

	closed, _ := HandleTestKey(m, "esc")
	if closed.cycleWelcomeDialog {
		t.Fatal("esc must close the welcome dialog")
	}

	m.cycleWelcomeDialog = true
	configured, _ := HandleTestKey(m, "enter")
	if configured.cycleWelcomeDialog {
		t.Fatal("Go to Config must close the welcome dialog")
	}
	if configured.screen != screenConfig {
		t.Fatalf("screen=%v, want Config", configured.screen)
	}
}

func TestCycleWelcomeDialogCloseButtonAndSmallTerminalFallback(t *testing.T) {
	m := NewTestModel(nil)
	m.cycleWelcomeDialog = true
	m.cycleWelcomeFocus = 1

	closed, _ := HandleTestKey(m, "enter")
	if closed.cycleWelcomeDialog {
		t.Fatal("Close button must dismiss the welcome dialog")
	}

	m = NewTestModel(nil)
	m.width = 50
	m.height = 20
	m.cycleWelcomeDialog = true
	view := stripANSI(ViewForTest(m))
	if !strings.Contains(view, "Resize the terminal") {
		t.Fatalf("small terminal fallback missing:\n%s", view)
	}
}
