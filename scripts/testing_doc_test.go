package scripts

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTestingDocC17GateStatus(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	docPath := filepath.Join(filepath.Dir(thisFile), "..", "docs", "testing", "TESTING.md")
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read TESTING.md: %v", err)
	}
	doc := string(data)
	stale := []struct{ phrase, actual string }{
		{"selected-method admission is not yet wired before live stage dispatch", "pre-dispatch selected-method admission is wired in internal/tui/stage_preparation.go (design D10)"},
		{"production pre-dispatch stage-session integration remains a release gate", "pre-dispatch stage-session integration is implemented and gates dispatch"},
		{"C17 final lint/static release acceptance is not green", "D9 baseline-rule lint acceptance passes on the current tree"},
	}
	for _, s := range stale {
		if strings.Contains(doc, s.phrase) {
			t.Errorf("TESTING.md C17 gate statement stale: %q; actual: %s", s.phrase, s.actual)
		}
	}
}
