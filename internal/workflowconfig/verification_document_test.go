package workflowconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const verificationDocYAML = `title: T
objective: O
workflow_config:
  user_preferred_language: EN
scope:
  backend: true
  frontend: false
  native: false
  script: false
  infrastructure: false
verification:
  repro:
    command: ["npm", "test", "--", "-t", "{{test}}"]
    evidence_stages: [browser_ui_validation]
stages:
  implementation:
    enabled: true
    purpose: p
    max_iterations: 2
    timeout_minutes: 10
    require_human_approval: false
agents:
  orchestration_agent:
    harness: cursor
    model: composer-2.5
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
    subagent:
      same_of_agent: true
  context_agent:
    harness: cursor
    model: composer-2.5
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
    subagent:
      same_of_agent: true
  backend_agent:
    harness: cursor
    model: composer-2.5
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
    subagent:
      same_of_agent: true
fallback_model:
  harness: cursor
  model: composer-2.5
  reasoning_effort: na
  enable_fast_model: false
  thinking: na
`

func writeVerificationDoc(t *testing.T) *Document {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workflow-config.yml")
	if err := os.WriteFile(path, []byte(verificationDocYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestWriteVerificationKeepsYAMLOnlyFields(t *testing.T) {
	doc := writeVerificationDoc(t)
	if len(doc.Config.Verification.Repro.Command) != 5 {
		t.Fatalf("command=%v", doc.Config.Verification.Repro.Command)
	}
	draft := doc.Config
	draft.Verification.Repro.Mode = "command"
	allow := false
	draft.Verification.Repro.AllowEvidence = &allow
	if err := doc.Write(draft, ValidationOptions{}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(doc.Path())
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, want := range []string{"mode: command", "allow_evidence: false", "npm", "evidence_stages"} {
		if !strings.Contains(body, want) {
			t.Fatalf("saved document lost %q:\n%s", want, body)
		}
	}

	reloaded, err := LoadDocument(doc.Path())
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Config.Verification.Repro.Mode != "command" ||
		reloaded.Config.Verification.Repro.AllowEvidence == nil ||
		*reloaded.Config.Verification.Repro.AllowEvidence {
		t.Fatalf("verification=%+v", reloaded.Config.Verification.Repro)
	}
	if len(reloaded.Config.Verification.Repro.Command) != 5 {
		t.Fatalf("command argv was not preserved: %v", reloaded.Config.Verification.Repro.Command)
	}
}

// "auto" must leave the document without the key so the project keeps deciding,
// instead of persisting an empty scalar that no longer means auto.
func TestWriteVerificationAutoRemovesKeys(t *testing.T) {
	doc := writeVerificationDoc(t)
	draft := doc.Config
	draft.Verification.Repro.Mode = "command"
	allow := true
	draft.Verification.Repro.AllowEvidence = &allow
	if err := doc.Write(draft, ValidationOptions{}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadDocument(doc.Path())
	if err != nil {
		t.Fatal(err)
	}
	back := reloaded.Config
	back.Verification.Repro.Mode = ""
	back.Verification.Repro.AllowEvidence = nil
	if err := reloaded.Write(back, ValidationOptions{}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(reloaded.Path())
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if strings.Contains(body, "mode:") || strings.Contains(body, "allow_evidence") {
		t.Fatalf("auto must remove the keys:\n%s", body)
	}
	if !strings.Contains(body, "npm") {
		t.Fatalf("removal must not touch the command argv:\n%s", body)
	}
}

func TestValidateRejectsCommandModeWithoutCommand(t *testing.T) {
	doc := writeVerificationDoc(t)
	draft := doc.Config
	draft.Verification.Repro.Mode = "command"
	draft.Verification.Repro.Command = nil
	err := draft.Validate(ValidationOptions{})
	if err == nil || !strings.Contains(err.Error(), "verification.repro.command") {
		t.Fatalf("err=%v", err)
	}
	draft.Verification.Repro.Mode = "nonsense"
	if err := draft.Validate(ValidationOptions{}); err == nil || !strings.Contains(err.Error(), "verification.repro.mode") {
		t.Fatalf("err=%v", err)
	}
}

func TestManagedDiffReportsVerificationPaths(t *testing.T) {
	before := ManagedConfig{}
	after := ManagedConfig{}
	after.Verification.Repro.Mode = "evidence"
	allow := true
	after.Verification.Repro.AllowEvidence = &allow
	paths := ManagedDiff(before, after)
	found := map[string]bool{}
	for _, p := range paths {
		found[p] = true
	}
	if !found["verification.repro.mode"] || !found["verification.repro.allow_evidence"] {
		t.Fatalf("paths=%v", paths)
	}
}
