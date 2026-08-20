package store

import (
	"database/sql"
	"fmt"
)

// ServeRegistryEntry records a Hero-managed harness serve process (ADR-035; design D13).
type ServeRegistryEntry struct {
	ID          int64
	Harness     string
	PID         int
	Port        int
	URL         string
	ProjectPath string
	CreatedAt   string
}

// InsertServeRegistry records a new serve process.
func (s *Store) InsertServeRegistry(entry ServeRegistryEntry) (int64, error) {
	res, err := s.db.Exec(`
INSERT INTO harness_serve_registry(harness, pid, port, url, project_path, created_at)
VALUES(?, ?, ?, ?, ?, ?)`,
		entry.Harness, entry.PID, entry.Port, entry.URL, entry.ProjectPath, entry.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert serve registry: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	s.log.Info("serve registry entry created", "harness", entry.Harness, "pid", entry.PID, "port", entry.Port)
	return id, nil
}

// ListServeRegistry returns all recorded serve entries for the project.
func (s *Store) ListServeRegistry() ([]ServeRegistryEntry, error) {
	rows, err := s.db.Query(`
SELECT id, harness, pid, port, url, project_path, created_at FROM harness_serve_registry ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ServeRegistryEntry
	for rows.Next() {
		var e ServeRegistryEntry
		if err := rows.Scan(&e.ID, &e.Harness, &e.PID, &e.Port, &e.URL, &e.ProjectPath, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteServeRegistry removes a registry row by id.
func (s *Store) DeleteServeRegistry(id int64) error {
	_, err := s.db.Exec(`DELETE FROM harness_serve_registry WHERE id = ?`, id)
	return err
}

// ClearServeRegistry removes all serve registry rows.
func (s *Store) ClearServeRegistry() error {
	_, err := s.db.Exec(`DELETE FROM harness_serve_registry`)
	return err
}

// SetStageHarnessID stores the harness id used for a stage session (multi-harness routing).
func (s *Store) SetStageHarnessID(cycleID int64, stageName, harnessID string) error {
	res, err := s.db.Exec(`
UPDATE stages SET harness_id = ? WHERE cycle_id = ? AND name = ?`, harnessID, cycleID, stageName)
	if err != nil {
		return fmt.Errorf("set harness_id: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	s.log.Debug("stage harness id updated", "cycle_id", cycleID, "stage", stageName, "harness_id", harnessID)
	return nil
}

// StageHarnessID returns the harness id bound to a stage, or empty when unset.
func (s *Store) StageHarnessID(cycleID int64, stageName string) (string, error) {
	var id sql.NullString
	err := s.db.QueryRow(`
SELECT harness_id FROM stages WHERE cycle_id = ? AND name = ?`, cycleID, stageName).Scan(&id)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return id.String, nil
}
