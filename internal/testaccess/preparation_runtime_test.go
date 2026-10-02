package testaccess

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type preparationHarnessStub struct {
	name string
	err  error
}

func (h preparationHarnessStub) Name() string                      { return h.name }
func (h preparationHarnessStub) IsAvailable(context.Context) error { return h.err }

type preparationCommandStub struct {
	output []byte
	err    error
	argv   []string
	dir    string
}

func (c *preparationCommandStub) Run(_ context.Context, dir string, argv []string) ([]byte, error) {
	c.argv = append([]string(nil), argv...)
	c.dir = dir
	return append([]byte(nil), c.output...), c.err
}

func TestRuntimePreparationChecksReadinessSelectedHarnessAndPlannedVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "" {
			t.Errorf("readiness request must be an unauthenticated GET: method=%s authorization=%q", r.Method, r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	projectDir := t.TempDir()
	plan := validBrowserPlan()
	plan.Execution.BaseURL = server.URL
	plan.Execution.ApprovedOrigins = append(plan.Execution.ApprovedOrigins, server.URL)
	commands := &preparationCommandStub{output: []byte("Version 1.63.2\n")}
	capability := &RuntimePreparationCapability{
		ProjectDir:  projectDir,
		HarnessID:   "cursor",
		Harness:     preparationHarnessStub{name: "cursor"},
		Credentials: &executorCredentialSource{snapshot: CredentialSnapshot{account: Account{}}},
		Commands:    commands,
		Plan:        &plan,
	}
	recipe := validExecutorRecipe()
	recipe.BaseURL = server.URL
	recipe.ApprovedOrigins = append(recipe.ApprovedOrigins, server.URL)
	recipe.PlaywrightVersion = MinimumPlaywrightVersion
	recipe.ToolVersionCommand = []string{"playwright", "--version"}
	recipe.Fixtures = []string{"synthetic-local"}

	result := NewPreparer(capability, newFakePreparationClock(time.Unix(1_800_000_000, 0)), preparationLogger()).Prepare(context.Background(), recipe, time.Minute)
	if result.Status != PreparationReady || len(result.Checked) != len(preparationPrerequisites) {
		t.Fatalf("runtime preparation result=%+v, want ready after all local checks", result)
	}
	if got, want := commands.argv, []string{"playwright", "--version"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("version command argv=%v, want exact fixed argv %v", got, want)
	}
	if commands.dir != projectDir {
		t.Fatalf("version command directory=%q, want project root", commands.dir)
	}
}

func TestRuntimePreparationBlocksBelowMinimumActualPlaywrightVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	projectDir := t.TempDir()
	plan := validBrowserPlan()
	plan.Execution.BaseURL = server.URL
	plan.Execution.ApprovedOrigins = append(plan.Execution.ApprovedOrigins, server.URL)
	capability := &RuntimePreparationCapability{
		ProjectDir:  projectDir,
		HarnessID:   "cursor",
		Harness:     preparationHarnessStub{name: "cursor"},
		Credentials: &executorCredentialSource{snapshot: CredentialSnapshot{account: Account{}}},
		Commands:    &preparationCommandStub{output: []byte("Version 1.62.9")},
		Plan:        &plan,
	}
	recipe := validExecutorRecipe()
	recipe.BaseURL = server.URL
	recipe.ApprovedOrigins = append(recipe.ApprovedOrigins, server.URL)
	result := NewPreparer(capability, newFakePreparationClock(time.Unix(1_800_000_000, 0)), preparationLogger()).Prepare(context.Background(), recipe, time.Minute)
	if result.Status != PreparationBlocked || result.Reason != PreparationReasonToolVersion || result.BlockedOn != PrerequisiteMethodAdmission {
		t.Fatalf("below-minimum actual tool version was not blocked: %+v", result)
	}
	if !strings.Contains(result.NextAction, MinimumPlaywrightVersion) {
		t.Fatalf("missing setup guidance for minimum version: %q", result.NextAction)
	}
}

func TestRuntimePreparationHTTPModeChecksOnlyReadiness(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet {
			t.Errorf("HTTP-only readiness used %s, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	projectDir := t.TempDir()
	plan := validBrowserPlan()
	plan.Execution.BaseURL = server.URL
	plan.Execution.ApprovedOrigins = append(plan.Execution.ApprovedOrigins, server.URL)
	plan.Method = BrowserMethodPlan{Stage: StageQAEndToEnd, Purpose: PurposeRepeatableE2E, Method: MethodHTTP}
	writeBrowserPlan(t, projectDir, plan)
	capability := &RuntimePreparationCapability{ProjectDir: projectDir, HTTPClient: server.Client()}
	result := NewPreparer(capability, newFakePreparationClock(time.Unix(1_800_000_000, 0)), preparationLogger()).PreparePlan(context.Background(), projectDir, "screen-operator-01", false, time.Minute)
	if result.Status != PreparationReady || !reflect.DeepEqual(result.Checked, []PreparationPrerequisite{PrerequisiteServiceReadiness}) || requests != 1 {
		t.Fatalf("HTTP-only mode did not restrict work to readiness: result=%+v requests=%d", result, requests)
	}
}

func TestRuntimePreparationSelectedAccountsRemainPrivateAndCoverPlanUsers(t *testing.T) {
	credentials := &executorCredentialSource{snapshot: CredentialSnapshot{account: Account{}}}
	plan := validBrowserPlan()
	capability := &RuntimePreparationCapability{ProjectDir: t.TempDir(), Credentials: credentials, Plan: &plan}
	observation := capability.CheckPrerequisite(context.Background(), validExecutorRecipe(), PrerequisiteSelectedAccount)
	if !observation.Ready {
		t.Fatalf("selected plan accounts were not verified: %+v", observation)
	}
	if got, want := credentials.users, []string{"operator", "administrator"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selected users checked=%v, want every planned user %v", got, want)
	}
	if observation.Reason != "" {
		t.Fatalf("account value appeared in observation: %+v", observation)
	}
}

func TestRuntimePreparationFixturesRejectEscapeAndSymlinkButSkipLabels(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectDir, "fixtures"), 0o700); err != nil {
		t.Fatal("create fixture directory")
	}
	if err := os.WriteFile(filepath.Join(projectDir, "fixtures", "local.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal("write fixture")
	}
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("{}"), 0o600); err != nil {
		t.Fatal("write outside fixture")
	}
	if err := os.Symlink(outside, filepath.Join(projectDir, "fixture-link.json")); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	capability := &RuntimePreparationCapability{ProjectDir: projectDir}
	for _, test := range []struct {
		fixture string
		ready   bool
	}{
		{fixture: "synthetic-local", ready: true},
		{fixture: "fixtures/local.json", ready: true},
		{fixture: "../outside.json", ready: false},
		{fixture: "fixture-link.json", ready: false},
	} {
		observation := capability.CheckPrerequisite(context.Background(), Recipe{Fixtures: []string{test.fixture}}, PrerequisiteFixtures)
		if observation.Ready != test.ready {
			t.Errorf("fixture %q ready=%t reason=%s, want %t", test.fixture, observation.Ready, observation.Reason, test.ready)
		}
	}
}

func TestObservedVersionDoesNotRetainToolOutput(t *testing.T) {
	const sentinel = "SECRET_SENTINEL"
	if got := observedVersion([]byte("playwright 1.64.1 " + sentinel)); got != "1.64.1" {
		t.Fatalf("observed version=%q, want only parsed version", got)
	}
	if got := observedVersion([]byte("no version")); got != "" {
		t.Fatalf("unparseable output produced version %q", got)
	}
}
