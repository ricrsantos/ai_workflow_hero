package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reports"
)

func TestStageHandoffPromotesAttestedScreenshotAndRewritesEvidence(t *testing.T) {
	m, svc, cycleID, _ := newBudgetTUIFixture(t, stageBrowserUI, 10)
	stagedReference := writeStageScreenshotForIngest(t, svc.ProjectDir, stageBrowserUI, 1)
	ctx, err := svc.ValidationDecodeContextForStage(stageBrowserUI)
	if err != nil {
		t.Fatal(err)
	}
	raw := stagedBlockedBrowserReport(t, stagedReference, true)
	result := m.ingestStageScreenshots(stageBrowserUI, raw, ctx)
	if len(result.BlockedIDs) != 0 || len(result.Warnings) != 0 || bytes.Equal(result.ReportJSON, raw) {
		t.Fatalf("attested screenshot ingestion result=%+v; expected rewritten ready evidence", result)
	}
	var decoded struct {
		Coverage struct {
			Items []reports.CoverageItem `json:"items"`
		} `json:"coverage"`
	}
	if err := json.Unmarshal(result.ReportJSON, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Coverage.Items) != 2 || len(decoded.Coverage.Items[0].Evidence) != 1 || strings.Contains(decoded.Coverage.Items[0].Evidence[0], ".staging/") {
		t.Fatalf("normalized evidence=%+v; staged path should be replaced", decoded.Coverage.Items)
	}
	if !ctx.EvidenceReferenceValidator(decoded.Coverage.Items[0].Evidence[0]) {
		ctx, err = svc.ValidationDecodeContextForStage(stageBrowserUI)
		if err != nil {
			t.Fatal(err)
		}
		if !ctx.EvidenceReferenceValidator(decoded.Coverage.Items[0].Evidence[0]) {
			t.Fatalf("published evidence is not in the validated ready set: %q", decoded.Coverage.Items[0].Evidence[0])
		}
	}
	if _, err := os.Stat(filepath.Join(svc.ProjectDir, filepath.FromSlash(stagedReference))); !os.IsNotExist(err) {
		t.Fatalf("staging image remains after ingestion, stat error=%v", err)
	}
	if _, err := reports.DecodeBrowserUI(result.ReportJSON, ctx); err != nil {
		t.Fatalf("rewritten report did not pass the normal evidence validator: %v", err)
	}
	ready, err := svc.Store.ListReadyScreenshotManifests(context.Background(), cycleID)
	if err != nil || len(ready) != 1 || ready[0].CoverageID != "screen-dashboard-admin" || ready[0].UserID != "administrator" || ready[0].ProfileID != "admin" {
		t.Fatalf("ready screenshot metadata=%+v err=%v", ready, err)
	}
}

func TestStageHandoffBlocksCoverageWhenCaptureDoesNotAttestSuppression(t *testing.T) {
	m, svc, _, _ := newBudgetTUIFixture(t, stageBrowserUI, 10)
	stagedReference := writeStageScreenshotForIngest(t, svc.ProjectDir, stageBrowserUI, 1)
	ctx, err := svc.ValidationDecodeContextForStage(stageBrowserUI)
	if err != nil {
		t.Fatal(err)
	}
	raw := stagedBlockedBrowserReport(t, stagedReference, false)
	result := m.ingestStageScreenshots(stageBrowserUI, raw, ctx)
	if len(result.BlockedIDs) != 1 || result.BlockedIDs[0] != "screen-dashboard-admin" {
		t.Fatalf("unsafe mandatory capture blocked IDs=%v, want its coverage item", result.BlockedIDs)
	}
	var normalized struct {
		Status   string `json:"status"`
		Blockers []struct {
			Reason string `json:"reason"`
		} `json:"blockers"`
		Coverage struct {
			Items []reports.CoverageItem `json:"items"`
		} `json:"coverage"`
	}
	if err := json.Unmarshal(result.ReportJSON, &normalized); err != nil {
		t.Fatal(err)
	}
	if normalized.Status != "blocked" || normalized.Coverage.Items[0].Result != "blocked" || len(normalized.Coverage.Items[0].Evidence) != 0 {
		t.Fatalf("unsafe evidence was not blocked and removed: %+v", normalized)
	}
	found := false
	for _, blocker := range normalized.Blockers {
		if blocker.Reason == "screenshot_evidence_unavailable" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing screenshot evidence blocker: %+v", normalized.Blockers)
	}
	if _, err := reports.DecodeBrowserUI(result.ReportJSON, ctx); err != nil {
		t.Fatalf("safe blocked report did not validate: %v", err)
	}
}

func stagedBlockedBrowserReport(t *testing.T, stagedReference string, suppressionAttested bool) []byte {
	t.Helper()
	report := map[string]any{
		"status":  "blocked",
		"summary": "synthetic stage report",
		"preparation": reports.Preparation{
			Status: "ok", Method: "cli", VerifiedProfileIDs: []string{"admin"},
			MethodAdmitted: true, ObservedToolName: "playwright", ObservedToolVersion: "1.63.0", ObservedPlaywrightVersion: "1.63.0",
		},
		"blockers": []reports.Blocker{{
			ID: "fixtures-pending", Reason: "fixtures_unavailable", AffectedCoverageIDs: []string{"screen-settings-operator"},
			AffectedProfileIDs: []string{"operator"}, Uncertainty: "the optional settings screen was not checked", NextAction: "make its fixture available and run /hero-continue",
		}},
		"coverage": reports.Coverage{
			PlannedIDs: []string{"screen-dashboard-admin", "screen-settings-operator"},
			Items: []reports.CoverageItem{
				{ID: "screen-dashboard-admin", Result: "passed", Evidence: []string{stagedReference}, Checks: reports.CoverageChecks{Render: true, CSS: true, Console: true, Network: true, HealthBeforeVisual: true, DesktopWidth: 1280}, CaptureSafety: reports.CaptureSafety{StablePointVerified: true, SensitiveFieldsAndTokensMasked: true, CredentialFlowArtifactsSuppressed: suppressionAttested}},
				{ID: "screen-settings-operator", Result: "blocked", Evidence: []string{}, Checks: reports.CoverageChecks{}},
			},
		},
		"failures": []any{},
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeStageScreenshotForIngest(t *testing.T, projectDir, stage string, attempt int) string {
	t.Helper()
	reference := stageScreenshotEvidencePrefix + "screenshots/.staging/" + stage + "/1/after-assertion.png"
	directory := filepath.Join(projectDir, filepath.FromSlash(filepath.Dir(reference)))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []struct {
		path string
		mode os.FileMode
	}{
		{filepath.Join(projectDir, ".workflow-hero"), 0o755},
		{filepath.Join(projectDir, ".workflow-hero", "cycles"), 0o755},
		{filepath.Join(projectDir, ".workflow-hero", "cycles", "current"), 0o755},
		{filepath.Join(projectDir, ".workflow-hero", "cycles", "current", "screenshots"), 0o700},
		{directory, 0o700},
	} {
		if err := os.Chmod(dir.path, dir.mode); err != nil {
			t.Fatal(err)
		}
	}
	var imageData bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 12, G: 23, B: 34, A: 255})
	if err := png.Encode(&imageData, img); err != nil {
		t.Fatal(err)
	}
	if attempt != 1 {
		t.Fatalf("test setup expects attempt 1, got %d", attempt)
	}
	if err := os.WriteFile(filepath.Join(projectDir, filepath.FromSlash(reference)), imageData.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return reference
}
