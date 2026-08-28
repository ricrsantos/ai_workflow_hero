package store

import (
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenMigrateAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hero.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	v, err := s.SchemaVersion()
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if v != currentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", v, currentSchemaVersion)
	}

	// Re-open should be idempotent.
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	defer s2.Close()

	id, err := s.CreateCycle(Cycle{
		Number:                 1,
		Title:                  "Test",
		Objective:              "Obj",
		Status:                 CycleStatusActive,
		StartedAt:              nowRFC3339(),
		SessionDurationSeconds: 12,
		ConfigSnapshotJSON:     `{"title":"Test"}`,
	})
	if err != nil {
		t.Fatalf("CreateCycle: %v", err)
	}

	if err := s.CreateStages([]Stage{
		{CycleID: id, Name: "research", Status: StageWaiting, MaxIterations: 50, SortOrder: 0},
		{CycleID: id, Name: "planning", Status: StageWaiting, MaxIterations: 3, RequireHumanApproval: true, SortOrder: 1},
	}); err != nil {
		t.Fatalf("CreateStages: %v", err)
	}

	stages, err := s.ListStages(id)
	if err != nil {
		t.Fatalf("ListStages: %v", err)
	}
	if len(stages) != 2 {
		t.Fatalf("stages len = %d, want 2", len(stages))
	}
	if stages[1].RequireHumanApproval != true {
		t.Fatal("planning should require approval")
	}

	st := stages[0]
	st.Status = StageRunning
	st.Iteration = 1
	st.StartedAt = nowRFC3339()
	if err := s.UpdateStage(st); err != nil {
		t.Fatalf("UpdateStage: %v", err)
	}

	if _, err := s.AppendEvent(Event{CycleID: id, Type: EventStageStarted, PayloadJSON: `{"stage":"research"}`}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	events, err := s.ListEvents(id, "", 10)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 || events[0].Type != EventStageStarted {
		t.Fatalf("events = %+v", events)
	}

	if err := s.UpsertMetric(Metric{
		CycleID: id, StageName: "research", Agent: "discover_agent", Model: "composer-2.5",
		InputTokens: 100, OutputTokens: 50, CostUSD: 0.01, DurationMS: 1200,
	}); err != nil {
		t.Fatalf("UpsertMetric: %v", err)
	}
	if err := s.UpsertMetric(Metric{
		CycleID: id, StageName: "research", Agent: "discover_agent", Model: "composer-2.5",
		InputTokens: 200, OutputTokens: 80, CostUSD: 0.02, DurationMS: 1500,
	}); err != nil {
		t.Fatalf("UpsertMetric replace: %v", err)
	}
	metrics, err := s.ListMetrics(id)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(metrics) != 1 || metrics[0].InputTokens != 200 {
		t.Fatalf("metrics = %+v", metrics)
	}

	if _, err := s.AddArtifact(Artifact{CycleID: id, Path: "docs/product/PRD.md", Kind: "prd", Label: "PRD"}); err != nil {
		t.Fatalf("AddArtifact: %v", err)
	}
	arts, err := s.ListArtifacts(id)
	if err != nil || len(arts) != 1 {
		t.Fatalf("artifacts: %v %+v", err, arts)
	}

	active, err := s.GetActiveCycle()
	if err != nil {
		t.Fatalf("GetActiveCycle: %v", err)
	}
	if active.ID != id || active.Title != "Test" || active.SessionDurationSeconds != 12 {
		t.Fatalf("active = %+v", active)
	}
	if err := s.UpdateCycleSessionDuration(id, 27); err != nil {
		t.Fatalf("UpdateCycleSessionDuration: %v", err)
	}
	if err := s.UpdateCycleSessionDuration(id, 9); err != nil {
		t.Fatalf("monotonic UpdateCycleSessionDuration: %v", err)
	}
	active, err = s.GetActiveCycle()
	if err != nil || active.SessionDurationSeconds != 27 {
		t.Fatalf("session duration = %d, want 27 (err=%v)", active.SessionDurationSeconds, err)
	}
}

func TestImportLegacyCycle(t *testing.T) {
	dir := t.TempDir()
	cycleDir := filepath.Join(dir, "current")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}

	workflow := `# Workflow — Cycle C3

**Title**: Legacy Feature
**Objective**: Import me
**Status**: In Progress
**Started**: 2026-07-01
**Completed**:

## Stages

| Stage | Status | Iteration | Human Approval | Extra Iterations Granted |
|-------|--------|-----------|----------------|--------------------------|
| Research | Completed | 1/50 | Auto | +0 |
| Planning | In Progress | 2/3 | N/A | +1 |
| QA | Waiting | 0/2 | Required | +0 |
| Browser UI Validation | Skipped | 0/2 | N/A | +0 |
`
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow.md"), []byte(workflow), 0o644); err != nil {
		t.Fatal(err)
	}

	metrics := `# Metrics — Cycle C3

## Stage Metrics

| Stage | Agent | Model | Input Tokens | Output Tokens | Cost (USD) | Duration |
|-------|-------|-------|-------------|---------------|------------|----------|
| Research | discover_agent | composer-2.5 | 400 | 100 | 0.05 | 2s |
| Planning | planning_agent | — | — | — | — | — |
| **Subtotal** | | | | | | |
`
	if err := os.WriteFile(filepath.Join(cycleDir, "metrics.md"), []byte(metrics), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Open(filepath.Join(dir, "hero.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	res, err := s.ImportLegacyCycle(cycleDir, 0, `{"imported":true}`)
	if err != nil {
		t.Fatalf("ImportLegacyCycle: %v", err)
	}
	if !res.Imported || res.CycleNumber != 3 || res.Stages != 4 {
		t.Fatalf("result = %+v", res)
	}
	if res.Metrics != 1 {
		t.Fatalf("metrics imported = %d, want 1 (skip empty dash rows)", res.Metrics)
	}

	c, err := s.GetCycle(res.CycleID)
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != "Legacy Feature" || c.Status != CycleStatusActive {
		t.Fatalf("cycle = %+v", c)
	}

	stages, err := s.ListStages(res.CycleID)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Stage{}
	for _, st := range stages {
		byName[st.Name] = st
	}
	if byName["research"].Status != StageCompleted {
		t.Fatalf("research status = %s", byName["research"].Status)
	}
	if byName["planning"].Status != StageRunning || byName["planning"].ExtraIterations != 1 {
		t.Fatalf("planning = %+v", byName["planning"])
	}
	if byName["browser_ui_validation"].Status != StageSkipped {
		t.Fatalf("browser = %+v", byName["browser_ui_validation"])
	}

	// Missing workflow → no-op.
	emptyDir := filepath.Join(dir, "empty")
	_ = os.MkdirAll(emptyDir, 0o755)
	res2, err := s.ImportLegacyCycle(emptyDir, 9, "{}")
	if err != nil || res2.Imported {
		t.Fatalf("expected no-op import, got %+v err=%v", res2, err)
	}
}

func TestOpenProject(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := os.Stat(filepath.Join(dir, RelativeDBPath)); err != nil {
		t.Fatalf("db file missing: %v", err)
	}
}

func TestOpenspecChangeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "hero.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	id, err := s.CreateCycle(Cycle{
		Number:             1,
		Title:              "C2",
		Status:             CycleStatusActive,
		StartedAt:          nowRFC3339(),
		ConfigSnapshotJSON: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.GetCycle(id)
	if err != nil {
		t.Fatal(err)
	}
	if c.OpenspecChange != "" {
		t.Fatalf("default openspec_change = %q, want empty", c.OpenspecChange)
	}

	if err := s.SetOpenspecChange(id, "slash-parity-tui-harness"); err != nil {
		t.Fatal(err)
	}
	c, err = s.GetCycle(id)
	if err != nil {
		t.Fatal(err)
	}
	if c.OpenspecChange != "slash-parity-tui-harness" {
		t.Fatalf("openspec_change = %q", c.OpenspecChange)
	}

	active, err := s.GetActiveCycle()
	if err != nil || active.OpenspecChange != "slash-parity-tui-harness" {
		t.Fatalf("active openspec_change = %+v %v", active, err)
	}

	if err := s.SetOpenspecChange(id, ""); err != nil {
		t.Fatal(err)
	}
	c, err = s.GetCycle(id)
	if err != nil || c.OpenspecChange != "" {
		t.Fatalf("cleared openspec_change = %+v %v", c, err)
	}
}

func TestMigrateV1ToV2AddsOpenspecChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hero.db")

	// Simulate a Hero 1.0 (schema v1) database, then open with current migrator.
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
)`); err != nil {
		t.Fatal(err)
	}
	s := &Store{db: db, log: slog.Default()}
	if err := s.applyMigration(1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO cycles(number, title, objective, status, config_snapshot_json)
VALUES(1, 'Legacy', 'obj', ?, '{}')`, CycleStatusActive); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("Open after v1: %v", err)
	}
	defer s2.Close()
	v, err := s2.SchemaVersion()
	if err != nil || v != currentSchemaVersion {
		t.Fatalf("schema version = %d %v, want %d", v, err, currentSchemaVersion)
	}
	c, err := s2.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != "Legacy" || c.OpenspecChange != "" {
		t.Fatalf("migrated cycle = %+v", c)
	}
	if err := s2.SetOpenspecChange(c.ID, "hero-1-0"); err != nil {
		t.Fatal(err)
	}
	c, err = s2.GetCycle(c.ID)
	if err != nil || c.OpenspecChange != "hero-1-0" {
		t.Fatalf("post-migration set: %+v %v", c, err)
	}
}

func TestHarnessSessionIDRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "hero.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	id, err := s.CreateCycle(Cycle{
		Number: 1, Title: "C3", Status: CycleStatusActive,
		StartedAt: nowRFC3339(), ConfigSnapshotJSON: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateStages([]Stage{
		{CycleID: id, Name: "research", Status: StageWaiting, MaxIterations: 50, SortOrder: 0},
	}); err != nil {
		t.Fatal(err)
	}

	st, err := s.GetStage(id, "research")
	if err != nil {
		t.Fatal(err)
	}
	if st.HarnessSessionID != "" {
		t.Fatalf("default harness_session_id = %q", st.HarnessSessionID)
	}

	if err := s.SetStageHarnessSessionID(id, "research", "sess-xyz"); err != nil {
		t.Fatal(err)
	}
	st, err = s.GetStage(id, "research")
	if err != nil || st.HarnessSessionID != "sess-xyz" {
		t.Fatalf("set session = %+v %v", st, err)
	}

	if err := s.ClearStageHarnessSessionID(id, "research"); err != nil {
		t.Fatal(err)
	}
	st, err = s.GetStage(id, "research")
	if err != nil || st.HarnessSessionID != "" {
		t.Fatalf("cleared session = %+v %v", st, err)
	}
}

func TestMigrateV2ToV3AddsHarnessSessionID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hero.db")

	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
)`); err != nil {
		t.Fatal(err)
	}
	s := &Store{db: db, log: slog.Default()}
	if err := s.applyMigration(1); err != nil {
		t.Fatal(err)
	}
	if err := s.applyMigration(2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO cycles(number, title, objective, status, config_snapshot_json, openspec_change)
VALUES(1, 'C2', 'obj', ?, '{}', '')`, CycleStatusActive); err != nil {
		t.Fatal(err)
	}
	var cycleID int64
	if err := s.db.QueryRow(`SELECT id FROM cycles LIMIT 1`).Scan(&cycleID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO stages(cycle_id, name, status, sort_order) VALUES(?, 'research', ?, 0)`,
		cycleID, StageWaiting); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("Open after v2: %v", err)
	}
	defer s2.Close()
	v, err := s2.SchemaVersion()
	if err != nil || v != currentSchemaVersion {
		t.Fatalf("schema version = %d %v, want %d", v, err, currentSchemaVersion)
	}
	st, err := s2.GetStage(cycleID, "research")
	if err != nil || st.HarnessSessionID != "" {
		t.Fatalf("migrated stage = %+v %v", st, err)
	}
	if err := s2.SetStageHarnessSessionID(cycleID, "research", "after-migrate"); err != nil {
		t.Fatal(err)
	}
}
