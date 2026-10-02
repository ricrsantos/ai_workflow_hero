package engine

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/testaccess"
)

func TestStartStageSnapshotsPlanningCoverageAndRejectsSilentScopeChange(t *testing.T) {
	e, st := openTestEngine(t)
	project := t.TempDir()
	current := filepath.Join(project, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(current, 0o700); err != nil {
		t.Fatal(err)
	}
	config := `title: synthetic
objective: exercise the traceable browser coverage gate
scope:
  frontend: true
stages:
  browser_ui_validation:
    enabled: true
    timeout_minutes: 10
`
	if err := os.WriteFile(filepath.Join(current, "workflow-config.yml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	e.ProjectDir = project

	plan := syntheticBrowserUIPPlan()
	planPath := filepath.Join(current, "browser-plan.json")
	writeSyntheticPlan(t, planPath, plan)

	cycleID, err := st.CreateCycle(store.Cycle{
		Number: 1, Title: "coverage", Objective: "preserve the approved denominator",
		Status: store.CycleStatusActive, StartedAt: "2026-08-07T12:00:00Z", ConfigSnapshotJSON: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateStages([]store.Stage{{CycleID: cycleID, Name: "browser_ui_validation", Status: store.StageWaiting, MaxIterations: 2}}); err != nil {
		t.Fatal(err)
	}
	if err := e.StartStage(cycleID, "browser_ui_validation"); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListStageCoverage(cycleID, "browser_ui_validation")
	if err != nil || len(rows) != 1 {
		t.Fatalf("seeded coverage rows = %+v, err = %v", rows, err)
	}
	if rows[0].ID != "screen-home-anonymous" || rows[0].RequirementRef != "FR-07/AC-1" || rows[0].ProfileID != "anonymous" || !rows[0].Mandatory {
		t.Fatalf("scheduler-owned coverage metadata = %+v", rows[0])
	}
	decodeCtx, err := e.ValidationDecodeContextForStage(cycleID, "browser_ui_validation")
	if err != nil || len(decodeCtx.CoveragePlan) != 1 {
		t.Fatalf("scheduler coverage decode context = %+v, err = %v", decodeCtx.CoveragePlan, err)
	}
	if decodeCtx.ExpectedBrowserMethod != "cli" || decodeCtx.ExpectedBrowserTool != "playwright" || decodeCtx.MinimumPlaywrightVersion != testaccess.MinimumPlaywrightVersion {
		t.Fatalf("scheduler did not pin the approved browser method and tool: method=%q tool=%q minimum=%q", decodeCtx.ExpectedBrowserMethod, decodeCtx.ExpectedBrowserTool, decodeCtx.MinimumPlaywrightVersion)
	}
	item := decodeCtx.CoveragePlan[0]
	if item.Requirement != "FR-07/AC-1" || item.Acceptance != rows[0].ExpectedResult || item.ScreenJourney != "home screen" ||
		!sameInts(item.OptionalReferenceWidths, []int{768, 375}) || len(item.EvidenceRequirements) != 1 {
		t.Fatalf("typed report lost Planning traceability: %+v", item)
	}
	if _, err := st.GetStageCoveragePlanSnapshot(cycleID, "browser_ui_validation"); err != nil {
		t.Fatalf("approved plan digest unavailable: %v", err)
	}

	stage, err := st.GetStage(cycleID, "browser_ui_validation")
	if err != nil {
		t.Fatal(err)
	}
	stage.Status = store.StageWaiting
	stage.CompletedAt = ""
	if err := st.UpdateStage(stage); err != nil {
		t.Fatal(err)
	}
	plan.Coverage[0].ExpectedResult = "an unapproved changed outcome"
	writeSyntheticPlan(t, planPath, plan)
	if err := e.StartStage(cycleID, "browser_ui_validation"); !errors.Is(err, store.ErrCoveragePlanChanged) {
		t.Fatalf("changed plan start error = %v, want ErrCoveragePlanChanged", err)
	}
	stage, err = st.GetStage(cycleID, "browser_ui_validation")
	if err != nil || stage.Status != store.StageWaiting {
		t.Fatalf("changed plan consumed a stage attempt: stage=%+v err=%v", stage, err)
	}
}

func TestPlanningApprovalAtomicallyAdmitsUpdatedCoveragePlan(t *testing.T) {
	e, st := openTestEngine(t)
	project := t.TempDir()
	current := filepath.Join(project, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(current, 0o700); err != nil {
		t.Fatal(err)
	}
	config := `title: synthetic
objective: approve updated browser coverage
workflow_config:
  user_preferred_language: EN
scope:
  frontend: true
stages:
  planning:
    enabled: true
  browser_ui_validation:
    enabled: true
    timeout_minutes: 10
`
	if err := os.WriteFile(filepath.Join(current, "workflow-config.yml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	e.ProjectDir = project
	plan := syntheticBrowserUIPPlan()
	planPath := filepath.Join(current, "browser-plan.json")
	writeSyntheticPlan(t, planPath, plan)

	cycleID, err := st.CreateCycle(store.Cycle{Number: 1, Title: "approval", Objective: "update coverage", Status: store.CycleStatusActive, ConfigSnapshotJSON: `{}`})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateStages([]store.Stage{
		{CycleID: cycleID, Name: "planning", Status: store.StagePendingApproval, MaxIterations: 2, SortOrder: 0},
		{CycleID: cycleID, Name: "browser_ui_validation", Status: store.StageWaiting, MaxIterations: 2, SortOrder: 1},
	}); err != nil {
		t.Fatal(err)
	}
	oldDigest, err := browserPlanDigest(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.InTx(func(tx *sql.Tx) error {
		return st.CreateStageCoverageSnapshotTx(tx, cycleID, "browser_ui_validation", stageCoverageRowsFromPlan(plan), oldDigest, e.now())
	}); err != nil {
		t.Fatal(err)
	}

	plan.Coverage[0].ExpectedResult = "updated, explicitly approved outcome"
	writeSyntheticPlan(t, planPath, plan)
	if err := e.Approve("test-approval", "updated browser plan", nil); err != nil {
		t.Fatal(err)
	}
	newDigest, err := browserPlanDigest(plan)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := st.GetStageCoveragePlanSnapshot(cycleID, "browser_ui_validation")
	if err != nil || snapshot.Digest != newDigest || snapshot.Digest == oldDigest {
		t.Fatalf("approved plan snapshot = %+v, err = %v", snapshot, err)
	}
	rows, err := st.ListStageCoverage(cycleID, "browser_ui_validation")
	if err != nil || len(rows) != 1 || rows[0].ExpectedResult != plan.Coverage[0].ExpectedResult || rows[0].Result != store.StageCoveragePlanned {
		t.Fatalf("approved denominator = %+v, err = %v", rows, err)
	}
}

func syntheticBrowserUIPPlan() testaccess.BrowserPlan {
	return testaccess.BrowserPlan{
		SchemaVersion: testaccess.BrowserPlanSchemaVersion,
		Execution: testaccess.BrowserExecutionContract{
			Environment: "synthetic local fixture", BaseURL: "http://127.0.0.1:43127",
			ApprovedOrigins: []string{"http://127.0.0.1:43127"}, StartCommand: "fixture starts in-process",
			ReadinessCommand: "fixture readiness endpoint", ActionTimeout: "2s", TestTimeout: "10s",
			Fixtures: []string{"synthetic-public-screen"}, EvidencePaths: []string{"current/screenshots"},
		},
		Authentication: testaccess.BrowserAuthenticationPlan{Requirement: testaccess.AuthenticationNotNeeded, Flow: testaccess.AuthenticationNone},
		Method: testaccess.BrowserMethodPlan{
			Stage: testaccess.StageBrowserUIValidation, Purpose: testaccess.PurposeBrowserControl,
			Method: testaccess.MethodPlaywrightCLINoSkill, ToolName: "playwright", ToolVersion: testaccess.MinimumPlaywrightVersion, ToolVersionCommand: []string{"playwright", "--version"},
			PlaywrightVersion: testaccess.MinimumPlaywrightVersion,
		},
		Coverage: []testaccess.CoverageItem{{
			ID: "screen-home-anonymous", RequirementRef: "FR-07/AC-1", ScreenOrJourney: "home screen",
			Profile: "anonymous", Mandatory: true, ExpectedResult: "home screen renders for an anonymous user",
			EvidenceRequirements: []string{"rendered screen"}, OptionalReferenceWidths: []int{768, 375},
			ProtectedTarget: testaccess.ProtectedTargetRecipe{
				URL: "http://127.0.0.1:43127/", ExpectedRole: "anonymous", ExpectedAccess: testaccess.AccessAllowed,
			},
		}},
	}
}

func writeSyntheticPlan(t *testing.T, path string, plan testaccess.BrowserPlan) {
	t.Helper()
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
