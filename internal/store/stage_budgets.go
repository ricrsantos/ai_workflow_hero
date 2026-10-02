package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// StageBudgetState describes whether a stage's cumulative active-time budget is
// running or waiting for an explicit scheduler action.
type StageBudgetState string

const (
	StageBudgetActive      StageBudgetState = "active"
	StageBudgetWaiting     StageBudgetState = "waiting"
	StageBudgetBlocked     StageBudgetState = "blocked"
	StageBudgetExpired     StageBudgetState = "expired"
	StageBudgetInterrupted StageBudgetState = "interrupted"
)

var (
	// ErrStageBudgetGeneration identifies a stale execution generation.
	ErrStageBudgetGeneration = errors.New("stage budget generation is stale")
	// ErrStageBudgetExpired identifies a budget that needs an explicit increase.
	ErrStageBudgetExpired = errors.New("stage budget expired; an explicit increase is required")
	// ErrStageBudgetInterrupted identifies a recovered execution awaiting continue.
	ErrStageBudgetInterrupted = errors.New("stage budget is interrupted; explicit continue is required")
	// ErrStageBudgetExhausted identifies an active budget with no positive remainder.
	ErrStageBudgetExhausted = errors.New("stage budget has no remaining time")
)

// StageBudget is the durable active-time ledger for one cycle stage.
type StageBudget struct {
	CycleID            int64
	StageName          string
	Limit              time.Duration
	Consumed           time.Duration
	State              StageBudgetState
	PauseReason        string
	InterruptionReason string
	Generation         int64
	StartedAt          time.Time
	ActiveSince        *time.Time
	CheckpointAt       time.Time
	UpdatedAt          time.Time
}

// RemainingAt includes active time since the last persisted checkpoint without
// changing the ledger. Paused and recovered budgets never accrue offline time.
func (b StageBudget) RemainingAt(now time.Time) time.Duration {
	consumed := b.Consumed
	if b.State == StageBudgetActive && b.ActiveSince != nil && now.After(b.CheckpointAt) {
		consumed += now.Sub(b.CheckpointAt)
	}
	if consumed >= b.Limit {
		return 0
	}
	return b.Limit - consumed
}

const stageBudgetSelect = `cycle_id, stage_name, limit_ms, consumed_ms, state,
pause_reason, interruption_reason, generation, started_at, active_since,
checkpoint_at, updated_at`

// BeginStageBudget creates a budget the first time a timed stage starts or
// resumes an existing one without resetting its consumed time.
func (s *Store) BeginStageBudget(cycleID int64, stageName string, limit time.Duration, now time.Time) (StageBudget, error) {
	if cycleID <= 0 || stageName == "" {
		return StageBudget{}, fmt.Errorf("cycle and stage are required")
	}
	limitMS := durationMillis(limit)
	if limitMS <= 0 {
		return StageBudget{}, fmt.Errorf("stage budget limit must be positive")
	}
	stamp := budgetTimestamp(now)
	var out StageBudget
	var exhausted bool
	err := s.InTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT OR IGNORE INTO stage_budgets
  (cycle_id, stage_name, limit_ms, consumed_ms, state, pause_reason,
   interruption_reason, generation, started_at, active_since, checkpoint_at, updated_at)
VALUES (?, ?, ?, 0, 'active', '', '', 1, ?, ?, ?, ?)`,
			cycleID, stageName, limitMS, stamp, stamp, stamp, stamp)
		if err != nil {
			return fmt.Errorf("create stage budget: %w", err)
		}
		current, err := scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		if err != nil {
			return err
		}
		switch current.State {
		case StageBudgetActive:
			// Idempotent start: do not issue a new time allowance or generation.
		case StageBudgetExpired:
			return ErrStageBudgetExpired
		case StageBudgetInterrupted:
			return ErrStageBudgetInterrupted
		case StageBudgetWaiting, StageBudgetBlocked:
			if current.RemainingAt(now) <= 0 {
				_, err := tx.Exec(`UPDATE stage_budgets SET state = 'expired', active_since = NULL,
  consumed_ms = limit_ms, pause_reason = '', interruption_reason = 'timeout',
  generation = generation + 1, checkpoint_at = ?, updated_at = ?
WHERE cycle_id = ? AND stage_name = ?`, stamp, stamp, cycleID, stageName)
				if err != nil {
					return fmt.Errorf("expire depleted stage budget: %w", err)
				}
				if err := escalateBudgetStageTx(tx, cycleID, stageName, "timeout", stamp); err != nil {
					return err
				}
				exhausted = true
				break
			}
			generationBump := int64(0)
			if current.PauseReason != "continued" && current.PauseReason != "increased" {
				generationBump = 1
			}
			_, err := tx.Exec(`UPDATE stage_budgets SET state = 'active', active_since = ?,
  pause_reason = '', interruption_reason = '', generation = generation + ?,
  checkpoint_at = ?, updated_at = ? WHERE cycle_id = ? AND stage_name = ?`,
				stamp, generationBump, stamp, stamp, cycleID, stageName)
			if err != nil {
				return fmt.Errorf("resume stage budget: %w", err)
			}
		default:
			return fmt.Errorf("unsupported stage budget state %q", current.State)
		}
		out, err = scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		return err
	})
	if err != nil {
		return StageBudget{}, err
	}
	if exhausted {
		return out, ErrStageBudgetExpired
	}
	return out, nil
}

// GetStageBudget returns a stage's persisted budget ledger.
func (s *Store) GetStageBudget(cycleID int64, stageName string) (StageBudget, error) {
	b, err := scanStageBudget(s.db.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
	if errors.Is(err, sql.ErrNoRows) {
		return StageBudget{}, ErrNotFound
	}
	return b, err
}

// ListActiveStageBudgets is a read-only startup check. It intentionally does
// not recover or revoke active generations: startup ownership must be
// established separately before an active execution can be adopted.
func (s *Store) ListActiveStageBudgets() ([]StageBudget, error) {
	rows, err := s.db.Query(`SELECT ` + stageBudgetSelect + ` FROM stage_budgets WHERE state = 'active' ORDER BY cycle_id, stage_name`)
	if err != nil {
		return nil, fmt.Errorf("list active stage budgets: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []StageBudget
	for rows.Next() {
		budget, err := scanStageBudget(rows)
		if err != nil {
			return nil, fmt.Errorf("scan active stage budget: %w", err)
		}
		out = append(out, budget)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read active stage budgets: %w", err)
	}
	return out, nil
}

// CheckpointStageBudget persists active elapsed time through now. The returned
// row is expired when that checkpoint reaches the limit.
func (s *Store) CheckpointStageBudget(cycleID int64, stageName string, generation int64, now time.Time) (StageBudget, error) {
	return s.advanceStageBudget(cycleID, stageName, generation, now, false)
}

// AcceptStageBudget serializes completion acceptance against expiry. A result
// is accepted only for the current active generation with positive time left.
func (s *Store) AcceptStageBudget(cycleID int64, stageName string, generation int64, now time.Time) (bool, StageBudget, error) {
	b, err := s.advanceStageBudget(cycleID, stageName, generation, now, true)
	if err != nil {
		return false, StageBudget{}, err
	}
	return b.State == StageBudgetActive && b.Generation == generation && b.RemainingAt(now) > 0, b, nil
}

// AcceptAndPauseStageBudget is the terminal completion boundary for one
// execution generation. Positive remaining time is checked and the active
// clock is stopped in the same transaction, so expiry cannot win between
// acceptance and stage-close persistence.
func (s *Store) AcceptAndPauseStageBudget(cycleID int64, stageName string, generation int64, state StageBudgetState, reason string, now time.Time) (bool, StageBudget, error) {
	if state != StageBudgetWaiting && state != StageBudgetBlocked {
		return false, StageBudget{}, fmt.Errorf("invalid accepted stage budget state %q", state)
	}
	if strings.TrimSpace(reason) == "" {
		return false, StageBudget{}, fmt.Errorf("accepted stage budget pause reason is required")
	}
	stamp := budgetTimestamp(now)
	var out StageBudget
	accepted := false
	err := s.InTx(func(tx *sql.Tx) error {
		current, err := scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if current.Generation != generation {
			return ErrStageBudgetGeneration
		}
		if current.State != StageBudgetActive {
			out = current
			return nil
		}
		consumedMS := durationMillis(current.Consumed + elapsedSince(current.CheckpointAt, now))
		if consumedMS >= durationMillis(current.Limit) {
			return expireBudgetTx(tx, cycleID, stageName, generation, stamp, &out)
		}
		if _, err := tx.Exec(`UPDATE stage_budgets SET state = ?, active_since = NULL,
  consumed_ms = ?, pause_reason = ?, interruption_reason = '', checkpoint_at = ?, updated_at = ?
WHERE cycle_id = ? AND stage_name = ? AND generation = ? AND state = 'active'`,
			string(state), consumedMS, reason, stamp, stamp, cycleID, stageName, generation); err != nil {
			return fmt.Errorf("accept and pause stage budget: %w", err)
		}
		out, err = scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		accepted = err == nil
		return err
	})
	if err != nil {
		return false, StageBudget{}, err
	}
	return accepted, out, nil
}

// ExpireStageBudget atomically revokes the active generation after its limit.
// It returns expired=false if the deadline has not actually elapsed yet.
func (s *Store) ExpireStageBudget(cycleID int64, stageName string, generation int64, now time.Time) (StageBudget, bool, error) {
	b, err := s.advanceStageBudget(cycleID, stageName, generation, now, false)
	if err != nil {
		return StageBudget{}, false, err
	}
	return b, b.State == StageBudgetExpired, nil
}

// PauseStageBudget charges active time through now and waits for a human or
// scheduler-owned transition. reason is a safe stable identifier, not user text.
func (s *Store) PauseStageBudget(cycleID int64, stageName string, generation int64, state StageBudgetState, reason string, now time.Time) (StageBudget, error) {
	if state != StageBudgetWaiting && state != StageBudgetBlocked {
		return StageBudget{}, fmt.Errorf("invalid paused stage budget state %q", state)
	}
	if reason == "" {
		return StageBudget{}, fmt.Errorf("stage budget pause reason is required")
	}
	stamp := budgetTimestamp(now)
	var out StageBudget
	err := s.InTx(func(tx *sql.Tx) error {
		current, err := scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return ErrStageBudgetGeneration
		}
		if current.State == StageBudgetExpired {
			return ErrStageBudgetExpired
		}
		consumed := current.Consumed
		if current.State == StageBudgetActive {
			consumed += elapsedSince(current.CheckpointAt, now)
		}
		consumedMS := durationMillis(consumed)
		if consumedMS >= durationMillis(current.Limit) {
			return expireBudgetTx(tx, cycleID, stageName, current.Generation, stamp, &out)
		}
		_, err = tx.Exec(`UPDATE stage_budgets SET state = ?, active_since = NULL,
  consumed_ms = ?, pause_reason = ?, interruption_reason = '', checkpoint_at = ?, updated_at = ?
WHERE cycle_id = ? AND stage_name = ? AND generation = ?`,
			string(state), consumedMS, reason, stamp, stamp, cycleID, stageName, generation)
		if err != nil {
			return fmt.Errorf("pause stage budget: %w", err)
		}
		out, err = scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		return err
	})
	if err != nil {
		return StageBudget{}, err
	}
	return out, nil
}

// PauseStageBudgetBlockedTx checkpoints an active budget and invalidates the
// current generation in a caller-owned transaction. When the budget has
// reached zero, it commits expiry and timeout escalation in that transaction
// and returns the expired row so the caller can skip its blocked-stage close.
func PauseStageBudgetBlockedTx(tx *sql.Tx, cycleID int64, stageName string, now time.Time, reason string) (StageBudget, error) {
	if tx == nil || cycleID <= 0 || strings.TrimSpace(stageName) == "" || strings.TrimSpace(reason) == "" {
		return StageBudget{}, fmt.Errorf("transaction, cycle, stage, and reason are required")
	}
	stamp := budgetTimestamp(now)
	current, err := scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return StageBudget{}, ErrNotFound
		}
		return StageBudget{}, err
	}
	if current.State == StageBudgetExpired {
		return current, nil
	}
	if current.State != StageBudgetActive {
		return StageBudget{}, fmt.Errorf("cannot block stage budget from state %q", current.State)
	}
	consumed := current.Consumed + elapsedSince(current.CheckpointAt, now)
	consumedMS := durationMillis(consumed)
	if consumedMS >= durationMillis(current.Limit) {
		var expired StageBudget
		if err := expireBudgetTx(tx, cycleID, stageName, current.Generation, stamp, &expired); err != nil {
			return StageBudget{}, err
		}
		return expired, nil
	}
	if _, err := tx.Exec(`UPDATE stage_budgets SET state = 'blocked', active_since = NULL,
  consumed_ms = ?, pause_reason = '', interruption_reason = ?, generation = generation + 1,
  checkpoint_at = ?, updated_at = ? WHERE cycle_id = ? AND stage_name = ? AND generation = ? AND state = 'active'`,
		consumedMS, reason, stamp, stamp, cycleID, stageName, current.Generation); err != nil {
		return StageBudget{}, fmt.Errorf("block stage budget: %w", err)
	}
	return scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
}

// ContinueBlockedStageBudgetTx returns a resolved blocked budget to a paused
// waiting state. It does not start active accrual; Engine.StartStage owns that
// transition when the scheduler actually dispatches work.
func ContinueBlockedStageBudgetTx(tx *sql.Tx, cycleID int64, stageName string, now time.Time) (StageBudget, error) {
	if tx == nil || cycleID <= 0 || strings.TrimSpace(stageName) == "" {
		return StageBudget{}, fmt.Errorf("transaction, cycle, and stage are required")
	}
	current, err := scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return StageBudget{}, ErrNotFound
		}
		return StageBudget{}, err
	}
	if current.State == StageBudgetExpired {
		return current, ErrStageBudgetExpired
	}
	if current.State == StageBudgetWaiting {
		return current, nil
	}
	if current.State != StageBudgetBlocked && current.State != StageBudgetInterrupted {
		return StageBudget{}, fmt.Errorf("cannot continue stage budget from state %q", current.State)
	}
	if current.RemainingAt(now) <= 0 {
		stamp := budgetTimestamp(now)
		if _, err := tx.Exec(`UPDATE stage_budgets SET state = 'expired', active_since = NULL,
  consumed_ms = limit_ms, pause_reason = '', interruption_reason = 'timeout',
  generation = generation + 1, checkpoint_at = ?, updated_at = ?
WHERE cycle_id = ? AND stage_name = ? AND generation = ?`, stamp, stamp, cycleID, stageName, current.Generation); err != nil {
			return StageBudget{}, fmt.Errorf("expire depleted continued budget: %w", err)
		}
		if err := escalateBudgetStageTx(tx, cycleID, stageName, "timeout", stamp); err != nil {
			return StageBudget{}, err
		}
		return scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
	}
	stamp := budgetTimestamp(now)
	if _, err := tx.Exec(`UPDATE stage_budgets SET state = 'waiting', active_since = NULL,
  pause_reason = 'continued', interruption_reason = '', checkpoint_at = ?, updated_at = ?
WHERE cycle_id = ? AND stage_name = ? AND generation = ?`, stamp, stamp, cycleID, stageName, current.Generation); err != nil {
		return StageBudget{}, fmt.Errorf("continue blocked stage budget: %w", err)
	}
	return scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
}

// ContinueStageBudget moves a revoked budget to a paused waiting state. It is
// intentionally separate from ResumeStageBudget: time starts only when the
// scheduler dispatches the next attempt through BeginStageBudget.
func (s *Store) ContinueStageBudget(cycleID int64, stageName string, now time.Time) (StageBudget, error) {
	var out StageBudget
	err := s.InTx(func(tx *sql.Tx) error {
		var err error
		out, err = ContinueBlockedStageBudgetTx(tx, cycleID, stageName, now)
		return err
	})
	if err != nil {
		return StageBudget{}, err
	}
	if out.State == StageBudgetExpired {
		return out, ErrStageBudgetExpired
	}
	return out, nil
}

// ResumeStageBudget resumes waiting/interrupted work. rotateGeneration revokes
// prior results when a new attempt begins; human wait release keeps its token.
func (s *Store) ResumeStageBudget(cycleID int64, stageName string, generation int64, now time.Time, rotateGeneration bool) (StageBudget, error) {
	stamp := budgetTimestamp(now)
	var out StageBudget
	var exhausted bool
	err := s.InTx(func(tx *sql.Tx) error {
		current, err := scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return ErrStageBudgetGeneration
		}
		if current.State == StageBudgetExpired {
			return ErrStageBudgetExpired
		}
		if current.State == StageBudgetActive {
			out = current
			return nil
		}
		if current.State != StageBudgetWaiting && current.State != StageBudgetBlocked && current.State != StageBudgetInterrupted {
			return fmt.Errorf("cannot resume stage budget in state %q", current.State)
		}
		if current.RemainingAt(now) <= 0 {
			_, err = tx.Exec(`UPDATE stage_budgets SET state = 'expired', active_since = NULL,
  consumed_ms = limit_ms, pause_reason = '', interruption_reason = 'timeout',
  generation = generation + 1, checkpoint_at = ?, updated_at = ?
WHERE cycle_id = ? AND stage_name = ?`, stamp, stamp, cycleID, stageName)
			if err != nil {
				return fmt.Errorf("expire depleted stage budget: %w", err)
			}
			if err := escalateBudgetStageTx(tx, cycleID, stageName, "timeout", stamp); err != nil {
				return err
			}
			exhausted = true
			out, err = scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
			return err
		}
		bump := int64(0)
		if rotateGeneration {
			bump = 1
		}
		_, err = tx.Exec(`UPDATE stage_budgets SET state = 'active', active_since = ?,
  pause_reason = '', interruption_reason = '', generation = generation + ?,
  checkpoint_at = ?, updated_at = ? WHERE cycle_id = ? AND stage_name = ? AND generation = ?`,
			stamp, bump, stamp, stamp, cycleID, stageName, generation)
		if err != nil {
			return fmt.Errorf("resume stage budget: %w", err)
		}
		out, err = scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		return err
	})
	if err != nil {
		return StageBudget{}, err
	}
	if exhausted {
		return out, ErrStageBudgetExhausted
	}
	return out, nil
}

// InterruptStageBudget records termination and invalidates outstanding result
// tokens. User-initiated termination charges through now; restart recovery uses
// RecoverActiveStageBudgets so uncheckpointed/offline time is excluded.
func (s *Store) InterruptStageBudget(cycleID int64, stageName string, generation int64, reason string, now time.Time) (StageBudget, error) {
	if reason == "" {
		return StageBudget{}, fmt.Errorf("stage budget interruption reason is required")
	}
	stamp := budgetTimestamp(now)
	var out StageBudget
	err := s.InTx(func(tx *sql.Tx) error {
		current, err := scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return ErrStageBudgetGeneration
		}
		consumed := current.Consumed
		if current.State == StageBudgetActive {
			consumed += elapsedSince(current.CheckpointAt, now)
		}
		consumedMS := durationMillis(consumed)
		if consumedMS > durationMillis(current.Limit) {
			consumedMS = durationMillis(current.Limit)
		}
		_, err = tx.Exec(`UPDATE stage_budgets SET state = 'interrupted', active_since = NULL,
  consumed_ms = ?, pause_reason = '', interruption_reason = ?, generation = generation + 1,
  checkpoint_at = ?, updated_at = ? WHERE cycle_id = ? AND stage_name = ? AND generation = ?`,
			consumedMS, reason, stamp, stamp, cycleID, stageName, generation)
		if err != nil {
			return fmt.Errorf("interrupt stage budget: %w", err)
		}
		if err := escalateBudgetStageTx(tx, cycleID, stageName, "interrupted", stamp); err != nil {
			return err
		}
		out, err = scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		return err
	})
	if err != nil {
		return StageBudget{}, err
	}
	return out, nil
}

// EscalateStageBudget stops stage-owned work and revokes its generation while
// retaining any positive remainder for an explicit /hero-continue.
func (s *Store) EscalateStageBudget(cycleID int64, stageName string, generation int64, reason string, now time.Time) (StageBudget, error) {
	if strings.TrimSpace(reason) == "" {
		return StageBudget{}, fmt.Errorf("stage budget escalation reason is required")
	}
	stamp := budgetTimestamp(now)
	var out StageBudget
	err := s.InTx(func(tx *sql.Tx) error {
		current, err := scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return ErrStageBudgetGeneration
		}
		if current.State == StageBudgetExpired {
			out = current
			return nil
		}
		consumed := current.Consumed
		if current.State == StageBudgetActive {
			consumed += elapsedSince(current.CheckpointAt, now)
		}
		consumedMS := durationMillis(consumed)
		if consumedMS >= durationMillis(current.Limit) {
			if err := expireBudgetTx(tx, cycleID, stageName, generation, stamp, &out); err != nil {
				return err
			}
			return nil
		}
		_, err = tx.Exec(`UPDATE stage_budgets SET state = 'blocked', active_since = NULL,
  consumed_ms = ?, pause_reason = '', interruption_reason = ?, generation = generation + 1,
  checkpoint_at = ?, updated_at = ? WHERE cycle_id = ? AND stage_name = ? AND generation = ?`,
			consumedMS, reason, stamp, stamp, cycleID, stageName, generation)
		if err != nil {
			return fmt.Errorf("block stage budget after escalation: %w", err)
		}
		if err := escalateBudgetStageTx(tx, cycleID, stageName, reason, stamp); err != nil {
			return err
		}
		out, err = scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		return err
	})
	if err != nil {
		return StageBudget{}, err
	}
	return out, nil
}

// IncreaseStageBudget adds a validated explicit amount without resetting
// consumption and leaves the stage waiting for its normal continue path.
func (s *Store) IncreaseStageBudget(cycleID int64, stageName string, additional time.Duration, now time.Time) (StageBudget, error) {
	additionalMS := durationMillis(additional)
	if additionalMS <= 0 {
		return StageBudget{}, fmt.Errorf("stage budget increase must be positive")
	}
	stamp := budgetTimestamp(now)
	var out StageBudget
	err := s.InTx(func(tx *sql.Tx) error {
		current, err := scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		if err != nil {
			return err
		}
		if current.State == StageBudgetActive {
			return fmt.Errorf("cannot increase an active stage budget")
		}
		if current.RemainingAt(now) > 0 {
			return fmt.Errorf("stage budget still has time remaining")
		}
		limitMS := durationMillis(current.Limit)
		if additionalMS > math.MaxInt64-limitMS {
			return fmt.Errorf("stage budget increase is too large")
		}
		_, err = tx.Exec(`UPDATE stage_budgets SET limit_ms = limit_ms + ?, state = 'waiting',
  active_since = NULL, pause_reason = 'increased', interruption_reason = '',
  generation = generation + 1, checkpoint_at = ?, updated_at = ?
WHERE cycle_id = ? AND stage_name = ?`, additionalMS, stamp, stamp, cycleID, stageName)
		if err != nil {
			return fmt.Errorf("increase stage budget: %w", err)
		}
		out, err = scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		return err
	})
	if err != nil {
		return StageBudget{}, err
	}
	return out, nil
}

// RecoverActiveStageBudgets marks active rows interrupted using their last
// durable checkpoint. It deliberately does not charge elapsed process-offline
// time and rotates every generation before returning it to the TUI.
func (s *Store) RecoverActiveStageBudgets(now time.Time) ([]StageBudget, error) {
	stamp := budgetTimestamp(now)
	var out []StageBudget
	err := s.InTx(func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT ` + stageBudgetSelect + ` FROM stage_budgets WHERE state = 'active' ORDER BY cycle_id, stage_name`)
		if err != nil {
			return fmt.Errorf("list active stage budgets: %w", err)
		}
		var active []StageBudget
		for rows.Next() {
			b, scanErr := scanStageBudget(rows)
			if scanErr != nil {
				_ = rows.Close()
				return fmt.Errorf("scan active stage budget: %w", scanErr)
			}
			active = append(active, b)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("close active stage budgets: %w", err)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("read active stage budgets: %w", err)
		}
		for _, b := range active {
			_, err := tx.Exec(`UPDATE stage_budgets SET state = 'interrupted', active_since = NULL,
  pause_reason = '', interruption_reason = 'process_restart', generation = generation + 1,
  updated_at = ? WHERE cycle_id = ? AND stage_name = ? AND generation = ? AND state = 'active'`,
				stamp, b.CycleID, b.StageName, b.Generation)
			if err != nil {
				return fmt.Errorf("recover stage budget %s: %w", b.StageName, err)
			}
			if err := escalateBudgetStageTx(tx, b.CycleID, b.StageName, "interrupted", stamp); err != nil {
				return err
			}
			b.State = StageBudgetInterrupted
			b.ActiveSince = nil
			b.InterruptionReason = "process_restart"
			b.Generation++
			b.UpdatedAt = now
			out = append(out, b)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) advanceStageBudget(cycleID int64, stageName string, generation int64, now time.Time, accept bool) (StageBudget, error) {
	stamp := budgetTimestamp(now)
	var out StageBudget
	err := s.InTx(func(tx *sql.Tx) error {
		current, err := scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return ErrStageBudgetGeneration
		}
		if current.State != StageBudgetActive {
			out = current
			return nil
		}
		consumed := current.Consumed + elapsedSince(current.CheckpointAt, now)
		consumedMS := durationMillis(consumed)
		limitMS := durationMillis(current.Limit)
		if consumedMS >= limitMS {
			return expireBudgetTx(tx, cycleID, stageName, generation, stamp, &out)
		}
		_, err = tx.Exec(`UPDATE stage_budgets SET consumed_ms = ?, checkpoint_at = ?, updated_at = ?
WHERE cycle_id = ? AND stage_name = ? AND generation = ? AND state = 'active'`,
			consumedMS, stamp, stamp, cycleID, stageName, generation)
		if err != nil {
			return fmt.Errorf("checkpoint stage budget: %w", err)
		}
		out, err = scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
		if err == nil && accept && out.RemainingAt(now) <= 0 {
			return expireBudgetTx(tx, cycleID, stageName, generation, stamp, &out)
		}
		return err
	})
	if err != nil {
		return StageBudget{}, err
	}
	return out, nil
}

func expireBudgetTx(tx *sql.Tx, cycleID int64, stageName string, generation int64, stamp string, out *StageBudget) error {
	_, err := tx.Exec(`UPDATE stage_budgets SET state = 'expired', active_since = NULL,
  consumed_ms = limit_ms, pause_reason = '', interruption_reason = 'timeout',
  generation = generation + 1, checkpoint_at = ?, updated_at = ?
WHERE cycle_id = ? AND stage_name = ? AND generation = ?`, stamp, stamp, cycleID, stageName, generation)
	if err != nil {
		return fmt.Errorf("expire stage budget: %w", err)
	}
	if err := escalateBudgetStageTx(tx, cycleID, stageName, "timeout", stamp); err != nil {
		return err
	}
	var scanErr error
	*out, scanErr = scanStageBudget(tx.QueryRow(`SELECT `+stageBudgetSelect+`
FROM stage_budgets WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName))
	return scanErr
}

func escalateBudgetStageTx(tx *sql.Tx, cycleID int64, stageName, reason, stamp string) error {
	result, err := tx.Exec(`UPDATE stages SET status = 'Escalated'
WHERE cycle_id = ? AND name = ? AND status IN ('Waiting', 'Running')`, cycleID, stageName)
	if err != nil {
		return fmt.Errorf("escalate stage after budget stop: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read stage escalation result: %w", err)
	}
	if changed == 0 {
		return nil
	}
	payload, err := json.Marshal(struct {
		Stage  string `json:"stage"`
		Reason string `json:"reason"`
	}{Stage: stageName, Reason: reason})
	if err != nil {
		return fmt.Errorf("encode stage escalation event: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO events(cycle_id, ts, type, payload_json)
VALUES (?, ?, 'escalated', ?)`, cycleID, stamp, string(payload)); err != nil {
		return fmt.Errorf("append stage escalation event: %w", err)
	}
	return nil
}

func scanStageBudget(row interface{ Scan(...any) error }) (StageBudget, error) {
	var b StageBudget
	var limitMS, consumedMS int64
	var started, checkpoint, updated sql.NullString
	var active sql.NullString
	err := row.Scan(&b.CycleID, &b.StageName, &limitMS, &consumedMS, &b.State,
		&b.PauseReason, &b.InterruptionReason, &b.Generation, &started, &active,
		&checkpoint, &updated)
	if err != nil {
		return StageBudget{}, err
	}
	b.Limit = time.Duration(limitMS) * time.Millisecond
	b.Consumed = time.Duration(consumedMS) * time.Millisecond
	if b.StartedAt, err = time.Parse(time.RFC3339Nano, started.String); err != nil {
		return StageBudget{}, fmt.Errorf("parse stage budget started_at: %w", err)
	}
	if active.Valid {
		activeTime, parseErr := time.Parse(time.RFC3339Nano, active.String)
		if parseErr != nil {
			return StageBudget{}, fmt.Errorf("parse stage budget active_since: %w", parseErr)
		}
		b.ActiveSince = &activeTime
	}
	if b.CheckpointAt, err = time.Parse(time.RFC3339Nano, checkpoint.String); err != nil {
		return StageBudget{}, fmt.Errorf("parse stage budget checkpoint_at: %w", err)
	}
	if b.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated.String); err != nil {
		return StageBudget{}, fmt.Errorf("parse stage budget updated_at: %w", err)
	}
	return b, nil
}

func durationMillis(d time.Duration) int64 {
	return int64(d / time.Millisecond)
}

func elapsedSince(start, now time.Time) time.Duration {
	if !now.After(start) {
		return 0
	}
	return now.Sub(start)
}

func budgetTimestamp(now time.Time) string {
	return now.UTC().Format(time.RFC3339Nano)
}
