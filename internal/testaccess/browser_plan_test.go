package testaccess

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func validBrowserPlan() BrowserPlan {
	return BrowserPlan{
		SchemaVersion: BrowserPlanSchemaVersion,
		Execution: BrowserExecutionContract{
			Environment:      "synthetic local fixture",
			BaseURL:          "http://127.0.0.1:43127",
			ApprovedOrigins:  []string{"http://127.0.0.1:43127"},
			StartCommand:     "fixture starts in-process",
			ReadinessCommand: "fixture readiness endpoint",
			E2ECommand:       "assert protected fixture outcome",
			ActionTimeout:    "2s",
			TestTimeout:      "10s",
			Fixtures:         []string{"synthetic-operator-admin"},
			EvidencePaths:    []string{"current/screenshots"},
		},
		Authentication: BrowserAuthenticationPlan{
			Requirement: AuthenticationRequired,
			Flow:        AuthenticationForm,
			Login: LoginFormRecipe{
				EntryURL:        "http://127.0.0.1:43127/login",
				LoginLocator:    "[name=login]",
				PasswordLocator: "[name=password]",
				SubmitLocator:   "button[type=submit]",
			},
		},
		Method: BrowserMethodPlan{
			Purpose:                 PurposeRepeatableE2E,
			Method:                  MethodPlaywrightTestSuite,
			ToolName:                "playwright",
			ToolVersion:             MinimumPlaywrightVersion,
			ToolVersionCommand:      []string{"playwright", "--version"},
			PlaywrightVersion:       MinimumPlaywrightVersion,
			ExistingPlaywrightSuite: true,
		},
		Coverage: []CoverageItem{
			{
				ID:                      "screen-operator-01",
				RequirementRef:          "PRD-C17 FR-07",
				ScreenOrJourney:         "/operator",
				UserID:                  "operator",
				Profile:                 "operator",
				Mandatory:               true,
				ExpectedResult:          "operator sees the protected fixture screen",
				EvidenceRequirements:    []string{"protected screen rendered", "business assertion passed"},
				OptionalReferenceWidths: []int{1280, 768, 375},
				ProtectedTarget: ProtectedTargetRecipe{
					URL:            "http://127.0.0.1:43127/operator",
					ExpectedRole:   "operator",
					ExpectedAccess: AccessAllowed,
				},
			},
			{
				ID:                   "screen-admin-01",
				RequirementRef:       "PRD-C17 FR-07",
				ScreenOrJourney:      "/admin",
				UserID:               "administrator",
				Profile:              "admin",
				Mandatory:            true,
				ExpectedResult:       "administrator sees the protected fixture screen",
				EvidenceRequirements: []string{"protected screen rendered"},
				ProtectedTarget: ProtectedTargetRecipe{
					URL:            "http://127.0.0.1:43127/admin",
					ExpectedRole:   "admin",
					ExpectedAccess: AccessAllowed,
				},
			},
		},
	}
}

func writeBrowserPlan(t *testing.T, projectDir string, plan BrowserPlan) []byte {
	t.Helper()
	path := filepath.Join(projectDir, filepath.FromSlash(BrowserPlanRelativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal("create current-cycle plan directory")
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatal("encode synthetic browser plan")
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal("write synthetic browser plan")
	}
	return data
}

func TestBrowserPlanLoadsSharedSchemaAndResolvesCoverageRecipe(t *testing.T) {
	projectDir := t.TempDir()
	writeBrowserPlan(t, projectDir, validBrowserPlan())

	plan, err := LoadBrowserPlan(projectDir)
	if err != nil {
		t.Fatalf("LoadBrowserPlan() error = %v", err)
	}
	if got, want := len(plan.Coverage), 2; got != want {
		t.Fatalf("coverage denominator has %d rows, want %d", got, want)
	}
	recipe, err := plan.RecipeForCoverage("screen-admin-01")
	if err != nil {
		t.Fatalf("RecipeForCoverage() error = %v", err)
	}
	if recipe.UserID != "administrator" || recipe.UserProfile != "admin" || recipe.ProtectedTarget.ExpectedRole != "admin" {
		t.Fatalf("application-specific account/role was not resolved from Planning: %+v", recipe)
	}
	if len(recipe.CoverageIDs) != 1 || recipe.CoverageIDs[0] != "screen-admin-01" || recipe.CoverageRequirement != "PRD-C17 FR-07" || recipe.CoverageScreenJourney != "/admin" || !recipe.CoverageMandatory {
		t.Fatalf("coverage identity was not copied into the per-attempt recipe: %+v", recipe)
	}
	if recipe.Authentication != AuthenticationForm || recipe.Method != MethodPlaywrightTestSuite || recipe.EvidenceRequirements[0] != "protected screen rendered" {
		t.Fatalf("login, selected method, and evidence contract were not resolved: %+v", recipe)
	}
	if got, want := plan.Coverage[0].OptionalReferenceWidths, []int{1280, 768, 375}; !slices.Equal(got, want) {
		t.Fatalf("optional responsive references = %v, want %v", got, want)
	}
}

func TestBrowserPlanAllowsExplicitHTTPOnlyQAE2EWithoutBrowserTool(t *testing.T) {
	plan := validBrowserPlan()
	plan.Method = BrowserMethodPlan{
		Stage:   StageQAEndToEnd,
		Purpose: PurposeRepeatableE2E,
		Method:  MethodHTTP,
	}

	if err := plan.Validate(); err != nil {
		t.Fatalf("Validate() rejected an explicitly selected HTTP-only QA end-to-end plan: %v", err)
	}
	if !plan.IsHTTPOnlyE2E() {
		t.Fatal("plan was not identified as explicit HTTP-only QA end-to-end")
	}

	projectDir := t.TempDir()
	writeBrowserPlan(t, projectDir, plan)
	loaded, err := LoadBrowserPlan(projectDir)
	if err != nil {
		t.Fatalf("LoadBrowserPlan() rejected the HTTP-only plan: %v", err)
	}
	if loaded.Method.ToolName != "" || loaded.Method.ToolVersion != "" || loaded.Method.PlaywrightVersion != "" {
		t.Fatalf("HTTP-only plan fabricated browser tool capability: %+v", loaded.Method)
	}
	if _, err := loaded.RecipeForCoverage("screen-operator-01"); err == nil || !strings.Contains(err.Error(), "browser executor") {
		t.Fatalf("HTTP-only plan produced a browser recipe or unclear error: %v", err)
	}
}

func TestBrowserPlanRequiresExplicitHTTPOnlyQAE2ESelection(t *testing.T) {
	tests := []struct {
		name   string
		stage  ValidationStage
		method ExecutionMethod
	}{
		{name: "missing stage", method: MethodHTTP},
		{name: "wrong stage", stage: StageBrowserUIValidation, method: MethodHTTP},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := validBrowserPlan()
			plan.Method = BrowserMethodPlan{Stage: tt.stage, Purpose: PurposeRepeatableE2E, Method: tt.method}
			if err := plan.Validate(); err == nil {
				t.Fatal("HTTP method without explicit QA end-to-end selection was accepted")
			}
		})
	}

	t.Run("HTTP cannot claim browser capability", func(t *testing.T) {
		plan := validBrowserPlan()
		plan.Method = BrowserMethodPlan{Stage: StageQAEndToEnd, Purpose: PurposeRepeatableE2E, Method: MethodHTTP, ToolName: "playwright"}
		if err := plan.Validate(); err == nil {
			t.Fatal("HTTP-only plan claiming a browser tool was accepted")
		}
	})
}

func TestBrowserPlanValidatesOptionalReferenceWidths(t *testing.T) {
	for _, tt := range []struct {
		name   string
		widths []int
	}{
		{name: "unsupported", widths: []int{1024}},
		{name: "duplicate", widths: []int{1280, 1280}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			plan := validBrowserPlan()
			plan.Coverage[0].OptionalReferenceWidths = tt.widths
			if err := plan.Validate(); err == nil {
				t.Fatalf("unsupported reference widths %v were accepted", tt.widths)
			}
		})
	}

	t.Run("references remain optional", func(t *testing.T) {
		plan := validBrowserPlan()
		plan.Coverage[0].OptionalReferenceWidths = nil
		if err := plan.Validate(); err != nil {
			t.Fatalf("coverage without optional references was rejected: %v", err)
		}
	})
}

func TestBrowserPlanStrictLoaderRejectsAmbiguousOrUnsafeInput(t *testing.T) {
	t.Run("unknown secret field", func(t *testing.T) {
		projectDir := t.TempDir()
		data := writeBrowserPlan(t, projectDir, validBrowserPlan())
		const sentinel = "SENTINEL_PLAN_PASSWORD_13ac"
		data = bytesReplaceOnce(data, []byte(`"coverage":`), []byte(`"password":"`+sentinel+`","coverage":`))
		path := filepath.Join(projectDir, filepath.FromSlash(BrowserPlanRelativePath))
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal("write malformed plan")
		}
		_, err := LoadBrowserPlan(projectDir)
		if err == nil || strings.Contains(err.Error(), sentinel) {
			t.Fatalf("unknown credential field was accepted or disclosed: %v", err)
		}
	})

	t.Run("duplicate field", func(t *testing.T) {
		projectDir := t.TempDir()
		data := writeBrowserPlan(t, projectDir, validBrowserPlan())
		data = bytesReplaceOnce(data, []byte(`"schema_version": 1,`), []byte(`"schema_version": 1, "schema_version": 1,`))
		path := filepath.Join(projectDir, filepath.FromSlash(BrowserPlanRelativePath))
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal("write duplicate-key plan")
		}
		if _, err := LoadBrowserPlan(projectDir); err == nil {
			t.Fatal("duplicate JSON key was accepted")
		}
	})

	t.Run("symlink plan file", func(t *testing.T) {
		projectDir := t.TempDir()
		path := filepath.Join(projectDir, filepath.FromSlash(BrowserPlanRelativePath))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal("create current-cycle plan directory")
		}
		outside := filepath.Join(t.TempDir(), "plan.json")
		if err := os.WriteFile(outside, []byte(`{}`), 0o600); err != nil {
			t.Fatal("write outside plan")
		}
		if err := os.Symlink(outside, path); err != nil {
			t.Skipf("symlinks are unavailable: %v", err)
		}
		if _, err := LoadBrowserPlan(projectDir); err == nil {
			t.Fatal("symlinked plan file was accepted")
		}
	})
}

func TestPreparationPlanUsesCurrentOptInAndSelectedCoverage(t *testing.T) {
	projectDir := t.TempDir()
	plan := validBrowserPlan()
	writeBrowserPlan(t, projectDir, plan)
	capability := &fakePreparationCapability{}
	preparer := NewPreparer(capability, newFakePreparationClock(time.Unix(1_800_000_000, 0)), preparationLogger())

	blocked := preparer.PreparePlan(context.Background(), projectDir, "screen-operator-01", false, time.Minute)
	if blocked.Status != PreparationBlocked || blocked.Reason != PreparationReasonTestAccessDisabled || len(capability.snapshotCalls()) != 0 {
		t.Fatalf("disabled required-login setup did not block before admission: %+v calls=%v", blocked, capability.snapshotCalls())
	}

	ready := preparer.PreparePlan(context.Background(), projectDir, "screen-admin-01", true, time.Minute)
	if ready.Status != PreparationReady || ready.Reason != "" {
		t.Fatalf("enabled selected coverage did not prepare: %+v", ready)
	}
	calls := capability.snapshotCalls()
	if len(calls) != len(preparationPrerequisites) {
		t.Fatalf("prerequisite call count=%d, want %d", len(calls), len(preparationPrerequisites))
	}
	last := calls[len(calls)-1]
	if last.prerequisite != PrerequisiteMethodAdmission || last.method != MethodPlaywrightTestSuite || len(last.coverageIDs) != 1 || last.coverageIDs[0] != "screen-admin-01" {
		t.Fatalf("active session did not admit the selected planned method/coverage: %+v", last)
	}
}

func TestPreparationPlanBlocksBelowMinimumPlaywrightWithSetupAction(t *testing.T) {
	projectDir := t.TempDir()
	plan := validBrowserPlan()
	plan.Method.PlaywrightVersion = "1.62.9"
	writeBrowserPlan(t, projectDir, plan)
	capability := &fakePreparationCapability{}
	result := NewPreparer(capability, newFakePreparationClock(time.Unix(1_800_000_000, 0)), preparationLogger()).PreparePlan(context.Background(), projectDir, "screen-operator-01", true, time.Minute)
	if result.Status != PreparationBlocked || result.Reason != PreparationReasonToolVersion || !strings.Contains(result.NextAction, "1.63.0") || len(capability.snapshotCalls()) != 0 {
		t.Fatalf("below-minimum planned tool did not block with setup guidance: %+v", result)
	}
}

func TestPreparationPlanAdmitsExplicitHTTPOnlyE2EWithReadinessOnly(t *testing.T) {
	projectDir := t.TempDir()
	plan := validBrowserPlan()
	plan.Method = BrowserMethodPlan{Stage: StageQAEndToEnd, Purpose: PurposeRepeatableE2E, Method: MethodHTTP}
	writeBrowserPlan(t, projectDir, plan)
	capability := &fakePreparationCapability{}

	result := NewPreparer(capability, newFakePreparationClock(time.Unix(1_800_000_000, 0)), preparationLogger()).PreparePlan(context.Background(), projectDir, "screen-operator-01", false, time.Minute)
	if result.Status != PreparationReady || result.Reason != "" {
		t.Fatalf("explicit HTTP-only plan did not pass readiness-only preparation: %+v", result)
	}
	calls := capability.snapshotCalls()
	if len(calls) != 1 || calls[0].prerequisite != PrerequisiteServiceReadiness || calls[0].method != MethodHTTP {
		t.Fatalf("HTTP-only plan did not restrict preparation to service readiness: %v", calls)
	}
	if result.Attempts[PrerequisiteMethodAdmission] != 0 || result.Attempts[PrerequisiteSelectedAccount] != 0 {
		t.Fatalf("HTTP-only plan attempted browser method or credential admission: %+v", result.Attempts)
	}
}

func bytesReplaceOnce(input, old, replacement []byte) []byte {
	return []byte(strings.Replace(string(input), string(old), string(replacement), 1))
}
