package store

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	ScreenshotCaptureReady   = "ready"
	ScreenshotCaptureOmitted = "omitted"
	ScreenshotCaptureFailed  = "failed"

	ScreenshotResultPassed  = "passed"
	ScreenshotResultFailed  = "failed"
	ScreenshotResultBlocked = "blocked"
	ScreenshotResultSkipped = "skipped"

	ScreenshotOmissionDisabled         = "capture_disabled"
	ScreenshotOmissionHTTPOnly         = "http_only_e2e"
	ScreenshotOmissionCredentialPhase  = "credential_phase_active"
	ScreenshotOmissionUnsafeCapability = "unsafe_capture_capability"
	ScreenshotOmissionUnavailable      = "capture_unavailable"
	ScreenshotOmissionCaptureFailed    = "capture_failed"
	ScreenshotOmissionInvalidImage     = "invalid_image"
	ScreenshotOmissionPublishFailed    = "publish_failed"
)

var (
	screenshotIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,95}$`)
	coverageIDPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	userIDPattern       = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	profileIDPattern    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,63}$`)
)

// ScreenshotManifest describes one immutable cycle screenshot or its safe
// omission. Path is relative to the cycle directory, so archive moves retain
// the same reference.
type ScreenshotManifest struct {
	ID             string
	CycleID        int64
	StageName      string
	Attempt        int
	CoverageID     string
	UserID         string
	ProfileID      string
	CapturedAt     string
	Path           string
	Result         string
	CaptureStatus  string
	OmissionReason string
}

// InsertScreenshotManifest is insert-only: a screenshot record cannot be
// replaced by a later attempt or role.
func (s *Store) InsertScreenshotManifest(ctx context.Context, manifest ScreenshotManifest) error {
	if err := validateScreenshotManifest(manifest); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO screenshot_manifests
  (screenshot_id, cycle_id, stage_name, attempt, coverage_id, user_id, profile_id,
   captured_at, path, result, capture_status, omission_reason)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		manifest.ID, manifest.CycleID, manifest.StageName, manifest.Attempt,
		manifest.CoverageID, manifest.UserID, manifest.ProfileID, manifest.CapturedAt,
		manifest.Path, manifest.Result, manifest.CaptureStatus, manifest.OmissionReason,
	)
	if err != nil {
		return fmt.Errorf("insert screenshot manifest: %w", err)
	}
	return nil
}

// ListReadyScreenshotManifests returns an ordered snapshot of published
// captures. Omitted and failed rows are never exposed as ready assets.
func (s *Store) ListReadyScreenshotManifests(ctx context.Context, cycleID int64) ([]ScreenshotManifest, error) {
	if cycleID <= 0 {
		return nil, fmt.Errorf("cycle ID must be positive")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT screenshot_id, cycle_id, stage_name, attempt, coverage_id, user_id, profile_id,
       captured_at, path, result, capture_status, omission_reason
FROM screenshot_manifests
WHERE cycle_id = ? AND capture_status = ?
ORDER BY captured_at ASC, screenshot_id ASC`, cycleID, ScreenshotCaptureReady)
	if err != nil {
		return nil, fmt.Errorf("list ready screenshot manifests: %w", err)
	}
	defer func() { _ = rows.Close() }()

	manifests := make([]ScreenshotManifest, 0)
	for rows.Next() {
		var manifest ScreenshotManifest
		if err := rows.Scan(
			&manifest.ID, &manifest.CycleID, &manifest.StageName, &manifest.Attempt,
			&manifest.CoverageID, &manifest.UserID, &manifest.ProfileID,
			&manifest.CapturedAt, &manifest.Path, &manifest.Result,
			&manifest.CaptureStatus, &manifest.OmissionReason,
		); err != nil {
			return nil, fmt.Errorf("scan ready screenshot manifest: %w", err)
		}
		manifests = append(manifests, manifest)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ready screenshot manifests: %w", err)
	}
	return manifests, nil
}

func validateScreenshotManifest(manifest ScreenshotManifest) error {
	if !screenshotIDPattern.MatchString(manifest.ID) {
		return fmt.Errorf("invalid screenshot ID")
	}
	if manifest.CycleID <= 0 {
		return fmt.Errorf("cycle ID must be positive")
	}
	if manifest.StageName != "browser_ui_validation" && manifest.StageName != "qa_end_to_end" {
		return fmt.Errorf("invalid screenshot stage")
	}
	if manifest.Attempt <= 0 {
		return fmt.Errorf("screenshot attempt must be positive")
	}
	if manifest.CoverageID != "" && !coverageIDPattern.MatchString(manifest.CoverageID) {
		return fmt.Errorf("invalid screenshot coverage ID")
	}
	if _, err := time.Parse(time.RFC3339Nano, manifest.CapturedAt); err != nil {
		return fmt.Errorf("screenshot timestamp must be RFC3339")
	}
	if manifest.UserID == "" != (manifest.ProfileID == "") {
		return fmt.Errorf("screenshot user and profile must be supplied together")
	}
	if manifest.UserID != "" && (!userIDPattern.MatchString(manifest.UserID) || !profileIDPattern.MatchString(manifest.ProfileID)) {
		return fmt.Errorf("invalid screenshot user or profile ID")
	}
	switch manifest.Result {
	case ScreenshotResultPassed, ScreenshotResultFailed, ScreenshotResultBlocked, ScreenshotResultSkipped:
	default:
		return fmt.Errorf("invalid screenshot result")
	}
	switch manifest.CaptureStatus {
	case ScreenshotCaptureReady:
		if !safeScreenshotPath(manifest.ID, manifest.Path) || manifest.OmissionReason != "" {
			return fmt.Errorf("ready screenshot requires a safe path and no omission reason")
		}
	case ScreenshotCaptureOmitted, ScreenshotCaptureFailed:
		if manifest.Path != "" || !knownScreenshotOmission(manifest.OmissionReason) {
			return fmt.Errorf("omitted screenshot requires a safe omission reason and no path")
		}
	default:
		return fmt.Errorf("invalid screenshot capture status")
	}
	return nil
}

func safeScreenshotPath(id, path string) bool {
	if !strings.HasPrefix(path, "screenshots/") || strings.Contains(path, `\`) {
		return false
	}
	name := strings.TrimPrefix(path, "screenshots/")
	if name == "" || filepath.Base(name) != name || strings.Contains(name, "/") || !strings.HasPrefix(name, id+".") {
		return false
	}
	switch filepath.Ext(name) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	default:
		return false
	}
}

func knownScreenshotOmission(reason string) bool {
	switch reason {
	case ScreenshotOmissionDisabled,
		ScreenshotOmissionHTTPOnly,
		ScreenshotOmissionCredentialPhase,
		ScreenshotOmissionUnsafeCapability,
		ScreenshotOmissionUnavailable,
		ScreenshotOmissionCaptureFailed,
		ScreenshotOmissionInvalidImage,
		ScreenshotOmissionPublishFailed:
		return true
	default:
		return false
	}
}
