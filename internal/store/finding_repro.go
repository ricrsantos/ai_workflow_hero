package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/findingrepro"
)

// FindingReproMatches reports whether incoming repro identity may attach to f.
// An empty stored repro (pre-v14 / in-flight / evidence-only) accepts the first
// lock. A stored automated repro only accepts the same mode, package and test,
// so a reopen can never silently swap the test that proves the fix.
func FindingReproMatches(f Finding, reproMode, reproPackage, reproTest string) bool {
	storedMode := EffectiveReproMode(f.ReproMode, f.ReproTest)
	inMode := EffectiveReproMode(reproMode, reproTest)
	storedPkg, storedTest := canonicalReproIdentity(storedMode, f.ReproPackage, f.ReproTest)
	inPkg, inTest := canonicalReproIdentity(inMode, reproPackage, reproTest)
	if storedPkg == "" && storedTest == "" {
		return true
	}
	// A stored automated repro is the finding's proof contract: an evidence-only
	// recurrence describes a different contract and must become a new find-* ID
	// rather than silently dropping the gate on this one.
	return storedMode == inMode && storedPkg == inPkg && storedTest == inTest
}

// EffectiveReproMode resolves a stored or incoming mode. Rows written before
// repro modes existed carry a package and test, which is always go_test.
func EffectiveReproMode(mode, test string) string {
	mode, err := findingrepro.CanonicalMode(mode)
	if err != nil {
		return ""
	}
	if mode != "" {
		return mode
	}
	if strings.TrimSpace(test) != "" {
		return findingrepro.ModeGoTest
	}
	return ""
}

func canonicalReproIdentity(mode, pkg, test string) (string, string) {
	if mode == "" || mode == findingrepro.ModeEvidence {
		return "", ""
	}
	cpkg, ctest, err := findingrepro.CanonicalIdentity(mode, pkg, test)
	if err != nil {
		return "", ""
	}
	return cpkg, ctest
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
	_, pkg, test, source = LatestFindingReproWithMode(f, occs)
	return pkg, test, source
}

// LatestFindingReproWithMode returns the newest locked repro identity for f,
// including the mode the scheduler must use to re-run it.
func LatestFindingReproWithMode(f Finding, occs []FindingOccurrence) (mode, pkg, test, source string) {
	for i := len(occs) - 1; i >= 0; i-- {
		if strings.TrimSpace(occs[i].ReproPackage) != "" && strings.TrimSpace(occs[i].ReproTest) != "" {
			return EffectiveReproMode(occs[i].ReproMode, occs[i].ReproTest),
				occs[i].ReproPackage, occs[i].ReproTest, occs[i].ReproSource
		}
	}
	storedMode := EffectiveReproMode(f.ReproMode, f.ReproTest)
	if storedMode == findingrepro.ModeEvidence {
		return storedMode, "", "", ""
	}
	return storedMode, f.ReproPackage, f.ReproTest, ""
}

func canonicalizeFindingRepro(in FindingInput) (mode, pkg, test, source string, err error) {
	mode, err = findingrepro.CanonicalMode(in.ReproMode)
	if err != nil {
		return "", "", "", "", fmt.Errorf("%w: %v", ErrInvalidFindingContent, err)
	}
	if mode == "" && strings.TrimSpace(in.ReproTest) != "" {
		mode = findingrepro.ModeGoTest
	}
	source = strings.TrimSpace(in.ReproSource)
	if mode == findingrepro.ModeEvidence {
		if strings.TrimSpace(in.ReproPackage) != "" || strings.TrimSpace(in.ReproTest) != "" || source != "" {
			return "", "", "", "", fmt.Errorf("%w: evidence repro must not carry package, test, or source", ErrInvalidFindingContent)
		}
		return mode, "", "", "", nil
	}
	pkg, test, err = findingrepro.CanonicalIdentity(mode, in.ReproPackage, in.ReproTest)
	if err != nil {
		return "", "", "", "", fmt.Errorf("%w: %v", ErrInvalidFindingContent, err)
	}
	if pkg == "" && test == "" {
		if source != "" {
			return "", "", "", "", fmt.Errorf("%w: repro source requires package and test", ErrInvalidFindingContent)
		}
		return "", "", "", "", nil
	}
	if pkg == "" || test == "" {
		return "", "", "", "", fmt.Errorf("%w: repro package and test are both required", ErrInvalidFindingContent)
	}
	if source != "" {
		if err := validateFindingTextField("repro.source", source); err != nil {
			return "", "", "", "", err
		}
		if err := findingrepro.ValidateSourceForMode(mode, source, test); err != nil {
			return "", "", "", "", fmt.Errorf("%w: %v", ErrInvalidFindingContent, err)
		}
	}
	return mode, pkg, test, source, nil
}

func lookupFindingForInputTx(q execQueryer, in FindingInput, mode, pkg, test string) (Finding, error) {
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
	if !FindingReproMatches(existing, mode, pkg, test) {
		return Finding{}, ErrNotFound
	}
	return existing, nil
}

func (s *Store) lookupFindingForInput(cycleID int64, in FindingInput) (Finding, error) {
	in.CycleID = cycleID
	mode, pkg, test, _, err := canonicalizeFindingRepro(in)
	if err != nil {
		return Finding{}, err
	}
	return lookupFindingForInputTx(s.db, in, mode, pkg, test)
}

func lockFindingReproIfEmptyTx(tx *sql.Tx, f Finding, mode, pkg, test, updatedAt string) error {
	if pkg == "" && test == "" {
		if mode == "" || strings.TrimSpace(f.ReproMode) != "" {
			return nil
		}
		// Evidence-only findings still record the mode so the Implementation
		// assignment can say there is no automated gate.
		_, err := tx.Exec(`UPDATE findings SET repro_mode = ?, updated_at = ? WHERE cycle_id = ? AND id = ?`,
			mode, updatedAt, f.CycleID, f.ID)
		if err != nil {
			return fmt.Errorf("lock finding repro: %w", err)
		}
		return nil
	}
	if strings.TrimSpace(f.ReproPackage) != "" || strings.TrimSpace(f.ReproTest) != "" {
		return nil
	}
	newFP := FindingFingerprint(f.CycleID, f.SourceStage, f.Owner, f.File, f.Requirement, f.AcceptanceCriteria, pkg, test)
	_, err := tx.Exec(`
UPDATE findings SET repro_mode = ?, repro_package = ?, repro_test = ?, fingerprint = ?, updated_at = ?
WHERE cycle_id = ? AND id = ?`,
		mode, pkg, test, newFP, updatedAt, f.CycleID, f.ID,
	)
	if err != nil {
		return fmt.Errorf("lock finding repro: %w", err)
	}
	return nil
}
