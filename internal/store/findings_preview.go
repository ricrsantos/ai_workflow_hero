package store

import (
	"errors"
	"strings"
)

// PredictFindingActionable reports whether PersistFindingTx would treat the input as actionable.
func (s *Store) PredictFindingActionable(cycleID int64, in FindingInput) (bool, error) {
	if err := validateFindingInput(in); err != nil {
		return false, err
	}
	reopenID := strings.TrimSpace(in.ReopenID)
	if reopenID != "" {
		f, err := s.GetFinding(cycleID, reopenID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return false, nil
			}
			return false, err
		}
		if f.Status == FindingStatusDone && f.SourceStage == in.SourceStage && f.Owner == in.Owner &&
			FindingContractMatches(f, in.File, in.Requirement, in.AcceptanceCriteria) &&
			FindingReproMatches(f, in.ReproPackage, in.ReproTest) {
			return true, nil
		}
		return false, nil
	}
	existing, err := s.lookupFindingForInput(cycleID, in)
	if errors.Is(err, ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	switch existing.Status {
	case FindingStatusOpen, FindingStatusReopened:
		// Rediscovery of still-open work remains actionable for failed-close gates.
		return true, nil
	case FindingStatusDeferredTodo:
		return false, nil
	case FindingStatusDone:
		return true, nil
	default:
		return false, nil
	}
}
