package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/findingrepro"
)

// FindingReproMatches reports whether incoming repro identity may attach to f.
// An empty stored repro (pre-v14 / in-flight) accepts the first lock.
func FindingReproMatches(f Finding, reproPackage, reproTest string) bool {
	storedPkg, _ := findingrepro.CanonicalPackage(f.ReproPackage)
	storedTest, _ := findingrepro.CanonicalTest(f.ReproTest)
	inPkg, _ := findingrepro.CanonicalPackage(reproPackage)
	inTest, _ := findingrepro.CanonicalTest(reproTest)
	if storedPkg == "" && storedTest == "" {
		return true
	}
	return storedPkg == inPkg && storedTest == inTest
}

// LatestFindingResidual is the newest non-done, non-deferred occurrence issue.
// Frozen Finding.Issue is identity only when history exists.
func LatestFindingResidual(f Finding, occs []FindingOccurrence) string {
	for i := len(occs) - 1; i >= 0; i-- {
		switch occs[i].Kind {
		case OccurrenceDone, OccurrenceDeferred:
			continue
		}
		if issue := strings.TrimSpace(occs[i].Issue); issue != "" {
			return issue
		}
	}
	return strings.TrimSpace(f.Issue)
}

// LatestFindingRepro returns the newest occurrence with a locked package+test,
// falling back to the finding row.
func LatestFindingRepro(f Finding, occs []FindingOccurrence) (pkg, test, source string) {
	for i := len(occs) - 1; i >= 0; i-- {
		if strings.TrimSpace(occs[i].ReproPackage) != "" && strings.TrimSpace(occs[i].ReproTest) != "" {
			return occs[i].ReproPackage, occs[i].ReproTest, occs[i].ReproSource
		}
	}
	return f.ReproPackage, f.ReproTest, ""
}

func canonicalizeFindingRepro(in FindingInput) (pkg, test, source string, err error) {
	pkg, err = findingrepro.CanonicalPackage(in.ReproPackage)
	if err != nil {
		return "", "", "", fmt.Errorf("%w: %v", ErrInvalidFindingContent, err)
	}
	test, err = findingrepro.CanonicalTest(in.ReproTest)
	if err != nil {
		return "", "", "", fmt.Errorf("%w: %v", ErrInvalidFindingContent, err)
	}
	source = strings.TrimSpace(in.ReproSource)
	if pkg == "" && test == "" {
		if source != "" {
			return "", "", "", fmt.Errorf("%w: repro source requires package and test", ErrInvalidFindingContent)
		}
		return "", "", "", nil
	}
	if pkg == "" || test == "" {
		return "", "", "", fmt.Errorf("%w: repro package and test are both required", ErrInvalidFindingContent)
	}
	if source != "" {
		if err := validateFindingTextField("repro.source", source); err != nil {
			return "", "", "", err
		}
		if err := findingrepro.ValidateSource(source, test); err != nil {
			return "", "", "", fmt.Errorf("%w: %v", ErrInvalidFindingContent, err)
		}
	}
	return pkg, test, source, nil
}

func lookupFindingForInputTx(q execQueryer, in FindingInput, pkg, test string) (Finding, error) {
	fp := FindingFingerprint(in.CycleID, in.SourceStage, in.Owner, in.File, in.Requirement, in.AcceptanceCriteria, pkg, test)
	existing, err := getFindingByFingerprintTx(q, in.CycleID, fp)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Finding{}, err
	}
	if pkg == "" && test == "" {
		return Finding{}, ErrNotFound
	}
	legacyFP := FindingFingerprint(in.CycleID, in.SourceStage, in.Owner, in.File, in.Requirement, in.AcceptanceCriteria, "", "")
	existing, err = getFindingByFingerprintTx(q, in.CycleID, legacyFP)
	if err != nil {
		return Finding{}, err
	}
	if !FindingReproMatches(existing, pkg, test) {
		return Finding{}, ErrNotFound
	}
	return existing, nil
}

func (s *Store) lookupFindingForInput(cycleID int64, in FindingInput) (Finding, error) {
	in.CycleID = cycleID
	pkg, test, _, err := canonicalizeFindingRepro(in)
	if err != nil {
		return Finding{}, err
	}
	return lookupFindingForInputTx(s.db, in, pkg, test)
}

func lockFindingReproIfEmptyTx(tx *sql.Tx, f Finding, pkg, test, updatedAt string) error {
	if pkg == "" && test == "" {
		return nil
	}
	if strings.TrimSpace(f.ReproPackage) != "" || strings.TrimSpace(f.ReproTest) != "" {
		return nil
	}
	newFP := FindingFingerprint(f.CycleID, f.SourceStage, f.Owner, f.File, f.Requirement, f.AcceptanceCriteria, pkg, test)
	_, err := tx.Exec(`
UPDATE findings SET repro_package = ?, repro_test = ?, fingerprint = ?, updated_at = ?
WHERE cycle_id = ? AND id = ?`,
		pkg, test, newFP, updatedAt, f.CycleID, f.ID,
	)
	if err != nil {
		return fmt.Errorf("lock finding repro: %w", err)
	}
	return nil
}
