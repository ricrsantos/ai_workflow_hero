package tui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reports"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/screenshots"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/testaccess"
)

const stageScreenshotEvidencePrefix = ".workflow-hero/cycles/current/"

type stagedReportCoverage struct {
	ID            string                `json:"id"`
	Result        string                `json:"result"`
	Evidence      []string              `json:"evidence"`
	CaptureSafety reports.CaptureSafety `json:"capture_safety"`
}

type stageScreenshotIngestResult struct {
	ReportJSON []byte
	Warnings   []string
	BlockedIDs []string
}

// ingestStageScreenshots promotes only report-referenced staged files. The
// typed decoder is first given a narrow validator for exact stage-attempt
// staging paths; the returned report is then revalidated by the normal gate
// after each path has become a ready manifest reference.
func (m model) ingestStageScreenshots(stage string, reportJSON []byte, ctx reports.DecodeContext) stageScreenshotIngestResult {
	result := stageScreenshotIngestResult{ReportJSON: append([]byte(nil), reportJSON...)}
	if m.svc == nil || m.svc.Store == nil || m.svc.ProjectDir == "" || !isValidationHandoffStage(stage) {
		return result
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(reportJSON, &root) != nil {
		return result
	}
	var coverageWire struct {
		Items []stagedReportCoverage `json:"items"`
	}
	if json.Unmarshal(root["coverage"], &coverageWire) != nil || !hasStagedScreenshotReference(coverageWire.Items) {
		return result
	}
	cycle, err := m.svc.Store.GetActiveCycle()
	if err != nil {
		return result
	}
	stageRow, err := m.svc.Store.GetStage(cycle.ID, stage)
	if err != nil || stageRow.Status != store.StageRunning || stageRow.Iteration <= 0 {
		return result
	}
	plan, err := testaccess.LoadBrowserPlan(m.svc.ProjectDir)
	if err != nil {
		return result
	}
	planByID := make(map[string]testaccess.CoverageItem, len(plan.Coverage))
	for _, item := range plan.Coverage {
		planByID[item.ID] = item
	}
	stagedContext := ctx
	originalValidator := ctx.EvidenceReferenceValidator
	stagedContext.EvidenceReferenceValidator = func(reference string) bool {
		if originalValidator != nil && originalValidator(reference) {
			return true
		}
		return screenshots.IsStagedEvidenceReference(reference, stage, stageRow.Iteration)
	}
	if validationReportAccepts(stage, reportJSON, stagedContext) != nil {
		return result
	}

	var coverageObject map[string]json.RawMessage
	if json.Unmarshal(root["coverage"], &coverageObject) != nil {
		return result
	}
	var rawItems []json.RawMessage
	if json.Unmarshal(coverageObject["items"], &rawItems) != nil {
		return result
	}
	service, err := screenshots.NewService(m.svc.ProjectDir, m.svc.Store)
	if err != nil {
		return result
	}
	for i, rawItem := range rawItems {
		var itemMap map[string]json.RawMessage
		if json.Unmarshal(rawItem, &itemMap) != nil {
			return result
		}
		var item stagedReportCoverage
		if json.Unmarshal(rawItem, &item) != nil {
			return result
		}
		planned, plannedOK := planByID[item.ID]
		if !plannedOK {
			return result
		}
		var keptEvidence []string
		itemSawStaged := false
		itemCaptureReady := false
		for _, reference := range item.Evidence {
			if !strings.Contains(reference, "/screenshots/.staging/") {
				keptEvidence = append(keptEvidence, reference)
				continue
			}
			itemSawStaged = true
			source, sourceErr := screenshots.NewStagedFileCaptureSource(m.svc.ProjectDir, reference, stage, stageRow.Iteration, screenshots.CaptureAttestation{
				StablePointVerified:               item.CaptureSafety.StablePointVerified,
				SensitiveFieldsAndTokensMasked:    item.CaptureSafety.SensitiveFieldsAndTokensMasked,
				CredentialFlowArtifactsSuppressed: item.CaptureSafety.CredentialFlowArtifactsSuppressed,
			})
			var outcome screenshots.Outcome
			var captureErr error
			if sourceErr != nil {
				captureErr = sourceErr
			} else {
				request := screenshots.Request{
					CycleID: cycle.ID, StageName: stage, Attempt: stageRow.Iteration,
					CoverageID: item.ID, UserID: planned.UserID, ProfileID: planned.Profile,
					Result: item.Result,
				}
				if planned.Mandatory {
					outcome, captureErr = service.CapturePlannedEvidence(context.Background(), request, source)
				} else {
					outcome, captureErr = service.Capture(context.Background(), request, source)
				}
			}
			removeErr := screenshots.RemoveStagedEvidence(m.svc.ProjectDir, reference, stage, stageRow.Iteration)
			switch {
			case captureErr == nil && outcome.State == screenshots.OutcomeReady && outcome.Manifest != nil:
				keptEvidence = append(keptEvidence, stageScreenshotEvidencePrefix+outcome.Manifest.Path)
				itemCaptureReady = true
			case planned.Mandatory:
				result.BlockedIDs = appendUnique(result.BlockedIDs, item.ID)
				if removeErr != nil {
					result.Warnings = appendUnique(result.Warnings, "A staged screenshot could not be safely removed after capture.")
				}
			case captureErr != nil || outcome.State != screenshots.OutcomeDisabled:
				result.Warnings = appendUnique(result.Warnings, "An optional screenshot could not be safely published; validation continues.")
			}
			if removeErr != nil && !planned.Mandatory {
				result.Warnings = appendUnique(result.Warnings, "An optional staged screenshot could not be safely removed; validation continues.")
			}
		}
		if itemSawStaged {
			encoded, err := json.Marshal(keptEvidence)
			if err != nil {
				return result
			}
			itemMap["evidence"] = encoded
			if containsStringValue(result.BlockedIDs, item.ID) {
				itemMap["result"], _ = json.Marshal("blocked")
			} else if !itemCaptureReady && len(keptEvidence) == 0 && planned.Mandatory && item.Result == "passed" {
				itemMap["result"], _ = json.Marshal("blocked")
				result.BlockedIDs = appendUnique(result.BlockedIDs, item.ID)
			}
			rawItems[i], err = json.Marshal(itemMap)
			if err != nil {
				return result
			}
		}
	}
	coverageObject["items"], err = json.Marshal(rawItems)
	if err != nil {
		return result
	}
	root["coverage"], err = json.Marshal(coverageObject)
	if err != nil {
		return result
	}
	if len(result.BlockedIDs) > 0 {
		root["status"], _ = json.Marshal("blocked")
		var blockers []reports.Blocker
		_ = json.Unmarshal(root["blockers"], &blockers)
		for _, id := range result.BlockedIDs {
			planned := planByID[id]
			blocker := reports.Blocker{
				ID:                  stableScreenshotBlockerID(id),
				Reason:              "screenshot_evidence_unavailable",
				AffectedCoverageIDs: []string{id},
				Uncertainty:         "The planned post-assertion screenshot was not safely published.",
				NextAction:          "Resolve the screenshot capture issue and run /hero-continue to recheck the blocked coverage item.",
			}
			if planned.Profile != "" {
				blocker.AffectedProfileIDs = []string{planned.Profile}
			}
			blockers = append(blockers, blocker)
		}
		root["blockers"], _ = json.Marshal(blockers)
	}
	result.ReportJSON, err = json.Marshal(root)
	if err != nil {
		result.ReportJSON = append([]byte(nil), reportJSON...)
		result.Warnings = nil
		result.BlockedIDs = nil
	}
	return result
}

func validationReportAccepts(stage string, raw []byte, ctx reports.DecodeContext) *reports.DiagnosticError {
	if stage == stageBrowserUI {
		_, err := reports.DecodeBrowserUI(raw, ctx)
		return err
	}
	if stage == stageQAEndToEnd {
		_, err := reports.DecodeQAEndToEnd(raw, ctx)
		return err
	}
	return nil
}

func hasStagedScreenshotReference(items []stagedReportCoverage) bool {
	for _, item := range items {
		for _, reference := range item.Evidence {
			if strings.Contains(reference, "/screenshots/.staging/") {
				return true
			}
		}
	}
	return false
}

func stableScreenshotBlockerID(coverageID string) string {
	hash := sha256.Sum256([]byte(coverageID))
	return "screenshot-evidence-" + hex.EncodeToString(hash[:6])
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func containsStringValue(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}
