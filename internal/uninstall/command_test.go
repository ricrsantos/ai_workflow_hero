package uninstall_test

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/uninstall"
)

func TestUninstallConfirmCopy(t *testing.T) {
	if !strings.Contains(uninstall.ConfirmTitleForTest(), "Proceed with uninstalling Hero") {
		t.Fatalf("title=%q", uninstall.ConfirmTitleForTest())
	}
	if !strings.Contains(uninstall.ConfirmBodyForTest(), ".workflow-hero/") {
		t.Fatalf("body=%q", uninstall.ConfirmBodyForTest())
	}
}

func TestResolveUninstallProceed_SkipPrompt(t *testing.T) {
	var out strings.Builder
	got, err := uninstall.ResolveUninstallProceedForTest(true, &out, &out)
	if err != nil || !got {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestResolveUninstallProceed_NonInteractiveRequiresYes(t *testing.T) {
	var out strings.Builder
	got, err := uninstall.ResolveUninstallProceedForTest(false, &out, &out)
	if got {
		t.Fatal("expected false without --yes in non-interactive mode")
	}
	if err == nil || err.Error() != "uninstall requires confirmation" {
		t.Fatalf("err=%v", err)
	}
	if err.Suggestion == "" || !strings.Contains(err.Suggestion, "--yes") {
		t.Fatalf("suggestion=%q", err.Suggestion)
	}
}
