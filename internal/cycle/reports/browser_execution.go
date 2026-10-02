package reports

import (
	"encoding/json"
	"path"
	"regexp"
	"strconv"
	"strings"
)

const ValidationStatusBlocked = "blocked"

// CoveragePlanItem is scheduler-owned Planning scope, not a report's claim.
// Changing mandatory scope requires approving and updating this plan first.
type CoveragePlanItem struct {
	ID                      string   `json:"id"`
	Requirement             string   `json:"requirement"`
	Acceptance              string   `json:"acceptance"`
	ScreenJourney           string   `json:"screen_journey"`
	UserID                  string   `json:"user_id"`
	ProfileID               string   `json:"profile_id"`
	Mandatory               bool     `json:"mandatory"`
	ExpectedResult          string   `json:"expected_result"`
	EvidenceRequirements    []string `json:"evidence_requirements,omitempty"`
	OptionalReferenceWidths []int    `json:"optional_reference_widths,omitempty"`
}

type Preparation struct {
	Status                    string   `json:"status"`
	Method                    string   `json:"method"`
	VerifiedProfileIDs        []string `json:"verified_profile_ids"`
	MethodAdmitted            bool     `json:"method_admitted,omitempty"`
	ObservedToolName          string   `json:"observed_tool_name,omitempty"`
	ObservedToolVersion       string   `json:"observed_tool_version,omitempty"`
	ObservedPlaywrightVersion string   `json:"observed_playwright_version,omitempty"`
}

type Blocker struct {
	ID                  string   `json:"id"`
	Reason              string   `json:"reason"`
	AffectedCoverageIDs []string `json:"affected_coverage_ids"`
	AffectedProfileIDs  []string `json:"affected_profile_ids"`
	Uncertainty         string   `json:"uncertainty"`
	NextAction          string   `json:"next_action"`
}

type CoverageChecks struct {
	Render             bool `json:"render"`
	CSS                bool `json:"css"`
	Console            bool `json:"console"`
	Network            bool `json:"network"`
	HealthBeforeVisual bool `json:"health_before_visual"`
	DesktopWidth       int  `json:"desktop_width"`
	BusinessOutcome    bool `json:"business_outcome"`
}

type CoverageItem struct {
	ID              string         `json:"id"`
	Result          string         `json:"result"`
	Evidence        []string       `json:"evidence"`
	Checks          CoverageChecks `json:"checks"`
	ReferenceWidths []int          `json:"reference_widths,omitempty"`
	CaptureSafety   CaptureSafety  `json:"capture_safety,omitempty"`
}

// CaptureSafety is a value-free attestation required before a staged image can
// become cycle evidence. It never carries page text, selectors, or credentials.
type CaptureSafety struct {
	StablePointVerified               bool `json:"stable_point_verified"`
	SensitiveFieldsAndTokensMasked    bool `json:"sensitive_fields_and_tokens_masked"`
	CredentialFlowArtifactsSuppressed bool `json:"credential_flow_artifacts_suppressed"`
}

type Coverage struct {
	PlannedIDs []string       `json:"planned_ids"`
	Items      []CoverageItem `json:"items"`
}

// CoverageCounts are recomputed from admitted items, never trusted from an agent.
type CoverageCounts struct{ Planned, Executed, Passed, Failed, Blocked, Skipped int }

type BrowserExecution struct {
	Preparation *Preparation
	Blockers    []Blocker
	Coverage    *Coverage
	Counts      CoverageCounts
	Warnings    []ReportWarning
}

var executionID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

func parseBrowserStatus(root object) (string, *DiagnosticError) {
	status, err := requireString(root, "status")
	if err != nil {
		return "", err
	}
	status = strings.ToLower(status)
	if status != ValidationStatusPassed && status != ValidationStatusFailed && status != ValidationStatusBlocked {
		return "", diag(CodeInvalidEnum, "status", "", "status must be passed, failed or blocked")
	}
	return status, nil
}

// SafeEvidenceReference accepts only managed relative references, never external
// URLs, shell expressions, credential paths or paths escaping cycle evidence.
func SafeEvidenceReference(ref string) bool {
	if ref == "" || path.IsAbs(ref) || strings.ContainsAny(ref, "\\\x00\r\n$`") || path.Clean(ref) != ref {
		return false
	}
	parts := strings.Split(ref, "/")
	for _, part := range parts {
		if part == "." || part == ".." || strings.HasPrefix(part, ".env") {
			return false
		}
	}
	return strings.HasPrefix(ref, ".workflow-hero/cycles/current/") && len(parts) > 4
}

func decodeBrowserExecution(root object, status, source string, ctx DecodeContext) (BrowserExecution, *DiagnosticError) {
	var result BrowserExecution
	_, present := root["coverage"]
	if !present && status != ValidationStatusBlocked && ctx.CoveragePlan == nil {
		return result, nil // Existing cycles with no planned denominator remain readable.
	}
	if ctx.CoveragePlan == nil {
		return result, diag(CodeMissingField, "coverage.plan", "", "browser validation requires a scheduler-owned Planning denominator")
	}
	for _, name := range []string{"preparation", "blockers", "coverage"} {
		raw, ok := root[name]
		if !ok || string(raw) == "null" {
			return result, diag(CodeMissingField, name, "", "browser execution field is required")
		}
		var target any
		switch name {
		case "preparation":
			target = &result.Preparation
		case "blockers":
			target = &result.Blockers
		case "coverage":
			target = &result.Coverage
		}
		if json.Unmarshal(raw, target) != nil {
			return result, diag(CodeInvalidEnum, name, "", "malformed browser execution field")
		}
	}
	p := result.Preparation
	if p == nil || (p.Status != "ok" && p.Status != "blocked") {
		return result, diag(CodeInvalidEnum, "preparation.status", "", "preparation must be ok or blocked")
	}
	if p.Method != "playwright_test" && p.Method != "cli_skill" && p.Method != "cli" && p.Method != "mcp" && p.Method != "http" {
		return result, diag(CodeInvalidEnum, "preparation.method", "", "unsupported preparation method")
	}
	if ctx.ExpectedBrowserMethod != "" && p.Method != ctx.ExpectedBrowserMethod {
		return result, diag(CodeFalseAcceptanceGate, "preparation.method", "", "reported method must match the Planning-approved method")
	}
	if ctx.ExpectedBrowserMethod != "" && ctx.ExpectedBrowserMethod != "http" && p.Status == "ok" {
		if !p.MethodAdmitted {
			return result, diag(CodeFalseAcceptanceGate, "preparation.method_admitted", "", "passing browser validation requires method admission in the active stage session")
		}
		if !safeObservedToolName(p.ObservedToolName) || !strings.EqualFold(strings.TrimSpace(p.ObservedToolName), strings.TrimSpace(ctx.ExpectedBrowserTool)) || !executionID.MatchString(p.ObservedToolVersion) {
			return result, diag(CodeFalseAcceptanceGate, "preparation.observed_tool", "", "passing browser validation requires the planned tool and its observed version")
		}
		minimum := ctx.MinimumPlaywrightVersion
		if minimum == "" {
			minimum = "1.63.0"
		}
		if !playwrightVersionAtLeast(p.ObservedPlaywrightVersion, minimum) {
			return result, diag(CodeFalseAcceptanceGate, "preparation.observed_playwright_version", "", "passing browser validation requires Playwright at or above the supported minimum")
		}
	}
	if source == SourceBrowserUI && p.Method == "http" {
		return result, diag(CodeInvalidEnum, "preparation.method", "", "Browser UI requires real browser execution")
	}
	if err := uniqueExecutionIDs(p.VerifiedProfileIDs, "preparation.verified_profile_ids"); err != nil {
		return result, err
	}
	if status != ValidationStatusBlocked && p.Status != "ok" {
		return result, diag(CodeFalseAcceptanceGate, "preparation.status", "", "validation requires successful preparation")
	}
	if result.Coverage == nil {
		return result, diag(CodeMissingField, "coverage", "", "coverage is required")
	}
	c := result.Coverage
	if err := uniqueExecutionIDs(c.PlannedIDs, "coverage.planned_ids"); err != nil {
		return result, err
	}
	planned := make(map[string]CoveragePlanItem, len(c.PlannedIDs))
	for _, id := range c.PlannedIDs {
		planned[id] = CoveragePlanItem{ID: id}
	}
	if ctx.CoveragePlan != nil {
		if len(ctx.CoveragePlan) != len(planned) {
			return result, diag(CodeAssignmentUnionMismatch, "coverage.planned_ids", "", "denominator must match approved Planning scope")
		}
		for _, item := range ctx.CoveragePlan {
			if _, ok := planned[item.ID]; !ok {
				return result, diag(CodeAssignmentUnionMismatch, "coverage.planned_ids", "", "denominator must match approved Planning scope")
			}
			planned[item.ID] = item
		}
	}
	result.Counts.Planned = len(planned)
	seen := make(map[string]bool)
	for _, item := range c.Items {
		plan, ok := planned[item.ID]
		if !ok {
			return result, diag(CodeUnassignedID, "coverage.items.id", "", "coverage item is not planned")
		}
		if seen[item.ID] {
			return result, diag(CodeDuplicateID, "coverage.items.id", "", "coverage IDs must be unique")
		}
		seen[item.ID] = true
		for _, ref := range item.Evidence {
			if !SafeEvidenceReference(ref) || (ctx.EvidenceReferenceValidator != nil && !ctx.EvidenceReferenceValidator(ref)) {
				return result, diag(CodeInvalidEnum, "coverage.items.evidence", "", "evidence must be a safe managed reference")
			}
		}
		switch item.Result {
		case "passed":
			result.Counts.Passed++
			result.Counts.Executed++
		case "failed":
			result.Counts.Failed++
			result.Counts.Executed++
		case "blocked":
			result.Counts.Blocked++
		case "skipped":
			result.Counts.Skipped++
		default:
			return result, diag(CodeInvalidEnum, "coverage.items.result", "", "coverage result must be passed, failed, blocked or skipped")
		}
		if status == ValidationStatusPassed && plan.Mandatory && item.Result != "passed" {
			return result, diag(CodeFalseAcceptanceGate, "coverage", "", "all mandatory coverage must pass")
		}
		if plan.Mandatory && item.Result == "passed" {
			if plan.ProfileID != "" && plan.UserID != "" {
				verified := false
				for _, profile := range p.VerifiedProfileIDs {
					if profile == plan.ProfileID {
						verified = true
						break
					}
				}
				if !verified {
					return result, diag(CodeFalseAcceptanceGate, "preparation.verified_profile_ids", "", "mandatory authenticated coverage requires its verified planned profile")
				}
			}
			if len(item.Evidence) == 0 {
				return result, diag(CodeFalseAcceptanceGate, "coverage.items.evidence", "", "mandatory passing items require evidence")
			}
			if len(item.Evidence) < len(plan.EvidenceRequirements) {
				return result, diag(CodeFalseAcceptanceGate, "coverage.items.evidence", "", "mandatory passing items must supply evidence for every planned evidence requirement")
			}
			if source == SourceBrowserUI && (!item.Checks.Render || !item.Checks.CSS || !item.Checks.Console || !item.Checks.Network || !item.Checks.HealthBeforeVisual || item.Checks.DesktopWidth != 1280) {
				return result, diag(CodeFalseAcceptanceGate, "coverage.items.checks", "", "mandatory screens require Health-before-Visual and 1280 desktop health checks")
			}
			if source == SourceQAEndToEnd && !item.Checks.BusinessOutcome {
				return result, diag(CodeFalseAcceptanceGate, "coverage.items.checks.business_outcome", "", "navigation alone does not validate a business outcome")
			}
		}
		for _, width := range plan.OptionalReferenceWidths {
			if width != 1280 && width != 768 && width != 375 {
				return result, diag(CodeInvalidEnum, "coverage.plan.optional_reference_widths", "", "reference widths must be 1280, 768 or 375")
			}
			found := false
			for _, supplied := range item.ReferenceWidths {
				if width == supplied {
					found = true
					break
				}
			}
			if !found {
				result.Warnings = append(result.Warnings, warn(Code("optional_reference_missing"), "coverage.items.reference_widths", item.ID, "missing optional visual reference is a warning, not a failing gate"))
			}
		}
	}
	if len(seen) != len(planned) {
		return result, diag(CodeAssignmentUnionMismatch, "coverage.items", "", "every planned item must be accounted for")
	}
	blockerIDs := make(map[string]bool)
	for _, blocker := range result.Blockers {
		if !executionID.MatchString(blocker.ID) || !executionID.MatchString(blocker.Reason) || blockerIDs[blocker.ID] || strings.TrimSpace(blocker.Uncertainty) == "" || strings.TrimSpace(blocker.NextAction) == "" {
			return result, diag(CodeInvalidEnum, "blockers", "", "blockers require unique IDs, stable reason, uncertainty and next action")
		}
		blockerIDs[blocker.ID] = true
		if len(blocker.AffectedCoverageIDs) == 0 && len(blocker.AffectedProfileIDs) == 0 {
			return result, diag(CodeMissingField, "blockers", "", "blocker must identify affected coverage or profiles")
		}
		if err := uniqueExecutionIDs(blocker.AffectedCoverageIDs, "blockers.affected_coverage_ids"); err != nil {
			return result, err
		}
		if err := uniqueExecutionIDs(blocker.AffectedProfileIDs, "blockers.affected_profile_ids"); err != nil {
			return result, err
		}
		for _, id := range blocker.AffectedCoverageIDs {
			if _, ok := planned[id]; !ok {
				return result, diag(CodeUnassignedID, "blockers.affected_coverage_ids", "", "blocked coverage must be planned")
			}
		}
	}
	if status == ValidationStatusBlocked && len(result.Blockers) == 0 {
		return result, diag(CodeMissingField, "blockers", "", "blocked report requires an actionable blocker")
	}
	if status != ValidationStatusBlocked && len(result.Blockers) != 0 {
		return result, diag(CodeFalseAcceptanceGate, "blockers", "", "reports with operational blockers must remain blocked")
	}
	return result, nil
}

func playwrightVersionAtLeast(version, minimum string) bool {
	parse := func(value string) ([3]int, bool) {
		var out [3]int
		value = strings.TrimPrefix(strings.TrimSpace(value), "v")
		if index := strings.IndexAny(value, "-+"); index >= 0 {
			value = value[:index]
		}
		parts := strings.Split(value, ".")
		if len(parts) != len(out) {
			return out, false
		}
		for i := range parts {
			n, err := strconv.Atoi(parts[i])
			if err != nil || n < 0 {
				return out, false
			}
			out[i] = n
		}
		return out, true
	}
	got, ok := parse(version)
	if !ok {
		return false
	}
	want, ok := parse(minimum)
	if !ok {
		return false
	}
	for i := range got {
		if got[i] > want[i] {
			return true
		}
		if got[i] < want[i] {
			return false
		}
	}
	return true
}

func safeObservedToolName(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 || strings.ContainsAny(value, "\x00\r\n$`;|") {
		return false
	}
	return true
}

func uniqueExecutionIDs(ids []string, field string) *DiagnosticError {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !executionID.MatchString(id) {
			return diag(CodeInvalidEnum, field, "", "IDs must be stable non-secret identifiers")
		}
		if seen[id] {
			return diag(CodeDuplicateID, field, "", "IDs must be unique")
		}
		seen[id] = true
	}
	return nil
}
