package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reports"
	cycleScreenshots "github.com/ricrsantos/ai_workflow_hero/internal/cycle/screenshots"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/testaccess"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

const blockedValidationPauseReason = "validation_blocked"

// BlockedCloseHandoffResult describes a validated report that is durably
// blocked pending an explicit Continue. Findings remain genuine C15 findings,
// but this operation never schedules their repair.
type BlockedCloseHandoffResult struct {
	FindingIDs           []string
	ActionableFindingIDs []string
	BlockerIDs           []string
	Summary              string
	Counts               reports.CoverageCounts
}

// blockedHandoffTestHook runs inside blocked report persistence transactions.
var blockedHandoffTestHook func(*sql.Tx) error

// CloseStageBlockedWithReport validates and atomically persists operational
// blockers, coverage outcomes, metrics and any genuine C15 findings. It does
// not loop back to Implementation or advance another stage.
func (e *Engine) CloseStageBlockedWithReport(cycleID int64, stageName string, reportJSON []byte, metrics []MetricInput) (BlockedCloseHandoffResult, error) {
	var empty BlockedCloseHandoffResult
	stageName = strings.TrimSpace(stageName)
	if stageName != reports.SourceBrowserUI && stageName != reports.SourceQAEndToEnd {
		return empty, fmt.Errorf("blocked validation close is not supported for stage %q", stageName)
	}
	if err := e.assertHandoffPreconditions(cycleID, stageName); err != nil {
		return empty, err
	}
	ctx, err := e.blockedReportDecodeContext(cycleID, stageName)
	if err != nil {
		return empty, err
	}
	report, derr := decodeBlockedValidationReport(stageName, reportJSON, ctx)
	if derr != nil {
		return empty, &ReportValidationError{Diagnostic: derr}
	}
	doc, err := workflowconfig.LoadCurrentDocument(e.ProjectDir)
	if err != nil {
		return empty, errors.New("current workflow configuration cannot be verified for the blocked report")
	}
	if err := validateBlockedReportMode(stageName, report.Method, report.UsePlaywright, doc); err != nil {
		return empty, err
	}
	summary := blockedValidationSafeSummary(stageName, report.PreparationStatus, report.Counts)

	var out BlockedCloseHandoffResult
	out.Summary = summary
	out.Counts = report.Counts
	var budgetExpired bool
	err = e.Store.InTx(func(tx *sql.Tx) error {
		budget, budgetErr := store.PauseStageBudgetBlockedTx(tx, cycleID, stageName, e.Now(), blockedValidationPauseReason)
		if errors.Is(budgetErr, store.ErrNotFound) {
			budgetErr = nil // Untimed stages have no ledger.
		}
		if budgetErr != nil {
			return budgetErr
		}
		if budget.State == store.StageBudgetExpired {
			budgetExpired = true
			return nil // Commit expiry; reject the report outside the transaction.
		}

		st, err := e.Store.GetStageTx(tx, cycleID, stageName)
		if err != nil {
			return err
		}
		if st.Status != store.StageRunning {
			return fmt.Errorf("stage %s is %s, expected Running", stageName, st.Status)
		}
		if err := e.persistMetricsTx(tx, cycleID, stageName, metrics); err != nil {
			return err
		}

		findingIDs := make([]string, 0, len(report.Failures))
		for _, entry := range report.Failures {
			result, err := e.Store.PersistFindingTx(tx, findingInputFromEntry(cycleID, stageName, entry))
			if err != nil {
				return err
			}
			findingIDs = appendUniqueString(findingIDs, result.Finding.ID)
			if result.Actionable {
				out.ActionableFindingIDs = appendUniqueString(out.ActionableFindingIDs, result.Finding.ID)
			}
		}
		out.FindingIDs = findingIDs

		if err := e.Store.PersistStageCoverageResultsTx(tx, cycleID, stageName, report.Coverage, e.now()); err != nil {
			return err
		}
		if err := e.Store.ReplaceActiveStageBlockersTx(tx, cycleID, stageName, report.Blockers, e.now()); err != nil {
			return err
		}

		// A pre-validation preparation block does not consume a validation
		// iteration. StartStage already incremented it at dispatch, so undo that
		// increment in this same acceptance transaction.
		if report.PreparationStatus == "blocked" && st.Iteration > 0 {
			st.Iteration--
		}
		st.Status = store.StageBlocked
		st.CompletedAt = e.now()
		st.Summary = summary
		if err := e.Store.UpdateStageTx(tx, st); err != nil {
			return err
		}
		for _, blocker := range report.Blockers {
			out.BlockerIDs = append(out.BlockerIDs, blocker.ID)
		}
		payload, err := json.Marshal(struct {
			Stage         string                 `json:"stage"`
			Status        string                 `json:"status"`
			Summary       string                 `json:"summary"`
			Preparation   string                 `json:"preparation"`
			Method        string                 `json:"method"`
			UsePlaywright *bool                  `json:"use_playwright,omitempty"`
			Coverage      reports.CoverageCounts `json:"coverage"`
			BlockerIDs    []string               `json:"blocker_ids"`
			FindingIDs    []string               `json:"finding_ids"`
			ActionableIDs []string               `json:"actionable_finding_ids"`
		}{
			Stage: stageName, Status: store.StageBlocked, Summary: summary,
			Preparation: report.PreparationStatus, Method: report.Method,
			UsePlaywright: report.UsePlaywright, Coverage: report.Counts,
			BlockerIDs: out.BlockerIDs, FindingIDs: out.FindingIDs,
			ActionableIDs: out.ActionableFindingIDs,
		})
		if err != nil {
			return fmt.Errorf("encode blocked-stage event: %w", err)
		}
		if _, err := e.Store.AppendEventTx(tx, store.Event{CycleID: cycleID, Type: store.EventStageBlocked, PayloadJSON: string(payload)}); err != nil {
			return err
		}
		if _, err := e.Store.AddConversationTx(tx, store.ConversationEntry{
			CycleID: cycleID, Role: "system", Kind: "validation_report", Body: string(payload),
		}); err != nil {
			return err
		}
		if blockedHandoffTestHook != nil {
			if err := blockedHandoffTestHook(tx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		e.Logger.Error("atomic blocked validation report failed", "cycle_id", cycleID, "stage", stageName, "error", err)
		return empty, err
	}
	if budgetExpired {
		return empty, store.ErrStageBudgetExpired
	}
	e.Logger.Info("atomic blocked validation report committed", "cycle_id", cycleID, "stage", stageName, "blocker_ids", out.BlockerIDs, "finding_ids", out.FindingIDs)
	return out, nil
}

// continueBlockedInCycle is called by the serialized Engine.Continue path
// before it searches for Escalated stages. It only changes durable scheduler
// state; the caller/TUI must explicitly dispatch any returned Waiting work.
func (e *Engine) continueBlockedInCycle(c store.Cycle, extra int) (bool, error) {
	stages, err := e.Store.ListStages(c.ID)
	if err != nil {
		return false, err
	}
	var blocked *store.Stage
	for i := range stages {
		if stages[i].Status == store.StageBlocked {
			blocked = &stages[i]
			break
		}
	}
	if blocked == nil {
		return false, nil
	}
	if extra <= 0 {
		extra = 1
	}
	if err := e.recheckBlockedPrerequisites(c.ID, blocked.Name); err != nil {
		return true, err
	}
	blockers, err := e.Store.ListActiveStageBlockers(c.ID, blocked.Name)
	if err != nil {
		return true, err
	}
	if len(blockers) == 0 {
		return true, errors.New("blocked stage has no active blocker record; refusing automatic continuation")
	}
	actionableIDs, summary, err := e.latestBlockedActionableFindings(c.ID, blocked.Name)
	if err != nil {
		return true, err
	}
	var budgetExpired bool
	err = e.Store.InTx(func(tx *sql.Tx) error {
		st, err := e.Store.GetStageTx(tx, c.ID, blocked.Name)
		if err != nil {
			return err
		}
		if st.Status != store.StageBlocked {
			return fmt.Errorf("stage %s changed while blocked continuation was being accepted", blocked.Name)
		}
		budget, budgetErr := store.ContinueBlockedStageBudgetTx(tx, c.ID, blocked.Name, e.Now())
		if errors.Is(budgetErr, store.ErrNotFound) {
			budgetErr = nil
		}
		if budgetErr != nil {
			return budgetErr
		}
		if budget.State == store.StageBudgetExpired {
			budgetExpired = true
			return nil
		}
		if err := e.Store.ResolveActiveStageBlockersTx(tx, c.ID, blocked.Name, e.now()); err != nil {
			return err
		}
		st.Status = store.StageWaiting
		st.StartedAt = ""
		st.CompletedAt = ""
		st.ExtraIterations += extra
		if err := e.Store.UpdateStageTx(tx, st); err != nil {
			return err
		}
		if len(actionableIDs) > 0 {
			if err := e.applyLoopBackInTx(tx, c.ID, blocked.Name, summary, actionableIDs); err != nil {
				return err
			}
		}
		payload, err := json.Marshal(struct {
			Stage      string   `json:"stage"`
			Blockers   int      `json:"resolved_blockers"`
			Extra      int      `json:"extra_iterations"`
			FindingIDs []string `json:"finding_ids,omitempty"`
		}{Stage: blocked.Name, Blockers: len(blockers), Extra: extra, FindingIDs: actionableIDs})
		if err != nil {
			return fmt.Errorf("encode blocked continuation event: %w", err)
		}
		_, err = e.Store.AppendEventTx(tx, store.Event{CycleID: c.ID, Type: store.EventContinued, PayloadJSON: string(payload)})
		return err
	})
	if err != nil {
		return true, err
	}
	if budgetExpired {
		return true, store.ErrStageBudgetExpired
	}
	return true, nil
}

func (e *Engine) recheckBlockedPrerequisites(cycleID int64, stageName string) error {
	if e == nil || e.Store == nil || !filepath.IsAbs(e.ProjectDir) || filepath.Clean(e.ProjectDir) != e.ProjectDir {
		return errors.New("current browser prerequisites cannot be verified; configure the project context and retry /hero-continue")
	}
	doc, err := workflowconfig.LoadCurrentDocument(e.ProjectDir)
	if err != nil {
		return errors.New("current workflow configuration cannot be verified; reload configuration and retry /hero-continue")
	}
	stage, ok := doc.Config.Stages[stageName]
	if !ok || !stage.Enabled {
		return fmt.Errorf("stage %s is disabled in current workflow configuration; enable it before /hero-continue", stageName)
	}
	plan, err := e.Store.ListStageCoverage(cycleID, stageName)
	if err != nil || len(plan) == 0 {
		return errors.New("approved coverage plan is unavailable; update Planning before /hero-continue")
	}
	if err := e.verifyApprovedCoveragePlan(cycleID, stageName); err != nil {
		return err
	}
	latest, err := e.latestBlockedReportMode(cycleID, stageName)
	if err != nil {
		return err
	}
	if err := validateBlockedReportMode(stageName, latest.Method, latest.UsePlaywright, doc); err != nil {
		return err
	}
	needsCredentials := false
	for _, item := range plan {
		if item.UserID != "" {
			needsCredentials = true
			break
		}
	}
	if needsCredentials {
		access, err := testaccess.OpenSafeStore(e.ProjectDir, e.Logger)
		if err != nil {
			return errors.New("required test-access configuration is unavailable or unsafe; reload the Test users editor before /hero-continue")
		}
		defer func() { _ = access.Close() }()
		for _, item := range plan {
			if item.UserID == "" {
				continue
			}
			snapshot, err := access.Snapshot(context.Background(), item.UserID)
			if err != nil || snapshot.Account().Profile() != item.ProfileID {
				return fmt.Errorf("required test account for profile %s is unavailable or unusable; update Test users and retry /hero-continue", item.ProfileID)
			}
		}
	}
	if e.BlockedPrerequisiteCheck == nil {
		return errors.New("the planned validation method cannot be verified by a trusted executor; configure it before /hero-continue")
	}
	if err := e.BlockedPrerequisiteCheck(context.Background(), cycleID, stageName); err != nil {
		return errors.New("the planned validation method remains unavailable; correct its setup before /hero-continue")
	}
	return nil
}

func validateBlockedReportMode(stageName, method string, usePlaywright *bool, doc *workflowconfig.Document) error {
	if doc == nil {
		return errors.New("current workflow configuration cannot be verified")
	}
	stage, ok := doc.Config.Stages[stageName]
	if !ok || !stage.Enabled {
		return fmt.Errorf("stage %s is disabled in current workflow configuration; enable it before /hero-continue", stageName)
	}
	if stageName == reports.SourceBrowserUI {
		if method == "http" {
			return errors.New("browser UI Validation requires a real browser method; HTTP fallback is not allowed")
		}
		return nil
	}
	if stageName != reports.SourceQAEndToEnd {
		return fmt.Errorf("stage %s does not have a browser execution mode", stageName)
	}
	if !doc.HasExplicitStageField(stageName, "use_playwright") {
		return errors.New("qa_end_to_end.use_playwright must be explicitly configured before /hero-continue")
	}
	if usePlaywright == nil {
		return errors.New("blocked E2E report must explicitly identify its selected browser or HTTP mode")
	}
	if *usePlaywright != stage.UsePlaywright {
		return errors.New("blocked E2E report mode does not match current qa_end_to_end.use_playwright configuration")
	}
	if stage.UsePlaywright {
		if method == "http" {
			return errors.New("qa_end_to_end requires browser execution; HTTP fallback is not allowed")
		}
		return nil
	}
	if stage.Screenshots.Enabled {
		return errors.New("HTTP-only qa_end_to_end cannot enable browser screenshots")
	}
	if method != "http" {
		return errors.New("qa_end_to_end.use_playwright:false requires the explicitly planned HTTP method")
	}
	return nil
}

func blockedValidationSafeSummary(stageName, preparation string, counts reports.CoverageCounts) string {
	stageLabel := "Browser validation"
	switch stageName {
	case reports.SourceBrowserUI:
		stageLabel = "Browser UI Validation"
	case reports.SourceQAEndToEnd:
		stageLabel = "QA End-to-End"
	}
	return fmt.Sprintf("%s blocked: preparation %s; coverage %d planned, %d passed, %d failed, %d blocked, %d skipped.",
		stageLabel, preparation, counts.Planned, counts.Passed, counts.Failed, counts.Blocked, counts.Skipped)
}

func (e *Engine) latestBlockedReportMode(cycleID int64, stageName string) (blockedEventPayload, error) {
	events, err := e.Store.ListEvents(cycleID, store.EventStageBlocked, 0)
	if err != nil {
		return blockedEventPayload{}, err
	}
	var latest blockedEventPayload
	for _, event := range events {
		var payload blockedEventPayload
		if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
			continue
		}
		if payload.Stage == stageName {
			latest = payload
		}
	}
	if latest.Stage == "" || latest.Method == "" {
		return blockedEventPayload{}, errors.New("blocked stage has no scheduler-owned validation method record")
	}
	return latest, nil
}

func (e *Engine) blockedReportDecodeContext(cycleID int64, stageName string) (reports.DecodeContext, error) {
	ctx, err := e.ValidationDecodeContextForStage(cycleID, stageName)
	if err != nil {
		if err.Error() != "approved browser coverage plan is unavailable" {
			return reports.DecodeContext{}, err
		}
		return reports.DecodeContext{}, &ReportValidationError{Diagnostic: &reports.DiagnosticError{
			Code: reports.CodeAssignmentUnionMismatch, Field: "coverage.planned_ids",
			Rule: "an approved Planning denominator is required before accepting browser validation",
		}}
	}
	return ctx, nil
}

func (e *Engine) blockedEvidenceReferenceValidator(cycleID int64) func(string) bool {
	const prefix = ".workflow-hero/cycles/current/"
	ready := make(map[string]struct{})
	if e != nil && e.ProjectDir != "" {
		service, err := cycleScreenshots.NewService(e.ProjectDir, e.Store)
		if err == nil {
			manifests, err := service.ReadySet(context.Background(), cycleID)
			if err == nil {
				for _, manifest := range manifests {
					ready[prefix+filepath.ToSlash(manifest.Path)] = struct{}{}
				}
			}
		}
	}
	artifacts, _ := e.Store.ListArtifacts(cycleID)
	registered := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		artifactPath := filepath.ToSlash(filepath.Clean(artifact.Path))
		registered[artifactPath] = struct{}{}
		registered[prefix+strings.TrimPrefix(artifactPath, prefix)] = struct{}{}
	}
	return func(reference string) bool {
		if _, ok := ready[reference]; ok {
			return true
		}
		if _, ok := registered[reference]; !ok || e == nil || e.ProjectDir == "" || !strings.HasPrefix(reference, prefix) {
			return false
		}
		// OpenRoot is anchored at the project, so retain the .workflow-hero
		// component of the registered managed evidence reference.
		relative := reference
		root, err := os.OpenRoot(e.ProjectDir)
		if err != nil {
			return false
		}
		defer func() { _ = root.Close() }()
		parts := strings.Split(filepath.FromSlash(relative), string(filepath.Separator))
		for i := range parts {
			part := strings.Join(parts[:i+1], string(filepath.Separator))
			info, err := root.Lstat(part)
			if err != nil || info.Mode()&os.ModeSymlink != 0 {
				return false
			}
			if i == len(parts)-1 && !info.Mode().IsRegular() {
				return false
			}
		}
		return true
	}
}

type decodedBlockedValidationReport struct {
	PreparationStatus string
	Method            string
	UsePlaywright     *bool
	Blockers          []store.StageBlocker
	Coverage          []store.StageCoverage
	Failures          []reports.FailureEntry
	Counts            reports.CoverageCounts
}

func decodeBlockedValidationReport(stageName string, data []byte, ctx reports.DecodeContext) (decodedBlockedValidationReport, *reports.DiagnosticError) {
	var result decodedBlockedValidationReport
	if stageName == reports.SourceBrowserUI {
		report, err := reports.DecodeBrowserUI(data, ctx)
		if err != nil {
			return result, err
		}
		if report.Status != reports.ValidationStatusBlocked {
			return result, reportsDiagInvalidEnum("status", report.Status, "atomic blocked close requires status blocked")
		}
		result.PreparationStatus = report.Preparation.Status
		result.Method = report.Preparation.Method
		result.Failures = report.Failures
		result.Counts = report.Counts
		for _, blocker := range report.Blockers {
			result.Blockers = append(result.Blockers, store.StageBlocker{
				ID: blocker.ID, Reason: safeBlockedValidationReason(blocker.Reason), AffectedCoverageIDs: blocker.AffectedCoverageIDs,
				AffectedProfileIDs: blocker.AffectedProfileIDs,
				NextAction:         "Review planned prerequisites and run /hero-continue.", Status: store.StageBlockerActive,
			})
		}
		result.Coverage = coverageRowsFromReport(stageName, report.Coverage, ctx.CoveragePlan)
		return result, nil
	}
	report, err := reports.DecodeQAEndToEnd(data, ctx)
	if err != nil {
		return result, err
	}
	if report.Status != reports.ValidationStatusBlocked {
		return result, reportsDiagInvalidEnum("status", report.Status, "atomic blocked close requires status blocked")
	}
	result.PreparationStatus = report.Preparation.Status
	result.Method = report.Preparation.Method
	result.UsePlaywright = report.UsePlaywright
	result.Failures = report.Failures
	result.Counts = report.Counts
	for _, blocker := range report.Blockers {
		result.Blockers = append(result.Blockers, store.StageBlocker{
			ID: blocker.ID, Reason: safeBlockedValidationReason(blocker.Reason), AffectedCoverageIDs: blocker.AffectedCoverageIDs,
			AffectedProfileIDs: blocker.AffectedProfileIDs,
			NextAction:         "Review planned prerequisites and run /hero-continue.", Status: store.StageBlockerActive,
		})
	}
	result.Coverage = coverageRowsFromReport(stageName, report.Coverage, ctx.CoveragePlan)
	return result, nil
}

func safeBlockedValidationReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case "service_unavailable", "browser_permission_unavailable", "selected_account_unavailable",
		"protected_access_unverified", "fixtures_unavailable", "method_unavailable",
		"tool_unavailable", "tool_missing", "tool_version", "tool_version_incompatible",
		"unsupported_authentication", "invalid_recipe", "test_access_disabled",
		"screenshot_evidence_unavailable",
		"preparation_attempt_timed_out", "stage_budget_exhausted", "prerequisite_attempt_limit",
		"preparation_interrupted", "preparation_capability_unavailable", "prerequisite_check_failed",
		"credentials_unavailable", "account_unusable", "invalid_account", "login_unverified",
		"role_unverified", "context_unavailable", "execution_unavailable", "session_cleanup",
		"artifact_suppression":
		return strings.TrimSpace(reason)
	default:
		return "prerequisite_check_failed"
	}
}

func coverageRowsFromReport(stageName string, reportCoverage *reports.Coverage, plan []reports.CoveragePlanItem) []store.StageCoverage {
	if reportCoverage == nil {
		return nil
	}
	planByID := make(map[string]reports.CoveragePlanItem, len(plan))
	for _, item := range plan {
		planByID[item.ID] = item
	}
	rows := make([]store.StageCoverage, 0, len(reportCoverage.Items))
	for _, item := range reportCoverage.Items {
		planned := planByID[item.ID]
		rows = append(rows, store.StageCoverage{
			StageName: stageName, ID: item.ID, UserID: planned.UserID, ProfileID: planned.ProfileID,
			Mandatory: planned.Mandatory, ExpectedResult: planned.ExpectedResult,
			Result: item.Result, Evidence: item.Evidence,
		})
	}
	return rows
}

func (e *Engine) latestBlockedActionableFindings(cycleID int64, stageName string) ([]string, string, error) {
	events, err := e.Store.ListEvents(cycleID, store.EventStageBlocked, 0)
	if err != nil {
		return nil, "", err
	}
	var latest blockedEventPayload
	for _, event := range events {
		var payload blockedEventPayload
		if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err == nil && payload.Stage == stageName {
			latest = payload
		}
	}
	if latest.Stage == "" {
		return nil, "", errors.New("blocked stage has no accepted report event")
	}
	findings, err := e.Store.ListFindingsByCycle(cycleID)
	if err != nil {
		return nil, "", err
	}
	statusByID := make(map[string]string, len(findings))
	for _, finding := range findings {
		statusByID[finding.ID] = finding.Status
	}
	active := make([]string, 0, len(latest.ActionableIDs))
	for _, id := range latest.ActionableIDs {
		status := statusByID[id]
		if status == store.FindingStatusOpen || status == store.FindingStatusReopened {
			active = appendUniqueString(active, id)
		}
	}
	return active, "browser validation findings require implementation correction before coverage can resume", nil
}

type blockedEventPayload struct {
	Stage         string   `json:"stage"`
	Status        string   `json:"status"`
	Preparation   string   `json:"preparation"`
	Method        string   `json:"method"`
	UsePlaywright *bool    `json:"use_playwright,omitempty"`
	BlockerIDs    []string `json:"blocker_ids"`
	FindingIDs    []string `json:"finding_ids"`
	ActionableIDs []string `json:"actionable_finding_ids"`
}
