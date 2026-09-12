package store

import (
	"database/sql"
	"encoding/json"
	"errors"
)

func appendLifecycleEventTx(tx *sql.Tx, s *Store, cycleID int64, eventType string, payload any) error {
	if s == nil || tx == nil {
		return errors.New("store transaction unavailable")
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.AppendEventTx(tx, Event{
		CycleID:     cycleID,
		Type:        eventType,
		PayloadJSON: string(b),
	})
	return err
}

func cycleNumberTx(tx *sql.Tx, cycleID int64) (int, error) {
	var n int
	err := tx.QueryRow(`SELECT number FROM cycles WHERE id = ?`, cycleID).Scan(&n)
	return n, err
}
