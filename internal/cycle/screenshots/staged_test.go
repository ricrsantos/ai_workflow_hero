package screenshots

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeStagedScreenshot(t *testing.T, fixture *screenshotFixture, stage string, attempt int) string {
	t.Helper()
	relative := stagedReferencePrefix + stage + "/" + "1" + "/after-assertion.png"
	directory := filepath.Join(fixture.project, filepath.FromSlash(filepath.Dir(relative)))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatalf("create private staging directory: %v", err)
	}
	for _, dir := range []struct {
		path string
		mode os.FileMode
	}{
		{filepath.Join(fixture.project, ".workflow-hero"), 0o755},
		{filepath.Join(fixture.project, ".workflow-hero", "cycles"), 0o755},
		{filepath.Join(fixture.project, ".workflow-hero", "cycles", "current"), 0o755},
		{filepath.Join(fixture.project, ".workflow-hero", "cycles", "current", "screenshots"), 0o700},
		{directory, 0o700},
	} {
		if err := os.Chmod(dir.path, dir.mode); err != nil {
			t.Fatalf("secure staging directory: %v", err)
		}
	}
	path := filepath.Join(fixture.project, filepath.FromSlash(relative))
	if err := os.WriteFile(path, onePixelPNG(t), 0o600); err != nil {
		t.Fatalf("write staged screenshot: %v", err)
	}
	if attempt != 1 {
		t.Fatalf("fixture helper currently writes attempt 1, got %d", attempt)
	}
	return relative
}

func TestStagedScreenshotPublishesOnlyAfterValidatedCopyAndRemovesSource(t *testing.T) {
	fixture := newScreenshotFixture(t, false, false, true)
	reference := writeStagedScreenshot(t, fixture, stageBrowserUI, 1)
	source, err := NewStagedFileCaptureSource(fixture.project, reference, stageBrowserUI, 1, CaptureAttestation{
		StablePointVerified: true, SensitiveFieldsAndTokensMasked: true, CredentialFlowArtifactsSuppressed: true,
	})
	if err != nil {
		t.Fatalf("create staged source: %v", err)
	}
	outcome, err := fixture.service.CapturePlannedEvidence(context.Background(), requestFor(fixture, 1, "operator", "operator"), source)
	if err != nil || outcome.State != OutcomeReady || outcome.Manifest == nil {
		t.Fatalf("CapturePlannedEvidence() = (%#v, %v), want ready", outcome, err)
	}
	if err := RemoveStagedEvidence(fixture.project, reference, stageBrowserUI, 1); err != nil {
		t.Fatalf("remove consumed staged image: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fixture.project, filepath.FromSlash(reference))); !os.IsNotExist(err) {
		t.Fatalf("staged source remains after ingestion, stat error=%v", err)
	}
	ready, err := fixture.service.ReadySet(context.Background(), fixture.cycleID)
	if err != nil || len(ready) != 1 || ready[0].ID != outcome.Manifest.ID {
		t.Fatalf("ready set after ingestion=(%d, %v), want one published manifest", len(ready), err)
	}
}

func TestStagedScreenshotWithoutSuppressionAttestationBlocksEvidence(t *testing.T) {
	fixture := newScreenshotFixture(t, true, true, true)
	reference := writeStagedScreenshot(t, fixture, stageBrowserUI, 1)
	source, err := NewStagedFileCaptureSource(fixture.project, reference, stageBrowserUI, 1, CaptureAttestation{
		StablePointVerified: true, SensitiveFieldsAndTokensMasked: true,
	})
	if err != nil {
		t.Fatalf("create staged source: %v", err)
	}
	outcome, err := fixture.service.CapturePlannedEvidence(context.Background(), requestFor(fixture, 1, "operator", "operator"), source)
	if err != nil || outcome.State != OutcomeBlocked || outcome.Manifest == nil {
		t.Fatalf("unsafe staged capture=(%#v, %v), want mandatory blocker", outcome, err)
	}
	ready, err := fixture.service.ReadySet(context.Background(), fixture.cycleID)
	if err != nil || len(ready) != 0 {
		t.Fatalf("unsafe staged capture became visible: ready=%d err=%v", len(ready), err)
	}
}

func TestStagedEvidenceReferenceIsBoundToStageAndAttempt(t *testing.T) {
	reference := stagedReferencePrefix + stageBrowserUI + "/2/image.png"
	if !IsStagedEvidenceReference(reference, stageBrowserUI, 2) {
		t.Fatal("valid stage attempt reference was rejected")
	}
	for _, test := range []struct {
		name, reference, stage string
		attempt                int
	}{
		{name: "wrong stage", reference: reference, stage: stageE2E, attempt: 2},
		{name: "wrong attempt", reference: reference, stage: stageBrowserUI, attempt: 1},
		{name: "parent traversal", reference: stagedReferencePrefix + stageBrowserUI + "/2/../image.png", stage: stageBrowserUI, attempt: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			if IsStagedEvidenceReference(test.reference, test.stage, test.attempt) {
				t.Fatalf("unsafe reference was accepted: %q", test.reference)
			}
		})
	}
}
