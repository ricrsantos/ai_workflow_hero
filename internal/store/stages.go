package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrNoActiveCycle is returned when no active cycle exists.
var ErrNoActiveCycle = errors.New("no active cycle")

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

// ErrBusy is returned when the cycle lock is held by another session.
var ErrBusy = errors.New("cycle is locked by another session")

const stageSelectCols = `id, cycle_id, name, status, iteration, max_iterations, extra_iterations,
  require_human_approval, timeout_minutes, started_at, completed_at, summary, sort_order, harness_session_id`

// CreateStages inserts stage rows for a cycle.
func (s *Store) CreateStages(stages []Stage) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
INSERT INTO stages(cycle_id, name, status, iteration, max_iterations, extra_iterations,
  require_human_approval, timeout_minutes, started_at, completed_at, summary, sort_order, harness_session_id)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, st := range stages {
		approval := 0
		if st.RequireHumanApproval {
			approval = 1
		}
		if _, err := stmt.Exec(
			st.CycleID, st.Name, st.Status, st.Iteration, st.MaxIterations, st.ExtraIterations,
			approval, st.TimeoutMinutes, nullStr(st.StartedAt), nullStr(st.CompletedAt),
			nullStr(st.Summary), st.SortOrder, st.HarnessSessionID,
		); err != nil {
			return fmt.Errorf("insert stage %s: %w", st.Name, err)
		}
	}
	return tx.Commit()
}

// ListStages returns stages for a cycle ordered by sort_order.
func (s *Store) ListStages(cycleID int64) ([]Stage, error) {
	rows, err := s.db.Query(`
SELECT `+stageSelectCols+`
FROM stages WHERE cycle_id = ? ORDER BY sort_order ASC, id ASC`, cycleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Stage
	for rows.Next() {
		st, err := scanStage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// GetStage returns a stage by cycle and name.
func (s *Store) GetStage(cycleID int64, name string) (Stage, error) {
	row := s.db.QueryRow(`
SELECT `+stageSelectCols+`
FROM stages WHERE cycle_id = ? AND name = ?`, cycleID, name)
	st, err := scanStage(row)
	if err == sql.ErrNoRows {
		return Stage{}, ErrNotFound
	}
	return st, err
}

// UpdateStage persists stage fields.
func (s *Store) UpdateStage(st Stage) error {
	approval := 0
	if st.RequireHumanApproval {
		approval = 1
	}
	res, err := s.db.Exec(`
UPDATE stages SET status = ?, iteration = ?, max_iterations = ?, extra_iterations = ?,
  require_human_approval = ?, timeout_minutes = ?, started_at = ?, completed_at = ?, summary = ?,
  harness_session_id = ?
WHERE id = ?`,
		st.Status, st.Iteration, st.MaxIterations, st.ExtraIterations,
		approval, st.TimeoutMinutes, nullStr(st.StartedAt), nullStr(st.CompletedAt),
		nullStr(st.Summary), st.HarnessSessionID, st.ID,
	)
	if err != nil {
		return fmt.Errorf("update stage: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetStageHarnessSessionID stores or clears the harness session id for a stage (design D6).
func (s *Store) SetStageHarnessSessionID(cycleID int64, stageName, sessionID string) error {
	res, err := s.db.Exec(`
UPDATE stages SET harness_session_id = ? WHERE cycle_id = ? AND name = ?`,
		sessionID, cycleID, stageName)
	if err != nil {
		return fmt.Errorf("set harness_session_id: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	s.log.Debug("harness session id updated", "cycle_id", cycleID, "stage", stageName, "session_id", sessionID)
	return nil
}

// ClearStageHarnessSessionID clears the harness session id (e.g. on stage complete).
func (s *Store) ClearStageHarnessSessionID(cycleID int64, stageName string) error {
	return s.SetStageHarnessSessionID(cycleID, stageName, "")
}

type scannable interface {
	Scan(dest ...any) error
}

func scanStage(row scannable) (Stage, error) {
	var st Stage
	var approval int
	var started, completed, summary sql.NullString
	err := row.Scan(
		&st.ID, &st.CycleID, &st.Name, &st.Status, &st.Iteration, &st.MaxIterations, &st.ExtraIterations,
		&approval, &st.TimeoutMinutes, &started, &completed, &summary, &st.SortOrder, &st.HarnessSessionID,
	)
	if err != nil {
		return Stage{}, err
	}
	st.RequireHumanApproval = approval != 0
	st.StartedAt = started.String
	st.CompletedAt = completed.String
	st.Summary = summary.String
	return st, nil
}
