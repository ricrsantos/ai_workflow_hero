package daemon

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Status is the durable delivery state of a pending message
// (pending → delivered → processed / cancelled / expired; ADR-063).
type Status string

const (
	StatusPending   Status = "pending"
	StatusDelivered Status = "delivered"
	StatusProcessed Status = "processed"
	StatusCancelled Status = "cancelled"
	StatusExpired   Status = "expired"
)

// PendingTTL is how long an undelivered message is retained (24 hours).
const PendingTTL = 24 * time.Hour

// PendingMessage is a durable queued inbound message (no secrets).
type PendingMessage struct {
	ID        int64
	Address   string
	Text      string
	IsCommand bool
	UpdateID  int64
	CreatedAt time.Time
	Status    Status
}

// Store is the daemon's private SQLite store. It holds only non-sensitive
// configuration, update de-duplication, and queue/audit data (ADR-062).
type Store struct {
	db *sql.DB
}

// OpenStore opens (or creates) the daemon store at path.
func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("daemon store mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("daemon store open: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS telegram_updates (
  update_id INTEGER PRIMARY KEY,
  processed_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS telegram_pending (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  address TEXT NOT NULL,
  text TEXT NOT NULL,
  is_command INTEGER NOT NULL DEFAULT 0,
  update_id INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending'
);
CREATE INDEX IF NOT EXISTS idx_telegram_pending_addr ON telegram_pending(address, status);
CREATE TABLE IF NOT EXISTS telegram_addresses (
  address TEXT PRIMARY KEY,
  mode TEXT NOT NULL,
  abbrev TEXT NOT NULL,
  last_seen TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS telegram_selection (
  singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
  address TEXT NOT NULL
);
`)
	return err
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// MarkUpdateProcessed records a processed update id (idempotent dedup; ADR-063).
func (s *Store) MarkUpdateProcessed(updateID int64, now time.Time) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO telegram_updates(update_id, processed_at) VALUES(?, ?)",
		updateID, now.UTC().Format(time.RFC3339),
	)
	return err
}

// MarkAddressKnown records a configured instance address so the daemon can
// distinguish "known but offline" (queue) from "unknown" (generic reject).
func (s *Store) MarkAddressKnown(address, mode, abbrev string, now time.Time) error {
	_, err := s.db.Exec(
		"INSERT INTO telegram_addresses(address, mode, abbrev, last_seen) VALUES(?, ?, ?, ?) ON CONFLICT(address) DO UPDATE SET last_seen = excluded.last_seen",
		address, mode, abbrev, now.UTC().Format(time.RFC3339),
	)
	return err
}

// AddressKnown reports whether address was ever registered.
func (s *Store) AddressKnown(address string) (bool, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM telegram_addresses WHERE address = ?", address).Scan(&n)
	return n > 0, err
}

// SetSelectedAddress persists the authorized chat's selected live instance.
// The daemon supports one authorized chat, so no chat identifier is stored in
// SQLite (ADR-062).
func (s *Store) SetSelectedAddress(address string) error {
	_, err := s.db.Exec(
		"INSERT INTO telegram_selection(singleton, address) VALUES(1, ?) ON CONFLICT(singleton) DO UPDATE SET address = excluded.address",
		address,
	)
	return err
}

// SelectedAddress returns the persisted selected instance address. An empty
// result means the authorized chat has not selected an instance yet.
func (s *Store) SelectedAddress() (string, error) {
	var address string
	err := s.db.QueryRow("SELECT address FROM telegram_selection WHERE singleton = 1").Scan(&address)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return address, err
}

// ClearSelectedAddress removes the selected instance, for example when the
// authorized chat is replaced or credentials are cleared.
func (s *Store) ClearSelectedAddress() error {
	_, err := s.db.Exec("DELETE FROM telegram_selection WHERE singleton = 1")
	return err
}

// UpdateProcessed reports whether updateID was already processed.
func (s *Store) UpdateProcessed(updateID int64) (bool, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM telegram_updates WHERE update_id = ?", updateID).Scan(&n)
	return n > 0, err
}

// EnqueuePending persists a pending message.
func (s *Store) EnqueuePending(m PendingMessage) error {
	_, err := s.db.Exec(
		"INSERT INTO telegram_pending(address, text, is_command, update_id, created_at, status) VALUES(?, ?, ?, ?, ?, ?)",
		m.Address, m.Text, boolToInt(m.IsCommand), m.UpdateID,
		m.CreatedAt.UTC().Format(time.RFC3339), string(StatusPending),
	)
	return err
}

// PendingForAddress returns undelivered pending messages for address, oldest first.
func (s *Store) PendingForAddress(address string) ([]PendingMessage, error) {
	rows, err := s.db.Query(
		"SELECT id, address, text, is_command, update_id, created_at, status FROM telegram_pending WHERE address = ? AND status = ? ORDER BY id ASC",
		address, string(StatusPending),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingMessage
	for rows.Next() {
		var m PendingMessage
		var created string
		var cmd int
		if err := rows.Scan(&m.ID, &m.Address, &m.Text, &cmd, &m.UpdateID, &created, &m.Status); err != nil {
			return nil, err
		}
		m.IsCommand = cmd != 0
		if t, err := time.Parse(time.RFC3339, created); err == nil {
			m.CreatedAt = t
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Transition updates a pending message status.
func (s *Store) Transition(id int64, to Status) error {
	_, err := s.db.Exec("UPDATE telegram_pending SET status = ? WHERE id = ?", string(to), id)
	return err
}

// CancelPendingForAddress cancels all still-pending messages for address and
// returns how many were cancelled (ADR-063). It never touches delivered rows.
func (s *Store) CancelPendingForAddress(address string) (int64, error) {
	res, err := s.db.Exec(
		"UPDATE telegram_pending SET status = ? WHERE address = ? AND status = ?",
		string(StatusCancelled), address, string(StatusPending),
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ExpirePending expires pending messages older than TTL and returns the count.
func (s *Store) ExpirePending(now time.Time) (int64, error) {
	cutoff := now.Add(-PendingTTL).UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		"UPDATE telegram_pending SET status = ? WHERE status = ? AND created_at < ?",
		string(StatusExpired), string(StatusPending), cutoff,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
