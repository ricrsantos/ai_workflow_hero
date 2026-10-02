package store

import (
	"path/filepath"
	"testing"
)

func TestMigrateV16ToV17AddsCoverageTraceability(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hero.db")
	st, err := openCapped(path, 16)
	if err != nil {
		t.Fatalf("open v16 fixture: %v", err)
	}
	defer func() { _ = st.Close() }()

	cycleID, err := insertStageForV16Test(t, st.db, "browser_ui_validation")
	if err != nil {
		t.Fatalf("insert validation stage: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO stage_coverage
  (cycle_id, stage_name, coverage_id, mandatory, expected_result, updated_at)
VALUES (?, 'browser_ui_validation', 'legacy-screen', 1, 'legacy outcome', ?)`, cycleID, v15FixtureTimestamp); err != nil {
		t.Fatalf("insert legacy coverage: %v", err)
	}
	if err := st.migrateTo(17); err != nil {
		t.Fatalf("migrate v16 to v17: %v", err)
	}
	if got := schemaVersionForTest(t, st); got != 17 {
		t.Fatalf("schema version = %d, want 17", got)
	}
	row, err := st.ListStageCoverage(cycleID, "browser_ui_validation")
	if err != nil || len(row) != 1 {
		t.Fatalf("legacy coverage after migration = %+v, err = %v", row, err)
	}
	if row[0].ID != "legacy-screen" || row[0].RequirementRef != "" || len(row[0].EvidenceRequirements) != 0 {
		t.Fatalf("legacy coverage metadata defaults = %+v", row[0])
	}
	if _, err := st.GetStageCoveragePlanSnapshot(cycleID, "browser_ui_validation"); err != ErrNotFound {
		t.Fatalf("legacy snapshot error = %v, want not found", err)
	}
}
