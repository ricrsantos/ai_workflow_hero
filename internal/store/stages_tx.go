package store

import (
	"database/sql"
	"fmt"
)

// GetStageTx returns a stage by cycle and name inside an open transaction.
func (s *Store) GetStageTx(tx *sql.Tx, cycleID int64, name string) (Stage, error) {
	row := tx.QueryRow(`
SELECT `+stageSelectCols+`
FROM stages WHERE cycle_id = ? AND name = ?`, cycleID, name)
	st, err := scanStage(row)
	if err == sql.ErrNoRows {
		return Stage{}, ErrNotFound
	}
	return st, err
}

// ListStagesTx returns stages for a cycle inside an open transaction.
func (s *Store) ListStagesTx(tx *sql.Tx, cycleID int64) ([]Stage, error) {
	rows, err := tx.Query(`
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

// UpdateStageTx persists stage fields inside an open transaction.
func (s *Store) UpdateStageTx(tx *sql.Tx, st Stage) error {
	approval := 0
	if st.RequireHumanApproval {
		approval = 1
	}
	res, err := tx.Exec(`
UPDATE stages SET status = ?, iteration = ?, max_iterations = ?, extra_iterations = ?,
  require_human_approval = ?, timeout_minutes = ?, started_at = ?, completed_at = ?, summary = ?,
	 harness_session_id = ?, harness_id = ?, harness_permission_paused = ?
WHERE id = ?`,
		st.Status, st.Iteration, st.MaxIterations, st.ExtraIterations,
		approval, st.TimeoutMinutes, nullStr(st.StartedAt), nullStr(st.CompletedAt),
		nullStr(st.Summary), st.HarnessSessionID, st.HarnessID, boolToInt(st.HarnessPermissionPaused), st.ID,
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
