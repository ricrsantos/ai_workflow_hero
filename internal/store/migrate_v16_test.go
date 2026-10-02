package store

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
)

const v15FixtureTimestamp = "2026-09-30T12:00:00Z"

var v16Tables = []string{
	"stage_budgets",
	"stage_blockers",
	"stage_coverage",
	"screenshot_manifests",
}

var v15FixtureTables = []string{
	"cycles",
	"stages",
	"events",
	"metrics",
	"artifacts",
	"conversation",
	"harness_serve_registry",
	"model_list_cache",
	"model_capability_cache",
	"model_refresh_state",
	"findings",
	"finding_occurrences",
	"todos",
	"todo_adoptions",
	"todo_projection_ops",
	"sessions",
	"session_events",
	"session_assets",
	"session_leases",
	"session_delete_ops",
	"session_delete_op_managed_paths",
}

func TestMigrateV15ToV16(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hero.db")
	st, err := openCapped(path, 15)
	if err != nil {
		t.Fatalf("open v15 fixture: %v", err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Error(err)
		}
	}()

	// Fail after the earlier CREATE statements have run inside the migration
	// transaction. A retry must see none of those partial tables.
	if _, err := st.db.Exec(`CREATE TABLE screenshot_manifests (conflicting_column TEXT)`); err != nil {
		t.Fatalf("create migration conflict: %v", err)
	}
	if err := st.migrateTo(16); err == nil {
		t.Fatal("migration succeeded despite an existing conflicting table")
	}
	if got := schemaVersionForTest(t, st); got != 15 {
		t.Fatalf("schema version after rollback = %d, want 15", got)
	}
	for _, table := range v16Tables[:len(v16Tables)-1] {
		if tableExistsForTest(t, st.db, table) {
			t.Fatalf("partial migration left table %q behind", table)
		}
	}
	if _, err := st.db.Exec(`DROP TABLE screenshot_manifests`); err != nil {
		t.Fatalf("remove migration conflict: %v", err)
	}

	if err := st.migrateTo(16); err != nil {
		t.Fatalf("migrate v15 to v16: %v", err)
	}
	if got := schemaVersionForTest(t, st); got != 16 {
		t.Fatalf("schema version = %d, want 16", got)
	}
	for _, table := range v16Tables {
		if !tableExistsForTest(t, st.db, table) {
			t.Fatalf("migration did not create %q", table)
		}
		if got := rowCountForTest(t, st.db, table); got != 0 {
			t.Fatalf("new table %q has %d rows, want 0", table, got)
		}
	}

	// Reapplying the capped migration is a no-op and does not duplicate schema
	// or insert rows into the additive tables.
	if err := st.migrateTo(16); err != nil {
		t.Fatalf("repeat v16 migration: %v", err)
	}
	if got := schemaVersionForTest(t, st); got != 16 {
		t.Fatalf("schema version after repeat = %d, want 16", got)
	}
	for _, table := range v16Tables {
		if got := rowCountForTest(t, st.db, table); got != 0 {
			t.Fatalf("new table %q has %d rows after repeat, want 0", table, got)
		}
	}

	cycleID, err := insertStageForV16Test(t, st.db, "qa")
	if err != nil {
		t.Fatalf("insert stage fixture: %v", err)
	}
	assertConstraintFailure(t, st.db, `INSERT INTO stage_budgets
  (cycle_id, stage_name, limit_ms, consumed_ms, state, generation, started_at, checkpoint_at, updated_at)
VALUES (?, 'qa', 0, 0, 'active', 0, ?, ?, ?)`, cycleID, v15FixtureTimestamp, v15FixtureTimestamp, v15FixtureTimestamp)
	assertConstraintFailure(t, st.db, `INSERT INTO stage_budgets
  (cycle_id, stage_name, limit_ms, consumed_ms, state, generation, started_at, checkpoint_at, updated_at)
VALUES (?, 'qa', 60000, -1, 'active', 0, ?, ?, ?)`, cycleID, v15FixtureTimestamp, v15FixtureTimestamp, v15FixtureTimestamp)
	assertConstraintFailure(t, st.db, `INSERT INTO stage_budgets
  (cycle_id, stage_name, limit_ms, consumed_ms, state, generation, started_at, checkpoint_at, updated_at)
VALUES (?, 'qa', 60000, 0, 'running', 0, ?, ?, ?)`, cycleID, v15FixtureTimestamp, v15FixtureTimestamp, v15FixtureTimestamp)
	assertConstraintFailure(t, st.db, `INSERT INTO stage_budgets
  (cycle_id, stage_name, limit_ms, consumed_ms, state, generation, started_at, checkpoint_at, updated_at)
VALUES (?, 'qa', 60000, 0, 'active', -1, ?, ?, ?)`, cycleID, v15FixtureTimestamp, v15FixtureTimestamp, v15FixtureTimestamp)
	if _, err := st.db.Exec(`INSERT INTO stage_budgets
  (cycle_id, stage_name, limit_ms, consumed_ms, state, generation, started_at, active_since, checkpoint_at, updated_at)
VALUES (?, 'qa', 60000, 1000, 'active', 1, ?, ?, ?, ?)`,
		cycleID, v15FixtureTimestamp, v15FixtureTimestamp, v15FixtureTimestamp, v15FixtureTimestamp); err != nil {
		t.Fatalf("insert valid stage budget: %v", err)
	}

	if _, err := st.db.Exec(`INSERT INTO stage_blockers
  (cycle_id, stage_name, blocker_id, reason, affected_coverage_ids_json, affected_profile_ids_json,
   diagnostic_uncertainty, next_action, created_at)
VALUES (?, 'qa', 'block-1', 'missing_account', '["screen-1"]', '["admin"]', 'unknown', 'configure account', ?)`,
		cycleID, v15FixtureTimestamp); err != nil {
		t.Fatalf("insert stage blocker: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO stage_coverage
  (cycle_id, stage_name, coverage_id, user_id, profile_id, mandatory, expected_result, result, updated_at)
VALUES (?, 'qa', 'screen-1', 'user-admin', 'admin', 1, 'protected page renders', 'blocked', ?)`,
		cycleID, v15FixtureTimestamp); err != nil {
		t.Fatalf("insert coverage record: %v", err)
	}
	var userID, profileID string
	if err := st.db.QueryRow(`SELECT user_id, profile_id FROM stage_coverage WHERE cycle_id = ? AND coverage_id = 'screen-1'`, cycleID).
		Scan(&userID, &profileID); err != nil {
		t.Fatalf("read coverage identities: %v", err)
	}
	if userID != "user-admin" || profileID != "admin" {
		t.Fatalf("coverage identities = user %q/profile %q, want distinct user/profile IDs", userID, profileID)
	}
	if _, err := st.db.Exec(`INSERT INTO screenshot_manifests
  (screenshot_id, cycle_id, stage_name, attempt, coverage_id, user_id, profile_id, captured_at,
   path, result, capture_status)
VALUES ('shot-1', ?, 'qa', 1, 'screen-1', 'user-admin', 'admin', ?,
   '.workflow-hero/cycles/current/screenshots/shot-1.png', 'passed', 'ready')`,
		cycleID, v15FixtureTimestamp); err != nil {
		t.Fatalf("insert screenshot manifest: %v", err)
	}
	assertConstraintFailure(t, st.db, `INSERT INTO screenshot_manifests
  (screenshot_id, cycle_id, stage_name, attempt, captured_at, path, result, capture_status)
VALUES ('shot-unsafe', ?, 'qa', 1, ?, '../../secret.png', 'failed', 'ready')`,
		cycleID, v15FixtureTimestamp)
	assertConstraintFailure(t, st.db, `INSERT INTO screenshot_manifests
  (screenshot_id, cycle_id, stage_name, attempt, captured_at, path, result, capture_status)
VALUES (NULL, ?, 'qa', 1, ?, 'screenshots/shot-null.png', 'passed', 'ready')`,
		cycleID, v15FixtureTimestamp)
	if _, err := st.db.Exec(`INSERT INTO screenshot_manifests
  (screenshot_id, cycle_id, stage_name, attempt, coverage_id, user_id, profile_id, captured_at,
   path, result, capture_status, omission_reason)
VALUES ('shot-omitted', ?, 'qa', 2, 'screen-1', 'user-admin', 'admin', ?,
   '', 'blocked', 'omitted', 'capture suspended during login')`,
		cycleID, v15FixtureTimestamp); err != nil {
		t.Fatalf("insert omission manifest: %v", err)
	}
}

func TestV15FixtureIntact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hero.db")
	st, err := openCapped(path, 15)
	if err != nil {
		t.Fatalf("open v15 fixture: %v", err)
	}
	seedV15Fixture(t, st.db)
	before := snapshotV15Fixture(t, st.db)
	if err := st.Close(); err != nil {
		t.Fatalf("close v15 fixture: %v", err)
	}

	migrated, err := openCapped(path, 16)
	if err != nil {
		t.Fatalf("migrate v15 fixture: %v", err)
	}
	defer func() {
		if err := migrated.Close(); err != nil {
			t.Error(err)
		}
	}()
	after := snapshotV15Fixture(t, migrated.db)
	for table, want := range before {
		if got := after[table]; !reflect.DeepEqual(got, want) {
			t.Errorf("v15 rows changed in %s", table)
		}
	}
	for _, table := range v16Tables {
		if got := rowCountForTest(t, migrated.db, table); got != 0 {
			t.Errorf("new table %q has %d rows, want 0", table, got)
		}
	}
}

func seedV15Fixture(t *testing.T, db *sql.DB) {
	t.Helper()
	fixtureExec(t, db, `INSERT INTO cycles
  (id, number, title, objective, status, started_at, config_snapshot_json, openspec_change,
   session_duration_seconds, orchestration_session_id, orchestration_harness_id)
VALUES (7, 17, 'v15 fixture', 'migration preservation', 'active', ?, '{"v":15}', 'browser-validation',
   123, 'orch-v15', 'cursor')`, v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO stages
  (id, cycle_id, name, status, iteration, max_iterations, extra_iterations, require_human_approval,
   timeout_minutes, started_at, completed_at, summary, sort_order, harness_session_id, harness_id,
   harness_permission_paused)
VALUES (3, 7, 'qa', 'running', 2, 4, 1, 0, 15, ?, NULL, 'existing stage', 0,
   'stage-native-v15', 'cursor', 0)`, v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO events (id, cycle_id, ts, type, payload_json)
VALUES (11, 7, ?, 'stage_started', '{"stage":"qa"}')`, v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO metrics
  (id, cycle_id, stage_name, model, agent, input_tokens, output_tokens, cost_usd, duration_ms)
VALUES (12, 7, 'qa', 'model-v15', 'qa_agent', 101, 202, 0.25, 303) `)
	fixtureExec(t, db, `INSERT INTO artifacts (id, cycle_id, path, kind, label, created_at)
VALUES (13, 7, 'openspec/changes/x/tasks.md', 'document', 'tasks', ?)`, v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO conversation (id, cycle_id, ts, role, kind, body)
VALUES (14, 7, ?, 'assistant', 'stage_agent_assignment', '{"agent":"qa_agent"}')`, v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO harness_serve_registry (id, harness, pid, port, url, created_at, project_path)
VALUES (15, 'opencode', 1500, 4096, 'http://127.0.0.1:4096', ?, '/tmp/v15-fixture')`, v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO model_list_cache (harness, models_json, refreshed_at)
VALUES ('cursor', '["model-v15"]', ?)`, v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO model_capability_cache (harness, model, properties_json, retrieved_at)
VALUES ('cursor', 'model-v15', '{"reasoning":{}}', ?)`, v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO model_refresh_state (harness, generation, pending, updated_at)
VALUES ('cursor', 2, 1, ?)`, v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO findings
  (id, cycle_id, source_stage, owner, status, fingerprint, file, requirement, issue,
   acceptance_criteria, evidence_json, round, todo_id, created_at, updated_at,
   repro_package, repro_test, repro_mode)
VALUES ('find-v15', 7, 'qa', 'generic_agent', 'open', 'fingerprint-v15', 'internal/example.go',
   'B00', 'existing finding', 'row must remain unchanged', '[]', 1, 'todo-v15', ?, ?,
   'internal/example', 'TestExample', 'go_test')`, v15FixtureTimestamp, v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO finding_occurrences
  (cycle_id, finding_id, sequence, kind, source_stage, round, issue, acceptance_criteria,
   evidence_json, created_at, repro_package, repro_test, repro_source, repro_mode)
VALUES (7, 'find-v15', 1, 'created', 'qa', 1, 'existing finding',
   'row must remain unchanged', '[]', ?, 'internal/example', 'TestExample', 'go', 'go_test')`,
		v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO todos
  (id, origin_type, summary, acceptance_criteria, status, created_at, updated_at)
VALUES ('todo-v15', 'legacy', 'existing todo', 'row must remain unchanged', 'pending', ?, ?)`,
		v15FixtureTimestamp, v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO sessions
  (id, kind, title, lifecycle, harness_id, native_session_id, model, model_properties_json,
   cycle_id, stage_name, agent_name, transcript_state, last_origin, created_at, last_activity_at)
VALUES ('session-v15', 'stage_agent', 'Existing session', 'active', 'cursor', 'native-v15',
   'model-v15', '{}', 7, 'qa', 'qa_agent', 'available', 'local', ?, ?)`,
		v15FixtureTimestamp, v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO session_events
  (session_id, seq, event_type, origin, origin_address, payload_json, provider_event_id, created_at)
VALUES ('session-v15', 1, 'message', 'local', '', '{"body":"history row"}', '', ?)`, v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO session_assets
  (session_id, asset_id, ownership, path, mime, original_name, card_meta_json)
VALUES ('session-v15', 'asset-v15', 'managed_copy', '/tmp/asset-v15.png', 'image/png', 'screen.png', '{}')`)
	fixtureExec(t, db, `INSERT INTO session_leases
  (session_id, owner_id, acquired_at, heartbeat_at, expires_at)
VALUES ('session-v15', 'tui-v15', ?, ?, ?)`,
		v15FixtureTimestamp, v15FixtureTimestamp, v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO session_delete_ops
  (id, session_id, native_session_id, harness_id, status, remote_warning, created_at, updated_at)
VALUES (16, 'session-v15', 'native-v15', 'cursor', 'completed', '', ?, ?)`,
		v15FixtureTimestamp, v15FixtureTimestamp)
	fixtureExec(t, db, `INSERT INTO session_delete_op_managed_paths (delete_op_id, path)
VALUES (16, '/tmp/asset-v15.png')`)
}

func fixtureExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("seed v15 fixture: %v", err)
	}
}

func snapshotV15Fixture(t *testing.T, db *sql.DB) map[string][][]any {
	t.Helper()
	snapshot := make(map[string][][]any, len(v15FixtureTables)+1)
	for _, table := range v15FixtureTables {
		snapshot[table] = queryRowsForTest(t, db, `SELECT * FROM `+table+` ORDER BY rowid`)
	}
	snapshot["schema_migrations"] = queryRowsForTest(t, db,
		`SELECT version, applied_at FROM schema_migrations WHERE version <= 15 ORDER BY version`)
	return snapshot
}

func queryRowsForTest(t *testing.T, db *sql.DB, query string, args ...any) [][]any {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("query fixture rows: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Error(err)
		}
	}()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatalf("read fixture columns: %v", err)
	}
	result := make([][]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			t.Fatalf("scan fixture row: %v", err)
		}
		for i, value := range values {
			if data, ok := value.([]byte); ok {
				values[i] = string(data)
			}
		}
		result = append(result, values)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate fixture rows: %v", err)
	}
	return result
}

func schemaVersionForTest(t *testing.T, st *Store) int {
	t.Helper()
	version, err := st.SchemaVersion()
	if err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	return version
}

func tableExistsForTest(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
		t.Fatalf("check table %q: %v", table, err)
	}
	return count == 1
}

func rowCountForTest(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
		t.Fatalf("count table %q: %v", table, err)
	}
	return count
}

func assertConstraintFailure(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err == nil {
		t.Fatal("insert with invalid schema values succeeded")
	}
}

func insertStageForV16Test(t *testing.T, db *sql.DB, stageName string) (int64, error) {
	t.Helper()
	result, err := db.Exec(`INSERT INTO cycles (number, status) VALUES (1, 'active')`)
	if err != nil {
		return 0, err
	}
	cycleID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := db.Exec(`INSERT INTO stages (cycle_id, name, status) VALUES (?, ?, 'waiting')`, cycleID, stageName); err != nil {
		return 0, err
	}
	return cycleID, nil
}
