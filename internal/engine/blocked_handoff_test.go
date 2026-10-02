package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reports"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/testaccess"
)

func seedBlockedValidation(t *testing.T, e *Engine, st *store.Store, stageName string, mandatory bool) (int64, string) {
	return seedBlockedValidationWithMode(t, e, st, stageName, mandatory, nil)
}

func seedBlockedValidationWithMode(t *testing.T, e *Engine, st *store.Store, stageName string, mandatory bool, e2eUsePlaywright *bool) (int64, string) {
	t.Helper()
	project := t.TempDir()
	current := filepath.Join(project, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(current, 0o700); err != nil {
		t.Fatal(err)
	}
	config := `title: synthetic
objective: exercise blocked browser validation
workflow_config:
  user_preferred_language: EN
scope:
  frontend: true
stages:
  browser_ui_validation:
    enabled: true
    max_iterations: 2
    timeout_minutes: 10
  qa_end_to_end:
    enabled: true
    max_iterations: 2
    timeout_minutes: 10
    use_playwright: true
`
	configuredPlaywright := true
	if stageName == reports.SourceQAEndToEnd && e2eUsePlaywright != nil {
		configuredPlaywright = *e2eUsePlaywright
		config = strings.Replace(config, "    use_playwright: true", "    use_playwright: "+map[bool]string{true: "true", false: "false"}[configuredPlaywright], 1)
	}
	if err := os.WriteFile(filepath.Join(current, "workflow-config.yml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	writeSyntheticBrowserPlan(t, project, stageName, mandatory, configuredPlaywright)
	e.ProjectDir = project
	e.BlockedPrerequisiteCheck = func(context.Context, int64, string) error { return nil }

	cycleID, err := st.CreateCycle(store.Cycle{
		Number: 1, Title: "blocked", Objective: "validate browser reports", Status: store.CycleStatusActive,
		StartedAt: "2026-08-07T12:00:00Z", ConfigSnapshotJSON: `{"scope":{"frontend":true}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	stages := []store.Stage{
		{CycleID: cycleID, Name: "research", Status: store.StageCompleted, Iteration: 1, MaxIterations: 2, SortOrder: 0},
		{CycleID: cycleID, Name: "planning", Status: store.StageCompleted, Iteration: 1, MaxIterations: 2, SortOrder: 1},
		{CycleID: cycleID, Name: "implementation", Status: store.StageCompleted, Iteration: 1, MaxIterations: 2, SortOrder: 2},
	}
	if stageName == reports.SourceQAEndToEnd {
		stages = append(stages, store.Stage{CycleID: cycleID, Name: reports.SourceQAEndToEnd, Status: store.StageWaiting, MaxIterations: 2, SortOrder: 3})
	} else {
		stages = append(stages,
			store.Stage{CycleID: cycleID, Name: stageName, Status: store.StageWaiting, MaxIterations: 2, SortOrder: 3},
			store.Stage{CycleID: cycleID, Name: reports.SourceQAEndToEnd, Status: store.StageWaiting, MaxIterations: 2, SortOrder: 4},
		)
	}
	if err := st.CreateStages(stages); err != nil {
		t.Fatal(err)
	}
	if err := e.ensureApprovedCoveragePlan(cycleID, stageName); err != nil {
		t.Fatal(err)
	}
	if err := e.StartStage(cycleID, stageName); err != nil {
		t.Fatal(err)
	}
	return cycleID, project
}

func writeSyntheticBrowserPlan(t *testing.T, project, stageName string, mandatory, usePlaywright bool) {
	t.Helper()
	stage := testaccess.StageBrowserUIValidation
	purpose := testaccess.PurposeBrowserControl
	method := testaccess.MethodPlaywrightCLI
	toolName := "playwright"
	toolVersion := testaccess.MinimumPlaywrightVersion
	playwrightVersion := testaccess.MinimumPlaywrightVersion
	e2eCommand := ""
	if stageName == reports.SourceQAEndToEnd {
		stage = testaccess.StageQAEndToEnd
		purpose = testaccess.PurposeRepeatableE2E
		e2eCommand = "run synthetic order outcome assertions"
		if !usePlaywright {
			method = testaccess.MethodHTTP
			toolName, toolVersion, playwrightVersion = "", "", ""
		} else {
			method = testaccess.MethodPlaywrightTestSuite
		}
	}
	toolVersionCommand := []string{"playwright", "--version"}
	if method == testaccess.MethodHTTP {
		toolVersionCommand = nil
	}
	plan := testaccess.BrowserPlan{
		SchemaVersion: testaccess.BrowserPlanSchemaVersion,
		Execution: testaccess.BrowserExecutionContract{
			Environment: "synthetic local fixture", BaseURL: "http://127.0.0.1:43127",
			ApprovedOrigins: []string{"http://127.0.0.1:43127"}, StartCommand: "start synthetic fixture",
			ReadinessCommand: "check synthetic fixture readiness", E2ECommand: e2eCommand,
			ActionTimeout: "2s", TestTimeout: "10s", Fixtures: []string{}, EvidencePaths: []string{},
		},
		Authentication: testaccess.BrowserAuthenticationPlan{
			Requirement: testaccess.AuthenticationNotNeeded, Flow: testaccess.AuthenticationNone,
		},
		Method: testaccess.BrowserMethodPlan{
			Stage: stage, Purpose: purpose, Method: method, ToolName: toolName,
			ToolVersion: toolVersion, ToolVersionCommand: toolVersionCommand, PlaywrightVersion: playwrightVersion,
			ExistingPlaywrightSuite:   method == testaccess.MethodPlaywrightTestSuite,
			OfficialCLISkillAvailable: method == testaccess.MethodPlaywrightCLI,
		},
		Coverage: []testaccess.CoverageItem{{
			ID: "screen-dashboard", RequirementRef: "FR-07", ScreenOrJourney: "/dashboard",
			Profile: "anonymous", Mandatory: mandatory, ExpectedResult: "protected dashboard renders",
			EvidenceRequirements: []string{"dashboard result"}, ProtectedTarget: testaccess.ProtectedTargetRecipe{
				URL: "http://127.0.0.1:43127/dashboard", ExpectedRole: "anonymous", ExpectedAccess: testaccess.AccessAllowed,
			},
		}},
	}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, filepath.FromSlash(testaccess.BrowserPlanRelativePath))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func blockedReportJSON(t *testing.T, stageName, preparationStatus string, failures []map[string]any) []byte {
	method := "cli_skill"
	var usePlaywright *bool
	if stageName == reports.SourceQAEndToEnd {
		method = "playwright_test"
		usePlaywright = boolPointer(true)
	}
	return blockedReportJSONWithMode(t, stageName, preparationStatus, failures, method, usePlaywright)
}

func blockedReportJSONWithMode(t *testing.T, stageName, preparationStatus string, failures []map[string]any, method string, usePlaywright *bool) []byte {
	t.Helper()
	blocker := map[string]any{
		"id": "browser-tool-unavailable", "reason": "tool_unavailable",
		"affected_coverage_ids": []string{"screen-dashboard"}, "affected_profile_ids": []string{},
		"uncertainty": "the protected browser journey was not verified",
		"next_action": "configure the planned browser method and retry",
	}
	preparation := reports.Preparation{Status: preparationStatus, Method: method, VerifiedProfileIDs: []string{}}
	if preparationStatus == "ok" && method != "http" {
		preparation.MethodAdmitted = true
		preparation.ObservedToolName = "playwright"
		preparation.ObservedToolVersion = testaccess.MinimumPlaywrightVersion
		preparation.ObservedPlaywrightVersion = testaccess.MinimumPlaywrightVersion
	}
	root := map[string]any{
		"status": "blocked", "summary": "synthetic blocked browser validation",
		"preparation": preparation,
		"blockers":    []any{blocker},
		"coverage":    reports.Coverage{PlannedIDs: []string{"screen-dashboard"}, Items: []reports.CoverageItem{{ID: "screen-dashboard", Result: "blocked", Evidence: []string{}, Checks: reports.CoverageChecks{}}}},
		"failures":    failures,
	}
	if stageName == reports.SourceQAEndToEnd && usePlaywright != nil {
		root["use_playwright"] = *usePlaywright
	}
	data, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func boolPointer(value bool) *bool { return &value }

func setE2EConfig(t *testing.T, project string, usePlaywright *bool, screenshots bool) {
	t.Helper()
	path := filepath.Join(project, ".workflow-hero", "cycles", "current", "workflow-config.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	config := string(raw)
	marker := "  qa_end_to_end:\n"
	idx := strings.Index(config, marker)
	if idx < 0 {
		t.Fatal("qa_end_to_end configuration is missing")
	}
	block := marker + "    enabled: true\n    max_iterations: 2\n    timeout_minutes: 10\n"
	if usePlaywright != nil {
		block += "    use_playwright: " + map[bool]string{true: "true", false: "false"}[*usePlaywright] + "\n"
	}
	if screenshots {
		block += "    screenshots:\n      enabled: true\n"
	}
	if err := os.WriteFile(path, []byte(config[:idx]+block), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBlockedStatePersistsAndDoesNotAutoAdvance(t *testing.T) {
	e, st := openTestEngine(t)
	cycleID, _ := seedBlockedValidation(t, e, st, reports.SourceBrowserUI, true)

	_, err := e.CloseStageBlockedWithReport(cycleID, reports.SourceBrowserUI, blockedReportJSON(t, reports.SourceBrowserUI, "blocked", nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	stage, err := st.GetStage(cycleID, reports.SourceBrowserUI)
	if err != nil || stage.Status != store.StageBlocked || stage.Iteration != 0 {
		t.Fatalf("blocked stage = %+v, err = %v; want Blocked and no consumed preflight iteration", stage, err)
	}
	implementation, _ := st.GetStage(cycleID, "implementation")
	endToEnd, _ := st.GetStage(cycleID, reports.SourceQAEndToEnd)
	if implementation.Status != store.StageCompleted || endToEnd.Status != store.StageWaiting {
		t.Fatalf("blocked close auto-transitioned stages: implementation=%s e2e=%s", implementation.Status, endToEnd.Status)
	}
	blockers, err := st.ListActiveStageBlockers(cycleID, reports.SourceBrowserUI)
	if err != nil || len(blockers) != 1 || blockers[0].Reason != "tool_unavailable" {
		t.Fatalf("blockers=%+v err=%v", blockers, err)
	}
	coverage, err := st.ListStageCoverage(cycleID, reports.SourceBrowserUI)
	if err != nil || len(coverage) != 1 || coverage[0].Result != store.StageCoverageBlocked || !coverage[0].Mandatory {
		t.Fatalf("coverage=%+v err=%v", coverage, err)
	}
	events, err := st.ListEvents(cycleID, store.EventStageBlocked, 0)
	if err != nil || len(events) != 1 {
		t.Fatalf("blocked events=%+v err=%v", events, err)
	}
	var event map[string]any
	if err := json.Unmarshal([]byte(events[0].PayloadJSON), &event); err != nil || event["stage"] != reports.SourceBrowserUI {
		t.Fatalf("blocked event=%s err=%v", events[0].PayloadJSON, err)
	}
	loopbacks, _ := st.ListEvents(cycleID, store.EventLoopBack, 0)
	if len(loopbacks) != 0 {
		t.Fatalf("blocked report scheduled repair: %+v", loopbacks)
	}
}

func TestBlockedReportPersistsOnlySchedulerOwnedSummary(t *testing.T) {
	e, st := openTestEngine(t)
	cycleID, _ := seedBlockedValidation(t, e, st, reports.SourceBrowserUI, true)
	var report map[string]any
	if err := json.Unmarshal(blockedReportJSON(t, reports.SourceBrowserUI, "blocked", nil), &report); err != nil {
		t.Fatal(err)
	}
	report["summary"] = "SENTINEL_REPORT_SECRET summary"
	blocker := report["blockers"].([]any)[0].(map[string]any)
	blocker["reason"] = "SENTINEL_REPORT_SECRET"
	blocker["uncertainty"] = "SENTINEL_REPORT_SECRET uncertainty"
	blocker["next_action"] = "SENTINEL_REPORT_SECRET action"
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.CloseStageBlockedWithReport(cycleID, reports.SourceBrowserUI, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.Summary, "SENTINEL_REPORT_SECRET") || !strings.Contains(out.Summary, "Browser UI Validation blocked") {
		t.Fatalf("close summary=%q, want scheduler-owned summary", out.Summary)
	}
	stage, err := st.GetStage(cycleID, reports.SourceBrowserUI)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stage.Summary, "SENTINEL_REPORT_SECRET") {
		t.Fatalf("stage summary persisted agent text: %q", stage.Summary)
	}
	conversation, err := st.ListConversation(cycleID)
	if err != nil {
		t.Fatal(err)
	}
	var storedReport string
	for _, entry := range conversation {
		if entry.Kind == "validation_report" {
			storedReport = entry.Body
		}
	}
	if storedReport == "" || strings.Contains(storedReport, "SENTINEL_REPORT_SECRET") || strings.Contains(storedReport, `"failures"`) {
		t.Fatalf("stored validation report is not a minimal scheduler summary: %q", storedReport)
	}
	events, err := st.ListEvents(cycleID, store.EventStageBlocked, 0)
	if err != nil || len(events) != 1 || strings.Contains(events[0].PayloadJSON, "SENTINEL_REPORT_SECRET") {
		t.Fatalf("blocked event retained report text: %+v err=%v", events, err)
	}
	blockers, err := st.ListActiveStageBlockers(cycleID, reports.SourceBrowserUI)
	if err != nil || len(blockers) != 1 {
		t.Fatalf("blockers=%+v err=%v", blockers, err)
	}
	if blockers[0].Reason != "prerequisite_check_failed" || strings.Contains(blockers[0].DiagnosticUncertainty, "SENTINEL_REPORT_SECRET") || strings.Contains(blockers[0].NextAction, "SENTINEL_REPORT_SECRET") {
		t.Fatalf("blocker persisted agent diagnostics: %+v", blockers[0])
	}
}

func TestBlockedBrowserUIRejectsHTTPFallback(t *testing.T) {
	e, st := openTestEngine(t)
	cycleID, _ := seedBlockedValidation(t, e, st, reports.SourceBrowserUI, true)
	_, err := e.CloseStageBlockedWithReport(cycleID, reports.SourceBrowserUI,
		blockedReportJSONWithMode(t, reports.SourceBrowserUI, "blocked", nil, "http", nil), nil)
	var validationErr *ReportValidationError
	if !errors.As(err, &validationErr) || validationErr.Diagnostic.Field != "preparation.method" {
		t.Fatalf("HTTP fallback error=%v, want Browser UI method rejection", err)
	}
	stage, err := st.GetStage(cycleID, reports.SourceBrowserUI)
	if err != nil || stage.Status != store.StageRunning {
		t.Fatalf("rejected HTTP report mutated stage: status=%s err=%v", stage.Status, err)
	}
}

func TestBlockedReportRegisteredEvidenceUsesProjectRoot(t *testing.T) {
	e, st := openTestEngine(t)
	cycleID, project := seedBlockedValidation(t, e, st, reports.SourceBrowserUI, true)
	ref := ".workflow-hero/cycles/current/health.txt"
	if err := os.WriteFile(filepath.Join(project, filepath.FromSlash(ref)), []byte("synthetic health evidence"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddArtifact(store.Artifact{CycleID: cycleID, Kind: "evidence", Path: ref}); err != nil {
		t.Fatal(err)
	}
	validate := e.blockedEvidenceReferenceValidator(cycleID)
	if !validate(ref) {
		t.Fatal("registered evidence was not resolved from its actual project-root path")
	}
	if validate(".workflow-hero/cycles/current/unregistered.txt") {
		t.Fatal("unregistered evidence was admitted")
	}
}

func TestBlockedReportRejectsUnapprovedDenominatorBeforeMutation(t *testing.T) {
	e, st := openTestEngine(t)
	cycleID, _ := seedBlockedValidation(t, e, st, reports.SourceBrowserUI, true)
	raw := strings.ReplaceAll(string(blockedReportJSON(t, reports.SourceBrowserUI, "blocked", nil)), "screen-dashboard", "invented-screen")
	_, err := e.CloseStageBlockedWithReport(cycleID, reports.SourceBrowserUI, []byte(raw), nil)
	var validationErr *ReportValidationError
	if !errors.As(err, &validationErr) || validationErr.Diagnostic.Code != reports.CodeAssignmentUnionMismatch {
		t.Fatalf("error=%v, want denominator validation diagnostic", err)
	}
	stage, _ := st.GetStage(cycleID, reports.SourceBrowserUI)
	blockers, _ := st.ListActiveStageBlockers(cycleID, reports.SourceBrowserUI)
	coverage, _ := st.ListStageCoverage(cycleID, reports.SourceBrowserUI)
	if stage.Status != store.StageRunning || len(blockers) != 0 || len(coverage) != 1 || coverage[0].Result != store.StageCoveragePlanned {
		t.Fatalf("invalid report mutated state: stage=%s blockers=%+v coverage=%+v", stage.Status, blockers, coverage)
	}
}

func TestMixedBlockedFindingsPersistWithoutRepairUntilContinue(t *testing.T) {
	e, st := openTestEngine(t)
	cycleID, _ := seedBlockedValidation(t, e, st, reports.SourceBrowserUI, true)
	failures := []map[string]any{{
		"failure_class": "frontend", "file": "screen.go", "requirement": "FR-07",
		"issue": "synthetic protected screen rendering fault", "acceptance_criteria": "dashboard renders",
		"evidence": []string{"synthetic failure"},
		"repro":    map[string]string{"mode": "evidence"},
	}}
	_, err := e.CloseStageBlockedWithReport(cycleID, reports.SourceBrowserUI, blockedReportJSON(t, reports.SourceBrowserUI, "ok", failures), nil)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := st.ListFindingsByCycle(cycleID)
	if err != nil || len(findings) != 1 || findings[0].Status != store.FindingStatusOpen {
		t.Fatalf("findings=%+v err=%v", findings, err)
	}
	implementation, _ := st.GetStage(cycleID, "implementation")
	if implementation.Status != store.StageCompleted {
		t.Fatalf("blocked mixed report prematurely scheduled repair: %s", implementation.Status)
	}
	loopbacks, _ := st.ListEvents(cycleID, store.EventLoopBack, 0)
	if len(loopbacks) != 0 {
		t.Fatalf("blocked mixed report looped back before Continue: %+v", loopbacks)
	}
	if err := e.Continue("blocked-test", 1); err != nil {
		t.Fatal(err)
	}
	implementation, _ = st.GetStage(cycleID, "implementation")
	validation, _ := st.GetStage(cycleID, reports.SourceBrowserUI)
	if implementation.Status != store.StageWaiting || validation.Status != store.StageWaiting {
		t.Fatalf("Continue did not queue repair then validation: implementation=%s validation=%s", implementation.Status, validation.Status)
	}
	loopbacks, _ = st.ListEvents(cycleID, store.EventLoopBack, 0)
	if len(loopbacks) != 1 {
		t.Fatalf("Continue should schedule exactly one repair loop-back: %+v", loopbacks)
	}
	active, _ := st.ListActiveStageBlockers(cycleID, reports.SourceBrowserUI)
	if len(active) != 0 {
		t.Fatalf("Continue left active blockers: %+v", active)
	}
}

func TestHeroContinueRechecksCurrentBrowserCapability(t *testing.T) {
	e, st := openTestEngine(t)
	cycleID, _ := seedBlockedValidation(t, e, st, reports.SourceBrowserUI, false)
	if _, err := e.CloseStageBlockedWithReport(cycleID, reports.SourceBrowserUI, blockedReportJSON(t, reports.SourceBrowserUI, "blocked", nil), nil); err != nil {
		t.Fatal(err)
	}
	e.BlockedPrerequisiteCheck = func(context.Context, int64, string) error { return errors.New("missing tool") }
	if err := e.Continue("blocked-test", 1); err == nil || !strings.Contains(err.Error(), "validation method remains unavailable") {
		t.Fatalf("Continue error=%v, want current capability recheck", err)
	}
	stage, _ := st.GetStage(cycleID, reports.SourceBrowserUI)
	active, _ := st.ListActiveStageBlockers(cycleID, reports.SourceBrowserUI)
	if stage.Status != store.StageBlocked || len(active) != 1 {
		t.Fatalf("failed prerequisite recheck mutated blocked state: stage=%s blockers=%+v", stage.Status, active)
	}
	e.BlockedPrerequisiteCheck = func(context.Context, int64, string) error { return nil }
	if err := e.Continue("blocked-test", 1); err != nil {
		t.Fatal(err)
	}
	stage, _ = st.GetStage(cycleID, reports.SourceBrowserUI)
	if stage.Status != store.StageWaiting {
		t.Fatalf("successful recheck status=%s want Waiting", stage.Status)
	}
}

func TestHeroContinueRecheckAllowsExplicitHTTPQAEndToEnd(t *testing.T) {
	e, st := openTestEngine(t)
	cycleID, _ := seedBlockedValidationWithMode(t, e, st, reports.SourceQAEndToEnd, false, boolPointer(false))
	raw := blockedReportJSONWithMode(t, reports.SourceQAEndToEnd, "blocked", nil, "http", boolPointer(false))
	if _, err := e.CloseStageBlockedWithReport(cycleID, reports.SourceQAEndToEnd, raw, nil); err != nil {
		t.Fatalf("close explicit HTTP-only blocked report: %v", err)
	}
	checks := 0
	e.BlockedPrerequisiteCheck = func(_ context.Context, gotCycle int64, stageName string) error {
		checks++
		if gotCycle != cycleID || stageName != reports.SourceQAEndToEnd {
			t.Fatalf("prerequisite check got cycle=%d stage=%q", gotCycle, stageName)
		}
		return nil
	}
	if err := e.Continue("blocked-test", 1); err != nil {
		t.Fatalf("Continue explicit HTTP-only plan: %v", err)
	}
	stage, err := st.GetStage(cycleID, reports.SourceQAEndToEnd)
	if err != nil || stage.Status != store.StageWaiting || checks != 1 {
		t.Fatalf("stage=%s checks=%d err=%v; want Waiting after selected-method recheck", stage.Status, checks, err)
	}
}

func TestHeroContinueRecheckRejectsImplicitOrInconsistentHTTPPlan(t *testing.T) {
	for _, tc := range []struct {
		name      string
		configure func(*testing.T, string)
		wantErr   string
	}{
		{
			name: "implicit mode",
			configure: func(t *testing.T, project string) {
				setE2EConfig(t, project, nil, false)
			},
			wantErr: "requires an explicit use_playwright:false setting",
		},
		{
			name: "screenshots enabled",
			configure: func(t *testing.T, project string) {
				setE2EConfig(t, project, boolPointer(false), true)
			},
			wantErr: "cannot enable browser screenshots",
		},
		{
			name: "current mode disagrees with accepted HTTP report",
			configure: func(t *testing.T, project string) {
				setE2EConfig(t, project, boolPointer(true), false)
			},
			wantErr: "selected Playwright but the approved plan selects HTTP",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, st := openTestEngine(t)
			cycleID, project := seedBlockedValidationWithMode(t, e, st, reports.SourceQAEndToEnd, false, boolPointer(false))
			raw := blockedReportJSONWithMode(t, reports.SourceQAEndToEnd, "blocked", nil, "http", boolPointer(false))
			if _, err := e.CloseStageBlockedWithReport(cycleID, reports.SourceQAEndToEnd, raw, nil); err != nil {
				t.Fatalf("close explicit HTTP-only blocked report: %v", err)
			}
			tc.configure(t, project)
			e.BlockedPrerequisiteCheck = func(context.Context, int64, string) error { return nil }
			if err := e.Continue("blocked-test", 1); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Continue error=%v, want %q", err, tc.wantErr)
			}
			stage, err := st.GetStage(cycleID, reports.SourceQAEndToEnd)
			if err != nil || stage.Status != store.StageBlocked {
				t.Fatalf("recheck mutated blocked stage: stage=%s err=%v", stage.Status, err)
			}
		})
	}
}

func TestBlockedReportTransactionRollsBackMixedData(t *testing.T) {
	e, st := openTestEngine(t)
	cycleID, _ := seedBlockedValidation(t, e, st, reports.SourceBrowserUI, true)
	failures := []map[string]any{{
		"failure_class": "frontend", "file": "screen.go", "requirement": "FR-07",
		"issue": "synthetic fault", "acceptance_criteria": "dashboard renders", "evidence": []string{"synthetic failure"},
		"repro": map[string]string{"mode": "evidence"},
	}}
	blockedHandoffTestHook = func(*sql.Tx) error { return errors.New("injected rollback") }
	t.Cleanup(func() { blockedHandoffTestHook = nil })
	_, err := e.CloseStageBlockedWithReport(cycleID, reports.SourceBrowserUI, blockedReportJSON(t, reports.SourceBrowserUI, "ok", failures), []MetricInput{{Agent: "browser_ui_agent", InputTokens: 20}})
	if err == nil {
		t.Fatal("injected transaction failure unexpectedly committed")
	}
	stage, _ := st.GetStage(cycleID, reports.SourceBrowserUI)
	findings, _ := st.ListFindingsByCycle(cycleID)
	blockers, _ := st.ListActiveStageBlockers(cycleID, reports.SourceBrowserUI)
	coverage, _ := st.ListStageCoverage(cycleID, reports.SourceBrowserUI)
	metrics, _ := st.ListMetrics(cycleID)
	events, _ := st.ListEvents(cycleID, store.EventStageBlocked, 0)
	if stage.Status != store.StageRunning || len(findings) != 0 || len(blockers) != 0 || len(metrics) != 0 || len(events) != 0 || len(coverage) != 1 || coverage[0].Result != store.StageCoveragePlanned {
		t.Fatalf("transaction leaked partial state: stage=%s findings=%+v blockers=%+v coverage=%+v metrics=%+v events=%+v", stage.Status, findings, blockers, coverage, metrics, events)
	}
}
