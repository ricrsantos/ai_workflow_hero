package cycle

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/findingrepro"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reprotest"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

// ReproGateBudget caps how long the scheduler may spend re-running the repros
// of a single Implementation wave. Without it a wave with several findings
// could hold the handoff for the per-test timeout multiplied by the finding
// count.
const ReproGateBudget = 10 * time.Minute

// ReproGateError is returned when a claimed-done finding's repro test did not pass.
type ReproGateError struct {
	FindingID string
	Mode      string
	Package   string
	Test      string
	Detail    string
}

func (e *ReproGateError) Error() string {
	if e == nil {
		return ""
	}
	identity := strings.TrimSpace(e.Package + " " + e.Test)
	if identity == "" {
		identity = e.Mode
	}
	return fmt.Sprintf("repro_test_failed: %s %s — %s", e.FindingID, identity, e.Detail)
}

var runFindingRepro = reprotest.Run

// SetFindingReproRunForTest replaces the repro runner. Tests must call the
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

// VerifyCompletedFindingRepros re-runs each completed finding's locked repro
// before the scheduler may mark the ID done. Findings without an automated
// repro (evidence-only, or pre-v14 rows) are skipped so in-flight cycles can
// still close; the next validation report must supply one and lock it.
func VerifyCompletedFindingRepros(ctx context.Context, st *store.Store, cycleID int64, projectDir string, findingIDs []string) error {
	results, err := VerifyFindingRepros(ctx, st, cycleID, projectDir, findingIDs)
	if err != nil {
		return err
	}
	for _, id := range findingIDs {
		if gate := results[strings.TrimSpace(id)]; gate != nil {
			return gate
		}
	}
	return nil
}

// VerifyFindingRepros re-runs the locked repro of each finding and returns the
// per-ID outcome. A nil entry means the repro passed or had no automated gate.
// The returned error is reserved for infrastructure failures (store, config).
func VerifyFindingRepros(ctx context.Context, st *store.Store, cycleID int64, projectDir string, findingIDs []string) (map[string]*ReproGateError, error) {
	if st == nil {
		return nil, fmt.Errorf("store is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, ReproGateBudget)
		defer cancel()
	}
	policy := workflowconfig.ReproPolicyForProject(projectDir, nil)
	out := make(map[string]*ReproGateError, len(findingIDs))
	for _, id := range findingIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		f, err := st.GetFinding(cycleID, id)
		if err != nil {
			return nil, fmt.Errorf("finding %q: %w", id, err)
		}
		occs, err := st.ListFindingOccurrences(cycleID, id)
		if err != nil {
			return nil, fmt.Errorf("finding %q occurrences: %w", id, err)
		}
		mode, pkg, test, _ := store.LatestFindingReproWithMode(f, occs)
		if mode == "" || mode == findingrepro.ModeEvidence {
			out[id] = nil
			continue
		}
		if strings.TrimSpace(pkg) == "" || strings.TrimSpace(test) == "" {
			out[id] = nil
			continue
		}
		spec := reprotest.Spec{Mode: mode, Package: pkg, Test: test}
		if mode == findingrepro.ModeCommand {
			argv, aerr := policy.CommandArgv(pkg, test)
			if aerr != nil {
				out[id] = &ReproGateError{FindingID: id, Mode: mode, Package: pkg, Test: test, Detail: aerr.Error()}
				continue
			}
			spec.Argv = argv
		}
		if err := runFindingRepro(ctx, projectDir, spec); err != nil {
			out[id] = &ReproGateError{
				FindingID: id,
				Mode:      mode,
				Package:   pkg,
				Test:      test,
				Detail:    err.Error(),
			}
			continue
		}
		out[id] = nil
	}
	return out, nil
}
