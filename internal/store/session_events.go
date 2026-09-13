package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// Session event types (design D1 / D10).
const (
	SessionEventUser         = "user"
	SessionEventAssistant    = "assistant"
	SessionEventThinking     = "thinking"
	SessionEventTool         = "tool"
	SessionEventWarning      = "warning"
	SessionEventPermission   = "permission"
	SessionEventQuestion     = "question"
	SessionEventAttachment   = "attachment"
	SessionEventAsset        = "asset"
	SessionEventInterruption = "interruption"
	SessionEventNote         = "note"
)

// DefaultSessionEventPage is the newest-page size for Chat restore.
const DefaultSessionEventPage = 200

// SessionEvent is an append-only transcript event.
type SessionEvent struct {
	SessionID       string
	Seq             int64
	EventType       string
	Origin          string
	OriginAddress   string
	PayloadJSON     string
	ProviderEventID string
	CreatedAt       string
}

// AppendSessionEventInput is the payload for appending one event.
type AppendSessionEventInput struct {
	// BoundSessionID is the Hero session bound to the Execute; must equal SessionID.
	BoundSessionID  string
	SessionID       string
	EventType       string
	Origin          string
	OriginAddress   string
	PayloadJSON     string
	ProviderEventID string
	CreatedAt       string // optional RFC3339
	// TouchLifecycle when non-empty updates sessions.lifecycle in the same tx (e.g. active after interrupt recovery).
	TouchLifecycle string
	// ReplaceExisting updates payload/origin for an existing provider_event_id instead of no-op.
	ReplaceExisting bool
}

// ErrDuplicateProviderEvent is returned when provider_event_id already exists for the session.
var ErrDuplicateProviderEvent = errors.New("duplicate provider event id")

// AppendSessionEvent allocates seq transactionally, inserts the event, and updates last_activity_at.
func (s *Store) AppendSessionEvent(in AppendSessionEventInput) (SessionEvent, error) {
	var out SessionEvent
	err := s.InTx(func(tx *sql.Tx) error {
		var err error
		out, err = appendSessionEventTx(tx, in)
		return err
	})
	if err != nil {
		return SessionEvent{}, err
	}
	s.log.Debug("session event appended", "seq", out.Seq, "event_type", out.EventType)
	return out, nil
}

func appendSessionEventTx(tx *sql.Tx, in AppendSessionEventInput) (SessionEvent, error) {
	sessionID := strings.TrimSpace(in.SessionID)
	bound := strings.TrimSpace(in.BoundSessionID)
	if bound == "" {
		bound = sessionID
	}
	if sessionID == "" || bound != sessionID {
		return SessionEvent{}, ErrCrossSessionRouting
	}
	if !validSessionEventType(in.EventType) {
		return SessionEvent{}, fmt.Errorf("invalid session event type %q", in.EventType)
	}
	origin := strings.TrimSpace(in.Origin)
	if origin == "" {
		origin = SessionOriginLocal
	}
	if !validSessionOrigin(origin) {
		return SessionEvent{}, fmt.Errorf("invalid event origin %q", in.Origin)
	}
	payload := strings.TrimSpace(in.PayloadJSON)
	if payload == "" {
		payload = "{}"
	}
	created := strings.TrimSpace(in.CreatedAt)
	if created == "" {
		created = nowRFC3339()
	}
	providerID := strings.TrimSpace(in.ProviderEventID)

	if _, err := getSessionTx(tx, sessionID); err != nil {
		return SessionEvent{}, err
	}

	if providerID != "" {
		var existingSeq int64
		err := tx.QueryRow(`
SELECT seq FROM session_events WHERE session_id = ? AND provider_event_id = ?`,
			sessionID, providerID).Scan(&existingSeq)
		if err == nil {
			if in.ReplaceExisting {
				return replaceSessionEventTx(tx, sessionID, existingSeq, in, origin, payload, providerID, created)
			}
			return getSessionEventTx(tx, sessionID, existingSeq)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return SessionEvent{}, fmt.Errorf("check provider event id: %w", err)
		}
	}

	var maxSeq sql.NullInt64
	if err := tx.QueryRow(`SELECT MAX(seq) FROM session_events WHERE session_id = ?`, sessionID).Scan(&maxSeq); err != nil {
		return SessionEvent{}, fmt.Errorf("allocate session event seq: %w", err)
	}
	seq := int64(1)
	if maxSeq.Valid {
		seq = maxSeq.Int64 + 1
	}

	_, err := tx.Exec(`
INSERT INTO session_events(
  session_id, seq, event_type, origin, origin_address, payload_json, provider_event_id, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, seq, in.EventType, origin, strings.TrimSpace(in.OriginAddress),
		payload, providerID, created)
	if err != nil {
		if isUniqueViolation(err) && providerID != "" {
			return SessionEvent{}, ErrDuplicateProviderEvent
		}
		return SessionEvent{}, fmt.Errorf("insert session event: %w", err)
	}

	lifecycleSQL := `UPDATE sessions SET last_activity_at = ?, last_origin = ?`
	args := []any{created, origin}
	if life := strings.TrimSpace(in.TouchLifecycle); life != "" {
		if !validSessionLifecycle(life) {
			return SessionEvent{}, fmt.Errorf("invalid touch lifecycle %q", life)
		}
		lifecycleSQL += `, lifecycle = ?`
		args = append(args, life)
	}
	lifecycleSQL += ` WHERE id = ?`
	args = append(args, sessionID)
	if _, err := tx.Exec(lifecycleSQL, args...); err != nil {
		return SessionEvent{}, fmt.Errorf("touch session activity: %w", err)
	}

	return SessionEvent{
		SessionID:       sessionID,
		Seq:             seq,
		EventType:       in.EventType,
		Origin:          origin,
		OriginAddress:   strings.TrimSpace(in.OriginAddress),
		PayloadJSON:     payload,
		ProviderEventID: providerID,
		CreatedAt:       created,
	}, nil
}

// CreateSessionWithFirstEvent atomically creates a session and appends the first event.
func (s *Store) CreateSessionWithFirstEvent(session CreateSessionInput, event AppendSessionEventInput) (Session, SessionEvent, error) {
	var sess Session
	var ev SessionEvent
	err := s.InTx(func(tx *sql.Tx) error {
		created, err := createSessionTx(tx, session)
		if err != nil {
			return err
		}
		event.SessionID = created.ID
		event.BoundSessionID = created.ID
		appended, err := appendSessionEventTx(tx, event)
		if err != nil {
			return err
		}
		sess = created
		// Refresh activity fields from append.
		sess.LastActivityAt = appended.CreatedAt
		sess.LastOrigin = appended.Origin
		ev = appended
		return nil
	})
	if err != nil {
		return Session{}, SessionEvent{}, err
	}
	s.log.Info("session created with first event", "kind", sess.Kind)
	return sess, ev, nil
}

// ListSessionEventsNewest returns up to limit events ending at beforeSeq (exclusive), newest-first then reversed to chronological.
// beforeSeq 0 means from the newest event. Returns chronological ascending order for Chat restore.
func (s *Store) ListSessionEventsNewest(sessionID string, beforeSeq int64, limit int) ([]SessionEvent, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, ErrSessionNotFound
	}
	if limit <= 0 {
		limit = DefaultSessionEventPage
	}
	var rows *sql.Rows
	var err error
	if beforeSeq > 0 {
		rows, err = s.db.Query(`
SELECT session_id, seq, event_type, origin, origin_address, payload_json, provider_event_id, created_at
FROM session_events
WHERE session_id = ? AND seq < ?
ORDER BY seq DESC
LIMIT ?`, sessionID, beforeSeq, limit)
	} else {
		rows, err = s.db.Query(`
SELECT session_id, seq, event_type, origin, origin_address, payload_json, provider_event_id, created_at
FROM session_events
WHERE session_id = ?
ORDER BY seq DESC
LIMIT ?`, sessionID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list session events: %w", err)
	}
	defer rows.Close()
	var newestFirst []SessionEvent
	for rows.Next() {
		ev, err := scanSessionEvent(rows)
		if err != nil {
			return nil, err
		}
		newestFirst = append(newestFirst, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Reverse to chronological ascending.
	out := make([]SessionEvent, len(newestFirst))
	for i := range newestFirst {
		out[len(newestFirst)-1-i] = newestFirst[i]
	}
	return out, nil
}

func replaceSessionEventTx(tx *sql.Tx, sessionID string, seq int64, in AppendSessionEventInput, origin, payload, providerID, created string) (SessionEvent, error) {
	if _, err := tx.Exec(`
UPDATE session_events
SET event_type = ?, origin = ?, origin_address = ?, payload_json = ?, created_at = ?
WHERE session_id = ? AND seq = ?`,
		in.EventType, origin, strings.TrimSpace(in.OriginAddress), payload, created, sessionID, seq); err != nil {
		return SessionEvent{}, fmt.Errorf("replace session event: %w", err)
	}
	lifecycleSQL := `UPDATE sessions SET last_activity_at = ?, last_origin = ?`
	args := []any{created, origin}
	if life := strings.TrimSpace(in.TouchLifecycle); life != "" {
		if !validSessionLifecycle(life) {
			return SessionEvent{}, fmt.Errorf("invalid touch lifecycle %q", life)
		}
		lifecycleSQL += `, lifecycle = ?`
		args = append(args, life)
	}
	lifecycleSQL += ` WHERE id = ?`
	args = append(args, sessionID)
	if _, err := tx.Exec(lifecycleSQL, args...); err != nil {
		return SessionEvent{}, fmt.Errorf("touch session activity: %w", err)
	}
	return SessionEvent{
		SessionID:       sessionID,
		Seq:             seq,
		EventType:       in.EventType,
		Origin:          origin,
		OriginAddress:   strings.TrimSpace(in.OriginAddress),
		PayloadJSON:     payload,
		ProviderEventID: providerID,
		CreatedAt:       created,
	}, nil
}

func getSessionEventTx(tx *sql.Tx, sessionID string, seq int64) (SessionEvent, error) {
	row := tx.QueryRow(`
SELECT session_id, seq, event_type, origin, origin_address, payload_json, provider_event_id, created_at
FROM session_events WHERE session_id = ? AND seq = ?`, sessionID, seq)
	return scanSessionEvent(row)
}

func scanSessionEvent(row sessionScanner) (SessionEvent, error) {
	var ev SessionEvent
	err := row.Scan(
		&ev.SessionID, &ev.Seq, &ev.EventType, &ev.Origin, &ev.OriginAddress,
		&ev.PayloadJSON, &ev.ProviderEventID, &ev.CreatedAt,
	)
	return ev, err
}

func validSessionEventType(t string) bool {
	switch t {
	case SessionEventUser, SessionEventAssistant, SessionEventThinking, SessionEventTool,
		SessionEventWarning, SessionEventPermission, SessionEventQuestion,
		SessionEventAttachment, SessionEventAsset, SessionEventInterruption, SessionEventNote:
		return true
	default:
		return false
	}
}

// Ensure slog import is used when debug is compiled away — keep Info path elsewhere.
var _ = slog.Default
