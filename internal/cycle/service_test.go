package cycle_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func setupProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cycleDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `title: CLI Feature
objective: Test CLI
stages:
  research:
    enabled: true
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: false
  qa:
    enabled: true
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: true
  judge:
    enabled: true
    max_iterations: 1
    timeout_minutes: 10
    require_human_approval: false
`
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow-config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	// Touch hero.json so FindProjectRoot works via .workflow-hero dir.
	_ = os.MkdirAll(filepath.Join(dir, ".workflow-hero", "config"), 0o755)
	return dir
}

func TestServiceCycleLifecycleAndReads(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	res, err := svc.NewCycle("", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Cycle.Number != 1 || len(res.Stages) < 2 {
		t.Fatalf("new cycle: %+v", res)
	}

	st, err := svc.Status()
	if err != nil || len(st.Stages) == 0 {
		t.Fatalf("status: %+v %v", st, err)
	}
	if st.OpenspecChange != "" {
		t.Fatalf("openspec_change should start empty, got %q", st.OpenspecChange)
	}
	if err := svc.SetOpenspecChange("slash-parity-tui-harness"); err != nil {
		t.Fatal(err)
	}
	st, err = svc.Status()
	if err != nil || st.OpenspecChange != "slash-parity-tui-harness" {
		t.Fatalf("status after set: %+v %v", st, err)
	}
	if err := svc.ClearOpenspecChange(); err != nil {
		t.Fatal(err)
	}
	st, err = svc.Status()
	if err != nil || st.OpenspecChange != "" {
		t.Fatalf("status after clear: %+v %v", st, err)
	}

	if err := svc.StartStage("research"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("research", "done", `{"agent":"discover_agent","input_tokens":40,"output_tokens":10}`, false); err != nil {
		t.Fatal(err)
	}

	m, err := svc.Metrics()
	if err != nil || m.TotalIn != 40 {
		t.Fatalf("metrics: %+v %v", m, err)
	}
	ev, err := svc.Events("", 20)
	if err != nil || len(ev.Events) == 0 {
		t.Fatalf("events: %+v %v", ev, err)
	}

	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("qa", "ready", "", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Approve("lgtm", `{"agent":"qa_agent","input_tokens":8}`); err != nil {
		t.Fatal(err)
	}

	if err := svc.Finish(""); err != nil {
		t.Fatal(err)
	}
	arch, err := svc.Archive()
	if err != nil {
		t.Fatal(err)
	}
	if arch.CycleNumber != 1 {
		t.Fatalf("archive: %+v", arch)
	}
	if _, err := os.Stat(arch.ArchiveDir); err != nil {
		t.Fatalf("archive dir missing: %v", err)
	}

	// Resume cancelled path — restore config after archive emptied current/.
	svc2, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc2.Close()
	cfgPath := filepath.Join(dir, ".workflow-hero", "cycles", "current", "workflow-config.yml")
	_ = os.MkdirAll(filepath.Dir(cfgPath), 0o755)
	if err := os.WriteFile(cfgPath, []byte(`title: Second
objective: obj
stages:
  research:
    enabled: true
    max_iterations: 1
    timeout_minutes: 5
    require_human_approval: false
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc2.NewCycle("Second", "obj"); err != nil {
		t.Fatal(err)
	}
	if err := svc2.Cancel("stop"); err != nil {
		t.Fatal(err)
	}
	if err := svc2.Resume(2); err != nil {
		t.Fatal(err)
	}
	c, err := svc2.Store.GetActiveCycle()
	if err != nil || c.Number != 2 || c.Status != store.CycleStatusActive {
		t.Fatalf("resume: %+v %v", c, err)
	}
}

func TestServiceRunRecordsHarnessInvokedFallback(t *testing.T) {
	dir := setupProject(t)
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# start"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("research"); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Run("")
	if err != nil {
		t.Fatal(err)
	}
	if res.Dispatched {
		t.Fatalf("expected fallback dispatch, got %+v", res)
	}
	if res.Stage != "research" {
		t.Fatalf("stage=%q want research", res.Stage)
	}
	if res.Message == "" {
		t.Fatal("expected fallback message")
	}

	ev, err := svc.Events(store.EventHarnessInvoked, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.Events) != 1 {
		t.Fatalf("events=%d want 1 harness_invoked", len(ev.Events))
	}
	if ev.Events[0].Type != store.EventHarnessInvoked {
		t.Fatalf("event type=%q", ev.Events[0].Type)
	}
}

func TestFindProjectRoot(t *testing.T) {
	dir := setupProject(t)
	nested := filepath.Join(dir, "pkg", "x")
	_ = os.MkdirAll(nested, 0o755)
	root, err := cycle.FindProjectRoot(nested)
	if err != nil || root != dir {
		t.Fatalf("root=%q err=%v want %q", root, err, dir)
	}
}
