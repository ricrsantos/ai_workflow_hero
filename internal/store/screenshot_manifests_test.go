package store

import (
	"context"
	"path/filepath"
	"testing"
)

const screenshotManifestTestTime = "2026-09-30T12:00:00Z"

func openScreenshotManifestStore(t *testing.T) (*Store, int64) {
	t.Helper()

	st, err := OpenProject(t.TempDir())
	if err != nil {
		t.Fatalf("open project store: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("close project store: %v", err)
		}
	})

	cycleID, err := st.CreateCycle(Cycle{
		Number:             1,
		Title:              "screenshots",
		Status:             CycleStatusActive,
		StartedAt:          screenshotManifestTestTime,
		ConfigSnapshotJSON: "{}",
	})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	if err := st.CreateStages([]Stage{{
		CycleID:       cycleID,
		Name:          "browser_ui_validation",
		Status:        StageRunning,
		SortOrder:     0,
		MaxIterations: 1,
	}}); err != nil {
		t.Fatalf("create stage: %v", err)
	}
	return st, cycleID
}

func TestManifestInsertAndReadySnapshot(t *testing.T) {
	st, cycleID := openScreenshotManifestStore(t)
	ctx := context.Background()
	first := ScreenshotManifest{
		ID:            "shot-one",
		CycleID:       cycleID,
		StageName:     "browser_ui_validation",
		Attempt:       1,
		CoverageID:    "screen-02",
		UserID:        "operator",
		ProfileID:     "operator",
		CapturedAt:    screenshotManifestTestTime,
		Path:          "screenshots/shot-one.png",
		Result:        ScreenshotResultPassed,
		CaptureStatus: ScreenshotCaptureReady,
	}
	if err := st.InsertScreenshotManifest(ctx, first); err != nil {
		t.Fatalf("insert ready screenshot: %v", err)
	}
	omitted := ScreenshotManifest{
		ID:             "shot-two",
		CycleID:        cycleID,
		StageName:      "browser_ui_validation",
		Attempt:        2,
		CoverageID:     "screen-02",
		UserID:         "admin",
		ProfileID:      "admin",
		CapturedAt:     "2026-09-30T12:00:01Z",
		Result:         ScreenshotResultBlocked,
		CaptureStatus:  ScreenshotCaptureOmitted,
		OmissionReason: ScreenshotOmissionCredentialPhase,
	}
	if err := st.InsertScreenshotManifest(ctx, omitted); err != nil {
		t.Fatalf("insert omitted screenshot: %v", err)
	}

	ready, err := st.ListReadyScreenshotManifests(ctx, cycleID)
	if err != nil {
		t.Fatalf("list ready screenshots: %v", err)
	}
	if len(ready) != 1 || ready[0] != first {
		t.Fatalf("ready snapshot = %#v, want only %#v", ready, first)
	}
	if err := st.InsertScreenshotManifest(ctx, first); err == nil {
		t.Fatal("duplicate screenshot ID replaced an immutable manifest")
	}
}

func TestManifestRejectsUnsafeOrSensitiveMetadata(t *testing.T) {
	st, cycleID := openScreenshotManifestStore(t)
	valid := ScreenshotManifest{
		ID:            "shot-valid",
		CycleID:       cycleID,
		StageName:     "browser_ui_validation",
		Attempt:       1,
		CapturedAt:    screenshotManifestTestTime,
		Path:          "screenshots/shot-valid.png",
		Result:        ScreenshotResultPassed,
		CaptureStatus: ScreenshotCaptureReady,
	}
	tests := []struct {
		name string
		edit func(*ScreenshotManifest)
	}{
		{"path traversal", func(m *ScreenshotManifest) { m.Path = "screenshots/../secret.png" }},
		{"absolute path", func(m *ScreenshotManifest) { m.Path = filepath.Join(string(filepath.Separator), "tmp", "shot.png") }},
		{"path ID mismatch", func(m *ScreenshotManifest) { m.Path = "screenshots/other.png" }},
		{"unlisted extension", func(m *ScreenshotManifest) { m.Path = "screenshots/shot-valid.txt" }},
		{"partial user profile", func(m *ScreenshotManifest) { m.UserID = "operator" }},
		{"invalid coverage identifier", func(m *ScreenshotManifest) { m.CoverageID = "screen password" }},
		{"invalid user identifier", func(m *ScreenshotManifest) { m.UserID = "token-sentinel"; m.ProfileID = "profile" }},
		{"invalid profile identifier", func(m *ScreenshotManifest) { m.UserID = "operator"; m.ProfileID = "password/value" }},
		{"unrecognized omission", func(m *ScreenshotManifest) {
			m.CaptureStatus = ScreenshotCaptureOmitted
			m.Path = ""
			m.OmissionReason = "sentinel-password-value"
		}},
		{"wrong stage", func(m *ScreenshotManifest) { m.StageName = "qa" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := valid
			tt.edit(&manifest)
			if err := st.InsertScreenshotManifest(context.Background(), manifest); err == nil {
				t.Fatal("invalid screenshot manifest was accepted")
			}
		})
	}
}

func TestManifestRequiresOwnedCycleStage(t *testing.T) {
	st, cycleID := openScreenshotManifestStore(t)
	manifest := ScreenshotManifest{
		ID:            "shot-foreign-stage",
		CycleID:       cycleID,
		StageName:     "qa_end_to_end",
		Attempt:       1,
		CapturedAt:    screenshotManifestTestTime,
		Path:          "screenshots/shot-foreign-stage.png",
		Result:        ScreenshotResultPassed,
		CaptureStatus: ScreenshotCaptureReady,
	}
	if err := st.InsertScreenshotManifest(context.Background(), manifest); err == nil {
		t.Fatal("manifest for a missing cycle stage was accepted")
	}
}
