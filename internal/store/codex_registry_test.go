package store

import (
	"path/filepath"
	"testing"
)

// codex-app-server-registry / sqlite-operational-store: the v6→v7 migration adds
// Codex registry support without dropping OpenCode serve rows (ADR-044; design D13).
func TestSchemaV7MigrationFromV6_PreservesOpenCodeRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DBFileName)

	v6, err := openCapped(path, 6)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v6.db.Exec(`INSERT INTO harness_serve_registry(harness, pid, port, url, project_path, created_at)
VALUES('opencode', 42, 4096, 'http://127.0.0.1:4096', ?, '2026-08-15T00:00:00Z')`, dir); err != nil {
		t.Fatal(err)
	}
	if err := v6.Close(); err != nil {
		t.Fatal(err)
	}

	st := newStoreAt(t, dir)
	v, err := st.SchemaVersion()
	if err != nil || v != currentSchemaVersion {
		t.Fatalf("version=%d err=%v want %d", v, err, currentSchemaVersion)
	}
	entries, err := st.ListServeRegistry()
	if err != nil || len(entries) != 1 {
		t.Fatalf("OpenCode serve row must survive migration: %v %+v", err, entries)
	}
	if entries[0].Harness != "opencode" || entries[0].URL != "http://127.0.0.1:4096" {
		t.Fatalf("opencode row altered: %+v", entries[0])
	}
	// Reap lookup index must exist after the bump.
	var idxName string
	err = st.db.QueryRow(`SELECT name FROM sqlite_master
WHERE type = 'index' AND name = 'idx_harness_serve_registry_harness_project'`).Scan(&idxName)
	if err != nil || idxName == "" {
		t.Fatalf("reap index missing after v7 migration: %v", err)
	}
}

// codex-app-server-registry: codex rows persist pid + project identity with no
// HTTP URL (port=0 / url='') and can be listed/scoped/removed (ADR-044).
func TestCodexServeRegistryRoundTrip(t *testing.T) {
	st := newStoreAt(t, t.TempDir())
	projectDir := "/proj/a"

	id, err := st.InsertCodexServeRegistry(projectDir, 4242)
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("expected a row id")
	}

	all, err := st.ListServeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Harness != HarnessCodex || all[0].PID != 4242 {
		t.Fatalf("registry rows = %+v", all)
	}
	entry := all[0]
	if entry.ProjectPath != projectDir {
		t.Fatalf("project_path=%q", entry.ProjectPath)
	}
	if entry.Port != 0 || entry.URL != "" {
		t.Fatalf("codex rows must not carry a fabricated serve URL: port=%d url=%q", entry.Port, entry.URL)
	}

	scoped, err := st.ListCodexServeRegistry(projectDir)
	if err != nil || len(scoped) != 1 || scoped[0].PID != 4242 {
		t.Fatalf("ListCodexServeRegistry(%q) = %+v %v", projectDir, scoped, err)
	}
	other, err := st.ListCodexServeRegistry("/proj/other")
	if err != nil || len(other) != 0 {
		t.Fatalf("codex rows must be project-scoped: %+v %v", other, err)
	}

	if err := st.DeleteServeRegistry(id); err != nil {
		t.Fatal(err)
	}
	left, err := st.ListCodexServeRegistry(projectDir)
	if err != nil || len(left) != 0 {
		t.Fatalf("row must be removable: %+v %v", left, err)
	}
}

// sqlite-operational-store: codex rows and opencode rows coexist in the same table.
func TestCodexAndOpenCodeRegistryRowsCoexist(t *testing.T) {
	st := newStoreAt(t, t.TempDir())
	projectDir := "/proj/b"
	if _, err := st.InsertCodexServeRegistry(projectDir, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertServeRegistry(ServeRegistryEntry{
		Harness: "opencode", PID: 2, Port: 4096, URL: "http://127.0.0.1:4096", ProjectPath: projectDir, CreatedAt: "2026-08-15T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	codex, err := st.ListCodexServeRegistry(projectDir)
	if err != nil || len(codex) != 1 || codex[0].Harness != HarnessCodex {
		t.Fatalf("codex rows = %+v %v", codex, err)
	}
	all, err := st.ListServeRegistry()
	if err != nil || len(all) != 2 {
		t.Fatalf("total rows = %+v %v", all, err)
	}
}

// sqlite-operational-store: stage/session binding records harness id so resume
// logic never crosses harness boundaries (PRD-C06-001 §4.3; ADR-044).
func TestSessionResumeAllowed_CodexThreadNeverResumesAsCursorOrOpenCode(t *testing.T) {
	st := newStoreAt(t, t.TempDir())
	cycleID, err := st.CreateCycle(Cycle{
		Number: 1, Title: "c6", Status: CycleStatusActive,
		StartedAt: nowRFC3339(), ConfigSnapshotJSON: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateStages([]Stage{
		{CycleID: cycleID, Name: "implementation", Status: StageRunning, MaxIterations: 4, SortOrder: 0},
	}); err != nil {
		t.Fatal(err)
	}

	// A Codex thread id is bound to harness codex.
	if err := st.SetStageHarnessID(cycleID, "implementation", "codex"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetStageHarnessSessionID(cycleID, "implementation", "thread-123"); err != nil {
		t.Fatal(err)
	}

	bound, session, err := st.StageSessionBinding(cycleID, "implementation")
	if err != nil {
		t.Fatal(err)
	}
	if bound != "codex" || session != "thread-123" {
		t.Fatalf("binding = (%q, %q)", bound, session)
	}

	// Resume as codex is allowed; resume as cursor/opencode is rejected.
	if ok, err := st.SessionResumeAllowed(cycleID, "implementation", "codex"); err != nil || !ok {
		t.Fatalf("codex resume should be allowed: ok=%v err=%v", ok, err)
	}
	for _, foreign := range []string{"cursor", "opencode"} {
		if ok, err := st.SessionResumeAllowed(cycleID, "implementation", foreign); err != nil || ok {
			t.Fatalf("codex thread must not resume as %s: ok=%v err=%v", foreign, ok, err)
		}
	}

	// Conversely a Cursor session must not resume as codex.
	if err := st.SetStageHarnessID(cycleID, "implementation", "cursor"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetStageHarnessSessionID(cycleID, "implementation", "sess-cursor"); err != nil {
		t.Fatal(err)
	}
	if ok, err := st.SessionResumeAllowed(cycleID, "implementation", "codex"); err != nil || ok {
		t.Fatalf("cursor session must not resume as codex: ok=%v err=%v", ok, err)
	}
	if ok, err := st.SessionResumeAllowed(cycleID, "implementation", "cursor"); err != nil || !ok {
		t.Fatalf("cursor session should resume as cursor: ok=%v err=%v", ok, err)
	}
}

// sqlite-operational-store: an unbound stage (no session yet) always allows resume.
func TestSessionResumeAllowed_UnboundStageAllowsResume(t *testing.T) {
	st := newStoreAt(t, t.TempDir())
	cycleID, err := st.CreateCycle(Cycle{
		Number: 2, Title: "c6b", Status: CycleStatusActive,
		StartedAt: nowRFC3339(), ConfigSnapshotJSON: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateStages([]Stage{
		{CycleID: cycleID, Name: "qa", Status: StageWaiting, MaxIterations: 2, SortOrder: 0},
	}); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"codex", "cursor", "opencode"} {
		if ok, err := st.SessionResumeAllowed(cycleID, "qa", h); err != nil || !ok {
			t.Fatalf("unbound stage must allow %s resume: ok=%v err=%v", h, ok, err)
		}
	}
}
