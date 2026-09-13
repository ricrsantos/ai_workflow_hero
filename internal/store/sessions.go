package store

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Session kinds (schema CHECK).
const (
	SessionKindFreechat      = "freechat"
	SessionKindOrchestration = "orchestration"
	SessionKindResearch      = "research"
	SessionKindStageAgent    = "stage_agent"
)

// Session lifecycle values (schema CHECK).
const (
	SessionLifecycleActive      = "active"
	SessionLifecycleArchived    = "archived"
	SessionLifecycleInterrupted = "interrupted"
	SessionLifecycleDeleting    = "deleting"
)

// Transcript availability (schema CHECK).
const (
	TranscriptAvailable         = "available"
	TranscriptUnavailableLegacy = "unavailable_legacy"
)

// Session origin for last_origin / event origin (schema CHECK).
const (
	SessionOriginLocal    = "local"
	SessionOriginTelegram = "telegram"
)

// ErrSessionNotFound is returned when a Hero session id is unknown.
var ErrSessionNotFound = errors.New("session not found")

// ErrSessionBusy is returned when another owner holds an unexpired lease.
var ErrSessionBusy = errors.New("session lease held by another owner")

// ErrCrossSessionRouting is returned when an append targets the wrong session.
var ErrCrossSessionRouting = errors.New("cross-session event routing rejected")

// ErrEmptySessionTitle is returned when a rename would leave an empty title.
var ErrEmptySessionTitle = errors.New("session title must not be empty")

// ErrDuplicateNativeSession is returned when (harness_id, native_session_id) collides.
var ErrDuplicateNativeSession = errors.New("native session id already bound")

// Session is a durable Hero conversation aggregate (schema v12).
type Session struct {
	ID                    string
	Kind                  string
	Title                 string
	Lifecycle             string
	HarnessID             string
	NativeSessionID       string
	Model                 string
	ModelPropertiesJSON   string
	CycleID               sql.NullInt64
	StageName             string
	AgentName             string
	TranscriptState       string
	LastOrigin            string
	CreatedAt             string
	LastActivityAt        string
	InterruptedAt         sql.NullString
	RemoteImportConfirmed bool
	LegacySourceKey       sql.NullString
}

// CreateSessionInput is the payload for inserting a new sessions row.
type CreateSessionInput struct {
	ID                  string // optional; generated when empty
	Kind                string
	Title               string
	Lifecycle           string // default active
	HarnessID           string
	NativeSessionID     string
	Model               string
	ModelPropertiesJSON string
	CycleID             *int64
	StageName           string
	AgentName           string
	TranscriptState     string // default available
	LastOrigin          string // default local
	LegacySourceKey     string // optional unique key for legacy import
	CreatedAt           string // optional RFC3339; default now
	LastActivityAt      string // optional RFC3339; default CreatedAt
}

const sessionSelectCols = `id, kind, title, lifecycle, harness_id, native_session_id, model,
model_properties_json, cycle_id, stage_name, agent_name, transcript_state, last_origin,
created_at, last_activity_at, interrupted_at, remote_import_confirmed, legacy_source_key`

// NewSessionID returns a random UUID suitable as a Hero session primary key.
func NewSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

// CreateSession inserts a sessions row. It does not create events.
func (s *Store) CreateSession(in CreateSessionInput) (Session, error) {
	var out Session
	err := s.InTx(func(tx *sql.Tx) error {
		var err error
		out, err = createSessionTx(tx, in)
		return err
	})
	if err != nil {
		return Session{}, err
	}
	s.log.Info("session created", "kind", out.Kind, "lifecycle", out.Lifecycle)
	return out, nil
}

func createSessionTx(tx *sql.Tx, in CreateSessionInput) (Session, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		generated, err := NewSessionID()
		if err != nil {
			return Session{}, err
		}
		id = generated
	}
	kind := strings.TrimSpace(in.Kind)
	if !validSessionKind(kind) {
		return Session{}, fmt.Errorf("invalid session kind %q", in.Kind)
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return Session{}, ErrEmptySessionTitle
	}
	lifecycle := strings.TrimSpace(in.Lifecycle)
	if lifecycle == "" {
		lifecycle = SessionLifecycleActive
	}
	if !validSessionLifecycle(lifecycle) {
		return Session{}, fmt.Errorf("invalid session lifecycle %q", in.Lifecycle)
	}
	transcript := strings.TrimSpace(in.TranscriptState)
	if transcript == "" {
		transcript = TranscriptAvailable
	}
	if !validTranscriptState(transcript) {
		return Session{}, fmt.Errorf("invalid transcript_state %q", in.TranscriptState)
	}
	origin := strings.TrimSpace(in.LastOrigin)
	if origin == "" {
		origin = SessionOriginLocal
	}
	if !validSessionOrigin(origin) {
		return Session{}, fmt.Errorf("invalid last_origin %q", in.LastOrigin)
	}
	props := strings.TrimSpace(in.ModelPropertiesJSON)
	if props == "" {
		props = "{}"
	}
	created := strings.TrimSpace(in.CreatedAt)
	if created == "" {
		created = nowRFC3339()
	}
	activity := strings.TrimSpace(in.LastActivityAt)
	if activity == "" {
		activity = created
	}
	var cycle any
	if in.CycleID != nil {
		cycle = *in.CycleID
	}
	var legacy any
	if key := strings.TrimSpace(in.LegacySourceKey); key != "" {
		legacy = key
	}
	_, err := tx.Exec(`
INSERT INTO sessions(
  id, kind, title, lifecycle, harness_id, native_session_id, model, model_properties_json,
  cycle_id, stage_name, agent_name, transcript_state, last_origin, created_at, last_activity_at,
  interrupted_at, remote_import_confirmed, legacy_source_key
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, 0, ?)`,
		id, kind, title, lifecycle,
		strings.TrimSpace(in.HarnessID), strings.TrimSpace(in.NativeSessionID),
		strings.TrimSpace(in.Model), props,
		cycle, strings.TrimSpace(in.StageName), strings.TrimSpace(in.AgentName),
		transcript, origin, created, activity, legacy,
	)
	if err != nil {
		if isUniqueViolation(err) {
			if strings.TrimSpace(in.NativeSessionID) != "" {
				return Session{}, ErrDuplicateNativeSession
			}
			return Session{}, fmt.Errorf("create session: %w", err)
		}
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	return getSessionTx(tx, id)
}

// GetSession returns a session by Hero id.
func (s *Store) GetSession(id string) (Session, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Session{}, ErrSessionNotFound
	}
	row := s.db.QueryRow(`SELECT `+sessionSelectCols+` FROM sessions WHERE id = ?`, id)
	sess, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}
	return sess, nil
}

func getSessionTx(tx *sql.Tx, id string) (Session, error) {
	row := tx.QueryRow(`SELECT `+sessionSelectCols+` FROM sessions WHERE id = ?`, id)
	sess, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}
	return sess, nil
}

// UpdateSessionTitle renames a session after trimming; empty titles are rejected.
// Duplicate titles across sessions are allowed.
func (s *Store) UpdateSessionTitle(id, title string) (Session, error) {
	id = strings.TrimSpace(id)
	title = strings.TrimSpace(title)
	if id == "" {
		return Session{}, ErrSessionNotFound
	}
	if title == "" {
		return Session{}, ErrEmptySessionTitle
	}
	res, err := s.db.Exec(`UPDATE sessions SET title = ? WHERE id = ?`, title, id)
	if err != nil {
		return Session{}, fmt.Errorf("update session title: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Session{}, ErrSessionNotFound
	}
	s.log.Info("session title updated")
	return s.GetSession(id)
}

// UpdateSessionLifecycle sets lifecycle (and optional interrupted_at).
func (s *Store) UpdateSessionLifecycle(id, lifecycle string, interruptedAt *string) (Session, error) {
	id = strings.TrimSpace(id)
	lifecycle = strings.TrimSpace(lifecycle)
	if id == "" {
		return Session{}, ErrSessionNotFound
	}
	if !validSessionLifecycle(lifecycle) {
		return Session{}, fmt.Errorf("invalid session lifecycle %q", lifecycle)
	}
	var interrupted any
	if interruptedAt != nil {
		interrupted = strings.TrimSpace(*interruptedAt)
	} else if lifecycle == SessionLifecycleInterrupted {
		interrupted = nowRFC3339()
	}
	res, err := s.db.Exec(`UPDATE sessions SET lifecycle = ?, interrupted_at = ? WHERE id = ?`,
		lifecycle, interrupted, id)
	if err != nil {
		return Session{}, fmt.Errorf("update session lifecycle: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Session{}, ErrSessionNotFound
	}
	s.log.Info("session lifecycle updated", "lifecycle", lifecycle)
	return s.GetSession(id)
}

// BindNativeSession sets harness/native/model attributes after first native bind.
func (s *Store) BindNativeSession(id, harnessID, nativeSessionID, model, modelPropertiesJSON string) (Session, error) {
	err := s.InTx(func(tx *sql.Tx) error {
		return bindNativeSessionTx(tx, id, harnessID, nativeSessionID, model, modelPropertiesJSON)
	})
	if err != nil {
		return Session{}, err
	}
	s.log.Info("session native identity bound", "harness_id", strings.TrimSpace(harnessID))
	return s.GetSession(id)
}

func bindNativeSessionTx(tx *sql.Tx, id, harnessID, nativeSessionID, model, modelPropertiesJSON string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrSessionNotFound
	}
	props := strings.TrimSpace(modelPropertiesJSON)
	if props == "" {
		props = "{}"
	}
	res, err := tx.Exec(`
UPDATE sessions SET harness_id = ?, native_session_id = ?, model = ?, model_properties_json = ?
WHERE id = ?`,
		strings.TrimSpace(harnessID), strings.TrimSpace(nativeSessionID),
		strings.TrimSpace(model), props, id)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateNativeSession
		}
		return fmt.Errorf("bind native session: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// SetRemoteImportConfirmed marks that the user confirmed remote history import.
func (s *Store) SetRemoteImportConfirmed(id string, confirmed bool) error {
	flag := 0
	if confirmed {
		flag = 1
	}
	res, err := s.db.Exec(`UPDATE sessions SET remote_import_confirmed = ? WHERE id = ?`, flag, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("set remote import confirmed: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func setRemoteImportConfirmedTx(tx *sql.Tx, id string, confirmed bool) error {
	flag := 0
	if confirmed {
		flag = 1
	}
	res, err := tx.Exec(`UPDATE sessions SET remote_import_confirmed = ? WHERE id = ?`, flag, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("set remote import confirmed: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// UpdateSessionTranscriptState sets transcript_state for a session row.
func (s *Store) UpdateSessionTranscriptState(id, transcriptState string) (Session, error) {
	id = strings.TrimSpace(id)
	transcriptState = strings.TrimSpace(transcriptState)
	if id == "" {
		return Session{}, ErrSessionNotFound
	}
	if !validTranscriptState(transcriptState) {
		return Session{}, fmt.Errorf("invalid transcript_state %q", transcriptState)
	}
	err := s.InTx(func(tx *sql.Tx) error {
		return updateSessionTranscriptStateTx(tx, id, transcriptState)
	})
	if err != nil {
		return Session{}, err
	}
	s.log.Info("session transcript state updated", "transcript_state", transcriptState)
	return s.GetSession(id)
}

func updateSessionTranscriptStateTx(tx *sql.Tx, id, transcriptState string) error {
	transcriptState = strings.TrimSpace(transcriptState)
	if !validTranscriptState(transcriptState) {
		return fmt.Errorf("invalid transcript_state %q", transcriptState)
	}
	res, err := tx.Exec(`UPDATE sessions SET transcript_state = ? WHERE id = ?`, transcriptState, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("update session transcript state: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// ImportRemoteSessionEventsInput batches remote history append with import confirmation.
type ImportRemoteSessionEventsInput struct {
	SessionID                string
	Events                   []AppendSessionEventInput
	SetRemoteImportConfirmed bool
	SetTranscriptAvailable   bool
}

// ImportRemoteSessionEvents appends events and optional import flags in one transaction.
func (s *Store) ImportRemoteSessionEvents(in ImportRemoteSessionEventsInput) (int, error) {
	sessionID := strings.TrimSpace(in.SessionID)
	if sessionID == "" {
		return 0, ErrSessionNotFound
	}
	imported := 0
	err := s.InTx(func(tx *sql.Tx) error {
		if _, err := getSessionTx(tx, sessionID); err != nil {
			return err
		}
		for _, ev := range in.Events {
			ev.SessionID = sessionID
			if strings.TrimSpace(ev.BoundSessionID) == "" {
				ev.BoundSessionID = sessionID
			}
			_, err := appendSessionEventTx(tx, ev)
			if errors.Is(err, ErrDuplicateProviderEvent) {
				continue
			}
			if err != nil {
				return err
			}
			imported++
		}
		if in.SetRemoteImportConfirmed {
			if err := setRemoteImportConfirmedTx(tx, sessionID, true); err != nil {
				return err
			}
		}
		if in.SetTranscriptAvailable {
			if err := updateSessionTranscriptStateTx(tx, sessionID, TranscriptAvailable); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if imported > 0 {
		s.log.Debug("remote session events imported", "imported", imported)
	}
	return imported, nil
}

// ListSessionsFilter selects Active or Archived History rows.
type ListSessionsFilter struct {
	// Archived true lists lifecycle=archived; false lists active+interrupted (non-archived, non-deleting).
	Archived bool
	// Query is optional case-insensitive title substring (COLLATE NOCASE LIKE).
	Query string
	// Limit caps rows; 0 means no explicit limit.
	Limit int
}

// ListSessions returns History rows ordered by last_activity_at DESC, id.
func (s *Store) ListSessions(filter ListSessionsFilter) ([]Session, error) {
	var args []any
	var b strings.Builder
	b.WriteString(`SELECT ` + sessionSelectCols + ` FROM sessions WHERE `)
	if filter.Archived {
		b.WriteString(`lifecycle = ?`)
		args = append(args, SessionLifecycleArchived)
	} else {
		b.WriteString(`lifecycle IN (?, ?)`)
		args = append(args, SessionLifecycleActive, SessionLifecycleInterrupted)
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		b.WriteString(` AND title LIKE ? COLLATE NOCASE`)
		args = append(args, "%"+q+"%")
	}
	b.WriteString(` ORDER BY last_activity_at DESC, id`)
	if filter.Limit > 0 {
		b.WriteString(` LIMIT ?`)
		args = append(args, filter.Limit)
	}
	rows, err := s.db.Query(b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// ListRegisteredSessionIDs returns all session primary keys (for media retention).
func (s *Store) ListRegisteredSessionIDs() ([]string, error) {
	rows, err := s.db.Query(`SELECT id FROM sessions`)
	if err != nil {
		return nil, fmt.Errorf("list session ids: %w", err)
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

// DeleteSessionRow removes the sessions row (cascades events/assets/lease). Prefer DeleteSessionWithOp for recoverable deletes.
func (s *Store) DeleteSessionRow(id string) error {
	res, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSessionNotFound
	}
	s.log.Info("session deleted")
	return nil
}

type sessionScanner interface {
	Scan(dest ...any) error
}

func scanSession(row sessionScanner) (Session, error) {
	var sess Session
	var remote int
	err := row.Scan(
		&sess.ID, &sess.Kind, &sess.Title, &sess.Lifecycle,
		&sess.HarnessID, &sess.NativeSessionID, &sess.Model, &sess.ModelPropertiesJSON,
		&sess.CycleID, &sess.StageName, &sess.AgentName, &sess.TranscriptState, &sess.LastOrigin,
		&sess.CreatedAt, &sess.LastActivityAt, &sess.InterruptedAt, &remote, &sess.LegacySourceKey,
	)
	if err != nil {
		return Session{}, err
	}
	sess.RemoteImportConfirmed = remote != 0
	return sess, nil
}

func validSessionKind(k string) bool {
	switch k {
	case SessionKindFreechat, SessionKindOrchestration, SessionKindResearch, SessionKindStageAgent:
		return true
	default:
		return false
	}
}

func validSessionLifecycle(l string) bool {
	switch l {
	case SessionLifecycleActive, SessionLifecycleArchived, SessionLifecycleInterrupted, SessionLifecycleDeleting:
		return true
	default:
		return false
	}
}

func validTranscriptState(t string) bool {
	switch t {
	case TranscriptAvailable, TranscriptUnavailableLegacy:
		return true
	default:
		return false
	}
}

func validSessionOrigin(o string) bool {
	switch o {
	case SessionOriginLocal, SessionOriginTelegram:
		return true
	default:
		return false
	}
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "constraint failed")
}

// Clock abstracts time for lease tests.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// DefaultClock is UTC wall clock.
func DefaultClock() Clock { return systemClock{} }
