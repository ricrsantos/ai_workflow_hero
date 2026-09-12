package store

import (
	"database/sql"
	"fmt"
)

// AppendEventTx inserts an append-only event inside an open transaction.
func (s *Store) AppendEventTx(tx *sql.Tx, e Event) (int64, error) {
	ts := e.TS
	if ts == "" {
		ts = nowRFC3339()
	}
	payload := e.PayloadJSON
	if payload == "" {
		payload = "{}"
	}
	res, err := tx.Exec(
		`INSERT INTO events(cycle_id, ts, type, payload_json) VALUES(?, ?, ?, ?)`,
		e.CycleID, ts, e.Type, payload,
	)
	if err != nil {
		return 0, fmt.Errorf("append event: %w", err)
	}
	return res.LastInsertId()
}

// AddConversationTx inserts a conversation entry inside an open transaction.
func (s *Store) AddConversationTx(tx *sql.Tx, c ConversationEntry) (int64, error) {
	ts := c.TS
	if ts == "" {
		ts = nowRFC3339()
	}
	res, err := tx.Exec(
		`INSERT INTO conversation(cycle_id, ts, role, kind, body) VALUES(?, ?, ?, ?, ?)`,
		c.CycleID, ts, c.Role, c.Kind, c.Body,
	)
	if err != nil {
		return 0, fmt.Errorf("insert conversation: %w", err)
	}
	return res.LastInsertId()
}
