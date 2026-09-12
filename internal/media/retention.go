package media

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CleanupOptions controls expiration cleanup for session directories.
type CleanupOptions struct {
	DataHome  string
	Retention time.Duration
	Now       func() time.Time
	Logger    *slog.Logger
	// Registered lists Hero session IDs that must survive age-based cleanup.
	// When ListRegistered is set it is called once per cleanup run (after Registered).
	Registered     RegisteredSessionIDs
	ListRegistered func() (RegisteredSessionIDs, error)
}

// CleanupResult reports cleanup counts without returning session paths.
type CleanupResult struct {
	ScannedSessions  int
	RemovedSessions  int
	RetainedSessions int
}

// CleanupExpiredSessions removes unregistered session directories older than
// Retention. Registered Hero session IDs are never age-deleted. It never
// follows a symlink at the session-directory level and reports only counts, so
// callers cannot accidentally log sensitive asset paths. User external_source
// paths are not referenced by this cleanup.
func CleanupExpiredSessions(ctx context.Context, options CleanupOptions) (CleanupResult, error) {
	ctx = nonNilContext(ctx)
	if err := ctx.Err(); err != nil {
		return CleanupResult{}, err
	}
	retention := options.Retention
	if retention == 0 {
		retention = DefaultRetention
	}
	if retention < 0 {
		return CleanupResult{}, fmt.Errorf("cleaning media sessions: retention must not be negative")
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	dataHome, err := resolveDataHome(options.DataHome)
	if err != nil {
		return CleanupResult{}, err
	}
	sessionsRoot := filepath.Join(dataHome, "hero", "sessions")
	rootInfo, err := os.Lstat(sessionsRoot)
	if errors.Is(err, os.ErrNotExist) {
		return CleanupResult{}, nil
	}
	if err != nil {
		return CleanupResult{}, wrapFilesystemError("checking media sessions", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return CleanupResult{}, fmt.Errorf("media sessions root is not a directory")
	}
	if err := os.Chmod(sessionsRoot, 0o700); err != nil {
		return CleanupResult{}, wrapFilesystemError("securing media sessions", err)
	}
	entries, err := os.ReadDir(sessionsRoot)
	if errors.Is(err, os.ErrNotExist) {
		return CleanupResult{}, nil
	}
	if err != nil {
		return CleanupResult{}, wrapFilesystemError("reading media sessions", err)
	}

	registered := options.Registered
	if options.ListRegistered != nil {
		listed, err := options.ListRegistered()
		if err != nil {
			return CleanupResult{}, fmt.Errorf("listing registered media sessions: %w", err)
		}
		if len(listed) > 0 {
			if len(registered) == 0 {
				registered = listed
			} else {
				for id := range listed {
					registered[id] = struct{}{}
				}
			}
		}
	}

	cutoff := now().Add(-retention)
	var result CleanupResult
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			continue
		}
		result.ScannedSessions++
		sessionPath := filepath.Join(sessionsRoot, entry.Name())
		info, err := os.Lstat(sessionPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return result, wrapFilesystemError("statting media session", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			continue
		}
		if !isWithinDirectory(sessionsRoot, sessionPath) {
			continue
		}
		if registered.Contains(entry.Name()) {
			result.RetainedSessions++
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.RemoveAll(sessionPath); err != nil {
				return result, wrapFilesystemError("removing expired media session", err)
			}
			result.RemovedSessions++
			continue
		}
		result.RetainedSessions++
	}
	if options.Logger != nil {
		options.Logger.Debug("media session cleanup complete",
			"scanned_sessions", result.ScannedSessions,
			"removed_sessions", result.RemovedSessions,
			"retained_sessions", result.RetainedSessions,
		)
	}
	return result, nil
}

// CleanupExpired runs retention cleanup using this store's data home and
// configured retention period.
func (s *Store) CleanupExpired(ctx context.Context) (CleanupResult, error) {
	return CleanupExpiredSessions(ctx, CleanupOptions{
		DataHome:       s.dataHome,
		Retention:      s.retention,
		Now:            s.now,
		Logger:         s.logger,
		Registered:     s.registeredSessionIDs,
		ListRegistered: s.listRegistered,
	})
}

// StartupCleanup is the startup lifecycle hook for retention cleanup.
func (s *Store) StartupCleanup(ctx context.Context) (CleanupResult, error) {
	return s.CleanupExpired(ctx)
}

// ShutdownCleanup is the graceful-shutdown lifecycle hook for retention
// cleanup. It intentionally retains the active session unless it has already
// exceeded the configured retention period.
func (s *Store) ShutdownCleanup(ctx context.Context) (CleanupResult, error) {
	return s.CleanupExpired(ctx)
}

// Close is a convenience lifecycle hook for callers that treat the store as a
// session resource. It performs retention cleanup and does not remove a
// current, non-expired session.
func (s *Store) Close(ctx context.Context) error {
	_, err := s.ShutdownCleanup(ctx)
	return err
}

// SessionRoot returns the XDG-compliant root used for all session stores.
func SessionRoot(dataHome string) (string, error) {
	resolved, err := resolveDataHome(dataHome)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, "hero", "sessions"), nil
}

// isWithinDirectory is kept here for cleanup's defensive path checks and is
// intentionally independent from manifest path handling.
func isWithinDirectory(root, child string) bool {
	relative, err := filepath.Rel(root, child)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) && !filepath.IsAbs(relative)
}
