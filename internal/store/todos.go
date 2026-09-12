package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ToDo origin and status values (schema v11).
const (
	TodoOriginFinding = "finding"
	TodoOriginLegacy  = "legacy"

	TodoStatusPending  = "pending"
	TodoStatusAdopted  = "adopted"
	TodoStatusResolved = "resolved"

	AdoptionStatusAdopted  = "adopted"
	AdoptionStatusReleased = "released"
	AdoptionStatusResolved = "resolved"

	ProjectionOpDefer    = "defer"
	ProjectionOpComplete = "complete"
	ProjectionOpAdopt    = "adopt"
	ProjectionOpRelease  = "release"

	ProjectionStatusIntentPersisted = "intent_persisted"
	ProjectionStatusCandidateReady  = "candidate_ready"
	ProjectionStatusInstalled       = "installed"
	ProjectionStatusVerified        = "verified"

	CompletionDispositionDeferredTodos = "completed_with_deferred_todos"
)

// ErrTodoInvalidState is returned when a ToDo transition is not allowed.
var ErrTodoInvalidState = errors.New("todo invalid state")

// ErrTodoResolutionNoteConflict is returned when an idempotent retry supplies a
// different resolution note than the one already persisted.
var ErrTodoResolutionNoteConflict = errors.New("todo resolution note conflict")

// Todo is a durable project ToDo row (schema v11).
type Todo struct {
	ID                 string
	OriginType         string
	OriginFindingID    string
	OriginCycleID      int64
	OriginSourceStage  string
	Summary            string
	AcceptanceCriteria string
	Status             string
	AdoptedCycleID     int64
	ResolutionNote     string
	CreatedAt          string
	UpdatedAt          string
	ResolvedAt         string
}

// TodoAdoption is an append-only adoption-history row.
type TodoAdoption struct {
	TodoID    string
	Sequence  int
	CycleID   int64
	Status    string
	Note      string
	CreatedAt string
}

// TodoProjectionOp tracks recoverable current-state.md projection work.
type TodoProjectionOp struct {
	ID              int64
	CycleID         int64
	OpKind          string
	IdempotencyKey  string
	Status          string
	TodoIDsJSON     string
	CandidateSHA256 string
	CreatedAt       string
	UpdatedAt       string
}

// CreateFindingTodoParams describes a finding-derived pending ToDo.
type CreateFindingTodoParams struct {
	ID                 string
	OriginCycleID      int64
	OriginSourceStage  string
	Summary            string
	AcceptanceCriteria string
}

// CreateLegacyTodoParams describes a legacy pending ToDo (ID allocated as todo-N).
type CreateLegacyTodoParams struct {
	Summary            string
	AcceptanceCriteria string
}

// UpsertProjectionOpParams inserts or returns an existing projection op by idempotency key.
type UpsertProjectionOpParams struct {
	CycleID         int64
	OpKind          string
	IdempotencyKey  string
	Status          string
	TodoIDsJSON     string
	CandidateSHA256 string
}

// CompleteTodoIdempotentParams completes a pending ToDo with projection idempotency.
type CompleteTodoIdempotentParams struct {
	TodoID           string
	ResolutionNote   string
	IdempotencyKey   string
	CycleID          int64
	ProjectionStatus string
}

// AllocateNextLegacyTodoID returns the next project-scoped todo-N identifier.
func (s *Store) AllocateNextLegacyTodoID() (string, error) {
	return allocateNextLegacyTodoID(s.db)
}

func allocateNextLegacyTodoID(q queryRower) (string, error) {
	rows, err := q.Query(`SELECT id FROM todos WHERE id GLOB 'todo-[0-9]*'`)
	if err != nil {
		return "", fmt.Errorf("list legacy todo ids: %w", err)
	}
	defer rows.Close()

	maxN := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		n, ok := parseLegacyTodoSuffix(id)
		if ok && n > maxN {
			maxN = n
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return fmt.Sprintf("todo-%d", maxN+1), nil
}

func parseLegacyTodoSuffix(id string) (int, bool) {
	if !strings.HasPrefix(id, "todo-") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(id, "todo-"))
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// CreateFindingTodo inserts a pending finding-derived ToDo using the finding ID.
func (s *Store) CreateFindingTodo(p CreateFindingTodoParams) error {
	return s.createFindingTodo(s.db, p)
}

func (s *Store) createFindingTodo(exec execer, p CreateFindingTodoParams) error {
	id := strings.TrimSpace(p.ID)
	if id == "" {
		return fmt.Errorf("finding todo id is required")
	}
	now := nowRFC3339()
	_, err := exec.Exec(`
INSERT INTO todos(
  id, origin_type, origin_finding_id, origin_cycle_id, origin_source_stage,
  summary, acceptance_criteria, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, TodoOriginFinding, id, nullInt64(p.OriginCycleID), nullStr(p.OriginSourceStage),
		p.Summary, nullStr(p.AcceptanceCriteria), TodoStatusPending, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert finding todo: %w", err)
	}
	s.log.Info("finding todo created", "todo_id", id, "origin_cycle_id", p.OriginCycleID)
	return nil
}

// CreateLegacyTodo allocates todo-N and inserts a pending legacy ToDo.
func (s *Store) CreateLegacyTodo(p CreateLegacyTodoParams) (string, error) {
	id, err := s.AllocateNextLegacyTodoID()
	if err != nil {
		return "", err
	}
	now := nowRFC3339()
	_, err = s.db.Exec(`
INSERT INTO todos(
  id, origin_type, summary, acceptance_criteria, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, TodoOriginLegacy, p.Summary, nullStr(p.AcceptanceCriteria), TodoStatusPending, now, now,
	)
	if err != nil {
		return "", fmt.Errorf("insert legacy todo: %w", err)
	}
	s.log.Info("legacy todo created", "todo_id", id)
	return id, nil
}

// ListPendingTodos returns every pending ToDo ordered by ID (Research adoption query).
func (s *Store) ListPendingTodos() ([]Todo, error) {
	rows, err := s.db.Query(`
SELECT id, origin_type, origin_finding_id, origin_cycle_id, origin_source_stage,
  summary, acceptance_criteria, status, adopted_cycle_id, resolution_note,
  created_at, updated_at, resolved_at
FROM todos
WHERE status = ?
ORDER BY id ASC`, TodoStatusPending)
	if err != nil {
		return nil, fmt.Errorf("list pending todos: %w", err)
	}
	defer rows.Close()
	return scanTodoRows(rows)
}

func scanTodoRows(rows *sql.Rows) ([]Todo, error) {
	var out []Todo
	for rows.Next() {
		var t Todo
		var originFinding, originStage, acceptance, resolution, resolved sql.NullString
		var originCycle, adoptedCycle sql.NullInt64
		if err := rows.Scan(
			&t.ID, &t.OriginType, &originFinding, &originCycle, &originStage,
			&t.Summary, &acceptance, &t.Status, &adoptedCycle, &resolution,
			&t.CreatedAt, &t.UpdatedAt, &resolved,
		); err != nil {
			return nil, err
		}
		t.OriginFindingID = originFinding.String
		t.OriginSourceStage = originStage.String
		t.AcceptanceCriteria = acceptance.String
		t.ResolutionNote = resolution.String
		t.ResolvedAt = resolved.String
		if originCycle.Valid {
			t.OriginCycleID = originCycle.Int64
		}
		if adoptedCycle.Valid {
			t.AdoptedCycleID = adoptedCycle.Int64
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListResolvedLegacySummaries returns normalized summaries for resolved legacy ToDos.
// Used by projection to suppress promoted prose lines after manual complete.
func (s *Store) ListResolvedLegacySummaries() ([]string, error) {
	rows, err := s.db.Query(`
SELECT summary FROM todos
WHERE origin_type = ? AND status = ?
ORDER BY id ASC`, TodoOriginLegacy, TodoStatusResolved)
	if err != nil {
		return nil, fmt.Errorf("list resolved legacy summaries: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var summary string
		if err := rows.Scan(&summary); err != nil {
			return nil, err
		}
		out = append(out, summary)
	}
	return out, rows.Err()
}

// ListTodosForProjection returns pending and adopted ToDos ordered by ID for markdown projection.
func (s *Store) ListTodosForProjection() ([]Todo, error) {
	rows, err := s.db.Query(`
SELECT id, origin_type, origin_finding_id, origin_cycle_id, origin_source_stage,
  summary, acceptance_criteria, status, adopted_cycle_id, resolution_note,
  created_at, updated_at, resolved_at
FROM todos
WHERE status IN (?, ?)
ORDER BY id ASC`, TodoStatusPending, TodoStatusAdopted)
	if err != nil {
		return nil, fmt.Errorf("list todos for projection: %w", err)
	}
	defer rows.Close()
	return scanTodoRows(rows)
}

// CountTodosByStatus returns how many ToDo rows have the given status.
func (s *Store) CountTodosByStatus(status string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM todos WHERE status = ?`, status).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count todos by status: %w", err)
	}
	return n, nil
}

// GetTodo returns a ToDo by primary key.
func (s *Store) GetTodo(id string) (Todo, error) {
	row := s.db.QueryRow(`
SELECT id, origin_type, origin_finding_id, origin_cycle_id, origin_source_stage,
  summary, acceptance_criteria, status, adopted_cycle_id, resolution_note,
  created_at, updated_at, resolved_at
FROM todos WHERE id = ?`, id)
	t, err := scanTodo(row)
	if err == sql.ErrNoRows {
		return Todo{}, ErrNotFound
	}
	return t, err
}

func scanTodo(row rowScanner) (Todo, error) {
	var t Todo
	var originFinding, originStage, acceptance, resolution, resolved sql.NullString
	var originCycle, adoptedCycle sql.NullInt64
	err := row.Scan(
		&t.ID, &t.OriginType, &originFinding, &originCycle, &originStage,
		&t.Summary, &acceptance, &t.Status, &adoptedCycle, &resolution,
		&t.CreatedAt, &t.UpdatedAt, &resolved,
	)
	if err != nil {
		return Todo{}, err
	}
	t.OriginFindingID = originFinding.String
	t.OriginSourceStage = originStage.String
	t.AcceptanceCriteria = acceptance.String
	t.ResolutionNote = resolution.String
	t.ResolvedAt = resolved.String
	if originCycle.Valid {
		t.OriginCycleID = originCycle.Int64
	}
	if adoptedCycle.Valid {
		t.AdoptedCycleID = adoptedCycle.Int64
	}
	return t, nil
}

// ListTodoAdoptions returns adoption history ordered by sequence.
func (s *Store) ListTodoAdoptions(todoID string) ([]TodoAdoption, error) {
	rows, err := s.db.Query(`
SELECT todo_id, sequence, cycle_id, status, note, created_at
FROM todo_adoptions WHERE todo_id = ? ORDER BY sequence ASC`, todoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TodoAdoption
	for rows.Next() {
		var a TodoAdoption
		var note sql.NullString
		if err := rows.Scan(&a.TodoID, &a.Sequence, &a.CycleID, &a.Status, &note, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Note = note.String
		out = append(out, a)
	}
	return out, rows.Err()
}

// AdoptTodo marks a pending ToDo adopted by cycleID and appends adoption history.
func (s *Store) AdoptTodo(todoID string, cycleID int64, note string) error {
	return s.InTx(func(tx *sql.Tx) error {
		return s.adoptTodoTx(tx, todoID, cycleID, note)
	})
}

func (s *Store) adoptTodoTx(tx *sql.Tx, todoID string, cycleID int64, note string) error {
	t, err := scanTodo(tx.QueryRow(`
SELECT id, origin_type, origin_finding_id, origin_cycle_id, origin_source_stage,
  summary, acceptance_criteria, status, adopted_cycle_id, resolution_note,
  created_at, updated_at, resolved_at
FROM todos WHERE id = ?`, todoID))
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if t.Status != TodoStatusPending {
		return fmt.Errorf("%w: adopt requires pending, got %s", ErrTodoInvalidState, t.Status)
	}
	seq, err := nextAdoptionSequence(tx, todoID)
	if err != nil {
		return err
	}
	now := nowRFC3339()
	if _, err := tx.Exec(`
INSERT INTO todo_adoptions(todo_id, sequence, cycle_id, status, note, created_at)
VALUES (?, ?, ?, ?, ?, ?)`,
		todoID, seq, cycleID, AdoptionStatusAdopted, nullStr(note), now); err != nil {
		return fmt.Errorf("insert todo adoption: %w", err)
	}
	if _, err := tx.Exec(`
UPDATE todos SET status = ?, adopted_cycle_id = ?, updated_at = ? WHERE id = ?`,
		TodoStatusAdopted, cycleID, now, todoID); err != nil {
		return fmt.Errorf("update todo adopted: %w", err)
	}
	s.log.Info("todo adopted", "todo_id", todoID, "cycle_id", cycleID)
	cycleNumber, err := cycleNumberTx(tx, cycleID)
	if err != nil {
		return err
	}
	return appendLifecycleEventTx(tx, s, cycleID, EventTodoAdopted, map[string]any{
		"todo_id": todoID, "cycle_number": cycleNumber,
	})
}

// ReleaseTodo returns an adopted ToDo to pending and records a released adoption row.
func (s *Store) ReleaseTodo(todoID string, cycleID int64, note string) error {
	return s.InTx(func(tx *sql.Tx) error {
		return s.releaseTodoTx(tx, todoID, cycleID, note)
	})
}

func (s *Store) releaseTodoTx(tx *sql.Tx, todoID string, cycleID int64, note string) error {
	t, err := scanTodo(tx.QueryRow(`
SELECT id, origin_type, origin_finding_id, origin_cycle_id, origin_source_stage,
  summary, acceptance_criteria, status, adopted_cycle_id, resolution_note,
  created_at, updated_at, resolved_at
FROM todos WHERE id = ?`, todoID))
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if t.Status != TodoStatusAdopted || t.AdoptedCycleID != cycleID {
		return fmt.Errorf("%w: release requires adopted by cycle %d", ErrTodoInvalidState, cycleID)
	}
	seq, err := nextAdoptionSequence(tx, todoID)
	if err != nil {
		return err
	}
	now := nowRFC3339()
	if _, err := tx.Exec(`
INSERT INTO todo_adoptions(todo_id, sequence, cycle_id, status, note, created_at)
VALUES (?, ?, ?, ?, ?, ?)`,
		todoID, seq, cycleID, AdoptionStatusReleased, nullStr(note), now); err != nil {
		return fmt.Errorf("insert todo release: %w", err)
	}
	if _, err := tx.Exec(`
UPDATE todos SET status = ?, adopted_cycle_id = NULL, updated_at = ? WHERE id = ?`,
		TodoStatusPending, now, todoID); err != nil {
		return fmt.Errorf("update todo released: %w", err)
	}
	s.log.Info("todo released", "todo_id", todoID, "cycle_id", cycleID)
	cycleNumber, err := cycleNumberTx(tx, cycleID)
	if err != nil {
		return err
	}
	return appendLifecycleEventTx(tx, s, cycleID, EventTodoReleased, map[string]any{
		"todo_id": todoID, "cycle_number": cycleNumber,
	})
}

// ListAdoptedTodoIDsForCycle returns ToDo IDs adopted by cycleID.
func (s *Store) ListAdoptedTodoIDsForCycle(cycleID int64) ([]string, error) {
	rows, err := s.db.Query(`
SELECT id FROM todos WHERE status = ? AND adopted_cycle_id = ? ORDER BY id ASC`,
		TodoStatusAdopted, cycleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ReleaseAdoptedTodosForCycle releases every ToDo adopted by cycleID back to pending.
func (s *Store) ReleaseAdoptedTodosForCycle(cycleID int64, note string) error {
	return s.InTx(func(tx *sql.Tx) error {
		rows, err := tx.Query(`
SELECT id FROM todos WHERE status = ? AND adopted_cycle_id = ?`,
			TodoStatusAdopted, cycleID)
		if err != nil {
			return err
		}
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, id := range ids {
			if err := s.releaseTodoTx(tx, id, cycleID, note); err != nil {
				return err
			}
		}
		if len(ids) > 0 {
			s.log.Info("released adopted todos for cycle", "cycle_id", cycleID, "count", len(ids))
		}
		return nil
	})
}

// ResolveAdoptedTodosForCycle marks adopted ToDos for cycleID resolved after validated completion.
func (s *Store) ResolveAdoptedTodosForCycle(cycleID int64) error {
	return s.InTx(func(tx *sql.Tx) error {
		rows, err := tx.Query(`
SELECT id FROM todos WHERE status = ? AND adopted_cycle_id = ?`,
			TodoStatusAdopted, cycleID)
		if err != nil {
			return err
		}
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, id := range ids {
			if err := s.resolveAdoptedTodoTx(tx, id, cycleID, ""); err != nil {
				return err
			}
		}
		if len(ids) > 0 {
			s.log.Info("adopted todos resolved for cycle", "cycle_id", cycleID, "count", len(ids))
		}
		return nil
	})
}

func (s *Store) resolveAdoptedTodoTx(tx *sql.Tx, todoID string, cycleID int64, resolutionNote string) error {
	t, err := scanTodo(tx.QueryRow(`
SELECT id, origin_type, origin_finding_id, origin_cycle_id, origin_source_stage,
  summary, acceptance_criteria, status, adopted_cycle_id, resolution_note,
  created_at, updated_at, resolved_at
FROM todos WHERE id = ?`, todoID))
	if err != nil {
		return err
	}
	if t.Status != TodoStatusAdopted || t.AdoptedCycleID != cycleID {
		return fmt.Errorf("%w: resolve requires adopted by cycle %d", ErrTodoInvalidState, cycleID)
	}
	seq, err := nextAdoptionSequence(tx, todoID)
	if err != nil {
		return err
	}
	now := nowRFC3339()
	if _, err := tx.Exec(`
INSERT INTO todo_adoptions(todo_id, sequence, cycle_id, status, note, created_at)
VALUES (?, ?, ?, ?, ?, ?)`,
		todoID, seq, cycleID, AdoptionStatusResolved, nullStr(resolutionNote), now); err != nil {
		return fmt.Errorf("insert todo resolve adoption: %w", err)
	}
	if _, err := tx.Exec(`
UPDATE todos SET status = ?, resolution_note = ?, resolved_at = ?, updated_at = ?,
  adopted_cycle_id = NULL WHERE id = ?`,
		TodoStatusResolved, nullStr(resolutionNote), now, now, todoID); err != nil {
		return fmt.Errorf("update todo resolved: %w", err)
	}
	s.log.Info("todo resolved", "todo_id", todoID, "cycle_id", cycleID)
	return nil
}

// CompleteTodoIdempotent resolves a pending ToDo with a non-empty note and projection idempotency.
func (s *Store) CompleteTodoIdempotent(p CompleteTodoIdempotentParams) error {
	note := strings.TrimSpace(p.ResolutionNote)
	if note == "" {
		return fmt.Errorf("resolution note is required")
	}
	status := p.ProjectionStatus
	if status == "" {
		// Intent must be reconciled onto current-state.md; verified here would skip projection.
		status = ProjectionStatusIntentPersisted
	}
	return s.InTx(func(tx *sql.Tx) error {
		existing, err := getProjectionOpByKeyTx(tx, p.IdempotencyKey)
		if err != nil && err != ErrNotFound {
			return err
		}
		if err == nil {
			if err := assertIdempotentComplete(existing, p.TodoID, note); err != nil {
				return err
			}
			if existing.Status == ProjectionStatusVerified {
				t, err := scanTodo(tx.QueryRow(`
SELECT id, origin_type, origin_finding_id, origin_cycle_id, origin_source_stage,
  summary, acceptance_criteria, status, adopted_cycle_id, resolution_note,
  created_at, updated_at, resolved_at
FROM todos WHERE id = ?`, p.TodoID))
				if err != nil {
					return err
				}
				if strings.TrimSpace(t.ResolutionNote) != note {
					return ErrTodoResolutionNoteConflict
				}
				s.log.Debug("complete todo idempotent retry", "idempotency_key", p.IdempotencyKey, "todo_id", p.TodoID)
				return nil
			}
		}
		t, err := scanTodo(tx.QueryRow(`
SELECT id, origin_type, origin_finding_id, origin_cycle_id, origin_source_stage,
  summary, acceptance_criteria, status, adopted_cycle_id, resolution_note,
  created_at, updated_at, resolved_at
FROM todos WHERE id = ?`, p.TodoID))
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if t.Status == TodoStatusResolved {
			if strings.TrimSpace(t.ResolutionNote) != note {
				return ErrTodoResolutionNoteConflict
			}
			s.log.Debug("complete todo already resolved", "todo_id", p.TodoID)
		} else if t.Status != TodoStatusPending {
			return fmt.Errorf("%w: complete requires pending, got %s", ErrTodoInvalidState, t.Status)
		} else {
			now := nowRFC3339()
			if _, err := tx.Exec(`
UPDATE todos SET status = ?, resolution_note = ?, resolved_at = ?, updated_at = ? WHERE id = ?`,
				TodoStatusResolved, note, now, now, p.TodoID); err != nil {
				return fmt.Errorf("resolve todo manual: %w", err)
			}
			s.log.Info("todo completed manually", "todo_id", p.TodoID)
			if p.CycleID != 0 {
				payload := map[string]any{"todo_id": p.TodoID}
				if strings.TrimSpace(t.OriginFindingID) != "" {
					payload["finding_id"] = t.OriginFindingID
				}
				if err := appendLifecycleEventTx(tx, s, p.CycleID, EventTodoCompletedManual, payload); err != nil {
					return err
				}
			}
		}
		todoJSON, err := json.Marshal([]string{p.TodoID})
		if err != nil {
			return err
		}
		op, isNew, err := upsertProjectionOpTx(tx, UpsertProjectionOpParams{
			CycleID:        p.CycleID,
			OpKind:         ProjectionOpComplete,
			IdempotencyKey: p.IdempotencyKey,
			Status:         status,
			TodoIDsJSON:    string(todoJSON),
		})
		if err != nil {
			return err
		}
		if !isNew && op.Status != status {
			now := nowRFC3339()
			if _, err := tx.Exec(`
UPDATE todo_projection_ops SET status = ?, updated_at = ? WHERE idempotency_key = ?`,
				status, now, p.IdempotencyKey); err != nil {
				return fmt.Errorf("update projection op status: %w", err)
			}
		}
		return nil
	})
}

func assertIdempotentComplete(op TodoProjectionOp, todoID, note string) error {
	ids, err := parseTodoIDsJSON(op.TodoIDsJSON)
	if err != nil {
		return err
	}
	if len(ids) != 1 || ids[0] != todoID {
		return fmt.Errorf("idempotency key %q already used for different todos", op.IdempotencyKey)
	}
	if op.OpKind != ProjectionOpComplete {
		return fmt.Errorf("idempotency key %q already used for op %s", op.IdempotencyKey, op.OpKind)
	}
	return nil
}

// DeferFindingTodoIdempotent creates a finding-derived pending ToDo once per idempotency key.
func (s *Store) DeferFindingTodoIdempotent(key string, cycleID int64, p CreateFindingTodoParams) error {
	return s.InTx(func(tx *sql.Tx) error {
		existing, err := getProjectionOpByKeyTx(tx, key)
		if err != nil && err != ErrNotFound {
			return err
		}
		if err == nil {
			if err := assertIdempotentDefer(existing, p.ID); err != nil {
				return err
			}
			if existing.Status == ProjectionStatusVerified {
				s.log.Debug("defer todo idempotent retry", "idempotency_key", key, "todo_id", p.ID)
				return nil
			}
		}
		findingID := strings.TrimSpace(p.ID)
		f, err := getFindingTx(tx, cycleID, findingID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return fmt.Errorf("finding %q not found in active cycle", findingID)
			}
			return err
		}
		switch f.Status {
		case FindingStatusOpen, FindingStatusReopened:
			if err := s.SetFindingDeferredTodoTx(tx, cycleID, f.ID, f.Issue, f.AcceptanceCriteria, f.EvidenceJSON); err != nil {
				return err
			}
		case FindingStatusDeferredTodo:
			// idempotent retry or partial reconciliation
		default:
			return fmt.Errorf("finding %q is %s, expected open or reopened", findingID, f.Status)
		}
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM todos WHERE id = ?`, p.ID).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			if err := s.createFindingTodo(tx, p); err != nil {
				return err
			}
		}
		todoJSON, err := json.Marshal([]string{p.ID})
		if err != nil {
			return err
		}
		status := ProjectionStatusIntentPersisted
		if existing.Status != "" {
			status = existing.Status
		}
		if _, _, err := upsertProjectionOpTx(tx, UpsertProjectionOpParams{
			CycleID:        cycleID,
			OpKind:         ProjectionOpDefer,
			IdempotencyKey: key,
			Status:         status,
			TodoIDsJSON:    string(todoJSON),
		}); err != nil {
			return err
		}
		return nil
	})
}

func assertIdempotentDefer(op TodoProjectionOp, todoID string) error {
	ids, err := parseTodoIDsJSON(op.TodoIDsJSON)
	if err != nil {
		return err
	}
	if len(ids) != 1 || ids[0] != todoID {
		return fmt.Errorf("idempotency key %q already used for different todos", op.IdempotencyKey)
	}
	if op.OpKind != ProjectionOpDefer {
		return fmt.Errorf("idempotency key %q already used for op %s", op.IdempotencyKey, op.OpKind)
	}
	return nil
}

// GetTodoProjectionOpByKey returns a projection op by idempotency key.
func (s *Store) GetTodoProjectionOpByKey(key string) (TodoProjectionOp, error) {
	return getProjectionOpByKeyTx(s.db, key)
}

// UpsertTodoProjectionOp inserts a projection op or returns the existing row for the key.
func (s *Store) UpsertTodoProjectionOp(p UpsertProjectionOpParams) (TodoProjectionOp, bool, error) {
	var created bool
	var out TodoProjectionOp
	err := s.InTx(func(tx *sql.Tx) error {
		op, isNew, err := upsertProjectionOpTx(tx, p)
		if err != nil {
			return err
		}
		out = op
		created = isNew
		return nil
	})
	return out, created, err
}

// UpdateTodoProjectionOpStatus updates status and optional candidate hash for a key.
func (s *Store) UpdateTodoProjectionOpStatus(idempotencyKey, status, candidateSHA string) error {
	now := nowRFC3339()
	res, err := s.db.Exec(`
UPDATE todo_projection_ops SET status = ?, candidate_sha256 = ?, updated_at = ?
WHERE idempotency_key = ?`, status, nullStr(candidateSHA), now, idempotencyKey)
	if err != nil {
		return fmt.Errorf("update projection op: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	s.log.Debug("projection op status updated", "idempotency_key", idempotencyKey, "status", status)
	return nil
}

// SetCycleCompletionDisposition stores terminal disposition fields on a cycle.
func (s *Store) SetCycleCompletionDisposition(cycleID int64, disposition, dispositionJSON string) error {
	res, err := s.db.Exec(`
UPDATE cycles SET completion_disposition = ?, completion_disposition_json = ? WHERE id = ?`,
		disposition, dispositionJSON, cycleID)
	if err != nil {
		return fmt.Errorf("update completion disposition: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	s.log.Info("cycle completion disposition set", "cycle_id", cycleID, "disposition", disposition)
	return nil
}

// GetCycleCompletionDisposition returns disposition fields for a cycle.
func (s *Store) GetCycleCompletionDisposition(cycleID int64) (disposition, dispositionJSON string, err error) {
	err = s.db.QueryRow(`
SELECT completion_disposition, completion_disposition_json FROM cycles WHERE id = ?`, cycleID).
		Scan(&disposition, &dispositionJSON)
	if err == sql.ErrNoRows {
		return "", "", ErrNotFound
	}
	return disposition, dispositionJSON, err
}

func nextAdoptionSequence(tx *sql.Tx, todoID string) (int, error) {
	var seq int
	err := tx.QueryRow(`SELECT COALESCE(MAX(sequence), 0) + 1 FROM todo_adoptions WHERE todo_id = ?`, todoID).Scan(&seq)
	return seq, err
}

func getProjectionOpByKeyTx(q queryRower, key string) (TodoProjectionOp, error) {
	row := q.QueryRow(`
SELECT id, cycle_id, op_kind, idempotency_key, status, todo_ids_json, candidate_sha256, created_at, updated_at
FROM todo_projection_ops WHERE idempotency_key = ?`, key)
	var op TodoProjectionOp
	var cycleID sql.NullInt64
	var candidate sql.NullString
	err := row.Scan(&op.ID, &cycleID, &op.OpKind, &op.IdempotencyKey, &op.Status,
		&op.TodoIDsJSON, &candidate, &op.CreatedAt, &op.UpdatedAt)
	if err == sql.ErrNoRows {
		return TodoProjectionOp{}, ErrNotFound
	}
	if err != nil {
		return TodoProjectionOp{}, err
	}
	if cycleID.Valid {
		op.CycleID = cycleID.Int64
	}
	op.CandidateSHA256 = candidate.String
	return op, nil
}

func upsertProjectionOpTx(tx *sql.Tx, p UpsertProjectionOpParams) (TodoProjectionOp, bool, error) {
	existing, err := getProjectionOpByKeyTx(tx, p.IdempotencyKey)
	if err == nil {
		return existing, false, nil
	}
	if err != ErrNotFound {
		return TodoProjectionOp{}, false, err
	}
	now := nowRFC3339()
	status := p.Status
	if status == "" {
		status = ProjectionStatusIntentPersisted
	}
	res, err := tx.Exec(`
INSERT INTO todo_projection_ops(
  cycle_id, op_kind, idempotency_key, status, todo_ids_json, candidate_sha256, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt64(p.CycleID), p.OpKind, p.IdempotencyKey, status, p.TodoIDsJSON, nullStr(p.CandidateSHA256), now, now)
	if err != nil {
		return TodoProjectionOp{}, false, fmt.Errorf("insert projection op: %w", err)
	}
	id, _ := res.LastInsertId()
	return TodoProjectionOp{
		ID:              id,
		CycleID:         p.CycleID,
		OpKind:          p.OpKind,
		IdempotencyKey:  p.IdempotencyKey,
		Status:          status,
		TodoIDsJSON:     p.TodoIDsJSON,
		CandidateSHA256: p.CandidateSHA256,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, true, nil
}

func parseTodoIDsJSON(raw string) ([]string, error) {
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil, fmt.Errorf("parse todo_ids_json: %w", err)
	}
	return ids, nil
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

type queryRower interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

func nullInt64(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}
