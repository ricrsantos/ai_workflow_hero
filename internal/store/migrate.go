package store

import (
	"fmt"
	"log/slog"
)

// currentSchemaVersion is the latest migration version applied by Open.
const currentSchemaVersion = 6

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
