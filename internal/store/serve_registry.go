package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// HarnessCodex is the harness id for the Codex TUI adapter (ADR-043; C6).
const HarnessCodex = "codex"

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

// InsertCodexServeRegistry records a Hero-managed Codex app-server child (ADR-044).
// Stdio/JSON-RPC transport has no HTTP URL or port; both are persisted as zero /
// empty — Hero never fabricates a serve URL for Codex (design D13).
func (s *Store) InsertCodexServeRegistry(projectPath string, pid int) (int64, error) {
	return s.InsertServeRegistry(ServeRegistryEntry{
		Harness:     HarnessCodex,
		PID:         pid,
		Port:        0,
		URL:         "",
		ProjectPath: projectPath,
		CreatedAt:   nowRFC3339(),
	})
}

// ListCodexServeRegistry returns Codex app-server registry rows for the project,
// optionally scoped to a project path. The rows carry pid + project identity for
// orphan reap; there is never a URL to connect to (ADR-044).
func (s *Store) ListCodexServeRegistry(projectPath string) ([]ServeRegistryEntry, error) {
	entries, err := s.ListServeRegistry()
	if err != nil {
		return nil, err
	}
	projectPath = strings.TrimSpace(projectPath)
	var out []ServeRegistryEntry
	for _, e := range entries {
		if e.Harness != HarnessCodex {
			continue
		}
		if projectPath != "" && strings.TrimSpace(e.ProjectPath) != "" && e.ProjectPath != projectPath {
			continue
		}
		out = append(out, e)
	}
	return out, nil
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

// StageSessionBinding returns the harness id and harness-native session/thread id
// bound to a stage. An empty harnessID means no binding has been recorded yet.
// Resume logic uses both together so a session is always resumed through the
// harness that owns it (ADR-044; PRD-C06-001 §4.3).
func (s *Store) StageSessionBinding(cycleID int64, stageName string) (harnessID, sessionID string, err error) {
	var h, sid sql.NullString
	err = s.db.QueryRow(`
SELECT harness_id, harness_session_id FROM stages WHERE cycle_id = ? AND name = ?`,
		cycleID, stageName).Scan(&h, &sid)
	if err == sql.ErrNoRows {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", err
	}
	return h.String, sid.String, nil
}

// SessionResumeAllowed reports whether the stored stage session may be resumed as
// harnessID. A non-empty stored session whose recorded harness differs must never
// resume: a Codex thread id must never be resumed as a Cursor/OpenCode session and
// vice versa (ADR-044; PRD-C06-001 §4.3). Unbound stages always allow resume.
func (s *Store) SessionResumeAllowed(cycleID int64, stageName, harnessID string) (bool, error) {
	boundHarness, sessionID, err := s.StageSessionBinding(cycleID, stageName)
	if err == ErrNotFound {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if strings.TrimSpace(sessionID) == "" {
		return true, nil
	}
	if bound := strings.TrimSpace(strings.ToLower(boundHarness)); bound != "" && harnessID != "" && bound != harnessID {
		return false, nil
	}
	return true, nil
}
