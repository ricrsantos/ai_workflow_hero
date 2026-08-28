package store

import (
	"database/sql"
	"fmt"
)

const cycleSelectCols = `id, number, title, objective, status, started_at, completed_at, session_duration_seconds, config_snapshot_json, lock_holder, lock_at, openspec_change`

// CreateCycle inserts a new cycle and returns its ID.
func (s *Store) CreateCycle(c Cycle) (int64, error) {
	res, err := s.db.Exec(`
INSERT INTO cycles(number, title, objective, status, started_at, completed_at, session_duration_seconds, config_snapshot_json, lock_holder, lock_at, openspec_change)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Number, c.Title, c.Objective, c.Status, nullStr(c.StartedAt), nullStr(c.CompletedAt),
		maxInt64(c.SessionDurationSeconds, 0), c.ConfigSnapshotJSON, nullStr(c.LockHolder), nullStr(c.LockAt), c.OpenspecChange,
	)
	if err != nil {
		return 0, fmt.Errorf("insert cycle: %w", err)
	}
	return res.LastInsertId()
}

// GetCycle returns a cycle by ID.
func (s *Store) GetCycle(id int64) (Cycle, error) {
	row := s.db.QueryRow(`SELECT `+cycleSelectCols+` FROM cycles WHERE id = ?`, id)
	return scanCycle(row)
}

// GetActiveCycle returns the active (non-archived) current cycle, if any.
// Prefers status=active; falls back to the highest-number non-archived cycle.
func (s *Store) GetActiveCycle() (Cycle, error) {
	row := s.db.QueryRow(`
SELECT `+cycleSelectCols+`
FROM cycles
WHERE status = ?
ORDER BY number DESC
LIMIT 1`, CycleStatusActive)
	c, err := scanCycle(row)
	if err == sql.ErrNoRows {
		return Cycle{}, ErrNoActiveCycle
	}
	return c, err
}

// UpdateCycleStatus updates status and optional completed_at.
func (s *Store) UpdateCycleStatus(id int64, status, completedAt string) error {
	_, err := s.db.Exec(`UPDATE cycles SET status = ?, completed_at = ? WHERE id = ?`,
		status, nullStr(completedAt), id)
	if err != nil {
		return fmt.Errorf("update cycle status: %w", err)
	}
	return nil
}

// UpdateCycleSessionDuration stores the greatest known active session time.
// Monotonic writes prevent an older asynchronous TUI tick from decreasing it.
func (s *Store) UpdateCycleSessionDuration(id, seconds int64) error {
	if seconds < 0 {
		return fmt.Errorf("session duration must not be negative")
	}
	result, err := s.db.Exec(`
UPDATE cycles
SET session_duration_seconds = CASE
  WHEN session_duration_seconds > ? THEN session_duration_seconds
  ELSE ?
END
WHERE id = ?`, seconds, seconds, id)
	if err != nil {
		return fmt.Errorf("update cycle session duration: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("check cycle session duration update: %w", err)
	} else if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateCycleMeta updates title/objective/config snapshot.
func (s *Store) UpdateCycleMeta(id int64, title, objective, configJSON string) error {
	_, err := s.db.Exec(`UPDATE cycles SET title = ?, objective = ?, config_snapshot_json = ? WHERE id = ?`,
		title, objective, configJSON, id)
	return err
}

// SetOpenspecChange sets or clears the OpenSpec change name on a cycle.
// Pass an empty string to clear.
func (s *Store) SetOpenspecChange(id int64, name string) error {
	_, err := s.db.Exec(`UPDATE cycles SET openspec_change = ? WHERE id = ?`, name, id)
	if err != nil {
		return fmt.Errorf("update openspec_change: %w", err)
	}
	return nil
}

// SetCycleLock sets or clears the cycle lock holder.
func (s *Store) SetCycleLock(id int64, holder, lockAt string) error {
	_, err := s.db.Exec(`UPDATE cycles SET lock_holder = ?, lock_at = ? WHERE id = ?`,
		nullStr(holder), nullStr(lockAt), id)
	return err
}

// NextCycleNumber returns max(number)+1 (or 1 if empty).
func (s *Store) NextCycleNumber() (int, error) {
	var n sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(number) FROM cycles`).Scan(&n); err != nil {
		return 0, err
	}
	if !n.Valid {
		return 1, nil
	}
	return int(n.Int64) + 1, nil
}

// ListCycles returns all cycles ordered by number ascending.
func (s *Store) ListCycles() ([]Cycle, error) {
	rows, err := s.db.Query(`SELECT ` + cycleSelectCols + ` FROM cycles ORDER BY number ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Cycle
	for rows.Next() {
		c, err := scanCycle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCycle(row rowScanner) (Cycle, error) {
	var c Cycle
	var started, completed, lockHolder, lockAt sql.NullString
	err := row.Scan(
		&c.ID, &c.Number, &c.Title, &c.Objective, &c.Status,
		&started, &completed, &c.SessionDurationSeconds, &c.ConfigSnapshotJSON, &lockHolder, &lockAt,
		&c.OpenspecChange,
	)
	if err != nil {
		return Cycle{}, err
	}
	c.StartedAt = started.String
	c.CompletedAt = completed.String
	c.LockHolder = lockHolder.String
	c.LockAt = lockAt.String
	return c, nil
}

func maxInt64(value, minimum int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
