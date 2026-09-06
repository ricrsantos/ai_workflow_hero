package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

func TestParseTelegramConfigNumberSet(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []int
		wantErr bool
	}{
		{name: "comma separated", input: "1, 3,5", want: []int{1, 3, 5}},
		{name: "spaces and semicolon", input: "2; 4", want: []int{2, 4}},
		{name: "duplicate", input: "1,1", wantErr: true},
		{name: "out of range", input: "0", wantErr: true},
		{name: "not a number", input: "abc", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseTelegramConfigNumberSet(test.input, 5)
			if (err != nil) != test.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, test.wantErr)
			}
			if test.wantErr {
				return
			}
			if len(got) != len(test.want) {
				t.Fatalf("got=%v want=%v", got, test.want)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Fatalf("got=%v want=%v", got, test.want)
				}
			}
		})
	}
}

func TestTelegramConfigWizardRoutesInputsAndKeepsDraft(t *testing.T) {
	var outbound []string
	m := NewTestModel(nil)
	m.telegram = &telegramState{
		connected: true,
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
		configWizard: &telegramConfigWizard{
			address:     "proj",
			cycleNumber: 4,
			step:        telegramConfigTitle,
			draft: workflowconfig.ManagedConfig{
				Title:     "Old title",
				Objective: "Old objective",
				WorkflowConfig: workflowconfig.WorkflowPreferences{
					UserPreferredLanguage: "PT-BR",
				},
				Stages: map[string]workflowconfig.ManagedStage{
					"research":       {Enabled: true, MaxIterations: 1, TimeoutMinutes: 5},
					"implementation": {Enabled: false, MaxIterations: 1, TimeoutMinutes: 5},
				},
			},
		},
	}

	for _, input := range []string{"New title", "New objective", "PT-BR", "1,2", "3", "2"} {
		next, cmd := m.handleTelegramInbound(telegramInboundMsg{text: input, address: "proj"})
		m = next
		if cmd != nil {
			_ = cmd()
		}
	}

	wizard := m.telegram.configWizard
	if wizard == nil {
		t.Fatal("wizard must remain active until explicit save or cancel")
	}
	if wizard.step != telegramConfigSummary {
		t.Fatalf("step=%q want summary", wizard.step)
	}
	if wizard.draft.Title != "New title" || wizard.draft.Objective != "New objective" {
		t.Fatalf("draft title/objective=%q/%q", wizard.draft.Title, wizard.draft.Objective)
	}
	if !wizard.draft.Scope.Backend || !wizard.draft.Scope.Frontend {
		t.Fatalf("scope=%+v", wizard.draft.Scope)
	}
	if wizard.draft.Stages["research"].Enabled {
		t.Fatal("research stage should be disabled by the selected stage set")
	}
	if !wizard.draft.Stages["implementation"].Enabled {
		t.Fatal("implementation stage should be enabled by the selected stage set")
	}
	if len(outbound) == 0 || !strings.Contains(outbound[len(outbound)-1], "Salvar configuração") {
		t.Fatalf("summary was not sent: %q", outbound)
	}
}

func TestTelegramConfigShowCommandReadsCanonicalDocument(t *testing.T) {
	m, _ := newTelegramConfigSaveModel(t)
	m.telegram.configWizard = nil
	next, rawCmd, handled := m.handleTelegramConfigCommand("/hero-config-show", "proj")
	if !handled || rawCmd == nil {
		t.Fatalf("handled=%v cmd=%v", handled, rawCmd != nil)
	}
	msg := rawCmd()
	show, ok := msg.(telegramConfigShowMsg)
	if !ok {
		t.Fatalf("message type=%T", msg)
	}
	if show.err != nil {
		t.Fatal(show.err)
	}
	var outbound []string
	next.telegram.recordOutbound = func(text string) { outbound = append(outbound, text) }
	updated, _ := next.handleTelegramConfigShow(show)
	if updated.telegram == nil || len(outbound) != 1 {
		t.Fatalf("outbound=%q", outbound)
	}
	if !strings.Contains(outbound[0], "Configuração do ciclo C1") || !strings.Contains(outbound[0], "Canonical title") {
		t.Fatalf("show=%q", outbound[0])
	}
}

func TestTelegramConfigModelSelectionUpdatesDraftOnly(t *testing.T) {
	m, dir := newPickerTestModel(t)
	var outbound []string
	m.telegram = &telegramState{
		connected: true,
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
		configWizard: &telegramConfigWizard{
			address: "proj",
			draft: workflowconfig.ManagedConfig{
				Agents: map[string]workflowconfig.AgentModelConfig{
					"orchestration_agent": {
						Harness:         "cursor",
						Model:           "old-model",
						ReasoningEffort: "medium",
						Thinking:        "off",
					},
				},
				FallbackModel: workflowconfig.AgentModelConfig{Harness: "cursor", Model: "fallback"},
			},
		},
	}
	m.telegram.configWizard.modelTargets = []string{"fallback_model"}
	m.telegram.modelSelection = &telegramModelSelection{
		address:     "proj",
		configAgent: "orchestration_agent",
		harnessID:   "cursor",
		modelSlug:   "full/model",
		properties: map[string]string{
			harness.PropertyFast:   "true",
			harness.PropertyThink:  "max",
			harness.PropertyEffort: "high",
		},
	}

	next, _ := m.commitTelegramModelSelection()
	agent := next.telegram.configWizard.draft.Agents["orchestration_agent"]
	if agent.Harness != "cursor" || agent.Model != "full/model" || !agent.EnableFastModel || agent.Thinking != "max" || agent.ReasoningEffort != "high" {
		t.Fatalf("updated agent=%+v", agent)
	}
	if next.telegram.modelSelection != nil {
		t.Fatal("model selection must be cleared after updating the draft")
	}
	if len(outbound) == 0 || !strings.Contains(outbound[0], "Orchestration Agent") {
		t.Fatalf("next model prompt=%q", outbound)
	}
	hero, err := os.ReadFile(filepath.Join(dir, ".workflow-hero", "config", "hero.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(hero), "full/model") {
		t.Fatal("cycle-agent selection must not mutate free-chat hero.json")
	}
}

func TestTelegramConfigSaveUsesSharedAtomicPath(t *testing.T) {
	m, dir := newTelegramConfigSaveModel(t)
	wizard := m.telegram.configWizard
	wizard.draft.Title = "Saved from Telegram"
	next, cmd := m.beginTelegramConfigSave()
	if cmd == nil || !next.telegram.configWizard.saving {
		t.Fatal("save should enter the asynchronous saving state")
	}
	msg, ok := cmd().(telegramConfigSavedMsg)
	if !ok {
		t.Fatalf("message type=%T", msg)
	}
	if msg.err != nil {
		t.Fatal(msg.err)
	}
	if _, cmd = next.handleTelegramConfigSaved(msg); cmd == nil {
		t.Fatal("successful save should refresh the cycle")
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".workflow-hero", "cycles", "current", "workflow-config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Saved from Telegram") || !strings.Contains(string(raw), "keep-this-rule") {
		t.Fatalf("saved YAML lost managed or unmanaged content: %s", raw)
	}
}

func newTelegramConfigSaveModel(t *testing.T) (model, string) {
	t.Helper()
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".workflow-hero", "config")
	cycleDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "hero.json"), []byte(`{"harnesses":{"cursor":{"enabled":true}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	yaml := `title: Canonical title
objective: Canonical objective
workflow_config:
  user_preferred_language: PT-BR
scope:
  backend: false
  frontend: false
  native: false
  script: false
  infrastructure: false
stages:
  research:
    enabled: true
    max_iterations: 1
    timeout_minutes: 5
    require_human_approval: false
agents:
  orchestration_agent:
    harness: cursor
    model: model-a
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
    subagent:
      same_of_agent: true
  context_agent:
    harness: cursor
    model: model-a
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
    subagent:
      same_of_agent: true
  discover_agent:
    harness: cursor
    model: model-a
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
    subagent:
      same_of_agent: true
fallback_model:
  harness: cursor
  model: model-a
  reasoning_effort: na
  enable_fast_model: false
  thinking: na
workflow_rules:
  - keep-this-rule
`
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow-config.yml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	status, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := workflowconfig.LoadCurrentDocument(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m.status = status
	m.telegram = &telegramState{connected: true}
	m.telegram.configWizard = &telegramConfigWizard{
		address:     "proj",
		cycleNumber: status.CycleNumber,
		step:        telegramConfigSummary,
		doc:         doc,
		baseline:    doc.Config,
		draft:       doc.Config,
	}
	return m, dir
}
