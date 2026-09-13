package store

import (
	"database/sql"
	"fmt"
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
func (s *Store) PersistSessionTranscriptSuffix(events []AppendSessionEventInput, assets []SessionAsset, bind *NativeSessionBind) error {
	if s == nil {
		return fmt.Errorf("store is not open")
	}
	if len(events) == 0 && len(assets) == 0 && bind == nil {
		return nil
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
				return err
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
