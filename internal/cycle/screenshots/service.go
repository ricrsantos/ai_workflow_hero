// Package screenshots stores cycle-owned browser captures as immutable files
// with ready manifests. Browser adapters remain responsible for safe capture.
package screenshots

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/media"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/testaccess"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

const (
	stageBrowserUI = "browser_ui_validation"
	stageE2E       = "qa_end_to_end"
	currentDir     = ".workflow-hero/cycles/current"
	screensDir     = "screenshots"
)

var (
	ErrInvalidCaptureRequest = errors.New("invalid screenshot capture request")
	ErrUnsafeProjectPath     = errors.New("screenshot project path is unsafe")
	ErrCycleOwnership        = errors.New("screenshot request does not belong to the active cycle and stage")
	ErrReadyAssetUnsafe      = errors.New("ready screenshot asset is missing or unsafe")
	metadataPattern          = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	profilePattern           = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,63}$`)
)

// CapturePolicy is always passed with the safety flags enabled. CaptureSource
// implementations must enforce these requirements in the browser executor.
type CapturePolicy struct {
	MaskSensitiveFieldsAndTokens                    bool
	MaskSelectors                                   []string
	SuppressScreenshotsDuringCredentialSubmission   bool
	SuppressTracesDuringCredentialSubmission        bool
	SuppressVideoDuringCredentialSubmission         bool
	SuppressSnapshotsDuringCredentialSubmission     bool
	SuppressRawLoginResponsesDuringCredentialSubmit bool
}

// CaptureReceipt is the browser executor's attestation for one stable,
// post-assertion capture. Images are discarded unless every safety property
// is confirmed.
type CaptureReceipt struct {
	StablePointVerified               bool
	SensitiveFieldsAndTokensMasked    bool
	CredentialFlowArtifactsSuppressed bool
	OmissionReason                    string
}

// CaptureSource is the trusted browser-side capability used to render one
// current page to the supplied bounded writer. It must not save or return
// traces, video, snapshots, login responses, or credential-bearing metadata.
type CaptureSource interface {
	CaptureScreenshot(context.Context, CapturePolicy, io.Writer) (CaptureReceipt, error)
}

// CredentialArtifactController is implemented by the trusted browser runtime.
// Suppression must switch screenshots, traces, video, snapshots, and raw login
// response capture off before returning true and keep them off through
// credential fill/submit. It must not persist browser auth state.
type CredentialArtifactController interface {
	SetCredentialArtifactSuppression(context.Context, bool) error
}

// Request describes one post-assertion capture. It contains identifiers and
// outcomes only; it has no credential fields.
type Request struct {
	CycleID       int64
	StageName     string
	Attempt       int
	CoverageID    string
	UserID        string
	ProfileID     string
	Result        string
	MaskSelectors []string
}

// OutcomeState distinguishes a ready image from an optional warning or a
// mandatory-evidence blocker.
type OutcomeState string

const (
	OutcomeReady    OutcomeState = "ready"
	OutcomeDisabled OutcomeState = "disabled"
	OutcomeWarning  OutcomeState = "warning"
	OutcomeBlocked  OutcomeState = "blocked"
)

// Outcome contains only safe, user-displayable metadata and a manifest.
type Outcome struct {
	State    OutcomeState
	Message  string
	Manifest *store.ScreenshotManifest
}

// Options supplies deterministic sources and an optional logger. Zero values
// use the wall clock, crypto-random screenshot IDs, and slog.Default().
type Options struct {
	Now    func() time.Time
	NewID  func() (string, error)
	Logger *slog.Logger
}

// Service publishes captures under the current cycle directory and records
// their safe, cycle-owned metadata in SQLite.
type Service struct {
	projectDir       string
	store            *store.Store
	log              *slog.Logger
	now              func() time.Time
	newID            func() (string, error)
	guard            sync.RWMutex
	controllerFailed atomic.Bool
}

// NewService creates a screenshot service bound to one real Hero project root.
func NewService(projectDir string, st *store.Store, options ...Options) (*Service, error) {
	if st == nil || len(options) > 1 {
		return nil, errors.New("screenshot service requires one project store and at most one options value")
	}
	if !filepath.IsAbs(projectDir) || filepath.Clean(projectDir) != projectDir || projectDir == string(filepath.Separator) {
		return nil, ErrUnsafeProjectPath
	}
	info, err := os.Lstat(projectDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrUnsafeProjectPath
	}
	canonical, err := filepath.EvalSymlinks(projectDir)
	if err != nil || canonical != projectDir {
		return nil, ErrUnsafeProjectPath
	}
	heroInfo, err := os.Lstat(filepath.Join(projectDir, ".workflow-hero"))
	if err != nil || !heroInfo.IsDir() || heroInfo.Mode()&os.ModeSymlink != 0 {
		return nil, ErrUnsafeProjectPath
	}

	var opts Options
	if len(options) == 1 {
		opts = options[0]
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.NewID == nil {
		opts.NewID = randomID
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Service{projectDir: projectDir, store: st, log: opts.Logger, now: opts.Now, newID: opts.NewID}, nil
}

// BeginCredentialEntry asks the trusted browser runtime to suppress every
// credential-bearing artifact channel before the caller may fill or submit
// credentials. If resuming capture fails, later screenshots stay fail-closed.
func (s *Service) BeginCredentialEntry(ctx context.Context, controller CredentialArtifactController) (func() error, error) {
	if s == nil || ctx == nil || controller == nil {
		return nil, errors.New("trusted credential artifact controller is required")
	}
	s.guard.Lock()
	if err := ctx.Err(); err != nil {
		s.guard.Unlock()
		return nil, err
	}
	if err := controller.SetCredentialArtifactSuppression(ctx, true); err != nil {
		s.controllerFailed.Store(true)
		s.log.Error("credential artifact capture suspension failed")
		s.guard.Unlock()
		return nil, errors.New("could not safely suspend credential artifact capture")
	}
	s.log.Debug("credential artifact capture suspended")
	var once sync.Once
	var releaseErr error
	return func() error {
		once.Do(func() {
			resumeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := controller.SetCredentialArtifactSuppression(resumeCtx, false); err != nil {
				s.controllerFailed.Store(true)
				s.log.Error("credential artifact capture resume failed")
				releaseErr = errors.New("credential artifact capture could not be safely resumed")
			} else {
				s.log.Debug("credential artifact capture resumed")
			}
			s.guard.Unlock()
		})
		return releaseErr
	}, nil
}

// Capture records an optional communication screenshot. Its per-stage opt-in
// controls only this extra capture and never disables planned evidence.
func (s *Service) Capture(ctx context.Context, request Request, source CaptureSource) (Outcome, error) {
	outcome, err := s.capture(ctx, request, source, false)
	s.logCaptureOutcome(request, outcome, err)
	return outcome, err
}

// CapturePlannedEvidence records a separately planned mandatory evidence
// screenshot. It bypasses screenshots.enabled; missing safe evidence is
// persisted as a blocker for the planned coverage item.
func (s *Service) CapturePlannedEvidence(ctx context.Context, request Request, source CaptureSource) (Outcome, error) {
	outcome, err := s.capture(ctx, request, source, true)
	s.logCaptureOutcome(request, outcome, err)
	return outcome, err
}

func (s *Service) logCaptureOutcome(request Request, outcome Outcome, err error) {
	if s == nil || s.log == nil {
		return
	}
	if err != nil {
		s.log.Error("screenshot capture request failed", "error_code", captureErrorCode(err))
		return
	}

	fields := []any{"cycle_id", request.CycleID, "stage_name", request.StageName, "attempt", request.Attempt}
	if request.CoverageID != "" {
		fields = append(fields, "coverage_id", request.CoverageID)
	}
	reason := store.ScreenshotOmissionUnavailable
	if outcome.Manifest != nil {
		reason = outcome.Manifest.OmissionReason
	}
	switch outcome.State {
	case OutcomeReady:
		if outcome.Manifest != nil {
			fields = append(fields,
				"screenshot_id", outcome.Manifest.ID,
				"result", outcome.Manifest.Result,
			)
		}
		s.log.Info("screenshot capture published", fields...)
	case OutcomeDisabled:
		s.log.Debug("optional screenshot capture disabled", fields...)
	case OutcomeWarning:
		s.log.Debug("optional screenshot capture omitted", append(fields, "reason", reason)...)
	case OutcomeBlocked:
		s.log.Error("mandatory screenshot evidence unavailable", append(fields, "reason", reason)...)
	default:
		s.log.Error("screenshot capture returned an invalid outcome", append(fields, "error_code", "invalid_outcome")...)
	}
}

func captureErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrInvalidCaptureRequest):
		return "invalid_request"
	case errors.Is(err, ErrCycleOwnership):
		return "cycle_ownership"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	default:
		return "capture_request_failed"
	}
}

func (s *Service) capture(ctx context.Context, request Request, source CaptureSource, mandatory bool) (Outcome, error) {
	if s == nil || s.store == nil || ctx == nil {
		return Outcome{}, ErrInvalidCaptureRequest
	}
	if err := validateRequest(request); err != nil {
		return Outcome{}, err
	}
	if err := ctx.Err(); err != nil {
		return Outcome{}, err
	}
	if err := s.validateActiveCycleStage(request); err != nil {
		return Outcome{}, err
	}

	enabled, browserE2E, err := s.stageCaptureSettings(request.StageName)
	if err != nil {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureOmitted, store.ScreenshotOmissionUnavailable)
	}
	if request.StageName == stageE2E && !browserE2E {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureOmitted, store.ScreenshotOmissionHTTPOnly)
	}
	if !mandatory && !enabled {
		return Outcome{State: OutcomeDisabled, Message: messageFor(store.ScreenshotOmissionDisabled)}, nil
	}
	// If evidence was already safely published for this exact identity, reuse
	// its card and file rather than duplicating bytes.
	if existing, err := s.findExistingReady(ctx, request); err != nil {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionPublishFailed)
	} else if existing != nil {
		return Outcome{State: OutcomeReady, Manifest: existing}, nil
	}
	if s.controllerFailed.Load() {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureOmitted, store.ScreenshotOmissionUnavailable)
	}

	if !s.guard.TryRLock() {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureOmitted, store.ScreenshotOmissionCredentialPhase)
	}
	defer s.guard.RUnlock()
	if source == nil {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureOmitted, store.ScreenshotOmissionUnavailable)
	}

	cycleRoot, screenshotsRoot, err := s.openCurrentRoots(true)
	if err != nil {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionPublishFailed)
	}
	defer func() {
		_ = screenshotsRoot.Close()
		_ = cycleRoot.Close()
	}()

	id, err := s.newID()
	if err != nil || !validScreenshotID(id) {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionPublishFailed)
	}
	temporary := "." + id + ".tmp"
	file, err := screenshotsRoot.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionPublishFailed)
	}
	keepTemp := false
	defer func() {
		_ = file.Close()
		if !keepTemp {
			_ = screenshotsRoot.Remove(temporary)
		}
	}()

	writer := &boundedWriter{writer: file, limit: media.DefaultMaxFileBytes}
	policy := CapturePolicy{
		MaskSensitiveFieldsAndTokens:                    true,
		MaskSelectors:                                   append([]string(nil), request.MaskSelectors...),
		SuppressScreenshotsDuringCredentialSubmission:   true,
		SuppressTracesDuringCredentialSubmission:        true,
		SuppressVideoDuringCredentialSubmission:         true,
		SuppressSnapshotsDuringCredentialSubmission:     true,
		SuppressRawLoginResponsesDuringCredentialSubmit: true,
	}
	receipt, captureErr := source.CaptureScreenshot(ctx, policy, writer)
	if captureErr != nil || writer.exceeded || ctx.Err() != nil {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionCaptureFailed)
	}
	if receipt.OmissionReason != "" {
		reason := safeOmission(receipt.OmissionReason)
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureOmitted, reason)
	}
	if !receipt.StablePointVerified || !receipt.SensitiveFieldsAndTokensMasked || !receipt.CredentialFlowArtifactsSuppressed {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureOmitted, store.ScreenshotOmissionUnsafeCapability)
	}
	if err := file.Sync(); err != nil {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionCaptureFailed)
	}
	if err := file.Close(); err != nil {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionCaptureFailed)
	}
	if writer.written == 0 {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionInvalidImage)
	}

	temporaryPath := filepath.Join(s.projectDir, currentDir, screensDir, temporary)
	validation, err := media.ValidateImage(ctx, temporaryPath, media.ValidationOptions{
		Workspace: s.projectDir,
		Limits: media.Limits{
			MaxFileBytes: media.DefaultMaxFileBytes,
			MaxWidth:     media.DefaultMaxWidth,
			MaxHeight:    media.DefaultMaxHeight,
		},
	})
	if err != nil || validation.ExternalPath || !s.sameFileAsOpenRoot(screenshotsRoot, temporary, temporaryPath) {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionInvalidImage)
	}
	if err := screenshotsRoot.Chmod(temporary, 0o400); err != nil {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionPublishFailed)
	}
	extension, ok := extensionForFormat(validation.Format)
	if !ok {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionInvalidImage)
	}
	if err := ctx.Err(); err != nil {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionCaptureFailed)
	}
	filename := id + extension
	if err := screenshotsRoot.Link(temporary, filename); err != nil {
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionPublishFailed)
	}
	if err := screenshotsRoot.Remove(temporary); err != nil {
		_ = screenshotsRoot.Remove(filename)
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionPublishFailed)
	}
	keepTemp = true
	if err := syncRootDirectory(screenshotsRoot); err != nil {
		_ = screenshotsRoot.Remove(filename)
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionPublishFailed)
	}

	manifest := store.ScreenshotManifest{
		ID:            id,
		CycleID:       request.CycleID,
		StageName:     request.StageName,
		Attempt:       request.Attempt,
		CoverageID:    request.CoverageID,
		UserID:        request.UserID,
		ProfileID:     request.ProfileID,
		CapturedAt:    s.now().UTC().Format(time.RFC3339Nano),
		Path:          filepath.ToSlash(filepath.Join(screensDir, filename)),
		Result:        request.Result,
		CaptureStatus: store.ScreenshotCaptureReady,
	}
	if err := s.store.InsertScreenshotManifest(ctx, manifest); err != nil {
		_ = screenshotsRoot.Remove(filename)
		_ = syncRootDirectory(screenshotsRoot)
		return s.omission(ctx, request, mandatory, store.ScreenshotCaptureFailed, store.ScreenshotOmissionPublishFailed)
	}
	return Outcome{State: OutcomeReady, Manifest: &manifest}, nil
}

// ReadySet returns a database snapshot of ready captures whose files still
// pass media validation under the active cycle's screenshots directory.
func (s *Service) ReadySet(ctx context.Context, cycleID int64) ([]store.ScreenshotManifest, error) {
	if s == nil || s.store == nil || ctx == nil || cycleID <= 0 {
		return nil, ErrInvalidCaptureRequest
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	active, err := s.store.GetActiveCycle()
	if err != nil || active.ID != cycleID {
		return nil, ErrCycleOwnership
	}
	manifests, err := s.store.ListReadyScreenshotManifests(ctx, cycleID)
	if err != nil {
		return nil, errors.New("could not list ready screenshots")
	}
	cycleRoot, screenshotsRoot, err := s.openCurrentRoots(false)
	if err != nil {
		if len(manifests) == 0 {
			return []store.ScreenshotManifest{}, nil
		}
		return nil, ErrReadyAssetUnsafe
	}
	defer func() {
		_ = screenshotsRoot.Close()
		_ = cycleRoot.Close()
	}()
	for _, manifest := range manifests {
		if !safeManifestPath(manifest) || !s.validReadyImage(ctx, cycleRoot, screenshotsRoot, manifest) {
			return nil, ErrReadyAssetUnsafe
		}
	}
	return manifests, nil
}

func (s *Service) validateActiveCycleStage(request Request) error {
	cycle, err := s.store.GetActiveCycle()
	if err != nil || cycle.ID != request.CycleID {
		return ErrCycleOwnership
	}
	stage, err := s.store.GetStage(request.CycleID, request.StageName)
	if err != nil || stage.Status != store.StageRunning {
		return ErrCycleOwnership
	}
	return nil
}

func (s *Service) stageCaptureSettings(stageName string) (enabled, browserE2E bool, err error) {
	document, err := workflowconfig.LoadCurrentDocument(s.projectDir)
	if err != nil {
		return false, false, err
	}
	stage, ok := document.Config.Stages[stageName]
	if !ok {
		return false, false, nil
	}
	return stage.Screenshots.Enabled, stage.UsePlaywright, nil
}

func (s *Service) findExistingReady(ctx context.Context, request Request) (*store.ScreenshotManifest, error) {
	ready, err := s.ReadySet(ctx, request.CycleID)
	if err != nil {
		return nil, err
	}
	for i := range ready {
		manifest := &ready[i]
		if manifest.StageName == request.StageName && manifest.Attempt == request.Attempt &&
			manifest.CoverageID == request.CoverageID && manifest.UserID == request.UserID &&
			manifest.ProfileID == request.ProfileID && manifest.Result == request.Result {
			return manifest, nil
		}
	}
	return nil, nil
}

func (s *Service) omission(_ context.Context, request Request, mandatory bool, captureStatus, reason string) (Outcome, error) {
	result := request.Result
	state := OutcomeWarning
	if mandatory {
		result = store.ScreenshotResultBlocked
		state = OutcomeBlocked
	}
	id, err := s.newID()
	if err != nil || !validScreenshotID(id) {
		return Outcome{State: state, Message: messageFor(reason)}, nil
	}
	manifest := store.ScreenshotManifest{
		ID:             id,
		CycleID:        request.CycleID,
		StageName:      request.StageName,
		Attempt:        request.Attempt,
		CoverageID:     request.CoverageID,
		UserID:         request.UserID,
		ProfileID:      request.ProfileID,
		CapturedAt:     s.now().UTC().Format(time.RFC3339Nano),
		Result:         result,
		CaptureStatus:  captureStatus,
		OmissionReason: reason,
	}
	// Omission/block records remain useful if capture was canceled. Use a
	// bounded context that carries no caller-supplied values.
	writeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.store.InsertScreenshotManifest(writeCtx, manifest); err != nil {
		return Outcome{State: state, Message: messageFor(reason)}, nil
	}
	return Outcome{State: state, Message: messageFor(reason), Manifest: &manifest}, nil
}

func (s *Service) openCurrentRoots(createScreenshots bool) (*os.Root, *os.Root, error) {
	projectRoot, err := os.OpenRoot(s.projectDir)
	if err != nil {
		return nil, nil, ErrUnsafeProjectPath
	}
	currentRoot, err := openDirectoryChain(projectRoot, []string{".workflow-hero", "cycles", "current"}, false)
	_ = projectRoot.Close()
	if err != nil {
		return nil, nil, ErrUnsafeProjectPath
	}
	screenshotsRoot, err := openDirectoryChain(currentRoot, []string{screensDir}, createScreenshots)
	if err != nil {
		_ = currentRoot.Close()
		return nil, nil, ErrUnsafeProjectPath
	}
	screenshotsInfo, err := screenshotsRoot.Stat(".")
	if err != nil || screenshotsInfo.Mode().Perm()&0o077 != 0 {
		_ = screenshotsRoot.Close()
		_ = currentRoot.Close()
		return nil, nil, ErrUnsafeProjectPath
	}
	return currentRoot, screenshotsRoot, nil
}

func openDirectoryChain(root *os.Root, names []string, createLast bool) (*os.Root, error) {
	current := root
	owned := false
	for index, name := range names {
		info, err := current.Lstat(name)
		if errors.Is(err, os.ErrNotExist) && createLast && index == len(names)-1 {
			if err := current.Mkdir(name, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
				if owned {
					_ = current.Close()
				}
				return nil, err
			}
			info, err = current.Lstat(name)
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			if owned {
				_ = current.Close()
			}
			return nil, ErrUnsafeProjectPath
		}
		child, err := current.OpenRoot(name)
		if err != nil {
			if owned {
				_ = current.Close()
			}
			return nil, err
		}
		opened, err := child.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			_ = child.Close()
			if owned {
				_ = current.Close()
			}
			return nil, ErrUnsafeProjectPath
		}
		if owned {
			_ = current.Close()
		}
		current = child
		owned = true
	}
	if !owned {
		return nil, ErrUnsafeProjectPath
	}
	return current, nil
}

func (s *Service) sameFileAsOpenRoot(root *os.Root, name, path string) bool {
	anchored, err := root.Lstat(name)
	if err != nil || anchored.Mode()&os.ModeSymlink != 0 || !anchored.Mode().IsRegular() {
		return false
	}
	byPath, err := os.Lstat(path)
	return err == nil && byPath.Mode()&os.ModeSymlink == 0 && os.SameFile(anchored, byPath)
}

func (s *Service) validReadyImage(ctx context.Context, cycleRoot, screenshotsRoot *os.Root, manifest store.ScreenshotManifest) bool {
	filename := strings.TrimPrefix(manifest.Path, screensDir+"/")
	anchored, err := screenshotsRoot.Lstat(filename)
	if err != nil || anchored.Mode()&os.ModeSymlink != 0 || !anchored.Mode().IsRegular() || anchored.Mode().Perm() != 0o400 {
		return false
	}
	path := filepath.Join(s.projectDir, currentDir, manifest.Path)
	validated, err := media.ValidateImage(ctx, path, media.ValidationOptions{Workspace: s.projectDir})
	if err != nil || validated.ExternalPath || !s.sameFileAsOpenRoot(screenshotsRoot, filename, path) {
		return false
	}
	return true
}

type boundedWriter struct {
	writer   io.Writer
	limit    int64
	written  int64
	exceeded bool
}

func (w *boundedWriter) Write(data []byte) (int, error) {
	if int64(len(data)) > w.limit-w.written {
		w.exceeded = true
		return 0, media.ErrFileTooLarge
	}
	n, err := w.writer.Write(data)
	w.written += int64(n)
	return n, err
}

func validateRequest(request Request) error {
	if request.CycleID <= 0 || (request.StageName != stageBrowserUI && request.StageName != stageE2E) || request.Attempt <= 0 {
		return ErrInvalidCaptureRequest
	}
	if request.CoverageID != "" && !metadataPattern.MatchString(request.CoverageID) {
		return ErrInvalidCaptureRequest
	}
	if request.UserID != "" || request.ProfileID != "" {
		if _, err := testaccess.ValidateUserID(request.UserID); err != nil || !profilePattern.MatchString(request.ProfileID) {
			return ErrInvalidCaptureRequest
		}
	}
	switch request.Result {
	case store.ScreenshotResultPassed, store.ScreenshotResultFailed, store.ScreenshotResultBlocked, store.ScreenshotResultSkipped:
	default:
		return ErrInvalidCaptureRequest
	}
	for _, selector := range request.MaskSelectors {
		if strings.TrimSpace(selector) == "" || strings.ContainsAny(selector, "\x00\r\n") || len(selector) > 512 {
			return ErrInvalidCaptureRequest
		}
	}
	return nil
}

func safeManifestPath(manifest store.ScreenshotManifest) bool {
	path := filepath.ToSlash(manifest.Path)
	if !strings.HasPrefix(path, screensDir+"/") || strings.Contains(path, "\\") || strings.Contains(path, "../") {
		return false
	}
	filename := strings.TrimPrefix(path, screensDir+"/")
	return filename != "" && filepath.Base(filename) == filename && strings.HasPrefix(filename, manifest.ID+".")
}

func safeOmission(reason string) string {
	switch reason {
	case store.ScreenshotOmissionCredentialPhase,
		store.ScreenshotOmissionUnsafeCapability,
		store.ScreenshotOmissionUnavailable:
		return reason
	default:
		return store.ScreenshotOmissionUnsafeCapability
	}
}

func messageFor(reason string) string {
	switch reason {
	case store.ScreenshotOmissionDisabled:
		return "automatic screenshot capture is disabled for this stage"
	case store.ScreenshotOmissionHTTPOnly:
		return "screenshots require browser-based end-to-end execution"
	case store.ScreenshotOmissionCredentialPhase:
		return "capture was omitted while credential entry or submission was in progress"
	case store.ScreenshotOmissionUnsafeCapability:
		return "capture was omitted because stable-point, masking, or sensitive-artifact suppression was not confirmed"
	case store.ScreenshotOmissionUnavailable:
		return "safe browser screenshot capture is unavailable"
	case store.ScreenshotOmissionInvalidImage:
		return "captured image did not pass media validation"
	case store.ScreenshotOmissionPublishFailed:
		return "screenshot could not be safely published"
	default:
		return "screenshot capture failed"
	}
}

func extensionForFormat(format media.ImageFormat) (string, bool) {
	switch format {
	case media.ImageFormatPNG:
		return ".png", true
	case media.ImageFormatJPEG:
		return ".jpg", true
	case media.ImageFormatGIF:
		return ".gif", true
	case media.ImageFormatWebP:
		return ".webp", true
	default:
		return "", false
	}
}

func validScreenshotID(id string) bool {
	return id != "" && len(id) <= 96 && strings.Trim(id, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-") == ""
}

func randomID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate screenshot ID: %w", err)
	}
	return "shot-" + hex.EncodeToString(raw[:]), nil
}

func syncRootDirectory(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	return errors.Join(syncErr, directory.Close())
}
