package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func openBlockedValidationStore(t *testing.T) (*Store, int64) {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "hero.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	cycleID, err := st.CreateCycle(Cycle{
		Number: 1, Title: "blocked validation", Objective: "persist safe coverage",
		Status: CycleStatusActive, StartedAt: nowRFC3339(), ConfigSnapshotJSON: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateStages([]Stage{{CycleID: cycleID, Name: "browser_ui_validation", Status: StageRunning, MaxIterations: 2}}); err != nil {
		t.Fatal(err)
	}
	return st, cycleID
}

func TestStageCoveragePlanPersistsApprovedDenominatorAcrossResults(t *testing.T) {
	st, cycleID := openBlockedValidationStore(t)
	plan := []StageCoverage{
		{ID: "screen-dashboard-operator", UserID: "operator", ProfileID: "operator", Mandatory: true, ExpectedResult: "protected dashboard renders"},
		{ID: "screen-help", Mandatory: false, ExpectedResult: "help page is reachable"},
	}
	if err := st.InTx(func(tx *sql.Tx) error {
		return st.CreateStageCoveragePlanTx(tx, cycleID, "browser_ui_validation", plan, "2026-08-07T12:00:00Z")
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.InTx(func(tx *sql.Tx) error {
		return st.CreateStageCoveragePlanTx(tx, cycleID, "browser_ui_validation", plan, "2026-08-07T12:01:00Z")
	}); !errors.Is(err, ErrCoveragePlanExists) {
		t.Fatalf("second plan error = %v, want ErrCoveragePlanExists", err)
	}
	rows, err := st.ListStageCoverage(cycleID, "browser_ui_validation")
	if err != nil || len(rows) != 2 {
		t.Fatalf("planned rows = %+v, err = %v", rows, err)
	}
	if rows[0].Result != StageCoveragePlanned || rows[0].ExpectedResult != plan[0].ExpectedResult {
		t.Fatalf("initial denominator row = %+v", rows[0])
	}
	if err := st.InTx(func(tx *sql.Tx) error {
		return st.PersistStageCoverageResultsTx(tx, cycleID, "browser_ui_validation", []StageCoverage{
			{ID: "screen-dashboard-operator", UserID: "operator", ProfileID: "operator", Result: StageCoverageBlocked, Reason: "tool_missing", Evidence: []string{}},
			{ID: "screen-help", Result: StageCoveragePassed, Evidence: []string{".workflow-hero/cycles/current/screenshots/shot-1.png"}},
		}, "2026-08-07T12:02:00Z")
	}); err != nil {
		t.Fatal(err)
	}
	rows, err = st.ListStageCoverage(cycleID, "browser_ui_validation")
	if err != nil || len(rows) != 2 {
		t.Fatalf("coverage rows after result = %+v, err = %v", rows, err)
	}
	if rows[0].ID != "screen-dashboard-operator" || rows[0].Mandatory != true || rows[0].ExpectedResult != plan[0].ExpectedResult || rows[0].Result != StageCoverageBlocked {
		t.Fatalf("blocked row lost approved plan metadata: %+v", rows[0])
	}
	if rows[1].ID != "screen-help" || rows[1].Result != StageCoveragePassed || !reflect.DeepEqual(rows[1].Evidence, []string{".workflow-hero/cycles/current/screenshots/shot-1.png"}) {
		t.Fatalf("passed row = %+v", rows[1])
	}
	if err := st.InTx(func(tx *sql.Tx) error {
		return st.PersistStageCoverageResultsTx(tx, cycleID, "browser_ui_validation", []StageCoverage{
			{ID: "screen-dashboard-operator", UserID: "operator", ProfileID: "admin", Result: StageCoveragePassed},
		}, "2026-08-07T12:03:00Z")
	}); err == nil {
		t.Fatal("mismatched profile unexpectedly matched the approved plan")
	}
}

func TestStageCoverageSnapshotPersistsTraceabilityAndDigest(t *testing.T) {
	st, cycleID := openBlockedValidationStore(t)
	plan := []StageCoverage{{
		ID: "screen-dashboard-operator", UserID: "operator", ProfileID: "operator", Mandatory: true,
		RequirementRef: "PRD-C17 FR-07", ScreenJourney: "dashboard journey",
		ExpectedResult:          "operator sees the protected dashboard",
		EvidenceRequirements:    []string{"screen capture", "protected role verified"},
		OptionalReferenceWidths: []int{768, 375},
	}}
	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := st.InTx(func(tx *sql.Tx) error {
		return st.CreateStageCoverageSnapshotTx(tx, cycleID, "browser_ui_validation", plan, digest, "2026-08-07T12:00:00Z")
	}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := st.GetStageCoveragePlanSnapshot(cycleID, "browser_ui_validation")
	if err != nil || snapshot.Digest != digest || snapshot.ApprovedAt == "" {
		t.Fatalf("snapshot = %+v, err = %v", snapshot, err)
	}
	rows, err := st.ListStageCoverage(cycleID, "browser_ui_validation")
	if err != nil || len(rows) != 1 {
		t.Fatalf("coverage rows = %+v, err = %v", rows, err)
	}
	if rows[0].RequirementRef != plan[0].RequirementRef || rows[0].ScreenJourney != plan[0].ScreenJourney ||
		!reflect.DeepEqual(rows[0].EvidenceRequirements, plan[0].EvidenceRequirements) ||
		!reflect.DeepEqual(rows[0].OptionalReferenceWidths, plan[0].OptionalReferenceWidths) {
		t.Fatalf("coverage traceability was not persisted: %+v", rows[0])
	}
}

func TestStageBlockersReplaceAndResolveAtomically(t *testing.T) {
	st, cycleID := openBlockedValidationStore(t)
	blockers := []StageBlocker{{
		ID: "browser-tool-unavailable", Reason: "tool_unavailable",
		AffectedCoverageIDs: []string{"screen-dashboard"}, AffectedProfileIDs: []string{"operator"},
		DiagnosticUncertainty: "the planned browser capability was not verified",
		NextAction:            "configure the planned browser method and continue",
	}}
	if err := st.InTx(func(tx *sql.Tx) error {
		return st.ReplaceActiveStageBlockersTx(tx, cycleID, "browser_ui_validation", blockers, "2026-08-07T12:00:00Z")
	}); err != nil {
		t.Fatal(err)
	}
	active, err := st.ListActiveStageBlockers(cycleID, "browser_ui_validation")
	if err != nil || len(active) != 1 || active[0].Status != StageBlockerActive || active[0].ID != blockers[0].ID {
		t.Fatalf("active blockers = %+v, err = %v", active, err)
	}
	replacement := []StageBlocker{{
		ID: "login-profile-unavailable", Reason: "credentials_unavailable",
		AffectedCoverageIDs: []string{"screen-dashboard"}, AffectedProfileIDs: []string{"operator"},
		DiagnosticUncertainty: "the selected profile is not usable",
		NextAction:            "configure a usable test account and continue",
	}}
	if err := st.InTx(func(tx *sql.Tx) error {
		return st.ReplaceActiveStageBlockersTx(tx, cycleID, "browser_ui_validation", replacement, "2026-08-07T12:01:00Z")
	}); err != nil {
		t.Fatal(err)
	}
	active, err = st.ListActiveStageBlockers(cycleID, "browser_ui_validation")
	if err != nil || len(active) != 1 || active[0].ID != replacement[0].ID {
		t.Fatalf("replacement blockers = %+v, err = %v", active, err)
	}
	var oldStatus, resolvedAt string
	if err := st.DB().QueryRow(`SELECT status, resolved_at FROM stage_blockers WHERE cycle_id = ? AND stage_name = ? AND blocker_id = ?`, cycleID, "browser_ui_validation", blockers[0].ID).Scan(&oldStatus, &resolvedAt); err != nil {
		t.Fatal(err)
	}
	if oldStatus != StageBlockerResolved || resolvedAt == "" {
		t.Fatalf("replaced blocker status=%q resolved_at=%q", oldStatus, resolvedAt)
	}
	if err := st.InTx(func(tx *sql.Tx) error {
		return st.ResolveActiveStageBlockersTx(tx, cycleID, "browser_ui_validation", "2026-08-07T12:02:00Z")
	}); err != nil {
		t.Fatal(err)
	}
	active, err = st.ListActiveStageBlockers(cycleID, "browser_ui_validation")
	if err != nil || len(active) != 0 {
		t.Fatalf("resolved blockers = %+v, err = %v", active, err)
	}
}
