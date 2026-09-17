package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// NativeSessionBind is the native harness identity written with a transcript suffix.
type NativeSessionBind struct {
	SessionID           string
	HarnessID           string
	NativeSessionID     string
	Model               string
	ModelPropertiesJSON string
}

// PersistSessionTranscriptSuffix writes events, assets, and optional native binding
// in one SQLite transaction so a later failure cannot leave a partial suffix.
// A native bind that collides with a different Hero session never fails the
// transcript: events/assets are still committed and the conflicting bind is
// skipped (the Chat transcript is shared across stage turns while each native
// session stays owned by a single History row). Same-session rebinds remain
// idempotent.
func (s *Store) PersistSessionTranscriptSuffix(events []AppendSessionEventInput, assets []SessionAsset, bind *NativeSessionBind) error {
	if s == nil {
		return fmt.Errorf("store is not open")
	}
	if len(events) == 0 && len(assets) == 0 && bind == nil {
		return nil
	}
	if bind != nil && isDuplicateBindForOtherSession(s, bind) {
		s.log.Warn("skipping native bind owned by another session",
			"session_id", strings.TrimSpace(bind.SessionID),
			"harness_id", strings.TrimSpace(bind.HarnessID),
			"native_session_id", strings.TrimSpace(bind.NativeSessionID))
		bind = nil
		if len(events) == 0 && len(assets) == 0 {
			return nil
		}
	}
	err := s.InTx(func(tx *sql.Tx) error {
		for i, ev := range events {
			if _, err := appendSessionEventTx(tx, ev); err != nil {
				return fmt.Errorf("persist suffix event %d: %w", i, err)
			}
		}
		for i, asset := range assets {
			if err := upsertSessionAssetTx(tx, asset); err != nil {
				return fmt.Errorf("persist suffix asset %d: %w", i, err)
			}
		}
		if bind != nil {
			if err := bindNativeSessionTx(tx, bind.SessionID, bind.HarnessID, bind.NativeSessionID, bind.Model, bind.ModelPropertiesJSON); err != nil {
				if errors.Is(err, ErrDuplicateNativeSession) {
					// Raced with another writer between the pre-check and the
					// tx: the transcript is already staged above, so commit it
					// and drop only the conflicting bind.
					s.log.Warn("skipping raced native bind owned by another session",
						"session_id", strings.TrimSpace(bind.SessionID),
						"harness_id", strings.TrimSpace(bind.HarnessID),
						"native_session_id", strings.TrimSpace(bind.NativeSessionID))
					return nil
				}
				return fmt.Errorf("persist suffix bind hero=%s harness=%s native=%s: %w",
					strings.TrimSpace(bind.SessionID), strings.TrimSpace(bind.HarnessID),
					strings.TrimSpace(bind.NativeSessionID), err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.log.Debug("session transcript suffix persisted", "events", len(events), "assets", len(assets))
	return nil
}

// isDuplicateBindForOtherSession reports whether (harness, native) is already
// owned by a different Hero session. Same-session rebinds are idempotent and
// return false.
func isDuplicateBindForOtherSession(s *Store, bind *NativeSessionBind) bool {
	if s == nil || s.db == nil || bind == nil {
		return false
	}
	harnessID := strings.TrimSpace(bind.HarnessID)
	nativeID := strings.TrimSpace(bind.NativeSessionID)
	heroID := strings.TrimSpace(bind.SessionID)
	if harnessID == "" || nativeID == "" || heroID == "" {
		return false
	}
	var owner string
	err := s.db.QueryRow(`
SELECT id FROM sessions WHERE harness_id = ? AND native_session_id = ? LIMIT 1`,
		harnessID, nativeID).Scan(&owner)
	if err != nil {
		return false
	}
	return strings.TrimSpace(owner) != heroID
}

// CreateSessionWithTranscript creates a session and writes its initial events and
// assets in one transaction so first-turn attachments cannot land partially.
func (s *Store) CreateSessionWithTranscript(session CreateSessionInput, events []AppendSessionEventInput, assets []SessionAsset) (Session, error) {
	if s == nil {
		return Session{}, fmt.Errorf("store is not open")
	}
	if len(events) == 0 {
		return Session{}, fmt.Errorf("first-turn transcript requires at least one event")
	}
	var sess Session
	err := s.InTx(func(tx *sql.Tx) error {
		created, err := createSessionTx(tx, session)
		if err != nil {
			return err
		}
		for i, ev := range events {
			ev.SessionID = created.ID
			ev.BoundSessionID = created.ID
			appended, err := appendSessionEventTx(tx, ev)
			if err != nil {
				return fmt.Errorf("first-turn event %d: %w", i, err)
			}
			created.LastActivityAt = appended.CreatedAt
			created.LastOrigin = appended.Origin
		}
		for i, asset := range assets {
			asset.SessionID = created.ID
			if err := upsertSessionAssetTx(tx, asset); err != nil {
				return fmt.Errorf("first-turn asset %d: %w", i, err)
			}
		}
		sess = created
		return nil
	})
	if err != nil {
		return Session{}, err
	}
	s.log.Info("session created with transcript", "kind", sess.Kind, "events", len(events), "assets", len(assets))
	return sess, nil
}
