package cycle

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reprotest"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// ReproGateError is returned when a claimed-done finding's repro test did not pass.
type ReproGateError struct {
	FindingID string
	Package   string
	Test      string
	Detail    string
}

func (e *ReproGateError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("repro_test_failed: %s %s %s — %s", e.FindingID, e.Package, e.Test, e.Detail)
}

var runFindingRepro = reprotest.Run

// SetFindingReproRunForTest replaces the go-test runner. Tests must call the
// returned restore function.
func SetFindingReproRunForTest(fn func(ctx context.Context, root string, spec reprotest.Spec) error) func() {
	prev := runFindingRepro
	if fn == nil {
		runFindingRepro = reprotest.Run
	} else {
		runFindingRepro = fn
	}
	return func() { runFindingRepro = prev }
}

// VerifyCompletedFindingRepros re-runs each completed finding's locked Go test
// before the scheduler may mark the ID done. Findings without a locked repro
// (pre-v14 rows) are skipped so in-flight cycles can still close; the next
// validation report must supply repro and lock it.
func VerifyCompletedFindingRepros(ctx context.Context, st *store.Store, cycleID int64, projectDir string, findingIDs []string) error {
	if st == nil {
		return fmt.Errorf("store is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for _, id := range findingIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		f, err := st.GetFinding(cycleID, id)
		if err != nil {
			return fmt.Errorf("finding %q: %w", id, err)
		}
		occs, err := st.ListFindingOccurrences(cycleID, id)
		if err != nil {
			return fmt.Errorf("finding %q occurrences: %w", id, err)
		}
		pkg, test, _ := store.LatestFindingRepro(f, occs)
		if strings.TrimSpace(pkg) == "" || strings.TrimSpace(test) == "" {
			continue
		}
		if err := runFindingRepro(ctx, projectDir, reprotest.Spec{Package: pkg, Test: test}); err != nil {
			return &ReproGateError{
				FindingID: id,
				Package:   pkg,
				Test:      test,
				Detail:    err.Error(),
			}
		}
	}
	return nil
}
