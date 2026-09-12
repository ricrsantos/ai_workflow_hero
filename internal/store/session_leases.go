package store

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// LeaseHeartbeatInterval is how often the owning TUI should heartbeat.
const LeaseHeartbeatInterval = 5 * time.Second

// LeaseTTL is how long a lease remains valid after the last heartbeat.
const LeaseTTL = 30 * time.Second

// SessionLease is the exclusive continuation lock for a Hero session.
type SessionLease struct {
	SessionID   string
	OwnerID     string
	AcquiredAt  string
	HeartbeatAt string
	ExpiresAt   string
}

// AcquireLeaseResult describes acquire outcomes.
type AcquireLeaseResult struct {
	Lease SessionLease
	// TookOver is true when a stale lease was replaced.
	TookOver bool
}

// AcquireSessionLease obtains an exclusive lease for ownerID using clock for TTL math.
// Live foreign owners return ErrSessionBusy. Stale leases are compare-and-swapped.
func (s *Store) AcquireSessionLease(sessionID, ownerID string, clock Clock) (AcquireLeaseResult, error) {
	if clock == nil {
		clock = DefaultClock()
	}
	sessionID = strings.TrimSpace(sessionID)
	ownerID = strings.TrimSpace(ownerID)
	if sessionID == "" {
		return AcquireLeaseResult{}, ErrSessionNotFound
	}
	if ownerID == "" {
		return AcquireLeaseResult{}, fmt.Errorf("lease owner id is required")
	}
	now := clock.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	expires := now.Add(LeaseTTL).Format(time.RFC3339)

	var result AcquireLeaseResult
	err := s.InTx(func(tx *sql.Tx) error {
		if _, err := getSessionTx(tx, sessionID); err != nil {
			return err
		}
		var existing SessionLease
		err := tx.QueryRow(`
SELECT session_id, owner_id, acquired_at, heartbeat_at, expires_at
FROM session_leases WHERE session_id = ?`, sessionID).Scan(
			&existing.SessionID, &existing.OwnerID, &existing.AcquiredAt, &existing.HeartbeatAt, &existing.ExpiresAt,
		)
		switch {
		case err == sql.ErrNoRows:
			_, err = tx.Exec(`
INSERT INTO session_leases(session_id, owner_id, acquired_at, heartbeat_at, expires_at)
VALUES (?, ?, ?, ?, ?)`, sessionID, ownerID, nowStr, nowStr, expires)
			if err != nil {
				return fmt.Errorf("insert session lease: %w", err)
			}
			result.Lease = SessionLease{
				SessionID: sessionID, OwnerID: ownerID,
				AcquiredAt: nowStr, HeartbeatAt: nowStr, ExpiresAt: expires,
			}
			return nil
		case err != nil:
			return fmt.Errorf("read session lease: %w", err)
		}

		if existing.OwnerID == ownerID {
			_, err = tx.Exec(`
UPDATE session_leases SET heartbeat_at = ?, expires_at = ? WHERE session_id = ? AND owner_id = ?`,
				nowStr, expires, sessionID, ownerID)
			if err != nil {
				return fmt.Errorf("refresh own lease: %w", err)
			}
			existing.HeartbeatAt = nowStr
			existing.ExpiresAt = expires
			result.Lease = existing
			return nil
		}

		expAt, parseErr := time.Parse(time.RFC3339, existing.ExpiresAt)
		if parseErr != nil {
			return fmt.Errorf("parse lease expires_at: %w", parseErr)
		}
		if expAt.After(now) {
			return ErrSessionBusy
		}

		// Stale compare-and-swap: only replace if expires_at still matches.
		res, err := tx.Exec(`
UPDATE session_leases
SET owner_id = ?, acquired_at = ?, heartbeat_at = ?, expires_at = ?
WHERE session_id = ? AND owner_id = ? AND expires_at = ?`,
			ownerID, nowStr, nowStr, expires,
			sessionID, existing.OwnerID, existing.ExpiresAt)
		if err != nil {
			return fmt.Errorf("take over stale lease: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrSessionBusy
		}
		result.TookOver = true
		result.Lease = SessionLease{
			SessionID: sessionID, OwnerID: ownerID,
			AcquiredAt: nowStr, HeartbeatAt: nowStr, ExpiresAt: expires,
		}
		return nil
	})
	if err != nil {
		return AcquireLeaseResult{}, err
	}
	if result.TookOver {
		s.log.Info("session lease taken over")
	} else {
		s.log.Debug("session lease acquired")
	}
	return result, nil
}

// HeartbeatSessionLease extends TTL for the owning instance.
func (s *Store) HeartbeatSessionLease(sessionID, ownerID string, clock Clock) (SessionLease, error) {
	if clock == nil {
		clock = DefaultClock()
	}
	sessionID = strings.TrimSpace(sessionID)
	ownerID = strings.TrimSpace(ownerID)
	now := clock.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	expires := now.Add(LeaseTTL).Format(time.RFC3339)

	res, err := s.db.Exec(`
UPDATE session_leases SET heartbeat_at = ?, expires_at = ?
WHERE session_id = ? AND owner_id = ?`, nowStr, expires, sessionID, ownerID)
	if err != nil {
		return SessionLease{}, fmt.Errorf("heartbeat session lease: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return SessionLease{}, ErrSessionBusy
	}
	return SessionLease{
		SessionID: sessionID, OwnerID: ownerID,
		HeartbeatAt: nowStr, ExpiresAt: expires,
	}, nil
}

// ReleaseSessionLease drops the lease when owned by ownerID.
func (s *Store) ReleaseSessionLease(sessionID, ownerID string) error {
	sessionID = strings.TrimSpace(sessionID)
	ownerID = strings.TrimSpace(ownerID)
	res, err := s.db.Exec(`DELETE FROM session_leases WHERE session_id = ? AND owner_id = ?`, sessionID, ownerID)
	if err != nil {
		return fmt.Errorf("release session lease: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		s.log.Debug("session lease release no-op")
		return nil
	}
	s.log.Info("session lease released")
	return nil
}

// GetSessionLease returns the current lease if any.
func (s *Store) GetSessionLease(sessionID string) (SessionLease, bool, error) {
	var lease SessionLease
	err := s.db.QueryRow(`
SELECT session_id, owner_id, acquired_at, heartbeat_at, expires_at
FROM session_leases WHERE session_id = ?`, strings.TrimSpace(sessionID)).Scan(
		&lease.SessionID, &lease.OwnerID, &lease.AcquiredAt, &lease.HeartbeatAt, &lease.ExpiresAt,
	)
	if err == sql.ErrNoRows {
		return SessionLease{}, false, nil
	}
	if err != nil {
		return SessionLease{}, false, fmt.Errorf("get session lease: %w", err)
	}
	return lease, true, nil
}

var _ = slog.Default
