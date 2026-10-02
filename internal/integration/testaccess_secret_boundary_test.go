package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/screenshots"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/testaccess"
	"gopkg.in/yaml.v3"
)

const (
	boundaryLogin    = "SENTINEL_BOUNDARY_LOGIN_1a92"
	boundaryPassword = "SENTINEL_BOUNDARY_PASSWORD_7b34"
	boundaryToken    = "SENTINEL_BOUNDARY_TOKEN_5e81"
)

type secretBoundaryProvider struct {
	session *secretBoundarySession
}

func (p *secretBoundaryProvider) Capabilities(_ context.Context, recipe testaccess.Recipe) (testaccess.RunnerCapabilities, error) {
	return testaccess.RunnerCapabilities{
		ToolAvailable:                 true,
		ToolName:                      recipe.ToolName,
		ToolVersion:                   testaccess.MinimumPlaywrightVersion,
		PlaywrightVersion:             testaccess.MinimumPlaywrightVersion,
		Method:                        recipe.Method,
		MethodSupported:               true,
		FreshIsolatedContext:          true,
		SameContextLoginAndTests:      true,
		PrivateCredentialChannel:      true,
		ApprovedOriginsEnforced:       true,
		AuthStateMemoryOnly:           true,
		CredentialArtifactSuppression: true,
	}, nil
}

func (p *secretBoundaryProvider) NewIsolatedSession(context.Context, testaccess.Recipe) (testaccess.BrowserSession, error) {
	return p.session, nil
}

type secretBoundarySession struct {
	suppressed bool
	account    testaccess.Account
	token      string
}

func (s *secretBoundarySession) SetCredentialArtifactSuppression(_ context.Context, suppressed bool) error {
	s.suppressed = suppressed
	return nil
}

func (s *secretBoundarySession) Login(_ context.Context, _ testaccess.LoginFormRecipe, account testaccess.Account) (testaccess.LoginOutcome, error) {
	if !s.suppressed {
		return testaccess.LoginOutcome{}, os.ErrPermission
	}
	s.account = account
	return testaccess.LoginOutcome{Authenticated: true}, nil
}

func (s *secretBoundarySession) VerifyProtectedTarget(_ context.Context, target testaccess.ProtectedTargetRecipe) (testaccess.AccessOutcome, error) {
	s.token = boundaryToken
	return testaccess.AccessOutcome{Authenticated: true, VerifiedRole: target.ExpectedRole, TargetAllowed: true}, nil
}

func (*secretBoundarySession) Run(context.Context, testaccess.Recipe) (testaccess.RunOutcome, error) {
	return testaccess.RunPassed, nil
}

func (*secretBoundarySession) Close() error { return nil }

type secretBoundaryCapture struct{ data []byte }

func (s secretBoundaryCapture) CaptureScreenshot(_ context.Context, policy screenshots.CapturePolicy, dst io.Writer) (screenshots.CaptureReceipt, error) {
	if !policy.MaskSensitiveFieldsAndTokens || !policy.SuppressScreenshotsDuringCredentialSubmission || !policy.SuppressTracesDuringCredentialSubmission || !policy.SuppressVideoDuringCredentialSubmission || !policy.SuppressSnapshotsDuringCredentialSubmission || !policy.SuppressRawLoginResponsesDuringCredentialSubmit {
		return screenshots.CaptureReceipt{}, os.ErrPermission
	}
	_, err := dst.Write(s.data)
	return screenshots.CaptureReceipt{StablePointVerified: true, SensitiveFieldsAndTokensMasked: true, CredentialFlowArtifactsSuppressed: true}, err
}

func TestExecutorCredentialSentinelsStayOutOfRuntimeAndPersistenceSurfaces(t *testing.T) {
	ctx := context.Background()
	project := t.TempDir()
	if err := exec.Command("git", "-C", project, "init", "--quiet").Run(); err != nil {
		t.Fatal("initialize synthetic project")
	}
	st, err := store.OpenProject(project)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Error(err)
		}
	})
	cycleID, err := st.CreateCycle(store.Cycle{Number: 1, Title: "synthetic boundary", Status: store.CycleStatusActive, StartedAt: time.Now().UTC().Format(time.RFC3339), ConfigSnapshotJSON: `{}`})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateStages([]store.Stage{{CycleID: cycleID, Name: "browser_ui_validation", Status: store.StageRunning, MaxIterations: 1}}); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(project, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(current, 0o700); err != nil {
		t.Fatal(err)
	}
	config := []byte("test_access:\n  enabled: true\nstages:\n  browser_ui_validation:\n    screenshots:\n      enabled: true\n")
	if err := os.WriteFile(filepath.Join(current, "workflow-config.yml"), config, 0o600); err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	credentialStore, err := testaccess.OpenSafeStore(project, logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := credentialStore.Close(); err != nil {
			t.Error(err)
		}
	})
	draft, err := credentialStore.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	account, err := testaccess.NewAccount("operator", boundaryLogin, boundaryPassword, "operator")
	if err != nil {
		t.Fatal(err)
	}
	if err := draft.Document.SetAccounts([]testaccess.Account{account}); err != nil {
		t.Fatal(err)
	}
	draft, err = credentialStore.Save(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := credentialStore.Snapshot(ctx, "operator"); err != nil {
		t.Fatal(err)
	}

	recipe := testaccess.Recipe{
		Environment: "synthetic local fixture", BaseURL: "http://127.0.0.1:43127",
		ApprovedOrigins: []string{"http://127.0.0.1:43127"}, Authentication: testaccess.AuthenticationForm,
		Login:           testaccess.LoginFormRecipe{EntryURL: "http://127.0.0.1:43127/login", LoginLocator: "[name=login]", PasswordLocator: "[name=password]", SubmitLocator: "button[type=submit]"},
		ProtectedTarget: testaccess.ProtectedTargetRecipe{URL: "http://127.0.0.1:43127/operator", ExpectedRole: "operator", ExpectedAccess: testaccess.AccessAllowed},
		UserID:          "operator", UserProfile: "operator", CoverageIDs: []string{"screen-operator-01"},
		CoverageRequirement: "PRD-C17 FR-07", CoverageScreenJourney: "operator protected screen", CoverageMandatory: true,
		CoverageExpectedResult: "operator sees the protected fixture screen", EvidenceRequirements: []string{"protected screen rendered"},
		Purpose: testaccess.PurposeRepeatableE2E, Method: testaccess.MethodPlaywrightTestSuite,
		ToolName: "playwright", ToolVersion: testaccess.MinimumPlaywrightVersion, PlaywrightVersion: testaccess.MinimumPlaywrightVersion,
		ToolVersionCommand:      []string{"playwright", "--version"},
		ExistingPlaywrightSuite: true, StartCommand: "start synthetic fixture", ReadinessCommand: "check synthetic fixture", E2ECommand: "assert operator outcome",
		ActionTimeout: time.Second, TestTimeout: 10 * time.Second,
	}
	session := &secretBoundarySession{}
	result := testaccess.NewExecutor(credentialStore, &secretBoundaryProvider{session: session}, logger).Execute(ctx, recipe)
	if result.Status != testaccess.PreparationReady || session.account.Password() != boundaryPassword || session.token != boundaryToken {
		t.Fatalf("synthetic private execution failed: status=%s reason=%s", result.Status, result.Reason)
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	plan := testaccess.BrowserPlan{
		SchemaVersion:  testaccess.BrowserPlanSchemaVersion,
		Execution:      testaccess.BrowserExecutionContract{Environment: recipe.Environment, BaseURL: recipe.BaseURL, ApprovedOrigins: recipe.ApprovedOrigins, ActionTimeout: "1s", TestTimeout: "10s"},
		Authentication: testaccess.BrowserAuthenticationPlan{Requirement: testaccess.AuthenticationRequired, Flow: testaccess.AuthenticationForm, Login: recipe.Login},
		Method:         testaccess.BrowserMethodPlan{Purpose: recipe.Purpose, Method: recipe.Method, ToolName: recipe.ToolName, ToolVersion: recipe.ToolVersion, ToolVersionCommand: append([]string(nil), recipe.ToolVersionCommand...), PlaywrightVersion: recipe.PlaywrightVersion, ExistingPlaywrightSuite: true},
		Coverage:       []testaccess.CoverageItem{{ID: "screen-operator-01", RequirementRef: recipe.CoverageRequirement, ScreenOrJourney: recipe.CoverageScreenJourney, UserID: "operator", Profile: "operator", Mandatory: true, ExpectedResult: recipe.CoverageExpectedResult, EvidenceRequirements: recipe.EvidenceRequirements, ProtectedTarget: recipe.ProtectedTarget}},
	}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	planYAML, err := yaml.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	agentPrompt, err := assets.FS.ReadFile("cursor/agents/browser_ui_agent.md")
	if err != nil {
		t.Fatal(err)
	}
	prompt := string(agentPrompt) + "\nApproved browser plan:\n" + string(planJSON) + "\nSanitized preparation result:\n" + string(resultJSON)
	if _, err := st.AppendEvent(store.Event{CycleID: cycleID, Type: store.EventStageBlocked, PayloadJSON: string(resultJSON)}); err != nil {
		t.Fatal(err)
	}
	events, err := st.ListEvents(cycleID, store.EventStageBlocked, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("stored events=%d, want one", len(events))
	}
	configYAML, err := os.ReadFile(filepath.Join(current, "workflow-config.yml"))
	if err != nil {
		t.Fatal(err)
	}

	var imageData bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 15, G: 25, B: 35, A: 255})
	if err := png.Encode(&imageData, img); err != nil {
		t.Fatal(err)
	}
	screenshotService, err := screenshots.NewService(project, st)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := screenshotService.Capture(ctx, screenshots.Request{
		CycleID: cycleID, StageName: "browser_ui_validation", Attempt: 1,
		CoverageID: "screen-operator-01", UserID: "operator", ProfileID: "operator", Result: store.ScreenshotResultPassed,
	}, secretBoundaryCapture{data: imageData.Bytes()})
	if err != nil || capture.State != screenshots.OutcomeReady || capture.Manifest == nil {
		t.Fatalf("safe synthetic capture = (%#v, %v)", capture, err)
	}
	imageBytes, err := os.ReadFile(filepath.Join(current, filepath.FromSlash(capture.Manifest.Path)))
	if err != nil {
		t.Fatal(err)
	}
	manifestJSON, err := json.Marshal(capture.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	databaseBytes, err := os.ReadFile(filepath.Join(project, store.RelativeDBPath))
	if err != nil {
		t.Fatal(err)
	}

	surfaces := map[string][]byte{
		"prompt": []byte(prompt), "SQLite": databaseBytes, "workflow YAML": configYAML,
		"browser plan YAML": planYAML, "event": []byte(events[0].PayloadJSON), "log": logs.Bytes(),
		"capture manifest": manifestJSON, "capture image": imageBytes,
	}
	for surface, data := range surfaces {
		for _, sentinel := range []string{boundaryLogin, boundaryPassword, boundaryToken} {
			if strings.Contains(string(data), sentinel) {
				t.Fatalf("credential sentinel leaked into %s", surface)
			}
		}
	}
}

func TestGofmtC17Integration(t *testing.T) {
	output, err := exec.Command("gofmt", "-l", "testaccess_secret_boundary_test.go").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(output)) != "" {
		t.Fatalf("gofmt reports unformatted file: %s", output)
	}
}
