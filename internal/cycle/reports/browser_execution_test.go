package reports

import (
	"encoding/json"
	"strings"
	"testing"
)

func browserCoverageFixture(status, result string) (map[string]any, DecodeContext) {
	plan := []CoveragePlanItem{{ID: "screen-dashboard-operator", Requirement: "FR-07", Acceptance: "AC-1", ScreenJourney: "dashboard", UserID: "operator", ProfileID: "operator", Mandatory: true, ExpectedResult: "protected dashboard renders"}}
	return map[string]any{
		"status": status, "summary": "sanitized validation", "failures": []any{},
		"preparation": Preparation{Status: "ok", Method: "cli_skill", VerifiedProfileIDs: []string{"operator"}},
		"blockers":    []Blocker{},
		"coverage":    Coverage{PlannedIDs: []string{plan[0].ID}, Items: []CoverageItem{{ID: plan[0].ID, Result: result, Evidence: []string{".workflow-hero/cycles/current/screenshots/shot-1.png"}, Checks: CoverageChecks{Render: true, CSS: true, Console: true, Network: true, HealthBeforeVisual: true, DesktopWidth: 1280, BusinessOutcome: true}}}},
	}, DecodeContext{CoveragePlan: plan, ActiveOwners: ActiveOwners{OwnerFrontend: {}, OwnerBackend: {}, OwnerGeneric: {}}}
}

func decodeBrowserFixture(t *testing.T, raw map[string]any, ctx DecodeContext) (*BrowserUIReport, *DiagnosticError) {
	t.Helper()
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	return DecodeBrowserUI(data, ctx)
}

func TestBlockedReportRequiresActionablePreparationAndCoverage(t *testing.T) {
	raw, ctx := browserCoverageFixture("blocked", "blocked")
	raw["preparation"] = Preparation{Status: "blocked", Method: "cli", VerifiedProfileIDs: []string{}}
	raw["blockers"] = []Blocker{{ID: "block-tool", Reason: "tool_missing", AffectedCoverageIDs: []string{ctx.CoveragePlan[0].ID}, AffectedProfileIDs: []string{"operator"}, Uncertainty: "protected rendering was not inspected", NextAction: "configure the required Playwright version and retry"}}
	r, err := decodeBrowserFixture(t, raw, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != ValidationStatusBlocked || r.Counts.Blocked != 1 || r.Counts.Executed != 0 || len(r.Failures) != 0 {
		t.Fatal("operational blocker was converted to validation work or findings")
	}
	delete(raw, "blockers")
	if _, err := decodeBrowserFixture(t, raw, ctx); err == nil || err.Code != CodeMissingField {
		t.Fatal("missing blocker was admitted")
	}
}

func TestMixedFindingsRemainGenuineInBlockedReport(t *testing.T) {
	raw, ctx := browserCoverageFixture("blocked", "blocked")
	raw["blockers"] = []Blocker{{ID: "block-account", Reason: "account_unusable", AffectedProfileIDs: []string{"admin"}, Uncertainty: "admin journey not exercised", NextAction: "reconfigure the selected account"}}
	raw["failures"] = []any{map[string]any{
		"failure_class": "frontend", "file": "screen.go", "requirement": "FR-07", "issue": "synthetic reproducible UI fault", "acceptance_criteria": "render protected dashboard", "evidence": []string{"synthetic failure"},
		"repro": map[string]string{"mode": "go_test", "package": "./internal/synthetic", "test": "TestSyntheticFault", "source": "package synthetic\nimport \"testing\"\nfunc TestSyntheticFault(t *testing.T) { t.Fatal(\"synthetic\") }"},
	}}
	r, err := decodeBrowserFixture(t, raw, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Failures) != 1 || r.Failures[0].Owner != OwnerFrontend {
		t.Fatal("blocked report lost genuine finding identity")
	}
}

func TestCoverageDenominatorCannotShrinkOrOmit(t *testing.T) {
	for _, mode := range []string{"shrink", "omit", "duplicate", "unplanned"} {
		t.Run(mode, func(t *testing.T) {
			raw, ctx := browserCoverageFixture("passed", "passed")
			c := raw["coverage"].(Coverage)
			switch mode {
			case "shrink":
				c.PlannedIDs = []string{}
				c.Items = []CoverageItem{}
			case "omit":
				c.Items = []CoverageItem{}
			case "duplicate":
				c.Items = append(c.Items, c.Items[0])
			case "unplanned":
				c.Items[0].ID = "not-approved"
			}
			raw["coverage"] = c
			if _, err := decodeBrowserFixture(t, raw, ctx); err == nil {
				t.Fatal("invalid denominator admitted")
			}
		})
	}
}

func TestMethodAdmissionUsesPlannedMethodAndObservedPlaywrightVersion(t *testing.T) {
	raw, ctx := browserCoverageFixture("passed", "passed")
	ctx.ExpectedBrowserMethod = "cli_skill"
	ctx.ExpectedBrowserTool = "playwright"
	ctx.MinimumPlaywrightVersion = "1.63.0"
	prep := raw["preparation"].(Preparation)
	prep.MethodAdmitted = true
	prep.ObservedToolName = "playwright"
	prep.ObservedToolVersion = "1.9.2"
	prep.ObservedPlaywrightVersion = "1.63.0"
	raw["preparation"] = prep
	if _, err := decodeBrowserFixture(t, raw, ctx); err != nil {
		t.Fatalf("admitted planned method rejected: %v", err)
	}

	tests := []struct {
		name   string
		change func(*Preparation)
	}{
		{name: "silent method fallback", change: func(p *Preparation) { p.Method = "cli" }},
		{name: "no live admission", change: func(p *Preparation) { p.MethodAdmitted = false }},
		{name: "different tool", change: func(p *Preparation) { p.ObservedToolName = "other-browser" }},
		{name: "below minimum Playwright", change: func(p *Preparation) { p.ObservedPlaywrightVersion = "1.62.9" }},
		{name: "unknown Playwright version", change: func(p *Preparation) { p.ObservedPlaywrightVersion = "unknown" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed, changedCtx := browserCoverageFixture("passed", "passed")
			changedCtx.ExpectedBrowserMethod = ctx.ExpectedBrowserMethod
			changedCtx.ExpectedBrowserTool = ctx.ExpectedBrowserTool
			changedCtx.MinimumPlaywrightVersion = ctx.MinimumPlaywrightVersion
			candidate := changed["preparation"].(Preparation)
			candidate.MethodAdmitted = prep.MethodAdmitted
			candidate.ObservedToolName = prep.ObservedToolName
			candidate.ObservedToolVersion = prep.ObservedToolVersion
			candidate.ObservedPlaywrightVersion = prep.ObservedPlaywrightVersion
			test.change(&candidate)
			changed["preparation"] = candidate
			if _, err := decodeBrowserFixture(t, changed, changedCtx); err == nil {
				t.Fatal("report passed without matching live method admission")
			}
		})
	}
}

func TestCoverageDenominatorCannotBeInventedByReport(t *testing.T) {
	raw, ctx := browserCoverageFixture("passed", "passed")
	ctx.CoveragePlan = nil
	if _, err := decodeBrowserFixture(t, raw, ctx); err == nil {
		t.Fatal("agent-owned denominator admitted without approved Planning scope")
	}
}

func TestBlockedReportCannotInventUnapprovedCoverage(t *testing.T) {
	raw, ctx := browserCoverageFixture("blocked", "blocked")
	ctx.CoveragePlan = nil
	raw["blockers"] = []Blocker{{ID: "block-tool", Reason: "tool_missing", AffectedCoverageIDs: []string{"screen-dashboard-operator"}, Uncertainty: "not inspected", NextAction: "configure tool"}}
	if _, err := decodeBrowserFixture(t, raw, ctx); err == nil {
		t.Fatal("blocked report invented an unapproved coverage denominator")
	}
}

func TestCoverageGateRequiresVerifiedPlannedProfile(t *testing.T) {
	raw, ctx := browserCoverageFixture("passed", "passed")
	raw["preparation"] = Preparation{Status: "ok", Method: "cli", VerifiedProfileIDs: []string{"admin"}}
	if _, err := decodeBrowserFixture(t, raw, ctx); err == nil || err.Field != "preparation.verified_profile_ids" {
		t.Fatal("unverified operator profile was admitted as passing mandatory coverage")
	}
}

func TestMandatoryAccountingAndCoverageGate(t *testing.T) {
	for _, outcome := range []string{"blocked", "skipped", "failed"} {
		raw, ctx := browserCoverageFixture("passed", outcome)
		if _, err := decodeBrowserFixture(t, raw, ctx); err == nil {
			t.Fatalf("passed mandatory %s", outcome)
		}
	}
	raw, ctx := browserCoverageFixture("passed", "passed")
	r, err := decodeBrowserFixture(t, raw, ctx)
	if err != nil || r.Counts.Planned != 1 || r.Counts.Executed != 1 || r.Counts.Passed != 1 {
		t.Fatalf("valid passing gate failed: %v", err)
	}
	c := raw["coverage"].(Coverage)
	c.Items[0].Checks.HealthBeforeVisual = false
	raw["coverage"] = c
	if _, err := decodeBrowserFixture(t, raw, ctx); err == nil {
		t.Fatal("Visual before Health admitted")
	}
	c.Items[0].Checks.HealthBeforeVisual = true
	c.Items[0].Checks.BusinessOutcome = false
	raw["coverage"] = c
	data, marshalErr := json.Marshal(raw)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if _, err := DecodeQAEndToEnd(data, ctx); err == nil {
		t.Fatal("navigation-only E2E admitted")
	}
}

func TestBlockedReportUnsafeEvidenceDiagnosticsNeverEchoValues(t *testing.T) {
	for _, ref := range []string{"../../.env.hero", "https://example.invalid/SENTINEL_SECRET", ".workflow-hero/cycles/current/.env.hero", ".workflow-hero/cycles/current/../secret", "$(SENTINEL_SECRET)"} {
		raw, ctx := browserCoverageFixture("passed", "passed")
		c := raw["coverage"].(Coverage)
		c.Items[0].Evidence = []string{ref}
		raw["coverage"] = c
		_, err := decodeBrowserFixture(t, raw, ctx)
		if err == nil {
			t.Fatalf("unsafe evidence admitted")
		}
		if strings.Contains(err.Error(), ref) || strings.Contains(err.Error(), "SENTINEL_SECRET") {
			t.Fatal("diagnostic leaked rejected values")
		}
	}
}

func TestOptionalReferenceMissingWarnsWithoutFailing(t *testing.T) {
	raw, ctx := browserCoverageFixture("passed", "passed")
	ctx.CoveragePlan[0].OptionalReferenceWidths = []int{1280, 768, 375}
	r, err := decodeBrowserFixture(t, raw, ctx)
	if err != nil || r.Status != "passed" {
		t.Fatalf("optional references caused failure: %v", err)
	}
	if len(r.ContractWarnings) != 3 {
		t.Fatalf("missing references: warnings=%v", r.ContractWarnings)
	}
	c := raw["coverage"].(Coverage)
	c.Items[0].ReferenceWidths = []int{1280, 768, 375}
	raw["coverage"] = c
	r, err = decodeBrowserFixture(t, raw, ctx)
	if err != nil || len(r.ContractWarnings) != 0 {
		t.Fatalf("supplied references: %v %v", r, err)
	}
}

func TestMandatoryEvidenceRequirementsNeedMatchingEvidenceCount(t *testing.T) {
	raw, ctx := browserCoverageFixture("passed", "passed")
	ctx.CoveragePlan[0].EvidenceRequirements = []string{"rendered screen", "role verification"}
	if _, err := decodeBrowserFixture(t, raw, ctx); err == nil || err.Code != CodeFalseAcceptanceGate {
		t.Fatal("mandatory evidence requirements were admitted with insufficient evidence")
	}
	c := raw["coverage"].(Coverage)
	c.Items[0].Evidence = append(c.Items[0].Evidence, ".workflow-hero/cycles/current/screenshots/role-1.png")
	raw["coverage"] = c
	if _, err := decodeBrowserFixture(t, raw, ctx); err != nil {
		t.Fatalf("complete mandatory evidence was rejected: %v", err)
	}
}
