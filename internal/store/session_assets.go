package store

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
)

// Asset ownership kinds (schema CHECK).
const (
	AssetOwnershipManagedCopy    = "managed_copy"
	AssetOwnershipExternalSource = "external_source"
)

// Session delete-op statuses (schema CHECK / design D8).
const (
	DeleteOpIntent          = "intent"
	DeleteOpLocalPurged     = "local_purged"
	DeleteOpRemoteAttempted = "remote_attempted"
	DeleteOpCompleted       = "completed"
)

// SessionAsset is a reference to a managed or external media file (no image bytes).
type SessionAsset struct {
	SessionID    string
	AssetID      string
	Ownership    string
	Path         string
	Mime         string
	OriginalName string
	CardMetaJSON string
}

// UpsertSessionAsset persists asset metadata for a session.
func (s *Store) UpsertSessionAsset(asset SessionAsset) error {
	err := s.InTx(func(tx *sql.Tx) error {
		return upsertSessionAssetTx(tx, asset)
	})
	if err != nil {
		return err
	}
	s.log.Debug("session asset upserted", "ownership", strings.TrimSpace(asset.Ownership))
	return nil
}

func upsertSessionAssetTx(tx *sql.Tx, asset SessionAsset) error {
	asset.SessionID = strings.TrimSpace(asset.SessionID)
	asset.AssetID = strings.TrimSpace(asset.AssetID)
	if asset.SessionID == "" || asset.AssetID == "" {
		return fmt.Errorf("session asset requires session_id and asset_id")
	}
	if !validAssetOwnership(asset.Ownership) {
		return fmt.Errorf("invalid asset ownership %q", asset.Ownership)
	}
	path := strings.TrimSpace(asset.Path)
	if path == "" {
		return fmt.Errorf("session asset path is required")
	}
	meta := strings.TrimSpace(asset.CardMetaJSON)
	if meta == "" {
		meta = "{}"
	}
	if _, err := getSessionTx(tx, asset.SessionID); err != nil {
		return err
	}
	_, err := tx.Exec(`
INSERT INTO session_assets(session_id, asset_id, ownership, path, mime, original_name, card_meta_json)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(session_id, asset_id) DO UPDATE SET
  ownership = excluded.ownership,
  path = excluded.path,
  mime = excluded.mime,
  original_name = excluded.original_name,
  card_meta_json = excluded.card_meta_json`,
		asset.SessionID, asset.AssetID, asset.Ownership, path,
		strings.TrimSpace(asset.Mime), strings.TrimSpace(asset.OriginalName), meta)
	if err != nil {
		return fmt.Errorf("upsert session asset: %w", err)
	}
	return nil
}

// ListSessionAssets returns assets for a session.
func (s *Store) ListSessionAssets(sessionID string) ([]SessionAsset, error) {
	rows, err := s.db.Query(`
SELECT session_id, asset_id, ownership, path, mime, original_name, card_meta_json
FROM session_assets WHERE session_id = ? ORDER BY asset_id`, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, fmt.Errorf("list session assets: %w", err)
	}
	defer rows.Close()
	var out []SessionAsset
	for rows.Next() {
		var a SessionAsset
		if err := rows.Scan(&a.SessionID, &a.AssetID, &a.Ownership, &a.Path, &a.Mime, &a.OriginalName, &a.CardMetaJSON); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListManagedAssetPaths returns only managed_copy paths for deletion.
func (s *Store) ListManagedAssetPaths(sessionID string) ([]string, error) {
	rows, err := s.db.Query(`
SELECT path FROM session_assets WHERE session_id = ? AND ownership = ?`,
		strings.TrimSpace(sessionID), AssetOwnershipManagedCopy)
	if err != nil {
		return nil, fmt.Errorf("list managed asset paths: %w", err)
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// SessionDeleteOp tracks recoverable permanent-delete progress.
type SessionDeleteOp struct {
	ID              int64
	SessionID       string
	NativeSessionID string
	HarnessID       string
	Status          string
	RemoteWarning   string
	CreatedAt       string
	UpdatedAt       string
}

// BeginSessionDeleteOp records intent before local purge. Call while the session still exists.
func (s *Store) BeginSessionDeleteOp(sessionID string) (SessionDeleteOp, error) {
	sessionID = strings.TrimSpace(sessionID)
	var op SessionDeleteOp
	err := s.InTx(func(tx *sql.Tx) error {
		sess, err := getSessionTx(tx, sessionID)
		if err != nil {
			return err
		}
		now := nowRFC3339()
		res, err := tx.Exec(`
INSERT INTO session_delete_ops(session_id, native_session_id, harness_id, status, remote_warning, created_at, updated_at)
VALUES (?, ?, ?, ?, '', ?, ?)`,
			sessionID, sess.NativeSessionID, sess.HarnessID, DeleteOpIntent, now, now)
		if err != nil {
			return fmt.Errorf("insert delete op: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		op = SessionDeleteOp{
			ID: id, SessionID: sessionID,
			NativeSessionID: sess.NativeSessionID, HarnessID: sess.HarnessID,
			Status: DeleteOpIntent, CreatedAt: now, UpdatedAt: now,
		}
		return nil
	})
	if err != nil {
		return SessionDeleteOp{}, err
	}
	s.log.Info("session delete op started", "status", op.Status)
	return op, nil
}

// UpdateSessionDeleteOpStatus advances a recoverable delete op.
func (s *Store) UpdateSessionDeleteOpStatus(opID int64, status, remoteWarning string) error {
	if !validDeleteOpStatus(status) {
		return fmt.Errorf("invalid delete op status %q", status)
	}
	now := nowRFC3339()
	res, err := s.db.Exec(`
UPDATE session_delete_ops SET status = ?, remote_warning = ?, updated_at = ? WHERE id = ?`,
		status, strings.TrimSpace(remoteWarning), now, opID)
	if err != nil {
		return fmt.Errorf("update delete op: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("delete op %d not found", opID)
	}
	s.log.Info("session delete op updated", "status", status)
	return nil
}

// ListManagedPathsForDeleteOp returns managed_copy paths captured for a delete op (survives session cascade).
func (s *Store) ListManagedPathsForDeleteOp(opID int64) ([]string, error) {
	rows, err := s.db.Query(`
SELECT path FROM session_delete_op_managed_paths WHERE delete_op_id = ? ORDER BY path`, opID)
	if err != nil {
		return nil, fmt.Errorf("list delete op managed paths: %w", err)
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// ListIncompleteSessionDeleteOps returns ops that need retry (not completed).
func (s *Store) ListIncompleteSessionDeleteOps() ([]SessionDeleteOp, error) {
	rows, err := s.db.Query(`
SELECT id, session_id, native_session_id, harness_id, status, remote_warning, created_at, updated_at
FROM session_delete_ops
WHERE status != ?
ORDER BY id`, DeleteOpCompleted)
	if err != nil {
		return nil, fmt.Errorf("list incomplete delete ops: %w", err)
	}
	defer rows.Close()
	var out []SessionDeleteOp
	for rows.Next() {
		var op SessionDeleteOp
		if err := rows.Scan(&op.ID, &op.SessionID, &op.NativeSessionID, &op.HarnessID,
			&op.Status, &op.RemoteWarning, &op.CreatedAt, &op.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, op)
	}
	return out, rows.Err()
}

// DeleteSessionLocalFirst performs design D8 steps 1–2: intent + SQLite cascade delete.
// Caller must then purge managed files and call UpdateSessionDeleteOpStatus.
// Returns managed paths captured before the row was deleted; never includes external_source.
func (s *Store) DeleteSessionLocalFirst(sessionID string) (op SessionDeleteOp, managedPaths []string, err error) {
	sessionID = strings.TrimSpace(sessionID)
	err = s.InTx(func(tx *sql.Tx) error {
		sess, err := getSessionTx(tx, sessionID)
		if err != nil {
			return err
		}
		rows, err := tx.Query(`
SELECT path FROM session_assets WHERE session_id = ? AND ownership = ?`,
			sessionID, AssetOwnershipManagedCopy)
		if err != nil {
			return err
		}
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err != nil {
				rows.Close()
				return err
			}
			managedPaths = append(managedPaths, p)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()

		now := nowRFC3339()
		res, err := tx.Exec(`
INSERT INTO session_delete_ops(session_id, native_session_id, harness_id, status, remote_warning, created_at, updated_at)
VALUES (?, ?, ?, ?, '', ?, ?)`,
			sessionID, sess.NativeSessionID, sess.HarnessID, DeleteOpIntent, now, now)
		if err != nil {
			return fmt.Errorf("insert delete op: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		op = SessionDeleteOp{
			ID: id, SessionID: sessionID,
			NativeSessionID: sess.NativeSessionID, HarnessID: sess.HarnessID,
			Status: DeleteOpIntent, CreatedAt: now, UpdatedAt: now,
		}
		for _, p := range managedPaths {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if _, err := tx.Exec(
				`INSERT INTO session_delete_op_managed_paths(delete_op_id, path) VALUES (?, ?)`,
				op.ID, p); err != nil {
				return fmt.Errorf("insert delete op managed path: %w", err)
			}
		}
		if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID); err != nil {
			return fmt.Errorf("delete session row: %w", err)
		}
		return nil
	})
	if err != nil {
		return SessionDeleteOp{}, nil, err
	}
	s.log.Info("session local-first deleted", "status", op.Status)
	return op, managedPaths, nil
}

func validAssetOwnership(o string) bool {
	switch o {
	case AssetOwnershipManagedCopy, AssetOwnershipExternalSource:
		return true
	default:
		return false
	}
}

func validDeleteOpStatus(s string) bool {
	switch s {
	case DeleteOpIntent, DeleteOpLocalPurged, DeleteOpRemoteAttempted, DeleteOpCompleted:
		return true
	default:
		return false
	}
}

var _ = slog.Default
