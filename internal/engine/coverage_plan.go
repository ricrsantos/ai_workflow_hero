package engine

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/testaccess"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

// ensureApprovedCoveragePlan snapshots Planning scope before a browser
// validation stage begins. A later plan edit is never accepted implicitly.
func (e *Engine) ensureApprovedCoveragePlan(cycleID int64, stageName string) error {
	if !isBrowserValidationStage(stageName) {
		return nil
	}
	plan, err := e.loadStageBrowserPlan(stageName)
	if err != nil {
		return err
	}
	digest, err := browserPlanDigest(plan)
	if err != nil {
		return err
	}
	snapshot, err := e.Store.GetStageCoveragePlanSnapshot(cycleID, stageName)
	switch {
	case err == nil:
		if snapshot.Digest != digest {
			return store.ErrCoveragePlanChanged
		}
		return e.verifyStoredCoverageMatchesPlan(cycleID, stageName, plan)
	case !errors.Is(err, store.ErrNotFound):
		return err
	}

	rows, err := e.Store.ListStageCoverage(cycleID, stageName)
	if err != nil {
		return err
	}
	if len(rows) != 0 {
		return errors.New("legacy coverage rows lack an approved plan snapshot; update Planning and approve the coverage plan before validation")
	}
	coverage := stageCoverageRowsFromPlan(plan)
	return e.Store.InTx(func(tx *sql.Tx) error {
		return e.Store.CreateStageCoverageSnapshotTx(tx, cycleID, stageName, coverage, digest, e.now())
	})
}

func stageCoverageRowsFromPlan(plan testaccess.BrowserPlan) []store.StageCoverage {
	coverage := make([]store.StageCoverage, 0, len(plan.Coverage))
	for _, item := range plan.Coverage {
		coverage = append(coverage, store.StageCoverage{
			ID: item.ID, UserID: item.UserID, ProfileID: item.Profile, Mandatory: item.Mandatory,
			RequirementRef: item.RequirementRef, ScreenJourney: item.ScreenOrJourney,
			ExpectedResult:          item.ExpectedResult,
			EvidenceRequirements:    append([]string(nil), item.EvidenceRequirements...),
			OptionalReferenceWidths: append([]int(nil), item.OptionalReferenceWidths...),
		})
	}
	return coverage
}

// verifyApprovedCoveragePlan rejects a plan changed after admission.
func (e *Engine) verifyApprovedCoveragePlan(cycleID int64, stageName string) error {
	if !isBrowserValidationStage(stageName) {
		return nil
	}
	plan, err := e.loadStageBrowserPlan(stageName)
	if err != nil {
		return err
	}
	digest, err := browserPlanDigest(plan)
	if err != nil {
		return err
	}
	snapshot, err := e.Store.GetStageCoveragePlanSnapshot(cycleID, stageName)
	if err != nil {
		return errors.New("approved browser coverage snapshot is unavailable; update Planning and approve the plan before continuing")
	}
	if snapshot.Digest != digest {
		return store.ErrCoveragePlanChanged
	}
	return e.verifyStoredCoverageMatchesPlan(cycleID, stageName, plan)
}

// applyApprovedCoveragePlanUpdate refreshes an existing denominator only as
// part of the explicit human approval of an updated Planning stage.
func (e *Engine) applyApprovedCoveragePlanUpdate(cycleID int64) error {
	if e == nil || e.Store == nil || !filepath.IsAbs(e.ProjectDir) || filepath.Clean(e.ProjectDir) != e.ProjectDir {
		return nil
	}
	plan, err := testaccess.LoadBrowserPlan(e.ProjectDir)
	if err != nil {
		return nil // Cycles without a Planning-owned browser plan have no denominator to update.
	}
	stageName := string(plan.Method.Stage)
	if stageName == "" {
		switch plan.Method.Purpose {
		case testaccess.PurposeBrowserControl:
			stageName = "browser_ui_validation"
		case testaccess.PurposeRepeatableE2E:
			stageName = "qa_end_to_end"
		}
	}
	if !isBrowserValidationStage(stageName) {
		return errors.New("updated browser plan does not identify a validation stage")
	}
	plan, err = e.loadStageBrowserPlan(stageName)
	if err != nil {
		return err
	}
	snapshot, err := e.Store.GetStageCoveragePlanSnapshot(cycleID, stageName)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	digest, err := browserPlanDigest(plan)
	if err != nil || snapshot.Digest == digest {
		return err
	}
	rows := stageCoverageRowsFromPlan(plan)
	return e.Store.InTx(func(tx *sql.Tx) error {
		return e.Store.ReplaceStageCoverageSnapshotApprovedTx(tx, cycleID, stageName, rows, digest, e.now(), true)
	})
}

func (e *Engine) loadStageBrowserPlan(stageName string) (testaccess.BrowserPlan, error) {
	if e == nil || e.Store == nil || !filepath.IsAbs(e.ProjectDir) || filepath.Clean(e.ProjectDir) != e.ProjectDir {
		return testaccess.BrowserPlan{}, errors.New("browser validation requires a project root and an approved Planning plan")
	}
	doc, err := workflowconfig.LoadCurrentDocument(e.ProjectDir)
	if err != nil {
		return testaccess.BrowserPlan{}, errors.New("current workflow configuration cannot be verified for browser validation")
	}
	stage, ok := doc.Config.Stages[stageName]
	if !ok || !stage.Enabled {
		return testaccess.BrowserPlan{}, fmt.Errorf("browser validation stage %s is not enabled in current configuration", stageName)
	}
	plan, err := testaccess.LoadBrowserPlan(e.ProjectDir)
	if err != nil {
		return testaccess.BrowserPlan{}, errors.New("planning-owned browser plan is missing or invalid")
	}
	if plan.Method.Stage != "" && string(plan.Method.Stage) != stageName {
		return testaccess.BrowserPlan{}, errors.New("planning-owned browser method is assigned to a different validation stage")
	}
	switch stageName {
	case "browser_ui_validation":
		if plan.Method.Purpose != testaccess.PurposeBrowserControl || plan.Method.Method == testaccess.MethodHTTP {
			return testaccess.BrowserPlan{}, errors.New("browser UI Validation requires the approved real-browser method")
		}
	case "qa_end_to_end":
		if plan.Method.Purpose != testaccess.PurposeRepeatableE2E {
			return testaccess.BrowserPlan{}, errors.New("qa end-to-end plan purpose does not match the selected stage")
		}
		if stage.UsePlaywright && plan.Method.Method == testaccess.MethodHTTP {
			return testaccess.BrowserPlan{}, errors.New("qa end-to-end selected Playwright but the approved plan selects HTTP")
		}
		if !stage.UsePlaywright {
			if !doc.HasExplicitStageField(stageName, "use_playwright") {
				return testaccess.BrowserPlan{}, errors.New("HTTP-only QA End-to-End requires an explicit use_playwright:false setting")
			}
			if stage.Screenshots.Enabled {
				return testaccess.BrowserPlan{}, errors.New("HTTP-only QA End-to-End cannot enable browser screenshots")
			}
			if plan.Method.Method != testaccess.MethodHTTP {
				return testaccess.BrowserPlan{}, errors.New("HTTP-only QA End-to-End requires an explicit HTTP plan")
			}
		}
	}
	return plan, nil
}

func (e *Engine) verifyStoredCoverageMatchesPlan(cycleID int64, stageName string, plan testaccess.BrowserPlan) error {
	rows, err := e.Store.ListStageCoverage(cycleID, stageName)
	if err != nil {
		return err
	}
	if len(rows) != len(plan.Coverage) {
		return store.ErrCoveragePlanChanged
	}
	byID := make(map[string]store.StageCoverage, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	for _, item := range plan.Coverage {
		row, ok := byID[item.ID]
		if !ok || row.UserID != item.UserID || row.ProfileID != item.Profile || row.Mandatory != item.Mandatory ||
			row.RequirementRef != item.RequirementRef || row.ScreenJourney != item.ScreenOrJourney ||
			row.ExpectedResult != item.ExpectedResult || !sameStrings(row.EvidenceRequirements, item.EvidenceRequirements) ||
			!sameInts(row.OptionalReferenceWidths, item.OptionalReferenceWidths) {
			return store.ErrCoveragePlanChanged
		}
	}
	return nil
}

func browserPlanDigest(plan testaccess.BrowserPlan) (string, error) {
	data, err := json.Marshal(plan)
	if err != nil {
		return "", errors.New("cannot fingerprint the approved browser plan")
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func isBrowserValidationStage(stageName string) bool {
	return stageName == "browser_ui_validation" || stageName == "qa_end_to_end"
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
