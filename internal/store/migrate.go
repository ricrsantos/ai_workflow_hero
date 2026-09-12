package store

import (
	"fmt"
	"log/slog"
)

// currentSchemaVersion is the latest migration version applied by Open.
const currentSchemaVersion = 11

func (s *Store) migrate() error {
	return s.migrateTo(currentSchemaVersion)
}

// migrateTo applies migrations up to maxVersion (test hook for v4→v5 coverage).
func (s *Store) migrateTo(maxVersion int) error {
	if maxVersion > currentSchemaVersion {
		maxVersion = currentSchemaVersion
	}
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var version int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	for v := version + 1; v <= maxVersion; v++ {
		s.log.Info("applying schema migration", "version", v)
		if err := s.applyMigration(v); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applyMigration(version int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", version, err)
	}
	defer func() { _ = tx.Rollback() }()

	switch version {
	case 1:
		stmts := []string{
			`CREATE TABLE cycles (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  number INTEGER NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  objective TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  started_at TEXT,
  completed_at TEXT,
  config_snapshot_json TEXT NOT NULL DEFAULT '{}',
  lock_holder TEXT,
  lock_at TEXT
)`,
			`CREATE TABLE stages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cycle_id INTEGER NOT NULL REFERENCES cycles(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  iteration INTEGER NOT NULL DEFAULT 0,
  max_iterations INTEGER NOT NULL DEFAULT 1,
  extra_iterations INTEGER NOT NULL DEFAULT 0,
  require_human_approval INTEGER NOT NULL DEFAULT 0,
  timeout_minutes INTEGER NOT NULL DEFAULT 0,
  started_at TEXT,
  completed_at TEXT,
  summary TEXT,
  sort_order INTEGER NOT NULL DEFAULT 0,
  UNIQUE(cycle_id, name)
)`,
			`CREATE TABLE events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cycle_id INTEGER NOT NULL REFERENCES cycles(id) ON DELETE CASCADE,
  ts TEXT NOT NULL,
  type TEXT NOT NULL,
  payload_json TEXT NOT NULL DEFAULT '{}'
)`,
			`CREATE INDEX events_cycle_ts ON events(cycle_id, ts)`,
			`CREATE TABLE metrics (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cycle_id INTEGER NOT NULL REFERENCES cycles(id) ON DELETE CASCADE,
  stage_name TEXT NOT NULL,
  model TEXT NOT NULL DEFAULT '',
  agent TEXT NOT NULL DEFAULT '',
  input_tokens INTEGER NOT NULL DEFAULT 0,
  output_tokens INTEGER NOT NULL DEFAULT 0,
  cost_usd REAL NOT NULL DEFAULT 0,
  duration_ms INTEGER NOT NULL DEFAULT 0,
  UNIQUE(cycle_id, stage_name, agent)
)`,
			`CREATE TABLE artifacts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cycle_id INTEGER NOT NULL REFERENCES cycles(id) ON DELETE CASCADE,
  path TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT '',
  label TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
)`,
			`CREATE TABLE conversation (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cycle_id INTEGER NOT NULL REFERENCES cycles(id) ON DELETE CASCADE,
  ts TEXT NOT NULL,
  role TEXT NOT NULL,
  kind TEXT NOT NULL,
  body TEXT NOT NULL
)`,
		}
		for _, stmt := range stmts {
			if _, err := tx.Exec(stmt); err != nil {
				return fmt.Errorf("migration %d: %w", version, err)
			}
		}
	case 2:
		// ADR-023 / C2: persist OpenSpec change name on the cycle.
		if _, err := tx.Exec(`ALTER TABLE cycles ADD COLUMN openspec_change TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
	case 3:
		// ADR-C03 / D6: Cursor CLI --resume continuity within an etapa.
		if _, err := tx.Exec(`ALTER TABLE stages ADD COLUMN harness_session_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
	case 4:
		// ADR-C04 / D13: OpenCode serve registry + per-stage harness binding.
		if _, err := tx.Exec(`CREATE TABLE harness_serve_registry (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  harness TEXT NOT NULL,
  pid INTEGER NOT NULL,
  port INTEGER NOT NULL,
  url TEXT NOT NULL,
  created_at TEXT NOT NULL
)`); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
		if _, err := tx.Exec(`ALTER TABLE stages ADD COLUMN harness_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
	case 5:
		// ADR-C05 / ADR-039: project-scoped model/capability cache (not global).
		if _, err := tx.Exec(`CREATE TABLE model_list_cache (
  harness TEXT NOT NULL,
  models_json TEXT NOT NULL,
  refreshed_at TEXT NOT NULL,
  PRIMARY KEY (harness)
)`); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
		if _, err := tx.Exec(`CREATE TABLE model_capability_cache (
  harness TEXT NOT NULL,
  model TEXT NOT NULL,
  properties_json TEXT NOT NULL,
  retrieved_at TEXT NOT NULL,
  PRIMARY KEY (harness, model)
)`); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
		if _, err := tx.Exec(`CREATE TABLE model_refresh_state (
  harness TEXT PRIMARY KEY,
  generation INTEGER NOT NULL DEFAULT 0,
  pending INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL DEFAULT ''
)`); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
	case 6:
		// v2.4: project_path on serve registry for lifecycle scoping.
		if _, err := tx.Exec(`ALTER TABLE harness_serve_registry ADD COLUMN project_path TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
	case 7:
		// v2.5 / ADR-044 (C6): Codex app-server registry rows live in the same
		// harness_serve_registry table (pid, harness=codex, project_path, created_at).
		// Stdio transport has no HTTP URL — codex rows store port=0 / url='' and
		// Hero never fabricates a serve URL. The existing columns already fit those
		// rows; this migration adds the (harness, project_path) lookup index used by
		// TUI boot orphan reap for both OpenCode serve and Codex app-server children.
		// OpenCode serve rows are untouched (forward compatible; ADR-044).
		if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_harness_serve_registry_harness_project
  ON harness_serve_registry(harness, project_path)`); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
	case 8:
		// TUI session timer: retain active seconds across TUI restarts while
		// keeping archive/resume boundaries independent of wall-clock time.
		if _, err := tx.Exec(`ALTER TABLE cycles ADD COLUMN session_duration_seconds INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
	case 9:
		// C13: deterministic Status can report the TUI's live permission pause
		// without starting a harness or inspecting credentials.
		if _, err := tx.Exec(`ALTER TABLE stages ADD COLUMN harness_permission_paused INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
	case 10:
		// Orchestrator session is cycle-scoped and must not share stages.harness_session_id
		// with named stage agents (mixed-harness resume). Persist the pair atomically.
		if _, err := tx.Exec(`ALTER TABLE cycles ADD COLUMN orchestration_session_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
		if _, err := tx.Exec(`ALTER TABLE cycles ADD COLUMN orchestration_harness_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
	case 11:
		// C15 / ADR-083: findings, ToDos, projection ops, cycle completion disposition.
		stmts := []string{
			`CREATE TABLE findings (
  id TEXT NOT NULL,
  cycle_id INTEGER NOT NULL REFERENCES cycles(id) ON DELETE CASCADE,
  source_stage TEXT NOT NULL,
  owner TEXT NOT NULL,
  status TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  file TEXT,
  requirement TEXT,
  issue TEXT NOT NULL,
  acceptance_criteria TEXT NOT NULL,
  evidence_json TEXT NOT NULL DEFAULT '[]',
  round INTEGER NOT NULL DEFAULT 1,
  todo_id TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (cycle_id, id),
  UNIQUE (cycle_id, fingerprint),
  CHECK (round >= 1),
  CHECK (length(issue) > 0 AND length(acceptance_criteria) > 0),
  CHECK (source_stage IN ('qa', 'judge', 'browser_ui_validation', 'qa_end_to_end')),
  CHECK (owner IN ('backend_agent', 'frontend_agent', 'generic_agent')),
  CHECK (status IN ('open', 'done', 'reopened', 'deferred_todo'))
)`,
			`CREATE INDEX idx_findings_cycle ON findings(cycle_id)`,
			`CREATE TABLE finding_occurrences (
  cycle_id INTEGER NOT NULL,
  finding_id TEXT NOT NULL,
  sequence INTEGER NOT NULL,
  kind TEXT NOT NULL,
  source_stage TEXT NOT NULL,
  round INTEGER NOT NULL,
  issue TEXT NOT NULL,
  acceptance_criteria TEXT NOT NULL,
  evidence_json TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL,
  PRIMARY KEY (cycle_id, finding_id, sequence),
  FOREIGN KEY (cycle_id, finding_id) REFERENCES findings(cycle_id, id) ON DELETE CASCADE,
  CHECK (round >= 1),
  CHECK (length(issue) > 0 AND length(acceptance_criteria) > 0),
  CHECK (kind IN ('created', 'done', 'reopened', 'rediscovered', 'deferred', 'deferred_recurrence')),
  CHECK (source_stage IN ('qa', 'judge', 'browser_ui_validation', 'qa_end_to_end'))
)`,
			`CREATE INDEX idx_finding_occurrences_cycle ON finding_occurrences(cycle_id)`,
			`CREATE TABLE todos (
  id TEXT PRIMARY KEY,
  origin_type TEXT NOT NULL,
  origin_finding_id TEXT,
  origin_cycle_id INTEGER,
  origin_source_stage TEXT,
  summary TEXT NOT NULL,
  acceptance_criteria TEXT,
  status TEXT NOT NULL,
  adopted_cycle_id INTEGER,
  resolution_note TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  resolved_at TEXT,
  CHECK (origin_type IN ('finding', 'legacy')),
  CHECK (status IN ('pending', 'adopted', 'resolved'))
)`,
			`CREATE TABLE todo_adoptions (
  todo_id TEXT NOT NULL REFERENCES todos(id) ON DELETE CASCADE,
  sequence INTEGER NOT NULL,
  cycle_id INTEGER NOT NULL,
  status TEXT NOT NULL,
  note TEXT,
  created_at TEXT NOT NULL,
  PRIMARY KEY (todo_id, sequence),
  CHECK (status IN ('adopted', 'released', 'resolved'))
)`,
			`CREATE TABLE todo_projection_ops (
  id INTEGER PRIMARY KEY,
  cycle_id INTEGER,
  op_kind TEXT NOT NULL,
  idempotency_key TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL,
  todo_ids_json TEXT NOT NULL,
  candidate_sha256 TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  CHECK (op_kind IN ('defer', 'complete', 'adopt', 'release')),
  CHECK (status IN ('intent_persisted', 'candidate_ready', 'installed', 'verified'))
)`,
			`CREATE INDEX idx_todo_projection_ops_cycle ON todo_projection_ops(cycle_id)`,
			`ALTER TABLE cycles ADD COLUMN completion_disposition TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE cycles ADD COLUMN completion_disposition_json TEXT NOT NULL DEFAULT ''`,
		}
		for _, stmt := range stmts {
			if _, err := tx.Exec(stmt); err != nil {
				return fmt.Errorf("migration %d: %w", version, err)
			}
		}
	default:
		return fmt.Errorf("unknown schema migration version %d", version)
	}

	if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`, version, nowRFC3339()); err != nil {
		return fmt.Errorf("record migration %d: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", version, err)
	}
	slog.Debug("schema migration applied", "version", version)
	return nil
}

// SchemaVersion returns the highest applied migration version.
func (s *Store) SchemaVersion() (int, error) {
	var v int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&v)
	return v, err
}
