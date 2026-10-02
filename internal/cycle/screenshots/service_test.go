package screenshots

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

const screenshotTestTime = "2026-09-30T12:00:00Z"

type captureStub struct {
	mu      sync.Mutex
	image   []byte
	receipt CaptureReceipt
	err     error
	calls   int
	policy  CapturePolicy
}

func (c *captureStub) CaptureScreenshot(_ context.Context, policy CapturePolicy, writer io.Writer) (CaptureReceipt, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.policy = policy
	if len(c.image) > 0 {
		if _, err := writer.Write(c.image); err != nil {
			return CaptureReceipt{}, err
		}
	}
	return c.receipt, c.err
}

func (c *captureStub) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func (c *captureStub) lastPolicy() CapturePolicy {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.policy
}

type controllerStub struct {
	mu       sync.Mutex
	states   []bool
	failOn   *bool
	sentinel string
}

func (c *controllerStub) SetCredentialArtifactSuppression(_ context.Context, suppressed bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.states = append(c.states, suppressed)
	if c.failOn != nil && *c.failOn == suppressed {
		return errors.New(c.sentinel)
	}
	return nil
}

func (c *controllerStub) stateCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.states)
}

type screenshotFixture struct {
	project string
	store   *store.Store
	service *Service
	cycleID int64
}

func newScreenshotFixture(t *testing.T, browserEnabled, e2eEnabled, usePlaywright bool, loggers ...*slog.Logger) *screenshotFixture {
	t.Helper()
	if len(loggers) > 1 {
		t.Fatal("at most one screenshot test logger is allowed")
	}
	project := t.TempDir()
	st, err := store.OpenProject(project)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})

	cycleID, err := st.CreateCycle(store.Cycle{
		Number:             1,
		Title:              "screenshot fixture",
		Status:             store.CycleStatusActive,
		StartedAt:          screenshotTestTime,
		ConfigSnapshotJSON: "{}",
	})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	if err := st.CreateStages([]store.Stage{
		{CycleID: cycleID, Name: stageBrowserUI, Status: store.StageRunning, SortOrder: 0, MaxIterations: 1},
		{CycleID: cycleID, Name: stageE2E, Status: store.StageRunning, SortOrder: 1, MaxIterations: 1},
	}); err != nil {
		t.Fatalf("create stages: %v", err)
	}
	current := filepath.Join(project, currentDir)
	if err := os.MkdirAll(current, 0o700); err != nil {
		t.Fatalf("create current cycle: %v", err)
	}
	config := fmt.Sprintf(`stages:
  browser_ui_validation:
    screenshots:
      enabled: %t
  qa_end_to_end:
    use_playwright: %t
    screenshots:
      enabled: %t
`, browserEnabled, usePlaywright, e2eEnabled)
	if err := os.WriteFile(filepath.Join(current, "workflow-config.yml"), []byte(config), 0o600); err != nil {
		t.Fatalf("write workflow config: %v", err)
	}
	nextID := 0
	options := Options{
		Now: func() time.Time { return time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC) },
		NewID: func() (string, error) {
			nextID++
			return fmt.Sprintf("shot-%03d", nextID), nil
		},
	}
	if len(loggers) == 1 {
		options.Logger = loggers[0]
	}
	service, err := NewService(project, st, options)
	if err != nil {
		t.Fatalf("create screenshot service: %v", err)
	}
	return &screenshotFixture{project: project, store: st, service: service, cycleID: cycleID}
}

func onePixelPNG(t testing.TB) []byte {
	t.Helper()
	var encoded bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatalf("encode fixture png: %v", err)
	}
	return encoded.Bytes()
}

func requestFor(f *screenshotFixture, attempt int, userID, profile string) Request {
	return Request{
		CycleID:    f.cycleID,
		StageName:  stageBrowserUI,
		Attempt:    attempt,
		CoverageID: "protected-screen-1",
		UserID:     userID,
		ProfileID:  profile,
		Result:     store.ScreenshotResultPassed,
	}
}

func TestScreenshotCapturePreservesAttemptsRolesAndArchiveFiles(t *testing.T) {
	f := newScreenshotFixture(t, true, true, true)
	source := &captureStub{image: onePixelPNG(t), receipt: CaptureReceipt{
		StablePointVerified: true, SensitiveFieldsAndTokensMasked: true, CredentialFlowArtifactsSuppressed: true,
	}}
	operator, err := f.service.Capture(context.Background(), requestFor(f, 1, "operator", "operator"), source)
	if err != nil || operator.State != OutcomeReady {
		t.Fatalf("capture operator = (%#v, %v), want ready", operator, err)
	}
	admin, err := f.service.Capture(context.Background(), requestFor(f, 2, "admin", "admin"), source)
	if err != nil || admin.State != OutcomeReady {
		t.Fatalf("capture admin = (%#v, %v), want ready", admin, err)
	}
	if operator.Manifest.ID == admin.Manifest.ID || operator.Manifest.Path == admin.Manifest.Path {
		t.Fatalf("attempts/roles overwrote one another: %#v %#v", operator.Manifest, admin.Manifest)
	}
	if operator.Manifest.UserID != "operator" || admin.Manifest.ProfileID != "admin" || operator.Manifest.Attempt != 1 || admin.Manifest.Attempt != 2 {
		t.Fatalf("capture metadata lost role/attempt identity: %#v %#v", operator.Manifest, admin.Manifest)
	}
	policy := source.lastPolicy()
	if !policy.MaskSensitiveFieldsAndTokens || !policy.SuppressScreenshotsDuringCredentialSubmission || !policy.SuppressTracesDuringCredentialSubmission || !policy.SuppressVideoDuringCredentialSubmission || !policy.SuppressSnapshotsDuringCredentialSubmission || !policy.SuppressRawLoginResponsesDuringCredentialSubmit {
		t.Fatalf("capture policy did not enforce every safety requirement: %#v", policy)
	}

	ready, err := f.service.ReadySet(context.Background(), f.cycleID)
	if err != nil || len(ready) != 2 {
		t.Fatalf("ready set = (%d, %v), want both captures", len(ready), err)
	}
	for _, manifest := range ready {
		path := filepath.Join(f.project, currentDir, filepath.FromSlash(manifest.Path))
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o400 || info.Size() == 0 {
			t.Fatalf("published screenshot is missing, empty, or writable: info=%v err=%v", info, err)
		}
	}

	archive := filepath.Join(f.project, ".workflow-hero", "cycles", "archive-c1")
	if err := os.Rename(filepath.Join(f.project, currentDir), archive); err != nil {
		t.Fatalf("move cycle to archive: %v", err)
	}
	archivedRows, err := f.store.ListReadyScreenshotManifests(context.Background(), f.cycleID)
	if err != nil || len(archivedRows) != 2 {
		t.Fatalf("archive move changed screenshot manifests: rows=%d err=%v", len(archivedRows), err)
	}
	for _, manifest := range archivedRows {
		if _, err := os.Stat(filepath.Join(archive, filepath.FromSlash(manifest.Path))); err != nil {
			t.Errorf("screenshot was not retained by archive move: %v", err)
		}
	}
}

func TestScreenshotCaptureOptInDoesNotDisablePlannedEvidence(t *testing.T) {
	f := newScreenshotFixture(t, false, false, true)
	source := &captureStub{image: onePixelPNG(t), receipt: CaptureReceipt{
		StablePointVerified: true, SensitiveFieldsAndTokensMasked: true, CredentialFlowArtifactsSuppressed: true,
	}}
	request := requestFor(f, 1, "operator", "operator")
	optional, err := f.service.Capture(context.Background(), request, source)
	if err != nil || optional.State != OutcomeDisabled || source.callCount() != 0 {
		t.Fatalf("default-off optional capture = (%#v, %v), source calls=%d", optional, err, source.callCount())
	}

	planned, err := f.service.CapturePlannedEvidence(context.Background(), request, source)
	if err != nil || planned.State != OutcomeReady || source.callCount() != 1 {
		t.Fatalf("planned evidence was disabled by opt-in: (%#v, %v), source calls=%d", planned, err, source.callCount())
	}
	// A second path to the same planned evidence identity reuses the already
	// published card rather than creating another copy or manifest.
	reused, err := f.service.CapturePlannedEvidence(context.Background(), request, source)
	if err != nil || reused.State != OutcomeReady || reused.Manifest.ID != planned.Manifest.ID || source.callCount() != 1 {
		t.Fatalf("planned evidence reuse = (%#v, %v), source calls=%d", reused, err, source.callCount())
	}
}

func TestScreenshotCaptureDefaultsOffWhenStageToggleIsMissing(t *testing.T) {
	f := newScreenshotFixture(t, true, true, true)
	config := `stages:
  browser_ui_validation: {}
  qa_end_to_end:
    use_playwright: true
`
	configPath := filepath.Join(f.project, currentDir, "workflow-config.yml")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write workflow config without screenshot toggles: %v", err)
	}

	source := &captureStub{image: onePixelPNG(t), receipt: CaptureReceipt{
		StablePointVerified: true, SensitiveFieldsAndTokensMasked: true, CredentialFlowArtifactsSuppressed: true,
	}}
	request := requestFor(f, 1, "operator", "operator")
	optional, err := f.service.Capture(context.Background(), request, source)
	if err != nil || optional.State != OutcomeDisabled || source.callCount() != 0 {
		t.Fatalf("missing stage toggle optional capture = (%#v, %v), source calls=%d", optional, err, source.callCount())
	}

	planned, err := f.service.CapturePlannedEvidence(context.Background(), request, source)
	if err != nil || planned.State != OutcomeReady || source.callCount() != 1 {
		t.Fatalf("missing stage toggle planned evidence = (%#v, %v), source calls=%d", planned, err, source.callCount())
	}
}

func TestScreenshotCaptureHTTPOnlyE2EIsBlocked(t *testing.T) {
	f := newScreenshotFixture(t, false, true, false)
	source := &captureStub{image: onePixelPNG(t), receipt: CaptureReceipt{
		StablePointVerified: true, SensitiveFieldsAndTokensMasked: true, CredentialFlowArtifactsSuppressed: true,
	}}
	request := requestFor(f, 1, "operator", "operator")
	request.StageName = stageE2E
	outcome, err := f.service.CapturePlannedEvidence(context.Background(), request, source)
	if err != nil || outcome.State != OutcomeBlocked || source.callCount() != 0 {
		t.Fatalf("HTTP-only planned screenshot = (%#v, %v), source calls=%d", outcome, err, source.callCount())
	}
	if outcome.Manifest == nil || outcome.Manifest.OmissionReason != store.ScreenshotOmissionHTTPOnly || outcome.Manifest.Result != store.ScreenshotResultBlocked {
		t.Fatalf("HTTP-only block did not include a safe blocker manifest: %#v", outcome.Manifest)
	}
}

func TestSecretSuppressionGuardsEveryArtifactChannel(t *testing.T) {
	f := newScreenshotFixture(t, true, true, true)
	source := &captureStub{image: onePixelPNG(t), receipt: CaptureReceipt{
		StablePointVerified: true, SensitiveFieldsAndTokensMasked: true, CredentialFlowArtifactsSuppressed: true,
	}}
	controller := &controllerStub{}
	release, err := f.service.BeginCredentialEntry(context.Background(), controller)
	if err != nil {
		t.Fatalf("begin credential entry: %v", err)
	}
	outcome, err := f.service.Capture(context.Background(), requestFor(f, 1, "operator", "operator"), source)
	if err != nil || outcome.State != OutcomeWarning || outcome.Manifest == nil || outcome.Manifest.OmissionReason != store.ScreenshotOmissionCredentialPhase || source.callCount() != 0 {
		t.Fatalf("capture during credential phase = (%#v, %v), calls=%d", outcome, err, source.callCount())
	}
	if controller.stateCount() != 1 {
		t.Fatalf("credential suppression not activated before phase: %d controller calls", controller.stateCount())
	}
	plannedRequest := requestFor(f, 2, "operator", "operator")
	planned, err := f.service.CapturePlannedEvidence(context.Background(), plannedRequest, source)
	if err != nil || planned.State != OutcomeBlocked || planned.Manifest == nil || planned.Manifest.Result != store.ScreenshotResultBlocked || source.callCount() != 0 {
		t.Fatalf("mandatory evidence during credential phase was not blocked: (%#v, %v), calls=%d", planned, err, source.callCount())
	}
	if err := release(); err != nil {
		t.Fatalf("resume artifact capture: %v", err)
	}
	if err := release(); err != nil || controller.stateCount() != 2 {
		t.Fatalf("idempotent release = %v, controller calls=%d", err, controller.stateCount())
	}
	ready, err := f.service.Capture(context.Background(), requestFor(f, 3, "operator", "operator"), source)
	if err != nil || ready.State != OutcomeReady {
		t.Fatalf("capture after credential phase = (%#v, %v)", ready, err)
	}
}

func TestSecretSuppressionResumeFailureKeepsServiceFailClosed(t *testing.T) {
	f := newScreenshotFixture(t, true, true, true)
	failFalse := false
	controller := &controllerStub{failOn: &failFalse, sentinel: "raw-token-sentinel"}
	release, err := f.service.BeginCredentialEntry(context.Background(), controller)
	if err != nil {
		t.Fatalf("begin credential entry: %v", err)
	}
	if err := release(); err == nil || strings.Contains(err.Error(), "raw-token-sentinel") {
		t.Fatalf("unsafe or secret-bearing resume error = %v", err)
	}
	source := &captureStub{image: onePixelPNG(t), receipt: CaptureReceipt{
		StablePointVerified: true, SensitiveFieldsAndTokensMasked: true, CredentialFlowArtifactsSuppressed: true,
	}}
	outcome, err := f.service.Capture(context.Background(), requestFor(f, 1, "operator", "operator"), source)
	if err != nil || outcome.State != OutcomeWarning || source.callCount() != 0 {
		t.Fatalf("capture did not remain fail-closed after resume error: (%#v, %v), calls=%d", outcome, err, source.callCount())
	}
}

func TestSecretSuppressionFailureDoesNotExposeControllerError(t *testing.T) {
	f := newScreenshotFixture(t, true, true, true)
	failure := true
	controller := &controllerStub{failOn: &failure, sentinel: "password sentinel-value"}
	_, err := f.service.BeginCredentialEntry(context.Background(), controller)
	if err == nil || strings.Contains(err.Error(), "sentinel-value") {
		t.Fatalf("unsafe or secret-bearing begin error = %v", err)
	}
	source := &captureStub{image: onePixelPNG(t), receipt: CaptureReceipt{
		StablePointVerified: true, SensitiveFieldsAndTokensMasked: true, CredentialFlowArtifactsSuppressed: true,
	}}
	outcome, err := f.service.Capture(context.Background(), requestFor(f, 1, "operator", "operator"), source)
	if err != nil || outcome.State != OutcomeWarning || source.callCount() != 0 {
		t.Fatalf("capture did not remain fail-closed: (%#v, %v), source calls=%d", outcome, err, source.callCount())
	}
	rows, err := f.store.ListReadyScreenshotManifests(context.Background(), f.cycleID)
	if err != nil || len(rows) != 0 {
		t.Fatalf("failed suppression created ready evidence: %v, %v", rows, err)
	}
}

func TestScreenshotCaptureDoesNotPersistExecutorSentinel(t *testing.T) {
	f := newScreenshotFixture(t, true, true, true)
	const sentinel = "password-token-sentinel-7842"
	source := &captureStub{
		image: onePixelPNG(t),
		receipt: CaptureReceipt{
			StablePointVerified: true, SensitiveFieldsAndTokensMasked: true, CredentialFlowArtifactsSuppressed: true,
		},
		err: errors.New(sentinel),
	}
	outcome, err := f.service.Capture(context.Background(), requestFor(f, 1, "operator", "operator"), source)
	if err != nil || outcome.State != OutcomeWarning || strings.Contains(outcome.Message, sentinel) {
		t.Fatalf("capture error was not safely downgraded: outcome=%#v err=%v", outcome, err)
	}
	if outcome.Manifest == nil || outcome.Manifest.OmissionReason != store.ScreenshotOmissionCaptureFailed {
		t.Fatalf("capture failure lacks safe omission metadata: %#v", outcome.Manifest)
	}
	var persisted string
	if err := f.store.DB().QueryRow(`SELECT group_concat(screenshot_id || '|' || stage_name || '|' || coverage_id || '|' || user_id || '|' || profile_id || '|' || omission_reason) FROM screenshot_manifests`).Scan(&persisted); err != nil {
		t.Fatalf("read screenshot metadata: %v", err)
	}
	if strings.Contains(persisted, sentinel) || strings.Contains(persisted, "password") {
		t.Fatalf("executor diagnostic leaked into screenshot manifest: %q", persisted)
	}
	entries, err := os.ReadDir(filepath.Join(f.project, currentDir, screensDir))
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed capture left a file available: entries=%v err=%v", entries, err)
	}
}

func TestScreenshotLoggingUsesSafeMetadataAndExplicitLevels(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	f := newScreenshotFixture(t, true, true, true, logger)
	const sentinel = "password-token-sentinel-logging"
	failedSource := &captureStub{
		image: onePixelPNG(t),
		receipt: CaptureReceipt{
			StablePointVerified: true, SensitiveFieldsAndTokensMasked: true, CredentialFlowArtifactsSuppressed: true,
		},
		err: errors.New(sentinel),
	}
	if outcome, err := f.service.Capture(context.Background(), requestFor(f, 1, "operator", "operator"), failedSource); err != nil || outcome.State != OutcomeWarning {
		t.Fatalf("optional capture failure = (%#v, %v)", outcome, err)
	}

	readySource := &captureStub{image: onePixelPNG(t), receipt: CaptureReceipt{
		StablePointVerified: true, SensitiveFieldsAndTokensMasked: true, CredentialFlowArtifactsSuppressed: true,
	}}
	if outcome, err := f.service.Capture(context.Background(), requestFor(f, 2, "admin", "admin"), readySource); err != nil || outcome.State != OutcomeReady {
		t.Fatalf("ready capture = (%#v, %v)", outcome, err)
	}
	if outcome, err := f.service.CapturePlannedEvidence(context.Background(), requestFor(f, 3, "admin", "admin"), failedSource); err != nil || outcome.State != OutcomeBlocked {
		t.Fatalf("mandatory capture failure = (%#v, %v)", outcome, err)
	}

	output := logs.String()
	for _, forbidden := range []string{sentinel, "operator", "admin", "user_id", "profile_id"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("screenshot log leaked %q: %s", forbidden, output)
		}
	}
	for _, level := range []string{"level=DEBUG", "level=INFO", "level=ERROR"} {
		if !strings.Contains(output, level) {
			t.Fatalf("screenshot logs did not include %s: %s", level, output)
		}
	}
	if !strings.Contains(output, "reason=capture_failed") {
		t.Fatalf("screenshot log omitted stable failure reason: %s", output)
	}
}

func TestScreenshotDebugLogsRespectInfoLevel(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	f := newScreenshotFixture(t, false, true, true, logger)
	outcome, err := f.service.Capture(context.Background(), requestFor(f, 1, "operator", "operator"), nil)
	if err != nil || outcome.State != OutcomeDisabled {
		t.Fatalf("disabled optional capture = (%#v, %v)", outcome, err)
	}
	if logs.Len() != 0 {
		t.Fatalf("debug capture detail was emitted at info level: %s", logs.String())
	}
}

func TestManifestRegistrationFailureRemovesPublishedImage(t *testing.T) {
	f := newScreenshotFixture(t, true, true, true)
	if _, err := f.store.DB().Exec(`CREATE TRIGGER reject_screenshot_manifest BEFORE INSERT ON screenshot_manifests BEGIN SELECT RAISE(ABORT, 'forced failure'); END`); err != nil {
		t.Fatalf("create insert failure trigger: %v", err)
	}
	source := &captureStub{image: onePixelPNG(t), receipt: CaptureReceipt{
		StablePointVerified: true, SensitiveFieldsAndTokensMasked: true, CredentialFlowArtifactsSuppressed: true,
	}}
	outcome, err := f.service.Capture(context.Background(), requestFor(f, 1, "operator", "operator"), source)
	if err != nil || outcome.State != OutcomeWarning || strings.Contains(outcome.Message, "forced failure") {
		t.Fatalf("database failure was not safely reported: (%#v, %v)", outcome, err)
	}
	screenshotsDir := filepath.Join(f.project, currentDir, screensDir)
	entries, err := os.ReadDir(screenshotsDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed manifest registration left a ready or partial file: entries=%v err=%v", entries, err)
	}
	rows, err := f.store.ListReadyScreenshotManifests(context.Background(), f.cycleID)
	if err != nil || len(rows) != 0 {
		t.Fatalf("failed manifest registration produced ready row: rows=%v err=%v", rows, err)
	}
}

func TestReadySetRejectsMissingOrPartialReadyFile(t *testing.T) {
	f := newScreenshotFixture(t, true, true, true)
	manifest := store.ScreenshotManifest{
		ID: "shot-orphan", CycleID: f.cycleID, StageName: stageBrowserUI, Attempt: 1,
		CapturedAt: screenshotTestTime, Path: "screenshots/shot-orphan.png",
		Result: store.ScreenshotResultPassed, CaptureStatus: store.ScreenshotCaptureReady,
	}
	if err := f.store.InsertScreenshotManifest(context.Background(), manifest); err != nil {
		t.Fatalf("insert orphan ready row: %v", err)
	}
	if _, err := f.service.ReadySet(context.Background(), f.cycleID); !errors.Is(err, ErrReadyAssetUnsafe) {
		t.Fatalf("ready set exposed a manifest before file publication: %v", err)
	}
}
