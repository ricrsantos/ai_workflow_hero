package cycle

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reprotest"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestVerifyCompletedFindingReprosSkipsLegacyAndRejectsFailure(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "hero.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	cycleID, err := s.CreateCycle(store.Cycle{Number: 1, Title: "t", Status: store.CycleStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := s.PersistFinding(store.FindingInput{
		CycleID: cycleID, SourceStage: store.FindingSourceQA, Owner: store.FindingOwnerGeneric,
		File: "internal/tui/a.go", Issue: "legacy", AcceptanceCriteria: "broad ac",
	})
	if err != nil {
		t.Fatal(err)
	}
	src := "package tui\n\nimport \"testing\"\n\nfunc TestFindQALocked(t *testing.T) {\n\tt.Fatal(\"x\")\n}\n"
	locked, err := s.PersistFinding(store.FindingInput{
		CycleID: cycleID, SourceStage: store.FindingSourceQA, Owner: store.FindingOwnerGeneric,
		File: "internal/tui/b.go", Issue: "locked", AcceptanceCriteria: "test passes",
		ReproPackage: "./internal/tui", ReproTest: "TestFindQALocked", ReproSource: src,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := VerifyCompletedFindingRepros(context.Background(), s, cycleID, t.TempDir(), []string{legacy.Finding.ID}); err != nil {
		t.Fatalf("legacy empty repro must skip: %v", err)
	}

	restore := SetFindingReproRunForTest(func(context.Context, string, reprotest.Spec) error {
		return errors.New("test still fails")
	})
	t.Cleanup(restore)
	err = VerifyCompletedFindingRepros(context.Background(), s, cycleID, t.TempDir(), []string{locked.Finding.ID})
	var gate *ReproGateError
	if !errors.As(err, &gate) || gate.FindingID != locked.Finding.ID {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(gate.Error(), "repro_test_failed") {
		t.Fatalf("error=%q", gate.Error())
	}

	restorePass := SetFindingReproRunForTest(func(context.Context, string, reprotest.Spec) error { return nil })
	t.Cleanup(restorePass)
	if err := VerifyCompletedFindingRepros(context.Background(), s, cycleID, t.TempDir(), []string{locked.Finding.ID}); err != nil {
		t.Fatalf("passing repro: %v", err)
	}
}
