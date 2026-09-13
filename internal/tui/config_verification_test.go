package tui

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

func verificationField(path string) configField {
	return configField{path: path, kind: "choice"}
}

func TestConfigReproModeCyclesAndClearsToAuto(t *testing.T) {
	m := NewTestModel(nil)
	field := verificationField("verification.repro.mode")
	if got := m.configFieldValue(field); got != configVerificationAuto {
		t.Fatalf("initial value=%q want auto", got)
	}
	for _, want := range []string{"go_test", "command", "evidence", configVerificationAuto} {
		m = m.cycleConfigVerificationChoice(field)
		if got := m.configFieldValue(field); got != want {
			t.Fatalf("value=%q want %q", got, want)
		}
	}
	// Back at auto the key must be empty so the document drops it on save.
	if m.config.draft.Verification.Repro.Mode != "" {
		t.Fatalf("auto must clear the stored mode, got %q", m.config.draft.Verification.Repro.Mode)
	}
	if !m.config.dirty {
		t.Fatal("cycling a verification field must mark the draft dirty")
	}
}

func TestConfigAllowEvidenceIsTriState(t *testing.T) {
	m := NewTestModel(nil)
	field := verificationField("verification.repro.allow_evidence")
	if got := m.configFieldValue(field); got != configVerificationAuto {
		t.Fatalf("initial value=%q want auto", got)
	}
	m = m.cycleConfigVerificationChoice(field)
	if m.config.draft.Verification.Repro.AllowEvidence == nil || !*m.config.draft.Verification.Repro.AllowEvidence {
		t.Fatalf("allow_evidence=%v want true", m.config.draft.Verification.Repro.AllowEvidence)
	}
	m = m.cycleConfigVerificationChoice(field)
	if m.config.draft.Verification.Repro.AllowEvidence == nil || *m.config.draft.Verification.Repro.AllowEvidence {
		t.Fatalf("allow_evidence=%v want false", m.config.draft.Verification.Repro.AllowEvidence)
	}
	m = m.cycleConfigVerificationChoice(field)
	if m.config.draft.Verification.Repro.AllowEvidence != nil {
		t.Fatal("auto must clear allow_evidence so the project decides")
	}
}

// The two inputs the screen cannot edit (project go.mod and the YAML-only argv)
// must still be visible, so a rejected report is never the first hint.
func TestConfigRendersResolvedReproPolicy(t *testing.T) {
	m := NewTestModel(nil)
	lines := strings.Join(m.configReproPolicyLines(), "\n")
	if !strings.Contains(lines, "Default mode:") || !strings.Contains(lines, "auto ·") {
		t.Fatalf("policy lines=%q", lines)
	}
	if !strings.Contains(lines, "Command: not set") {
		t.Fatalf("an unset command argv must be visible: %q", lines)
	}

	command := []string{"npm", "test", "--", "-t", "{{test}}"}
	m.config.draft.Verification = workflowconfig.Verification{
		Repro: workflowconfig.ReproVerification{Mode: "command", Command: command},
	}
	lines = strings.Join(m.configReproPolicyLines(), "\n")
	if !strings.Contains(lines, "Default mode: command (configured)") {
		t.Fatalf("policy lines=%q", lines)
	}
	if !strings.Contains(lines, "Command: npm test -- -t {{test}}") {
		t.Fatalf("configured argv must be shown: %q", lines)
	}
	if !strings.Contains(lines, "Evidence always allowed for: browser_ui_validation") {
		t.Fatalf("per-stage evidence allowance must be shown: %q", lines)
	}
}

func TestConfigVerificationFieldsAreListedAndSectioned(t *testing.T) {
	m := NewTestModel(nil)
	var found []string
	for _, field := range m.configFields() {
		if strings.HasPrefix(field.path, "verification.") {
			found = append(found, field.path)
			if field.kind != "choice" {
				t.Fatalf("%s kind=%q want choice", field.path, field.kind)
			}
			if section := configFieldSection(field); section != "Verification" {
				t.Fatalf("%s section=%q", field.path, section)
			}
		}
	}
	if len(found) != 2 {
		t.Fatalf("verification fields=%v", found)
	}
}
