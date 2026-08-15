package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func openTestEngine(t *testing.T) (*Engine, *store.Store) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "hero.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	e := New(s)
	e.Now = func() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC) }
	return e, s
}

func seedCycle(t *testing.T, s *store.Store, approval bool) int64 {
	t.Helper()
	id, err := s.CreateCycle(store.Cycle{
		Number: 1, Title: "T", Objective: "O", Status: store.CycleStatusActive,
		StartedAt: "2026-08-07T12:00:00Z", ConfigSnapshotJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	stages := []store.Stage{
		{CycleID: id, Name: "research", Status: store.StageWaiting, MaxIterations: 2, SortOrder: 0},
		{CycleID: id, Name: "qa", Status: store.StageWaiting, MaxIterations: 2, RequireHumanApproval: approval, SortOrder: 1},
		{CycleID: id, Name: "judge", Status: store.StageWaiting, MaxIterations: 1, SortOrder: 2},
	}
	if err := s.CreateStages(stages); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTransitionsApproveRejectCancelFinishContinue(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, e *Engine, s *store.Store)
	}{
		{
			name: "auto complete without approval",
			run: func(t *testing.T, e *Engine, s *store.Store) {
				id := seedCycle(t, s, false)
				if err := e.StartStage(id, "research"); err != nil {
					t.Fatal(err)
				}
				if err := e.CloseStage(id, "research", StageCloseInput{
					Summary: "done",
					Metrics: []MetricInput{{Agent: "discover_agent", InputTokens: 40, OutputTokens: 10}},
				}); err != nil {
					t.Fatal(err)
				}
				st, _ := s.GetStage(id, "research")
				if st.Status != store.StageCompleted {
					t.Fatalf("status=%s", st.Status)
				}
				metrics, _ := s.ListMetrics(id)
				if len(metrics) != 1 || metrics[0].InputTokens != 40 {
					t.Fatalf("metrics=%+v", metrics)
				}
			},
		},
		{
			name: "pending approval then approve",
			run: func(t *testing.T, e *Engine, s *store.Store) {
				id := seedCycle(t, s, true)
				_ = e.StartStage(id, "qa")
				if err := e.CloseStage(id, "qa", StageCloseInput{Summary: "ready"}); err != nil {
					t.Fatal(err)
				}
				st, _ := s.GetStage(id, "qa")
				if st.Status != store.StagePendingApproval {
					t.Fatalf("want PendingApproval got %s", st.Status)
				}
				if err := e.Approve("cli-1", "ok", []MetricInput{{StageName: "qa", InputTokens: 10}}); err != nil {
					t.Fatal(err)
				}
				st, _ = s.GetStage(id, "qa")
				if st.Status != store.StageCompleted {
					t.Fatalf("after approve status=%s", st.Status)
				}
			},
		},
		{
			name: "reject returns to waiting",
			run: func(t *testing.T, e *Engine, s *store.Store) {
				id := seedCycle(t, s, true)
				_ = e.StartStage(id, "qa")
				_ = e.CloseStage(id, "qa", StageCloseInput{})
				if err := e.Reject("cli-1", "needs work"); err != nil {
					t.Fatal(err)
				}
				st, _ := s.GetStage(id, "qa")
				if st.Status != store.StageWaiting {
					t.Fatalf("status=%s", st.Status)
				}
			},
		},
		{
			name: "cancel cycle",
			run: func(t *testing.T, e *Engine, s *store.Store) {
				_ = seedCycle(t, s, false)
				if err := e.Cancel("cli-1", "user abort"); err != nil {
					t.Fatal(err)
				}
				c, err := s.GetActiveCycle()
				if err == nil {
					t.Fatalf("expected no active cycle, got %+v", c)
				}
			},
		},
		{
			name: "finish cycle",
			run: func(t *testing.T, e *Engine, s *store.Store) {
				id := seedCycle(t, s, true)
				_ = e.StartStage(id, "qa")
				_ = e.CloseStage(id, "qa", StageCloseInput{})
				if err := e.Finish("cli-1", nil); err != nil {
					t.Fatal(err)
				}
				c, _ := s.GetCycle(id)
				if c.Status != store.CycleStatusCompleted || c.CompletedAt == "" {
					t.Fatalf("cycle=%+v", c)
				}
			},
		},
		{
			name: "iteration escalate and continue",
			run: func(t *testing.T, e *Engine, s *store.Store) {
				id := seedCycle(t, s, false)
				_ = e.StartStage(id, "research") // iter 1
				_ = e.CloseStage(id, "research", StageCloseInput{})
				// Reset research to Waiting with iteration=2 already used max=2 → start should escalate
				st, _ := s.GetStage(id, "research")
				st.Status = store.StageWaiting
				st.CompletedAt = ""
				_ = s.UpdateStage(st)
				_ = e.StartStage(id, "research") // iter 2
				_ = e.CloseStage(id, "research", StageCloseInput{})
				st, _ = s.GetStage(id, "research")
				st.Status = store.StageWaiting
				_ = s.UpdateStage(st)
				err := e.StartStage(id, "research") // iter would be 3 > max 2
				if err == nil {
					t.Fatal("expected escalation error")
				}
				st, _ = s.GetStage(id, "research")
				if st.Status != store.StageEscalated {
					t.Fatalf("status=%s", st.Status)
				}
				if err := e.Continue("cli-1", 2); err != nil {
					t.Fatal(err)
				}
				st, _ = s.GetStage(id, "research")
				if st.Status != store.StageWaiting || st.ExtraIterations != 2 {
					t.Fatalf("after continue %+v", st)
				}
			},
		},
		{
			name: "lock busy",
			run: func(t *testing.T, e *Engine, s *store.Store) {
				id := seedCycle(t, s, true)
				if _, err := e.AcquireLock("session-a"); err != nil {
					t.Fatal(err)
				}
				_, err := e.AcquireLock("session-b")
				if err != store.ErrBusy {
					t.Fatalf("want ErrBusy got %v", err)
				}
				_ = e.ReleaseLock(id, "session-a")
				if _, err := e.AcquireLock("session-b"); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, s := openTestEngine(t)
			tt.run(t, e, s)
		})
	}
}

func TestCreateCycleFromConfig(t *testing.T) {
	dir := t.TempDir()
	cycleDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `title: Feature X
objective: Build X
stages:
  research:
    enabled: true
    max_iterations: 5
    timeout_minutes: 15
    require_human_approval: false
  planning:
    enabled: true
    max_iterations: 3
    timeout_minutes: 20
    require_human_approval: false
  implementation:
    enabled: true
    max_iterations: 4
    timeout_minutes: 30
    require_human_approval: false
  qa:
    enabled: true
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: true
  judge:
    enabled: true
    max_iterations: 3
    timeout_minutes: 10
    require_human_approval: false
  browser_ui_validation:
    enabled: false
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: true
  qa_end_to_end:
    enabled: true
    max_iterations: 3
    timeout_minutes: 15
    require_human_approval: true
`
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow-config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	e, _ := openTestEngine(t)
	res, err := e.CreateCycleFromConfig(NewCycleOptions{ProjectDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Cycle.Number != 1 || res.Cycle.Title != "Feature X" {
		t.Fatalf("cycle=%+v", res.Cycle)
	}
	if len(res.Stages) != 7 {
		t.Fatalf("stages=%d", len(res.Stages))
	}
	byName := map[string]store.Stage{}
	for _, st := range res.Stages {
		byName[st.Name] = st
	}
	if byName["browser_ui_validation"].Status != store.StageSkipped {
		t.Fatalf("browser should be skipped: %+v", byName["browser_ui_validation"])
	}
	if !byName["qa"].RequireHumanApproval {
		t.Fatal("qa should require approval")
	}
	if res.Cycle.ConfigSnapshotJSON == "" || res.Cycle.ConfigSnapshotJSON == "{}" {
		t.Fatal("expected config snapshot from yaml")
	}
}

func TestCreateCycleFromConfigDeferMeta(t *testing.T) {
	dir := t.TempDir()
	cycleDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `title: Feature X
objective: Build X
stages:
  research:
    enabled: true
    max_iterations: 1
    require_human_approval: false
`
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow-config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	e, _ := openTestEngine(t)
	res, err := e.CreateCycleFromConfig(NewCycleOptions{ProjectDir: dir, DeferMeta: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Cycle.Title != "" || res.Cycle.Objective != "" {
		t.Fatalf("expected empty meta, got title=%q objective=%q", res.Cycle.Title, res.Cycle.Objective)
	}
	if res.Cycle.Status != store.CycleStatusActive {
		t.Fatalf("status=%q", res.Cycle.Status)
	}
}

func TestSyncCycleConfigFromWorkflow(t *testing.T) {
	dir := t.TempDir()
	cycleDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `title: Placeholder
objective: Pending
stages:
  research:
    enabled: true
    max_iterations: 1
    require_human_approval: false
`
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow-config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	e, _ := openTestEngine(t)
	res, err := e.CreateCycleFromConfig(NewCycleOptions{ProjectDir: dir, DeferMeta: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Cycle.Title != "" {
		t.Fatal("expected empty title before sync")
	}

	updated := `title: Synced Title
objective: Synced Objective
stages:
  research:
    enabled: true
    max_iterations: 1
    require_human_approval: false
`
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow-config.yml"), []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.SyncCycleConfigFromWorkflow(dir); err != nil {
		t.Fatal(err)
	}
	c, err := e.Store.GetCycle(res.Cycle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != "Synced Title" || c.Objective != "Synced Objective" {
		t.Fatalf("cycle=%+v", c)
	}
	if !strings.Contains(c.ConfigSnapshotJSON, "Synced Title") {
		t.Fatalf("snapshot not updated: %q", c.ConfigSnapshotJSON)
	}
}

func TestSyncCycleConfigUpdatesStageBudgets(t *testing.T) {
	dir := t.TempDir()
	cycleDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `title: Budgets
objective: Pending
stages:
  research:
    enabled: true
    max_iterations: 1
    timeout_minutes: 5
    require_human_approval: false
  planning:
    enabled: true
    max_iterations: 1
    timeout_minutes: 5
    require_human_approval: false
  qa_end_to_end:
    enabled: true
    max_iterations: 1
    timeout_minutes: 5
    require_human_approval: true
`
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow-config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	e, s := openTestEngine(t)
	res, err := e.CreateCycleFromConfig(NewCycleOptions{ProjectDir: dir, DeferMeta: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.StartStage(res.Cycle.ID, "research"); err != nil {
		t.Fatal(err)
	}
	if err := e.CloseStage(res.Cycle.ID, "research", StageCloseInput{Summary: "done"}); err != nil {
		t.Fatal(err)
	}
	if err := e.StartStage(res.Cycle.ID, "planning"); err != nil {
		t.Fatal(err)
	}

	updated := `title: Budgets
objective: Synced
stages:
  research:
    enabled: true
    max_iterations: 50
    timeout_minutes: 15
    require_human_approval: false
  planning:
    enabled: true
    max_iterations: 4
    timeout_minutes: 20
    require_human_approval: true
  qa_end_to_end:
    enabled: false
    max_iterations: 3
    timeout_minutes: 15
    require_human_approval: false
`
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow-config.yml"), []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.SyncCycleConfigFromWorkflow(dir); err != nil {
		t.Fatal(err)
	}

	research, err := s.GetStage(res.Cycle.ID, "research")
	if err != nil {
		t.Fatal(err)
	}
	if research.Status != store.StageCompleted || research.MaxIterations != 1 {
		t.Fatalf("completed research must keep original budget: %+v", research)
	}

	planning, err := s.GetStage(res.Cycle.ID, "planning")
	if err != nil {
		t.Fatal(err)
	}
	if planning.Status != store.StageRunning {
		t.Fatalf("running planning status=%s", planning.Status)
	}
	if planning.MaxIterations != 4 || planning.TimeoutMinutes != 20 || !planning.RequireHumanApproval {
		t.Fatalf("running planning budgets not synced: %+v", planning)
	}
	if planning.Iteration != 1 {
		t.Fatalf("running planning iteration reset: %+v", planning)
	}

	e2e, err := s.GetStage(res.Cycle.ID, "qa_end_to_end")
	if err != nil {
		t.Fatal(err)
	}
	if e2e.Status != store.StageSkipped {
		t.Fatalf("waiting e2e should become skipped, got %s", e2e.Status)
	}
	if e2e.MaxIterations != 3 || e2e.TimeoutMinutes != 15 || e2e.RequireHumanApproval {
		t.Fatalf("waiting e2e budgets not synced: %+v", e2e)
	}
}

func TestParseMetricsJSON(t *testing.T) {
	m, err := ParseMetricsJSON(`{"agent":"qa_agent","input_tokens":12,"output_tokens":3}`)
	if err != nil || len(m) != 1 || m[0].InputTokens != 12 {
		t.Fatalf("%+v %v", m, err)
	}
	m, err = ParseMetricsJSON(`[{"agent":"a"},{"agent":"b"}]`)
	if err != nil || len(m) != 2 {
		t.Fatalf("%+v %v", m, err)
	}
}

func TestTimeoutEscalationBetweenIterations(t *testing.T) {
	e, s := openTestEngine(t)
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	e.Now = func() time.Time { return now }

	id, err := s.CreateCycle(store.Cycle{
		Number: 1, Title: "T", Objective: "O", Status: store.CycleStatusActive,
		StartedAt: "2026-08-07T12:00:00Z", ConfigSnapshotJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateStages([]store.Stage{
		{
			CycleID: id, Name: "research", Status: store.StageWaiting,
			MaxIterations: 5, TimeoutMinutes: 10, SortOrder: 0,
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := e.StartStage(id, "research"); err != nil {
		t.Fatal(err)
	}
	st, _ := s.GetStage(id, "research")
	if st.StartedAt != "2026-08-07T12:00:00Z" {
		t.Fatalf("started_at=%q", st.StartedAt)
	}
	if err := e.CloseStage(id, "research", StageCloseInput{Summary: "iter1"}); err != nil {
		t.Fatal(err)
	}
	// Rework loop: back to Waiting with iterations remaining, clock still from first start.
	st, _ = s.GetStage(id, "research")
	st.Status = store.StageWaiting
	st.CompletedAt = ""
	if err := s.UpdateStage(st); err != nil {
		t.Fatal(err)
	}

	// Advance past timeout_minutes (10) between iterations.
	now = now.Add(11 * time.Minute)
	err = e.StartStage(id, "research")
	if err == nil {
		t.Fatal("expected timeout escalation on StartStage")
	}
	st, _ = s.GetStage(id, "research")
	if st.Status != store.StageEscalated {
		t.Fatalf("status=%s want Escalated", st.Status)
	}
	// Refuse further work until continue/cancel/finish.
	if err := e.StartStage(id, "research"); err == nil {
		t.Fatal("expected refuse while Escalated")
	}

	if err := e.Continue("cli-1", 1); err != nil {
		t.Fatal(err)
	}
	st, _ = s.GetStage(id, "research")
	if st.Status != store.StageWaiting || st.StartedAt != "" {
		t.Fatalf("after continue want Waiting + cleared StartedAt, got %+v", st)
	}
	// Fresh clock after continue — start must succeed.
	now = now.Add(time.Minute)
	if err := e.StartStage(id, "research"); err != nil {
		t.Fatal(err)
	}
}

func TestEscalateIfExhaustedTimeout(t *testing.T) {
	e, s := openTestEngine(t)
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	e.Now = func() time.Time { return now }

	id, err := s.CreateCycle(store.Cycle{
		Number: 1, Title: "T", Objective: "O", Status: store.CycleStatusActive,
		StartedAt: "2026-08-07T12:00:00Z", ConfigSnapshotJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateStages([]store.Stage{
		{
			CycleID: id, Name: "qa", Status: store.StageWaiting,
			MaxIterations: 5, TimeoutMinutes: 5, SortOrder: 0,
			StartedAt: "2026-08-07T12:00:00Z", Iteration: 1,
		},
	}); err != nil {
		t.Fatal(err)
	}

	now = now.Add(6 * time.Minute)
	if err := e.EscalateIfExhausted(id, "qa"); err != nil {
		t.Fatal(err)
	}
	st, _ := s.GetStage(id, "qa")
	if st.Status != store.StageEscalated {
		t.Fatalf("status=%s", st.Status)
	}
}

func TestTimeoutZeroDisablesEscalation(t *testing.T) {
	e, s := openTestEngine(t)
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	e.Now = func() time.Time { return now }

	id, err := s.CreateCycle(store.Cycle{
		Number: 1, Title: "T", Objective: "O", Status: store.CycleStatusActive,
		StartedAt: "2026-08-07T12:00:00Z", ConfigSnapshotJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateStages([]store.Stage{
		{
			CycleID: id, Name: "research", Status: store.StageWaiting,
			MaxIterations: 5, TimeoutMinutes: 0, SortOrder: 0,
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.StartStage(id, "research"); err != nil {
		t.Fatal(err)
	}
	_ = e.CloseStage(id, "research", StageCloseInput{})
	st, _ := s.GetStage(id, "research")
	st.Status = store.StageWaiting
	st.CompletedAt = ""
	_ = s.UpdateStage(st)

	now = now.Add(24 * time.Hour)
	if err := e.StartStage(id, "research"); err != nil {
		t.Fatalf("timeout_minutes=0 must not escalate: %v", err)
	}
}
