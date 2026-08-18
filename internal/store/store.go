package store

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const (
	// DBFileName is the SQLite file under .workflow-hero/.
	DBFileName = "hero.db"

	// RelativeDBPath is the project-relative path to the operational store.
	RelativeDBPath = ".workflow-hero/hero.db"
)

// Store wraps an open Hero operational SQLite database.
type Store struct {
	db  *sql.DB
	log *slog.Logger
}

// Open opens (or creates) hero.db at path and applies migrations.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create store directory: %w", err)
	}

	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	s := &Store{db: db, log: slog.Default()}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	s.log.Info("store opened", "path", path)
	return s, nil
}

// OpenProject opens the store at projectDir/.workflow-hero/hero.db.
func OpenProject(projectDir string) (*Store, error) {
	return Open(filepath.Join(projectDir, RelativeDBPath))
}

// openCapped opens a store but only applies migrations up to maxVersion.
// Test hook for schema v4→v5 migration coverage.
func openCapped(path string, maxVersion int) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create store directory: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, log: slog.Default()}
	if err := s.migrateTo(maxVersion); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB exposes the underlying *sql.DB for advanced queries (tests/engine).
func (s *Store) DB() *sql.DB {
	return s.db
}

// Path helpers ---------------------------------------------------------------

// nowRFC3339 returns UTC time in RFC3339 for persistence.
func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
