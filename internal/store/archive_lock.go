package store

import (
	"errors"
	"fmt"
)

// RecoverArchiveLock clears an abandoned archive claim only while the caller
// holds the project's exclusive filesystem lease. That lease is released by
// the OS on process death; a live archive cannot coexist with its new owner.
// Harness/stage locks are deliberately never cleared by this recovery path.
func (s *Store) RecoverArchiveLock(cycleID int64) error {
	_, err := s.db.Exec(`UPDATE cycles SET lock_holder = NULL, lock_at = NULL
WHERE id = ? AND lock_holder GLOB 'archive-[0-9]*-[0-9]*'`, cycleID)
	return err
}

// FinalizeArchive checks the database write before destructive cleanup. The
// status remains uncommitted until cleanup succeeds; cleanup errors roll it back.
// A crash/commit failure is retried via the persisted archive intent, without
// ever making a backup of credentials.
func (s *Store) FinalizeArchive(cycleID int64, holder string, cleanup func() error) (err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	result, err := tx.Exec(`UPDATE cycles SET status = ? WHERE id = ? AND lock_holder = ?`, CycleStatusArchived, cycleID, holder)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrBusy
	}
	if err := cleanup(); err != nil {
		return err
	}
	return tx.Commit()
}

// AcquireArchiveLock claims a cycle atomically without replacing another
// session's lock. Completed/cancelled cycles can still await archive.
func (s *Store) AcquireArchiveLock(cycleID int64, holder, at string) error {
	if holder == "" {
		return fmt.Errorf("archive lock owner is required")
	}
	result, err := s.db.Exec(`UPDATE cycles SET lock_holder = ?, lock_at = ?
WHERE id = ? AND status IN (?, ?, ?) AND (lock_holder IS NULL OR lock_holder = '')`,
		holder, at, cycleID, CycleStatusActive, CycleStatusCompleted, CycleStatusCancelled)
	if err != nil {
		return fmt.Errorf("claim archive cycle: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify archive cycle ownership: %w", err)
	}
	if count != 1 {
		return ErrBusy
	}
	return nil
}

// ReleaseArchiveLock releases only the exact operation's claim.
func (s *Store) ReleaseArchiveLock(cycleID int64, holder string) error {
	_, err := s.db.Exec(`UPDATE cycles SET lock_holder = NULL, lock_at = NULL
WHERE id = ? AND lock_holder = ?`, cycleID, holder)
	if err != nil {
		return fmt.Errorf("release archive cycle ownership: %w", err)
	}
	return nil
}
