package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/envhygiene"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/findingrepro"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
	"golang.org/x/text/unicode/norm"
)

// Finding source stages (schema CHECK).
const (
	FindingSourceQA         = "qa"
	FindingSourceJudge      = "judge"
	FindingSourceBrowserUI  = "browser_ui_validation"
	FindingSourceQAEndToEnd = "qa_end_to_end"
)

// Finding owners (schema CHECK).
const (
	FindingOwnerBackend  = "backend_agent"
	FindingOwnerFrontend = "frontend_agent"
	FindingOwnerGeneric  = "generic_agent"
)

// Finding statuses (schema CHECK).
const (
	FindingStatusOpen         = "open"
	FindingStatusDone         = "done"
	FindingStatusReopened     = "reopened"
	FindingStatusDeferredTodo = "deferred_todo"
)

// Finding occurrence kinds (schema CHECK).
const (
	OccurrenceCreated            = "created"
	OccurrenceDone               = "done"
	OccurrenceReopened           = "reopened"
	OccurrenceRediscovered       = "rediscovered"
	OccurrenceDeferred           = "deferred"
	OccurrenceDeferredRecurrence = "deferred_recurrence"
)

// ErrInvalidFindingContent is returned when issue, acceptance, or evidence fail safe-content rules.
var ErrInvalidFindingContent = errors.New("invalid finding content")

// ErrInvalidReopenID is returned when reopen_id does not reference a done finding
// whose stored file, requirement, and acceptance criteria match the report.
var ErrInvalidReopenID = errors.New("invalid reopen_id")

// Finding is a scheduler-owned validation finding row.
type Finding struct {
	ID                 string
	CycleID            int64
	SourceStage        string
	Owner              string
	Status             string
	Fingerprint        string
	File               string
	Requirement        string
	Issue              string
	AcceptanceCriteria string
	EvidenceJSON       string
	Round              int
	TodoID             string
	ReproPackage       string
	ReproTest          string
	CreatedAt          string
	UpdatedAt          string
}

// FindingInput is the payload for persist/upsert lifecycle (pre-validated enums).
type FindingInput struct {
	CycleID            int64
	SourceStage        string
	Owner              string
	File               string
	Requirement        string
	Issue              string
	AcceptanceCriteria string
	Evidence           []string
	ReopenID           string
	ReproPackage       string
	ReproTest          string
	ReproSource        string
}

// PersistFindingResult describes the outcome of one lifecycle upsert.
type PersistFindingResult struct {
	Finding        Finding
	OccurrenceKind string
	Actionable     bool
}

const findingSelectCols = `id, cycle_id, source_stage, owner, status, fingerprint, file, requirement,
issue, acceptance_criteria, evidence_json, round, todo_id, repro_package, repro_test, created_at, updated_at`

// CanonicalizeFindingFile slash-normalizes, trims, and strips leading ./ segments.
func CanonicalizeFindingFile(file string) string {
	file = strings.TrimSpace(file)
	file = filepath.ToSlash(file)
	for strings.HasPrefix(file, "./") {
		file = strings.TrimPrefix(file, "./")
	}
	return file
}

// NormalizeFindingText applies Unicode NFC, trim, and collapses unicode whitespace to ASCII space.
func NormalizeFindingText(s string) string {
	s = norm.NFC.String(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	lastSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !lastSpace && b.Len() > 0 {
				b.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	return b.String()
}

// FindingFingerprint returns lowercase SHA-256 hex for the cycle identity (issue excluded).
func FindingFingerprint(cycleID int64, sourceStage, owner, file, requirement, acceptance, reproPackage, reproTest string) string {
	file = CanonicalizeFindingFile(file)
	requirement = NormalizeFindingText(requirement)
	acceptance = NormalizeFindingText(acceptance)
	reproPackage, _ = findingrepro.CanonicalPackage(reproPackage)
	reproTest, _ = findingrepro.CanonicalTest(reproTest)
	parts := []string{
		strconv.FormatInt(cycleID, 10),
		sourceStage,
		owner,
		file,
		requirement,
		acceptance,
	}
	if reproPackage != "" || reproTest != "" {
		parts = append(parts, reproPackage, reproTest)
	}
	material := strings.Join(parts, "\x1f")
	sum := sha256.Sum256([]byte(material))
	return hex.EncodeToString(sum[:])
}

// FindingContractMatches reports whether file, requirement, and acceptance
// match the stored finding contract after canonicalization. Issue text is ignored.
func FindingContractMatches(f Finding, file, requirement, acceptance string) bool {
	return CanonicalizeFindingFile(file) == CanonicalizeFindingFile(f.File) &&
		NormalizeFindingText(requirement) == NormalizeFindingText(f.Requirement) &&
		NormalizeFindingText(acceptance) == NormalizeFindingText(f.AcceptanceCriteria)
}

// PersistFinding applies the finding lifecycle outside an existing transaction.
func (s *Store) PersistFinding(in FindingInput) (PersistFindingResult, error) {
	var out PersistFindingResult
	err := s.InTx(func(tx *sql.Tx) error {
		res, err := s.PersistFindingTx(tx, in)
		if err != nil {
			return err
		}
		out = res
		return nil
	})
	return out, err
}

// PersistFindingTx creates, reopens, or appends occurrences for one failure entry.
func (s *Store) PersistFindingTx(tx *sql.Tx, in FindingInput) (PersistFindingResult, error) {
	if err := validateFindingInput(in); err != nil {
		return PersistFindingResult{}, err
	}

	canonicalFile := CanonicalizeFindingFile(in.File)
	canonicalReq := NormalizeFindingText(in.Requirement)
	canonicalAC := NormalizeFindingText(in.AcceptanceCriteria)
	canonicalPkg, canonicalTest, canonicalSource, err := canonicalizeFindingRepro(in)
	if err != nil {
		return PersistFindingResult{}, err
	}
	evidenceJSON, err := marshalEvidenceJSON(in.Evidence)
	if err != nil {
		return PersistFindingResult{}, err
	}

	now := nowRFC3339()
	log := s.findingLog()

	reopenID := strings.TrimSpace(in.ReopenID)
	if reopenID != "" {
		f, err := getFindingTx(tx, in.CycleID, reopenID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return PersistFindingResult{}, ErrInvalidReopenID
			}
			return PersistFindingResult{}, err
		}
		if f.Status != FindingStatusDone || f.SourceStage != in.SourceStage || f.Owner != in.Owner {
			return PersistFindingResult{}, ErrInvalidReopenID
		}
		if !FindingContractMatches(f, in.File, in.Requirement, in.AcceptanceCriteria) {
			return PersistFindingResult{}, ErrInvalidReopenID
		}
		if !FindingReproMatches(f, canonicalPkg, canonicalTest) {
			return PersistFindingResult{}, ErrInvalidReopenID
		}
		newRound := f.Round + 1
		if err := updateFindingStatusRoundTx(tx, in.CycleID, reopenID, FindingStatusReopened, newRound, now); err != nil {
			log.Error("finding reopen persist failed", "cycle_id", in.CycleID, "finding_id", reopenID, "error", err)
			return PersistFindingResult{}, err
		}
		if err := lockFindingReproIfEmptyTx(tx, f, canonicalPkg, canonicalTest, now); err != nil {
			return PersistFindingResult{}, err
		}
		if err := appendOccurrenceTx(tx, in.CycleID, reopenID, OccurrenceReopened, in.SourceStage, newRound, in.Issue, canonicalAC, evidenceJSON, now, canonicalPkg, canonicalTest, canonicalSource); err != nil {
			log.Error("finding reopen occurrence failed", "cycle_id", in.CycleID, "finding_id", reopenID, "error", err)
			return PersistFindingResult{}, err
		}
		updated, err := getFindingTx(tx, in.CycleID, reopenID)
		if err != nil {
			return PersistFindingResult{}, err
		}
		log.Info("finding reopened", "cycle_id", in.CycleID, "finding_id", reopenID, "round", newRound, "via", "reopen_id")
		if err := appendLifecycleEventTx(tx, s, in.CycleID, EventFindingReopened, map[string]any{
			"finding_id": reopenID, "source_stage": in.SourceStage, "owner": in.Owner, "round": newRound,
		}); err != nil {
			return PersistFindingResult{}, err
		}
		return PersistFindingResult{
			Finding:        updated,
			OccurrenceKind: OccurrenceReopened,
			Actionable:     true,
		}, nil
	}

	existing, err := lookupFindingForInputTx(tx, in, canonicalPkg, canonicalTest)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return PersistFindingResult{}, err
	}
	if err == nil {
		log.Debug("finding fingerprint match", "cycle_id", in.CycleID, "finding_id", existing.ID, "status", existing.Status, "fingerprint", existing.Fingerprint)
		switch existing.Status {
		case FindingStatusOpen, FindingStatusReopened:
			if err := lockFindingReproIfEmptyTx(tx, existing, canonicalPkg, canonicalTest, now); err != nil {
				return PersistFindingResult{}, err
			}
			if err := appendOccurrenceTx(tx, in.CycleID, existing.ID, OccurrenceRediscovered, in.SourceStage, existing.Round, in.Issue, canonicalAC, evidenceJSON, now, canonicalPkg, canonicalTest, canonicalSource); err != nil {
				log.Error("finding rediscovery occurrence failed", "cycle_id", in.CycleID, "finding_id", existing.ID, "error", err)
				return PersistFindingResult{}, err
			}
			if err := touchFindingUpdatedAtTx(tx, in.CycleID, existing.ID, now); err != nil {
				return PersistFindingResult{}, err
			}
			updated, err := getFindingTx(tx, in.CycleID, existing.ID)
			if err != nil {
				return PersistFindingResult{}, err
			}
			log.Info("finding rediscovered", "cycle_id", in.CycleID, "finding_id", existing.ID, "status", existing.Status)
			return PersistFindingResult{
				Finding:        updated,
				OccurrenceKind: OccurrenceRediscovered,
				Actionable:     true,
			}, nil
		case FindingStatusDone:
			newRound := existing.Round + 1
			if err := updateFindingStatusRoundTx(tx, in.CycleID, existing.ID, FindingStatusReopened, newRound, now); err != nil {
				log.Error("finding fingerprint reopen failed", "cycle_id", in.CycleID, "finding_id", existing.ID, "error", err)
				return PersistFindingResult{}, err
			}
			if err := lockFindingReproIfEmptyTx(tx, existing, canonicalPkg, canonicalTest, now); err != nil {
				return PersistFindingResult{}, err
			}
			if err := appendOccurrenceTx(tx, in.CycleID, existing.ID, OccurrenceReopened, in.SourceStage, newRound, in.Issue, canonicalAC, evidenceJSON, now, canonicalPkg, canonicalTest, canonicalSource); err != nil {
				return PersistFindingResult{}, err
			}
			updated, err := getFindingTx(tx, in.CycleID, existing.ID)
			if err != nil {
				return PersistFindingResult{}, err
			}
			log.Info("finding reopened", "cycle_id", in.CycleID, "finding_id", existing.ID, "round", newRound, "via", "fingerprint")
			if err := appendLifecycleEventTx(tx, s, in.CycleID, EventFindingReopened, map[string]any{
				"finding_id": existing.ID, "source_stage": in.SourceStage, "owner": in.Owner, "round": newRound,
			}); err != nil {
				return PersistFindingResult{}, err
			}
			return PersistFindingResult{
				Finding:        updated,
				OccurrenceKind: OccurrenceReopened,
				Actionable:     true,
			}, nil
		case FindingStatusDeferredTodo:
			if err := lockFindingReproIfEmptyTx(tx, existing, canonicalPkg, canonicalTest, now); err != nil {
				return PersistFindingResult{}, err
			}
			if err := appendOccurrenceTx(tx, in.CycleID, existing.ID, OccurrenceDeferredRecurrence, in.SourceStage, existing.Round, in.Issue, canonicalAC, evidenceJSON, now, canonicalPkg, canonicalTest, canonicalSource); err != nil {
				log.Error("deferred recurrence occurrence failed", "cycle_id", in.CycleID, "finding_id", existing.ID, "error", err)
				return PersistFindingResult{}, err
			}
			updated, err := getFindingTx(tx, in.CycleID, existing.ID)
			if err != nil {
				return PersistFindingResult{}, err
			}
			log.Info("deferred finding recurrence recorded", "cycle_id", in.CycleID, "finding_id", existing.ID)
			if err := appendLifecycleEventTx(tx, s, in.CycleID, EventFindingDeferredRecurrence, map[string]any{
				"finding_id": existing.ID, "source_stage": in.SourceStage, "round": existing.Round,
			}); err != nil {
				return PersistFindingResult{}, err
			}
			return PersistFindingResult{
				Finding:        updated,
				OccurrenceKind: OccurrenceDeferredRecurrence,
				Actionable:     false,
			}, nil
		default:
			return PersistFindingResult{}, fmt.Errorf("unexpected finding status %q", existing.Status)
		}
	}

	fp := FindingFingerprint(in.CycleID, in.SourceStage, in.Owner, in.File, in.Requirement, in.AcceptanceCriteria, canonicalPkg, canonicalTest)
	id, err := allocateFindingIDTx(tx, in.CycleID, in.SourceStage)
	if err != nil {
		log.Error("finding id allocation failed", "cycle_id", in.CycleID, "source_stage", in.SourceStage, "error", err)
		return PersistFindingResult{}, err
	}
	if err := insertFindingTx(tx, Finding{
		ID:                 id,
		CycleID:            in.CycleID,
		SourceStage:        in.SourceStage,
		Owner:              in.Owner,
		Status:             FindingStatusOpen,
		Fingerprint:        fp,
		File:               canonicalFile,
		Requirement:        canonicalReq,
		Issue:              strings.TrimSpace(in.Issue),
		AcceptanceCriteria: canonicalAC,
		EvidenceJSON:       evidenceJSON,
		Round:              1,
		ReproPackage:       canonicalPkg,
		ReproTest:          canonicalTest,
		CreatedAt:          now,
		UpdatedAt:          now,
	}); err != nil {
		log.Error("finding insert failed", "cycle_id", in.CycleID, "finding_id", id, "error", err)
		return PersistFindingResult{}, err
	}
	if err := appendOccurrenceTx(tx, in.CycleID, id, OccurrenceCreated, in.SourceStage, 1, in.Issue, canonicalAC, evidenceJSON, now, canonicalPkg, canonicalTest, canonicalSource); err != nil {
		return PersistFindingResult{}, err
	}
	created, err := getFindingTx(tx, in.CycleID, id)
	if err != nil {
		return PersistFindingResult{}, err
	}
	log.Info("finding created", "cycle_id", in.CycleID, "finding_id", id, "owner", in.Owner, "source_stage", in.SourceStage)
	if err := appendLifecycleEventTx(tx, s, in.CycleID, EventFindingCreated, map[string]any{
		"finding_id": id, "source_stage": in.SourceStage, "owner": in.Owner,
	}); err != nil {
		return PersistFindingResult{}, err
	}
	return PersistFindingResult{
		Finding:        created,
		OccurrenceKind: OccurrenceCreated,
		Actionable:     true,
	}, nil
}

// FindingOccurrence is one audit row for a finding.
type FindingOccurrence struct {
	Sequence     int
	Kind         string
	Round        int
	Issue        string
	ReproPackage string
	ReproTest    string
	ReproSource  string
	CreatedAt    string
}

// ListFindingOccurrences returns occurrence history for a finding.
func (s *Store) ListFindingOccurrences(cycleID int64, findingID string) ([]FindingOccurrence, error) {
	rows, err := s.db.Query(`
SELECT sequence, kind, round, issue, repro_package, repro_test, repro_source, created_at
FROM finding_occurrences
WHERE cycle_id = ? AND finding_id = ?
ORDER BY sequence ASC`, cycleID, findingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FindingOccurrence
	for rows.Next() {
		var o FindingOccurrence
		if err := rows.Scan(&o.Sequence, &o.Kind, &o.Round, &o.Issue, &o.ReproPackage, &o.ReproTest, &o.ReproSource, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// GetFinding returns a finding by cycle and ID.
func (s *Store) GetFinding(cycleID int64, id string) (Finding, error) {
	return getFindingTx(s.db, cycleID, id)
}

// GetFindingByFingerprint returns a finding by cycle and fingerprint.
func (s *Store) GetFindingByFingerprint(cycleID int64, fingerprint string) (Finding, error) {
	return getFindingByFingerprintTx(s.db, cycleID, fingerprint)
}

// ListFindingsByCycle returns all findings for a cycle in stable id order.
func (s *Store) ListFindingsByCycle(cycleID int64) ([]Finding, error) {
	rows, err := s.db.Query(`
SELECT `+findingSelectCols+`
FROM findings WHERE cycle_id = ? ORDER BY id ASC`, cycleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFindings(rows)
}

// ListActionableFindings returns open and reopened findings for a cycle, optionally filtered by owner.
func (s *Store) ListActionableFindings(cycleID int64, owner string) ([]Finding, error) {
	owner = strings.TrimSpace(owner)
	query := `
SELECT ` + findingSelectCols + `
FROM findings
WHERE cycle_id = ? AND status IN (?, ?)`
	args := []any{cycleID, FindingStatusOpen, FindingStatusReopened}
	if owner != "" {
		query += ` AND owner = ?`
		args = append(args, owner)
	}
	query += ` ORDER BY created_at ASC, id ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFindings(rows)
}

// ListFindingsByStatus returns findings for a cycle with any of the given statuses.
func (s *Store) ListFindingsByStatus(cycleID int64, statuses ...string) ([]Finding, error) {
	if len(statuses) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(statuses))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, 0, 1+len(statuses))
	args = append(args, cycleID)
	for _, st := range statuses {
		args = append(args, st)
	}
	rows, err := s.db.Query(`
SELECT `+findingSelectCols+`
FROM findings
WHERE cycle_id = ? AND status IN (`+placeholders+`)
ORDER BY created_at ASC, id ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFindings(rows)
}

// ValidateReopenID reports whether reopen_id may reopen a done finding whose
// stored file, requirement, and acceptance criteria match the report contract.
func (s *Store) ValidateReopenID(cycleID int64, sourceStage, owner, reopenID, file, requirement, acceptance, reproPackage, reproTest string) error {
	f, err := s.GetFinding(cycleID, reopenID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrInvalidReopenID
		}
		return err
	}
	if f.Status != FindingStatusDone || f.SourceStage != sourceStage || f.Owner != owner {
		return ErrInvalidReopenID
	}
	if !FindingContractMatches(f, file, requirement, acceptance) {
		return ErrInvalidReopenID
	}
	if !FindingReproMatches(f, reproPackage, reproTest) {
		return ErrInvalidReopenID
	}
	return nil
}

// MarkFindingDoneTx marks a finding done and appends a done occurrence (scheduler API).
func (s *Store) MarkFindingDoneTx(tx *sql.Tx, cycleID int64, id string, issue, acceptanceCriteria, evidenceJSON string) error {
	f, err := getFindingTx(tx, cycleID, id)
	if err != nil {
		return err
	}
	now := nowRFC3339()
	if err := updateFindingStatusRoundTx(tx, cycleID, id, FindingStatusDone, f.Round, now); err != nil {
		s.findingLog().Error("mark finding done failed", "cycle_id", cycleID, "finding_id", id, "error", err)
		return err
	}
	if err := appendOccurrenceTx(tx, cycleID, id, OccurrenceDone, f.SourceStage, f.Round, issue, acceptanceCriteria, evidenceJSON, now, f.ReproPackage, f.ReproTest, ""); err != nil {
		return err
	}
	s.findingLog().Info("finding marked done", "cycle_id", cycleID, "finding_id", id)
	return appendLifecycleEventTx(tx, s, cycleID, EventFindingDone, map[string]any{
		"finding_id": id, "source_stage": f.SourceStage, "round": f.Round,
	})
}

// SetFindingDeferredTodoTx marks a finding deferred_todo (escalation path).
func (s *Store) SetFindingDeferredTodoTx(tx *sql.Tx, cycleID int64, id string, issue, acceptanceCriteria, evidenceJSON string) error {
	f, err := getFindingTx(tx, cycleID, id)
	if err != nil {
		return err
	}
	now := nowRFC3339()
	if err := updateFindingStatusRoundTx(tx, cycleID, id, FindingStatusDeferredTodo, f.Round, now); err != nil {
		return err
	}
	if err := appendOccurrenceTx(tx, cycleID, id, OccurrenceDeferred, f.SourceStage, f.Round, issue, acceptanceCriteria, evidenceJSON, now, f.ReproPackage, f.ReproTest, ""); err != nil {
		return err
	}
	cycleNumber, err := cycleNumberTx(tx, cycleID)
	if err != nil {
		return err
	}
	return appendLifecycleEventTx(tx, s, cycleID, EventFindingDeferred, map[string]any{
		"finding_id": id, "source_stage": f.SourceStage, "cycle_number": cycleNumber,
	})
}

func (s *Store) findingLog() *slog.Logger {
	if s != nil && s.log != nil {
		return s.log
	}
	return slog.Default()
}

func validateFindingInput(in FindingInput) error {
	if strings.TrimSpace(in.Issue) == "" || strings.TrimSpace(in.AcceptanceCriteria) == "" {
		return fmt.Errorf("%w: issue and acceptance_criteria are required", ErrInvalidFindingContent)
	}
	if CanonicalizeFindingFile(in.File) == "" && NormalizeFindingText(in.Requirement) == "" {
		return fmt.Errorf("%w: file or requirement is required", ErrInvalidFindingContent)
	}
	if err := validateFindingTextField("issue", in.Issue); err != nil {
		return err
	}
	if err := validateFindingTextField("acceptance_criteria", in.AcceptanceCriteria); err != nil {
		return err
	}
	for i, p := range in.Evidence {
		if err := validateEvidencePath(p); err != nil {
			return fmt.Errorf("%w: evidence[%d]: %v", ErrInvalidFindingContent, i, err)
		}
	}
	if _, _, _, err := canonicalizeFindingRepro(in); err != nil {
		return err
	}
	return nil
}

func validateFindingTextField(field, value string) error {
	if redact.HasToken(value) {
		return fmt.Errorf("%w: %s contains a forbidden token pattern", ErrInvalidFindingContent, field)
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "data:") {
		return fmt.Errorf("%w: %s must not embed data URLs", ErrInvalidFindingContent, field)
	}
	if strings.Contains(value, "\x00") {
		return fmt.Errorf("%w: %s contains invalid characters", ErrInvalidFindingContent, field)
	}
	return nil
}

func validateEvidencePath(p string) error {
	p = strings.TrimSpace(p)
	if p == "" {
		return errors.New("empty path")
	}
	if strings.HasPrefix(strings.ToLower(p), "data:") {
		return errors.New("data URLs are not allowed")
	}
	if filepath.IsAbs(p) || strings.HasPrefix(p, "/") || (len(p) >= 2 && p[1] == ':') {
		return errors.New("absolute paths are not allowed")
	}
	if envhygiene.HasParentTraversal(p) {
		return errors.New("parent traversal is not allowed")
	}
	if envhygiene.IsSensitivePath(p) {
		return errors.New("sensitive path")
	}
	if redact.HasToken(p) {
		return errors.New("forbidden token pattern")
	}
	return nil
}

func marshalEvidenceJSON(evidence []string) (string, error) {
	if evidence == nil {
		return "[]", nil
	}
	out := make([]string, len(evidence))
	for i, p := range evidence {
		out[i] = strings.TrimSpace(p)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("marshal evidence: %w", err)
	}
	return string(b), nil
}

func findingIDPrefix(sourceStage string) (string, error) {
	switch sourceStage {
	case FindingSourceQA:
		return "find-qa-", nil
	case FindingSourceJudge:
		return "find-judge-", nil
	case FindingSourceBrowserUI:
		return "find-bui-", nil
	case FindingSourceQAEndToEnd:
		return "find-e2e-", nil
	default:
		return "", fmt.Errorf("unknown source stage %q", sourceStage)
	}
}

func allocateFindingIDTx(q queryer, cycleID int64, sourceStage string) (string, error) {
	prefix, err := findingIDPrefix(sourceStage)
	if err != nil {
		return "", err
	}
	rows, err := q.Query(`SELECT id FROM findings WHERE cycle_id = ? AND id LIKE ?`, cycleID, prefix+"%")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	maxN := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(id, prefix)
		n, err := strconv.Atoi(suffix)
		if err != nil || n <= 0 {
			continue
		}
		if n > maxN {
			maxN = n
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return prefix + strconv.Itoa(maxN+1), nil
}

type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

type execQueryer interface {
	queryer
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

func getFindingTx(q execQueryer, cycleID int64, id string) (Finding, error) {
	row := q.QueryRow(`SELECT `+findingSelectCols+` FROM findings WHERE cycle_id = ? AND id = ?`, cycleID, id)
	return scanFinding(row)
}

func getFindingByFingerprintTx(q execQueryer, cycleID int64, fingerprint string) (Finding, error) {
	row := q.QueryRow(`SELECT `+findingSelectCols+` FROM findings WHERE cycle_id = ? AND fingerprint = ?`, cycleID, fingerprint)
	f, err := scanFinding(row)
	if err == sql.ErrNoRows {
		return Finding{}, ErrNotFound
	}
	return f, err
}

func insertFindingTx(tx *sql.Tx, f Finding) error {
	_, err := tx.Exec(`
INSERT INTO findings(id, cycle_id, source_stage, owner, status, fingerprint, file, requirement,
  issue, acceptance_criteria, evidence_json, round, todo_id, repro_package, repro_test, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.CycleID, f.SourceStage, f.Owner, f.Status, f.Fingerprint,
		nullStr(f.File), nullStr(f.Requirement), f.Issue, f.AcceptanceCriteria, f.EvidenceJSON,
		f.Round, nullStr(f.TodoID), f.ReproPackage, f.ReproTest, f.CreatedAt, f.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert finding: %w", err)
	}
	return nil
}

func updateFindingStatusRoundTx(tx *sql.Tx, cycleID int64, id, status string, round int, updatedAt string) error {
	_, err := tx.Exec(`
UPDATE findings SET status = ?, round = ?, updated_at = ?
WHERE cycle_id = ? AND id = ?`,
		status, round, updatedAt, cycleID, id,
	)
	if err != nil {
		return fmt.Errorf("update finding status: %w", err)
	}
	return nil
}

func touchFindingUpdatedAtTx(tx *sql.Tx, cycleID int64, id, updatedAt string) error {
	_, err := tx.Exec(`
UPDATE findings SET updated_at = ?
WHERE cycle_id = ? AND id = ?`,
		updatedAt, cycleID, id,
	)
	return err
}

func nextOccurrenceSequenceTx(tx *sql.Tx, cycleID int64, findingID string) (int, error) {
	var seq sql.NullInt64
	err := tx.QueryRow(`
SELECT MAX(sequence) FROM finding_occurrences WHERE cycle_id = ? AND finding_id = ?`,
		cycleID, findingID).Scan(&seq)
	if err != nil {
		return 0, err
	}
	if !seq.Valid {
		return 1, nil
	}
	return int(seq.Int64) + 1, nil
}

func appendOccurrenceTx(tx *sql.Tx, cycleID int64, findingID, kind, sourceStage string, round int, issue, acceptance, evidenceJSON, createdAt, reproPackage, reproTest, reproSource string) error {
	seq, err := nextOccurrenceSequenceTx(tx, cycleID, findingID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
INSERT INTO finding_occurrences(cycle_id, finding_id, sequence, kind, source_stage, round, issue, acceptance_criteria, evidence_json, created_at, repro_package, repro_test, repro_source)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cycleID, findingID, seq, kind, sourceStage, round, issue, acceptance, evidenceJSON, createdAt, reproPackage, reproTest, reproSource,
	)
	if err != nil {
		return fmt.Errorf("insert finding occurrence: %w", err)
	}
	return nil
}

func scanFinding(row rowScanner) (Finding, error) {
	var f Finding
	var file, req, todo sql.NullString
	err := row.Scan(
		&f.ID, &f.CycleID, &f.SourceStage, &f.Owner, &f.Status, &f.Fingerprint,
		&file, &req, &f.Issue, &f.AcceptanceCriteria, &f.EvidenceJSON,
		&f.Round, &todo, &f.ReproPackage, &f.ReproTest, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Finding{}, ErrNotFound
		}
		return Finding{}, err
	}
	f.File = file.String
	f.Requirement = req.String
	f.TodoID = todo.String
	return f, nil
}

func scanFindings(rows *sql.Rows) ([]Finding, error) {
	var out []Finding
	for rows.Next() {
		f, err := scanFinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
