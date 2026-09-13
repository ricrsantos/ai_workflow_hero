package reports

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/findingrepro"
)

func commandPolicy() findingrepro.Policy {
	return findingrepro.Policy{
		DefaultMode:    findingrepro.ModeCommand,
		Modes:          map[string]bool{findingrepro.ModeCommand: true},
		Command:        []string{"npm", "test", "--", "-t", "{{test}}"},
		EvidenceStages: []string{SourceBrowserUI},
	}
}

func qaCtx(policy findingrepro.Policy) DecodeContext {
	return DecodeContext{
		ActiveOwners: ActiveOwners{OwnerBackend: {}, OwnerFrontend: {}, OwnerGeneric: {}},
		ReproPolicy:  policy,
	}
}

func TestDecodeQAAcceptsCommandRepro(t *testing.T) {
	raw := []byte(`{"status":"failed","summary":"s","failures":[{
		"owner":"frontend_agent","file":"src/checkout.ts","issue":"total is wrong",
		"acceptance_criteria":"checkout total matches the cart",
		"repro":{"mode":"command","package":"src/checkout.test.ts","test":"renders total"}}]}`)
	r, err := DecodeQA(raw, qaCtx(commandPolicy()))
	if err != nil {
		t.Fatalf("decode err=%v", err)
	}
	got := r.Failures[0].Repro
	if got.Mode != findingrepro.ModeCommand || got.Package != "src/checkout.test.ts" || got.Test != "renders total" {
		t.Fatalf("repro=%+v", got)
	}
}

func TestDecodeQARejectsModeNotEnabled(t *testing.T) {
	raw := []byte(`{"status":"failed","summary":"s","failures":[{
		"owner":"generic_agent","file":"internal/tui/x.go","issue":"i","acceptance_criteria":"a",
		"repro":{"mode":"go_test","package":"./internal/tui","test":"TestX","source":"package tui\n\nfunc TestX(t *testing.T) {}\n"}}]}`)
	_, err := DecodeQA(raw, qaCtx(commandPolicy()))
	if err == nil {
		t.Fatal("go_test must be rejected when the project does not enable it")
	}
	if err.Code != CodeInvalidEnum || !strings.Contains(err.Field, "repro.mode") {
		t.Fatalf("diag=%+v", err)
	}
}

func TestDecodeQAEvidenceModeRequiresEvidence(t *testing.T) {
	policy := findingrepro.Policy{
		DefaultMode: findingrepro.ModeEvidence,
		Modes:       map[string]bool{findingrepro.ModeEvidence: true},
	}
	raw := []byte(`{"status":"failed","summary":"s","failures":[{
		"owner":"generic_agent","file":"x.go","issue":"i","acceptance_criteria":"a",
		"repro":{"mode":"evidence"}}]}`)
	_, err := DecodeQA(raw, qaCtx(policy))
	if err == nil || err.Code != CodeMissingField {
		t.Fatalf("diag=%+v", err)
	}

	withEvidence := []byte(`{"status":"failed","summary":"s","failures":[{
		"owner":"generic_agent","file":"x.go","issue":"i","acceptance_criteria":"a",
		"evidence":["docs/ui/report.png"],"repro":{"mode":"evidence"}}]}`)
	r, derr := DecodeQA(withEvidence, qaCtx(policy))
	if derr != nil {
		t.Fatalf("decode err=%v", derr)
	}
	if r.Failures[0].Repro.Mode != findingrepro.ModeEvidence {
		t.Fatalf("repro=%+v", r.Failures[0].Repro)
	}
}

func TestDecodeQAEvidenceModeRejectsIdentity(t *testing.T) {
	policy := findingrepro.Policy{
		DefaultMode: findingrepro.ModeEvidence,
		Modes:       map[string]bool{findingrepro.ModeEvidence: true},
	}
	raw := []byte(`{"status":"failed","summary":"s","failures":[{
		"owner":"generic_agent","file":"x.go","issue":"i","acceptance_criteria":"a",
		"evidence":["a.png"],"repro":{"mode":"evidence","test":"TestX"}}]}`)
	_, err := DecodeQA(raw, qaCtx(policy))
	if err == nil || err.Code != CodeInvalidEnum {
		t.Fatalf("diag=%+v", err)
	}
}

func TestDecodeBrowserUIKeepsEvidenceEscapeHatch(t *testing.T) {
	// A rendering failure has no deterministic unit-test re-run, so Browser UI
	// may use evidence mode even when the project defaults to go_test.
	policy := findingrepro.Policy{
		DefaultMode:    findingrepro.ModeGoTest,
		Modes:          map[string]bool{findingrepro.ModeGoTest: true},
		EvidenceStages: []string{SourceBrowserUI},
	}
	raw := []byte(`{"status":"failed","summary":"s","health_passed":false,"failures":[{
		"failure_class":"frontend","file":"src/app.css","issue":"header overlaps content",
		"acceptance_criteria":"header does not overlap at 1280x800",
		"evidence":[".workflow-hero/cycles/current/browser-ui/home.png"],
		"repro":{"mode":"evidence"}}]}`)
	r, err := DecodeBrowserUI(raw, DecodeContext{
		ActiveOwners: ActiveOwners{OwnerFrontend: {}, OwnerBackend: {}},
		ReproPolicy:  policy,
	})
	if err != nil {
		t.Fatalf("decode err=%v", err)
	}
	if r.Failures[0].Repro.Mode != findingrepro.ModeEvidence {
		t.Fatalf("repro=%+v", r.Failures[0].Repro)
	}
}

func TestDecodeQADefaultModeAppliesWhenOmitted(t *testing.T) {
	raw := []byte(`{"status":"failed","summary":"s","failures":[{
		"owner":"frontend_agent","file":"src/a.ts","issue":"i","acceptance_criteria":"a",
		"repro":{"package":"src/a.test.ts","test":"a case"}}]}`)
	r, err := DecodeQA(raw, qaCtx(commandPolicy()))
	if err != nil {
		t.Fatalf("decode err=%v", err)
	}
	if r.Failures[0].Repro.Mode != findingrepro.ModeCommand {
		t.Fatalf("repro=%+v", r.Failures[0].Repro)
	}
}

func TestDecodeQAGoContractStillRequiresSource(t *testing.T) {
	raw := []byte(`{"status":"failed","summary":"s","failures":[{
		"owner":"generic_agent","file":"internal/tui/x.go","issue":"i","acceptance_criteria":"a",
		"repro":{"package":"./internal/tui","test":"TestX"}}]}`)
	_, err := DecodeQA(raw, qaCtx(findingrepro.DefaultGoPolicy()))
	if err == nil || err.Code != CodeMissingField || !strings.Contains(err.Field, "repro.source") {
		t.Fatalf("diag=%+v", err)
	}
}
