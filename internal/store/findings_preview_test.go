package store

import "testing"

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
