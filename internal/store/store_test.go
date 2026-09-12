package store

import (
	"database/sql"
	"errors"
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

func TestGetCurrentCyclePrefersActiveAndRetainsCompleted(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "hero.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	completedID, err := s.CreateCycle(Cycle{
		Number: 2, Title: "Completed", Status: CycleStatusCompleted,
		CompletedAt: nowRFC3339(), ConfigSnapshotJSON: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	activeID, err := s.CreateCycle(Cycle{
		Number: 1, Title: "Active", Status: CycleStatusActive,
		StartedAt: nowRFC3339(), ConfigSnapshotJSON: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	current, err := s.GetCurrentCycle()
	if err != nil || current.ID != activeID {
		t.Fatalf("current with active cycle = %+v, err=%v", current, err)
	}
	if _, err := s.GetActiveCycle(); err != nil {
		t.Fatalf("GetActiveCycle with active cycle: %v", err)
	}

	if err := s.UpdateCycleStatus(activeID, CycleStatusArchived, ""); err != nil {
		t.Fatal(err)
	}
	current, err = s.GetCurrentCycle()
	if err != nil || current.ID != completedID || current.Status != CycleStatusCompleted {
		t.Fatalf("current after active archive = %+v, err=%v", current, err)
	}
	if _, err := s.GetActiveCycle(); !errors.Is(err, ErrNoActiveCycle) {
		t.Fatalf("GetActiveCycle after active archive: %v", err)
	}

	if err := s.UpdateCycleStatus(completedID, CycleStatusArchived, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetCurrentCycle(); !errors.Is(err, ErrNoActiveCycle) {
		t.Fatalf("GetCurrentCycle after all archives: %v", err)
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

func TestOrchestrationSessionRoundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "hero.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	id, err := s.CreateCycle(Cycle{
		Number: 1, Title: "C10", Status: CycleStatusActive,
		StartedAt: nowRFC3339(), ConfigSnapshotJSON: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.GetCycle(id)
	if err != nil {
		t.Fatal(err)
	}
	if c.OrchestrationSessionID != "" || c.OrchestrationHarnessID != "" {
		t.Fatalf("default orch session = %+v", c)
	}
	if err := s.SetOrchestrationSession(id, "uuid-orch", ""); err == nil {
		t.Fatal("expected harness id required")
	}
	if err := s.SetOrchestrationSession(id, "uuid-orch", "cursor"); err != nil {
		t.Fatal(err)
	}
	sid, hid, err := s.OrchestrationSession(id)
	if err != nil || sid != "uuid-orch" || hid != "cursor" {
		t.Fatalf("orch session = (%q, %q) %v", sid, hid, err)
	}
	if err := s.SetOrchestrationSession(id, "", "cursor"); err != nil {
		t.Fatal(err)
	}
	sid, hid, err = s.OrchestrationSession(id)
	if err != nil || sid != "" || hid != "" {
		t.Fatalf("cleared orch session = (%q, %q) %v", sid, hid, err)
	}
}

func TestSetStageSessionBindingRequiresHarness(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "hero.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id, err := s.CreateCycle(Cycle{
		Number: 1, Title: "C10b", Status: CycleStatusActive,
		StartedAt: nowRFC3339(), ConfigSnapshotJSON: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateStages([]Stage{
		{CycleID: id, Name: "qa", Status: StageWaiting, MaxIterations: 2, SortOrder: 0},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStageSessionBinding(id, "qa", "", "ses_opencode"); err == nil {
		t.Fatal("expected harness id required")
	}
	if err := s.SetStageSessionBinding(id, "qa", "opencode", "ses_opencode"); err != nil {
		t.Fatal(err)
	}
	hid, sid, err := s.StageSessionBinding(id, "qa")
	if err != nil || hid != "opencode" || sid != "ses_opencode" {
		t.Fatalf("binding = (%q, %q) %v", hid, sid, err)
	}
}

func TestMigrateV9ToV10AddsOrchestrationSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hero.db")
	s, err := openCapped(path, 9)
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.CreateCycle(Cycle{
		Number: 1, Title: "pre-v10", Status: CycleStatusActive,
		StartedAt: nowRFC3339(), ConfigSnapshotJSON: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("Open after v9: %v", err)
	}
	defer s2.Close()
	v, err := s2.SchemaVersion()
	if err != nil || v != currentSchemaVersion {
		t.Fatalf("schema version = %d %v, want %d", v, err, currentSchemaVersion)
	}
	c, err := s2.GetCycle(id)
	if err != nil {
		t.Fatal(err)
	}
	if c.OrchestrationSessionID != "" || c.OrchestrationHarnessID != "" {
		t.Fatalf("migrated orch session = %+v", c)
	}
	if err := s2.SetOrchestrationSession(id, "after-v10", "cursor"); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateV10ToV11PreservesOperationalRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hero.db")
	ts := "2026-09-11T12:00:00Z"

	s, err := openCapped(path, 10)
	if err != nil {
		t.Fatal(err)
	}

	cycleID, err := s.CreateCycle(Cycle{
		Number:             7,
		Title:              "pre-v11",
		Status:             CycleStatusActive,
		StartedAt:          ts,
		ConfigSnapshotJSON: `{"k":"v"}`,
		OpenspecChange:     "loopback-findings-handoff",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetOrchestrationSession(cycleID, "orch-pre", "cursor"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateStages([]Stage{
		{CycleID: cycleID, Name: "qa", Status: StageWaiting, MaxIterations: 2, SortOrder: 0},
	}); err != nil {
		t.Fatal(err)
	}
	eventID, err := s.AppendEvent(Event{
		CycleID: cycleID, TS: ts, Type: "stage_started", PayloadJSON: `{"stage":"qa"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertMetric(Metric{
		CycleID: cycleID, StageName: "qa", Agent: "qa_agent",
		InputTokens: 10, OutputTokens: 5, CostUSD: 0.01, DurationMS: 100,
	}); err != nil {
		t.Fatal(err)
	}
	convID, err := s.AddConversation(ConversationEntry{
		CycleID: cycleID, TS: ts, Role: "user", Kind: "message", Body: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	artID, err := s.AddArtifact(Artifact{
		CycleID: cycleID, Path: "openspec/x.md", Kind: "doc", Label: "spec", CreatedAt: ts,
	})
	if err != nil {
		t.Fatal(err)
	}
	serveID, err := s.InsertServeRegistry(ServeRegistryEntry{
		Harness: "opencode", PID: 4242, Port: 4096,
		URL: "http://127.0.0.1:4096", ProjectPath: dir, CreatedAt: ts,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertModelList("cursor", []string{"gpt-4"}, ts); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertCapabilities(CapabilityCacheRow{
		Harness: "cursor", Model: "gpt-4", PropertiesJSON: `{"fs":{}}`, RetrievedAt: ts,
	}); err != nil {
		t.Fatal(err)
	}
	gen, err := s.BeginRefresh("cursor")
	if err != nil || gen != 1 {
		t.Fatalf("BeginRefresh: gen=%d err=%v", gen, err)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("Open after v10: %v", err)
	}
	defer s2.Close()

	v, err := s2.SchemaVersion()
	if err != nil || v != currentSchemaVersion {
		t.Fatalf("schema version = %d %v, want %d", v, err, currentSchemaVersion)
	}

	c, err := s2.GetCycle(cycleID)
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != "pre-v11" || c.OpenspecChange != "loopback-findings-handoff" ||
		c.OrchestrationSessionID != "orch-pre" || c.OrchestrationHarnessID != "cursor" {
		t.Fatalf("cycle mutated: %+v", c)
	}
	var disposition, dispositionJSON string
	if err := s2.db.QueryRow(
		`SELECT completion_disposition, completion_disposition_json FROM cycles WHERE id = ?`, cycleID,
	).Scan(&disposition, &dispositionJSON); err != nil {
		t.Fatal(err)
	}
	if disposition != "" || dispositionJSON != "" {
		t.Fatalf("default disposition = (%q, %q)", disposition, dispositionJSON)
	}

	st, err := s2.GetStage(cycleID, "qa")
	if err != nil || st.Name != "qa" {
		t.Fatalf("stage: %+v %v", st, err)
	}
	events, err := s2.ListEvents(cycleID, "", 0)
	if err != nil || len(events) != 1 || events[0].ID != eventID {
		t.Fatalf("events: %+v err=%v want id %d", events, err, eventID)
	}
	metrics, err := s2.ListMetrics(cycleID)
	if err != nil || len(metrics) != 1 || metrics[0].InputTokens != 10 {
		t.Fatalf("metrics: %+v %v", metrics, err)
	}
	convs, err := s2.ListConversation(cycleID)
	if err != nil || len(convs) != 1 || convs[0].ID != convID {
		t.Fatalf("conversation: %+v %v", convs, err)
	}
	arts, err := s2.ListArtifacts(cycleID)
	if err != nil || len(arts) != 1 || arts[0].ID != artID {
		t.Fatalf("artifacts: %+v %v", arts, err)
	}
	entries, err := s2.ListServeRegistry()
	if err != nil || len(entries) != 1 || entries[0].ID != serveID {
		t.Fatalf("serve registry: %+v %v", entries, err)
	}
	models, refreshedAt, err := s2.ModelList("cursor")
	if err != nil || len(models) != 1 || refreshedAt != ts {
		t.Fatalf("model list: %v %q %v", models, refreshedAt, err)
	}
	caps, err := s2.ListCapabilities("cursor")
	if err != nil || len(caps) != 1 {
		t.Fatalf("capabilities: %+v %v", caps, err)
	}
	rgen, pending, err := s2.RefreshState("cursor")
	if err != nil || rgen != 1 || !pending {
		t.Fatalf("refresh state: gen=%d pending=%v err=%v", rgen, pending, err)
	}

	for _, table := range []string{
		"findings", "finding_occurrences", "todos", "todo_adoptions", "todo_projection_ops",
	} {
		var n int
		if err := s2.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != 0 {
			t.Fatalf("%s count = %d, want 0", table, n)
		}
	}
}
