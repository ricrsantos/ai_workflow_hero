package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	blockedValidationID        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	blockedValidationUserID    = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	blockedValidationProfileID = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,63}$`)
	blockedValidationDigest    = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// StageBlocker is a sanitized operational blocker attached to a validation
// stage. It deliberately has no field for raw browser output or credentials.
type StageBlocker struct {
	CycleID               int64
	StageName             string
	ID                    string
	Reason                string
	AffectedCoverageIDs   []string
	AffectedProfileIDs    []string
	DiagnosticUncertainty string
	NextAction            string
	Status                string
	CreatedAt             string
	ResolvedAt            string
}

// StageCoverage is one row of the scheduler-owned coverage denominator and
// its latest admitted result. Scope fields are immutable after initial plan
// creation; report persistence updates only result, reason, evidence and time.
type StageCoverage struct {
	CycleID                 int64
	StageName               string
	ID                      string
	UserID                  string
	ProfileID               string
	Mandatory               bool
	RequirementRef          string
	ScreenJourney           string
	ExpectedResult          string
	EvidenceRequirements    []string
	OptionalReferenceWidths []int
	Result                  string
	Reason                  string
	Evidence                []string
	UpdatedAt               string
}

const (
	StageBlockerActive   = "active"
	StageBlockerResolved = "resolved"

	StageCoveragePlanned  = "planned"
	StageCoverageExecuted = "executed"
	StageCoveragePassed   = "passed"
	StageCoverageFailed   = "failed"
	StageCoverageBlocked  = "blocked"
	StageCoverageSkipped  = "skipped"
)

var ErrCoveragePlanExists = errors.New("coverage plan already exists; changing its scope requires an approved plan update")
var ErrCoveragePlanChanged = errors.New("browser plan differs from the approved coverage snapshot; explicit approval and plan update are required")

// CreateStageCoveragePlanTx stores an initial approved denominator. Later
// attempts cannot silently rewrite mandatory scope or profile assignments.
// Explicitly approved plan editing is owned by the coverage-gate service.
func (s *Store) CreateStageCoveragePlanTx(tx *sql.Tx, cycleID int64, stageName string, plan []StageCoverage, now string) error {
	if tx == nil || cycleID <= 0 || !browserValidationStage(stageName) || len(plan) == 0 {
		return errors.New("invalid stage coverage plan")
	}
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM stage_coverage WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName).Scan(&count); err != nil {
		return fmt.Errorf("check stage coverage plan: %w", err)
	}
	if count != 0 {
		return ErrCoveragePlanExists
	}
	seen := make(map[string]struct{}, len(plan))
	for _, row := range plan {
		if row.CycleID != 0 && row.CycleID != cycleID || row.StageName != "" && row.StageName != stageName {
			return errors.New("coverage plan row has a different cycle or stage")
		}
		if err := validateStageCoverage(row, true); err != nil {
			return err
		}
		if _, exists := seen[row.ID]; exists {
			return errors.New("coverage plan IDs must be unique")
		}
		seen[row.ID] = struct{}{}
		result := row.Result
		if result == "" {
			result = StageCoveragePlanned
		}
		if result != StageCoveragePlanned {
			return errors.New("initial coverage plan rows must be planned")
		}
		updated := row.UpdatedAt
		if updated == "" {
			updated = now
		}
		if updated == "" {
			updated = nowRFC3339()
		}
		evidenceRequirements, err := json.Marshal(row.EvidenceRequirements)
		if err != nil {
			return fmt.Errorf("encode coverage evidence requirements: %w", err)
		}
		optionalWidths, err := json.Marshal(row.OptionalReferenceWidths)
		if err != nil {
			return fmt.Errorf("encode optional reference widths: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO stage_coverage
  (cycle_id, stage_name, coverage_id, user_id, profile_id, mandatory, requirement_ref, screen_journey,
   expected_result, evidence_requirements_json, optional_reference_widths_json, result, reason, evidence_json, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '[]', ?)`,
			cycleID, stageName, row.ID, row.UserID, row.ProfileID, boolInt(row.Mandatory), strings.TrimSpace(row.RequirementRef),
			strings.TrimSpace(row.ScreenJourney), strings.TrimSpace(row.ExpectedResult), string(evidenceRequirements), string(optionalWidths), result, updated); err != nil {
			return fmt.Errorf("insert stage coverage plan: %w", err)
		}
	}
	return nil
}

// CreateStageCoverageSnapshotTx atomically admits the immutable Planning
// denominator and the digest of the complete non-secret browser plan.
func (s *Store) CreateStageCoverageSnapshotTx(tx *sql.Tx, cycleID int64, stageName string, plan []StageCoverage, digest, now string) error {
	if tx == nil || !blockedValidationDigest.MatchString(digest) {
		return errors.New("invalid approved browser plan digest")
	}
	if now == "" {
		now = nowRFC3339()
	}
	if err := s.CreateStageCoveragePlanTx(tx, cycleID, stageName, plan, now); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO stage_coverage_plans (cycle_id, stage_name, plan_digest, approved_at)
VALUES (?, ?, ?, ?)`, cycleID, stageName, digest, now); err != nil {
		return fmt.Errorf("persist approved browser plan digest: %w", err)
	}
	return nil
}

// ReplaceStageCoverageSnapshotApprovedTx changes an existing denominator only
// when the caller represents an explicit human approval of the updated
// Planning stage. Callers must pass true only from that scheduler transition.
func (s *Store) ReplaceStageCoverageSnapshotApprovedTx(tx *sql.Tx, cycleID int64, stageName string, plan []StageCoverage, digest, now string, approved bool) error {
	if tx == nil || !approved || !blockedValidationDigest.MatchString(digest) {
		return errors.New("approved browser plan update is required")
	}
	if cycleID <= 0 || !browserValidationStage(stageName) {
		return errors.New("invalid approved browser plan update")
	}
	if now == "" {
		now = nowRFC3339()
	}
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM stage_coverage_plans WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName).Scan(&count); err != nil {
		return fmt.Errorf("check approved browser plan update: %w", err)
	}
	if count != 1 {
		return ErrNotFound
	}
	if _, err := tx.Exec(`DELETE FROM stage_coverage WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName); err != nil {
		return fmt.Errorf("replace approved browser coverage: %w", err)
	}
	if err := s.CreateStageCoveragePlanTx(tx, cycleID, stageName, plan, now); err != nil {
		return err
	}
	result, err := tx.Exec(`UPDATE stage_coverage_plans SET plan_digest = ?, approved_at = ? WHERE cycle_id = ? AND stage_name = ?`, digest, now, cycleID, stageName)
	if err != nil {
		return fmt.Errorf("record approved browser plan update: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify approved browser plan update: %w", err)
	}
	if changed != 1 {
		return ErrNotFound
	}
	return nil
}

// GetStageCoveragePlanSnapshot returns the immutable approved-plan digest.
func (s *Store) GetStageCoveragePlanSnapshot(cycleID int64, stageName string) (StageCoveragePlanSnapshot, error) {
	if cycleID <= 0 || !browserValidationStage(stageName) {
		return StageCoveragePlanSnapshot{}, errors.New("invalid browser plan snapshot query")
	}
	var snapshot StageCoveragePlanSnapshot
	err := s.db.QueryRow(`SELECT cycle_id, stage_name, plan_digest, approved_at
FROM stage_coverage_plans WHERE cycle_id = ? AND stage_name = ?`, cycleID, stageName).
		Scan(&snapshot.CycleID, &snapshot.StageName, &snapshot.Digest, &snapshot.ApprovedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return StageCoveragePlanSnapshot{}, ErrNotFound
	}
	if err != nil {
		return StageCoveragePlanSnapshot{}, fmt.Errorf("read approved browser plan digest: %w", err)
	}
	if !blockedValidationDigest.MatchString(snapshot.Digest) {
		return StageCoveragePlanSnapshot{}, errors.New("stored browser plan digest is invalid")
	}
	return snapshot, nil
}

// ListStageCoverage returns every planned denominator row regardless of its
// most recent result, so blocked retries never lose their approved scope.
func (s *Store) ListStageCoverage(cycleID int64, stageName string) ([]StageCoverage, error) {
	if cycleID <= 0 || !browserValidationStage(stageName) {
		return nil, errors.New("invalid stage coverage query")
	}
	rows, err := s.db.Query(`SELECT cycle_id, stage_name, coverage_id, user_id, profile_id,
       mandatory, requirement_ref, screen_journey, expected_result, evidence_requirements_json,
       optional_reference_widths_json, result, reason, evidence_json, updated_at
FROM stage_coverage WHERE cycle_id = ? AND stage_name = ? ORDER BY coverage_id, user_id`, cycleID, stageName)
	if err != nil {
		return nil, fmt.Errorf("list stage coverage: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []StageCoverage
	for rows.Next() {
		row, err := scanStageCoverage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stage coverage: %w", err)
	}
	return out, nil
}

// ListStageCoverageTx is the transaction-scoped counterpart of
// ListStageCoverage.
func (s *Store) ListStageCoverageTx(tx *sql.Tx, cycleID int64, stageName string) ([]StageCoverage, error) {
	if tx == nil || cycleID <= 0 || !browserValidationStage(stageName) {
		return nil, errors.New("invalid stage coverage query")
	}
	rows, err := tx.Query(`SELECT cycle_id, stage_name, coverage_id, user_id, profile_id,
       mandatory, requirement_ref, screen_journey, expected_result, evidence_requirements_json,
       optional_reference_widths_json, result, reason, evidence_json, updated_at
FROM stage_coverage WHERE cycle_id = ? AND stage_name = ? ORDER BY coverage_id, user_id`, cycleID, stageName)
	if err != nil {
		return nil, fmt.Errorf("list stage coverage: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []StageCoverage
	for rows.Next() {
		row, err := scanStageCoverage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stage coverage: %w", err)
	}
	return out, nil
}

// PersistStageCoverageResultsTx updates only result data for rows already in
// the approved denominator. Unknown IDs or user assignments abort the caller's
// transaction rather than expanding scope from an agent report.
func (s *Store) PersistStageCoverageResultsTx(tx *sql.Tx, cycleID int64, stageName string, results []StageCoverage, now string) error {
	if tx == nil || cycleID <= 0 || !browserValidationStage(stageName) {
		return errors.New("invalid stage coverage result")
	}
	seen := make(map[string]struct{}, len(results))
	for _, row := range results {
		if row.CycleID != 0 && row.CycleID != cycleID || row.StageName != "" && row.StageName != stageName {
			return errors.New("coverage result row has a different cycle or stage")
		}
		if err := validateStageCoverage(row, false); err != nil {
			return err
		}
		key := row.ID + "\x00" + row.UserID
		if _, exists := seen[key]; exists {
			return errors.New("coverage result rows must be unique")
		}
		seen[key] = struct{}{}
		evidence, err := json.Marshal(row.Evidence)
		if err != nil {
			return fmt.Errorf("encode stage coverage evidence: %w", err)
		}
		updated := row.UpdatedAt
		if updated == "" {
			updated = now
		}
		if updated == "" {
			updated = nowRFC3339()
		}
		res, err := tx.Exec(`UPDATE stage_coverage
SET result = ?, reason = ?, evidence_json = ?, updated_at = ?
WHERE cycle_id = ? AND stage_name = ? AND coverage_id = ? AND user_id = ? AND profile_id = ?`,
			row.Result, row.Reason, string(evidence), updated, cycleID, stageName, row.ID, row.UserID, row.ProfileID)
		if err != nil {
			return fmt.Errorf("update stage coverage result: %w", err)
		}
		changed, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("verify stage coverage result: %w", err)
		}
		if changed != 1 {
			return fmt.Errorf("coverage result %q does not match the approved plan", row.ID)
		}
	}
	return nil
}

// ReplaceActiveStageBlockersTx resolves the previous active snapshot and
// inserts the new sanitized blocker set atomically with its blocked report.
func (s *Store) ReplaceActiveStageBlockersTx(tx *sql.Tx, cycleID int64, stageName string, blockers []StageBlocker, now string) error {
	if tx == nil || cycleID <= 0 || !browserValidationStage(stageName) || len(blockers) == 0 {
		return errors.New("invalid blocked-stage blocker set")
	}
	if now == "" {
		now = nowRFC3339()
	}
	if _, err := tx.Exec(`UPDATE stage_blockers SET status = 'resolved', resolved_at = ?
WHERE cycle_id = ? AND stage_name = ? AND status = 'active'`, now, cycleID, stageName); err != nil {
		return fmt.Errorf("resolve previous stage blockers: %w", err)
	}
	seen := make(map[string]struct{}, len(blockers))
	for _, blocker := range blockers {
		if blocker.CycleID != 0 && blocker.CycleID != cycleID || blocker.StageName != "" && blocker.StageName != stageName {
			return errors.New("stage blocker has a different cycle or stage")
		}
		if !blockedValidationID.MatchString(blocker.ID) || strings.TrimSpace(blocker.Reason) == "" || strings.TrimSpace(blocker.NextAction) == "" {
			return errors.New("stage blocker requires safe ID, reason and next action")
		}
		if _, exists := seen[blocker.ID]; exists {
			return errors.New("stage blocker IDs must be unique")
		}
		seen[blocker.ID] = struct{}{}
		for _, id := range blocker.AffectedCoverageIDs {
			if !blockedValidationID.MatchString(id) {
				return errors.New("stage blocker has an invalid affected coverage ID")
			}
		}
		for _, id := range blocker.AffectedProfileIDs {
			if !blockedValidationID.MatchString(id) {
				return errors.New("stage blocker has an invalid affected profile ID")
			}
		}
		coverageJSON, err := json.Marshal(blocker.AffectedCoverageIDs)
		if err != nil {
			return fmt.Errorf("encode affected coverage IDs: %w", err)
		}
		profilesJSON, err := json.Marshal(blocker.AffectedProfileIDs)
		if err != nil {
			return fmt.Errorf("encode affected profile IDs: %w", err)
		}
		created := blocker.CreatedAt
		if created == "" {
			created = now
		}
		if _, err := tx.Exec(`INSERT INTO stage_blockers
  (cycle_id, stage_name, blocker_id, reason, affected_coverage_ids_json, affected_profile_ids_json,
   diagnostic_uncertainty, next_action, status, created_at, resolved_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, NULL)
ON CONFLICT(cycle_id, stage_name, blocker_id) DO UPDATE SET
  reason = excluded.reason,
  affected_coverage_ids_json = excluded.affected_coverage_ids_json,
  affected_profile_ids_json = excluded.affected_profile_ids_json,
  diagnostic_uncertainty = excluded.diagnostic_uncertainty,
  next_action = excluded.next_action,
  status = 'active', created_at = excluded.created_at, resolved_at = NULL`,
			cycleID, stageName, blocker.ID, blocker.Reason, string(coverageJSON), string(profilesJSON),
			blocker.DiagnosticUncertainty, blocker.NextAction, created); err != nil {
			return fmt.Errorf("insert stage blocker: %w", err)
		}
	}
	return nil
}

// ResolveActiveStageBlockersTx clears the operational pause only after the
// caller has rechecked current prerequisites.
func (s *Store) ResolveActiveStageBlockersTx(tx *sql.Tx, cycleID int64, stageName, now string) error {
	if tx == nil || cycleID <= 0 || !browserValidationStage(stageName) {
		return errors.New("invalid stage blocker resolution")
	}
	if now == "" {
		now = nowRFC3339()
	}
	if _, err := tx.Exec(`UPDATE stage_blockers SET status = 'resolved', resolved_at = ?
WHERE cycle_id = ? AND stage_name = ? AND status = 'active'`, now, cycleID, stageName); err != nil {
		return fmt.Errorf("resolve active stage blockers: %w", err)
	}
	return nil
}

// ListActiveStageBlockers returns safe operational state for one stage.
func (s *Store) ListActiveStageBlockers(cycleID int64, stageName string) ([]StageBlocker, error) {
	if cycleID <= 0 || !browserValidationStage(stageName) {
		return nil, errors.New("invalid active stage blocker query")
	}
	rows, err := s.db.Query(`SELECT cycle_id, stage_name, blocker_id, reason,
       affected_coverage_ids_json, affected_profile_ids_json, diagnostic_uncertainty,
       next_action, status, created_at, resolved_at
FROM stage_blockers WHERE cycle_id = ? AND stage_name = ? AND status = 'active' ORDER BY blocker_id`, cycleID, stageName)
	if err != nil {
		return nil, fmt.Errorf("list active stage blockers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []StageBlocker
	for rows.Next() {
		var blocker StageBlocker
		var coverageJSON, profilesJSON string
		var resolvedAt sql.NullString
		if err := rows.Scan(&blocker.CycleID, &blocker.StageName, &blocker.ID, &blocker.Reason,
			&coverageJSON, &profilesJSON, &blocker.DiagnosticUncertainty, &blocker.NextAction,
			&blocker.Status, &blocker.CreatedAt, &resolvedAt); err != nil {
			return nil, fmt.Errorf("scan active stage blocker: %w", err)
		}
		if resolvedAt.Valid {
			blocker.ResolvedAt = resolvedAt.String
		}
		if err := json.Unmarshal([]byte(coverageJSON), &blocker.AffectedCoverageIDs); err != nil {
			return nil, errors.New("stored blocker coverage metadata is invalid")
		}
		if err := json.Unmarshal([]byte(profilesJSON), &blocker.AffectedProfileIDs); err != nil {
			return nil, errors.New("stored blocker profile metadata is invalid")
		}
		out = append(out, blocker)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active stage blockers: %w", err)
	}
	return out, nil
}

type stageCoverageScanner interface {
	Scan(dest ...any) error
}

func scanStageCoverage(scanner stageCoverageScanner) (StageCoverage, error) {
	var row StageCoverage
	var mandatory int
	var evidenceRequirementsJSON, optionalWidthsJSON, evidenceJSON string
	if err := scanner.Scan(&row.CycleID, &row.StageName, &row.ID, &row.UserID, &row.ProfileID,
		&mandatory, &row.RequirementRef, &row.ScreenJourney, &row.ExpectedResult, &evidenceRequirementsJSON,
		&optionalWidthsJSON, &row.Result, &row.Reason, &evidenceJSON, &row.UpdatedAt); err != nil {
		return StageCoverage{}, fmt.Errorf("scan stage coverage: %w", err)
	}
	row.Mandatory = mandatory != 0
	if err := json.Unmarshal([]byte(evidenceRequirementsJSON), &row.EvidenceRequirements); err != nil {
		return StageCoverage{}, errors.New("stored coverage evidence requirements are invalid")
	}
	if err := json.Unmarshal([]byte(optionalWidthsJSON), &row.OptionalReferenceWidths); err != nil {
		return StageCoverage{}, errors.New("stored optional reference widths are invalid")
	}
	if err := json.Unmarshal([]byte(evidenceJSON), &row.Evidence); err != nil {
		return StageCoverage{}, errors.New("stored coverage evidence is invalid")
	}
	return row, nil
}

func validateStageCoverage(row StageCoverage, plan bool) error {
	if !blockedValidationID.MatchString(row.ID) || row.UserID != "" && !blockedValidationUserID.MatchString(row.UserID) || row.ProfileID != "" && !blockedValidationProfileID.MatchString(row.ProfileID) {
		return errors.New("invalid stage coverage identifier")
	}
	if row.UserID != "" && row.ProfileID == "" || row.UserID == "" && row.ProfileID != "" && row.ProfileID != "anonymous" {
		return errors.New("coverage user and profile must be supplied together")
	}
	if plan && strings.TrimSpace(row.ExpectedResult) == "" {
		return errors.New("coverage expected result is required")
	}
	if plan {
		for _, value := range append([]string{row.RequirementRef, row.ScreenJourney}, row.EvidenceRequirements...) {
			if strings.ContainsAny(value, "\x00\r\n") || strings.TrimSpace(value) != value {
				return errors.New("coverage plan metadata is invalid")
			}
		}
		seenWidths := make(map[int]struct{}, len(row.OptionalReferenceWidths))
		for _, width := range row.OptionalReferenceWidths {
			if width != 1280 && width != 768 && width != 375 {
				return errors.New("optional reference width is unsupported")
			}
			if _, exists := seenWidths[width]; exists {
				return errors.New("optional reference widths must be unique")
			}
			seenWidths[width] = struct{}{}
		}
	}
	if plan {
		return nil
	}
	switch row.Result {
	case StageCoverageExecuted, StageCoveragePassed, StageCoverageFailed, StageCoverageBlocked, StageCoverageSkipped:
	default:
		return errors.New("invalid stage coverage result")
	}
	return nil
}

func browserValidationStage(stageName string) bool {
	return stageName == "browser_ui_validation" || stageName == "qa_end_to_end"
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
