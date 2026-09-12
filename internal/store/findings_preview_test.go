package store

import (
	"database/sql"
	"testing"
)

func TestPredictFindingActionableOpenRediscovery(t *testing.T) {
	s, cycleID := openTestStoreWithCycle(t)
	in := FindingInput{
		CycleID:            cycleID,
		SourceStage:        FindingSourceQA,
		Owner:              FindingOwnerGeneric,
		File:               "internal/store/findings_preview.go",
		Requirement:        "PRD-C15 actionable rediscovery",
		Issue:              "still broken",
		AcceptanceCriteria: "failed close accepts rediscovery",
	}
	if _, err := s.PersistFinding(in); err != nil {
		t.Fatal(err)
	}
	ok, err := s.PredictFindingActionable(cycleID, in)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("open rediscovery must remain actionable for failed-close gates")
	}
}

func TestPredictFindingActionableReopenRequiresContract(t *testing.T) {
	s, cycleID := openTestStoreWithCycle(t)
	in := FindingInput{
		CycleID:            cycleID,
		SourceStage:        FindingSourceQA,
		Owner:              FindingOwnerGeneric,
		File:               "internal/store/findings_preview.go",
		Requirement:        "PRD-C15 reopen contract",
		Issue:              "original",
		AcceptanceCriteria: "same acceptance",
	}
	res, err := s.PersistFinding(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.InTx(func(tx *sql.Tx) error {
		return s.MarkFindingDoneTx(tx, cycleID, res.Finding.ID, in.Issue, in.AcceptanceCriteria, "[]")
	}); err != nil {
		t.Fatal(err)
	}
	match := in
	match.ReopenID = res.Finding.ID
	match.Issue = "still the same residual"
	ok, err := s.PredictFindingActionable(cycleID, match)
	if err != nil || !ok {
		t.Fatalf("matching contract actionable=%v err=%v", ok, err)
	}
	drift := match
	drift.AcceptanceCriteria = "a different acceptance"
	ok, err = s.PredictFindingActionable(cycleID, drift)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("changed acceptance must not predict reopen as actionable")
	}
}
