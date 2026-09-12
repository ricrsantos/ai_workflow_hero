package store

import (
	"database/sql"
	"fmt"
)

// InTx runs fn inside a single SQLite transaction. The transaction is rolled back
// when fn returns an error and committed otherwise.
func (s *Store) InTx(fn func(tx *sql.Tx) error) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store is not open")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
