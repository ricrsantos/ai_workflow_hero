package tui

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/modelprops"
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

	for _, input := range []string{"New title", "New objective", "PT-BR", "2,3", "3", "2", "1"} {
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
	if !wizard.draft.Stages["implementation"].RequireHumanApproval {
		t.Fatal("implementation approval should be enabled by the approval answer")
	}
	if len(outbound) == 0 || !strings.Contains(outbound[len(outbound)-1], "Salvar configuração") {
		t.Fatalf("summary was not sent: %q", outbound)
	}
}

func TestTelegramConfigApprovalReviewAsksEveryEnabledStage(t *testing.T) {
	var outbound []string
	m := NewTestModel(nil)
	m.telegram = &telegramState{
		connected:      true,
		recordOutbound: func(text string) { outbound = append(outbound, text) },
		configWizard: &telegramConfigWizard{
			address: "proj",
			draft: workflowconfig.ManagedConfig{
				Stages: map[string]workflowconfig.ManagedStage{
					"research": {Enabled: true, RequireHumanApproval: false},
					"planning": {Enabled: false, RequireHumanApproval: true},
					"qa":       {Enabled: true, RequireHumanApproval: true},
				},
			},
		},
	}

	next, _ := m.beginTelegramConfigStageApprovalReview()
	if next.telegram.configWizard.step != telegramConfigStageApproval {
		t.Fatalf("step=%q want stage approval", next.telegram.configWizard.step)
	}
	if !strings.Contains(outbound[len(outbound)-1], "Research") || !strings.Contains(outbound[len(outbound)-1], "1 - Manter a configuração atual") {
		t.Fatalf("first approval prompt=%q", outbound[len(outbound)-1])
	}

	next, _ = next.handleTelegramConfigInput("proj", "2")
	if next.telegram.configWizard.step != telegramConfigStageApproval {
		t.Fatalf("after first answer step=%q want next stage approval", next.telegram.configWizard.step)
	}
	if !strings.Contains(outbound[len(outbound)-1], "Qa") {
		t.Fatalf("second approval prompt=%q", outbound[len(outbound)-1])
	}

	next, _ = next.handleTelegramConfigInput("proj", "3")
	if next.telegram.configWizard.step != telegramConfigModelsQuestion {
		t.Fatalf("after all approval answers step=%q want models question", next.telegram.configWizard.step)
	}
	if !next.telegram.configWizard.draft.Stages["research"].RequireHumanApproval {
		t.Fatal("research approval should be enabled")
	}
	if next.telegram.configWizard.draft.Stages["qa"].RequireHumanApproval {
		t.Fatal("qa approval should be disabled")
	}
	if next.telegram.configWizard.draft.Stages["planning"].RequireHumanApproval != true {
		t.Fatal("disabled planning approval must remain unchanged")
	}
}

func TestTelegramConfigKeepOptionIsAlwaysFirst(t *testing.T) {
	m := NewTestModel(nil)
	m.telegram = &telegramState{
		configWizard: &telegramConfigWizard{
			draft: workflowconfig.ManagedConfig{
				Scope: workflowconfig.Scope{Backend: true},
				Stages: map[string]workflowconfig.ManagedStage{
					"research": {Enabled: true, RequireHumanApproval: false},
				},
				Agents: map[string]workflowconfig.AgentModelConfig{
					"orchestration_agent": {Harness: "cursor", Model: "model-a"},
				},
			},
			approvalStages: []string{"research"},
		},
	}

	scopePrompt := telegramConfigScopePrompt(m.telegram.configWizard.draft.Scope)
	if !strings.Contains(scopePrompt, "1 - Manter escopo atual") {
		t.Fatalf("scope prompt missing keep-first option:\n%s", scopePrompt)
	}

	stagesPrompt := telegramConfigStagesPrompt(m.telegram.configWizard.draft)
	if !strings.Contains(stagesPrompt, "1 - Manter stages atuais") {
		t.Fatalf("stages prompt missing keep-first option:\n%s", stagesPrompt)
	}

	approvalText := m.telegramConfigStageApprovalTextFor("research")
	if !strings.Contains(approvalText, "1 - Manter a configuração atual") {
		t.Fatalf("approval prompt missing keep-first option:\n%s", approvalText)
	}

	m.telegram.configWizard.step = telegramConfigModelsQuestion
	if modelsQuestion := m.telegramConfigPrompt(); !strings.Contains(modelsQuestion, "1 - Manter os modelos atuais") {
		t.Fatalf("models question missing keep-first option:\n%s", modelsQuestion)
	}

	m.telegram.configWizard.step = telegramConfigModelChoice
	m.telegram.configWizard.modelTargets = []telegramConfigModelTarget{{agentName: "orchestration_agent"}}
	if modelText := m.telegramConfigModelText(); !strings.Contains(modelText, "1 - Manter o atual") {
		t.Fatalf("model choice missing keep-first option:\n%s", modelText)
	}

	m.telegram.configWizard.modelTargets = []telegramConfigModelTarget{{agentName: "orchestration_agent", subagent: true}}
	if subagentText := m.telegramConfigSubagentPrompt(); !strings.Contains(subagentText, "1 - Manter a configuração atual") {
		t.Fatalf("subagent choice missing keep-first option:\n%s", subagentText)
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
						Subagent: workflowconfig.SubagentConfig{
							SameOfAgent: false,
							Model:       "old-subagent-model",
						},
					},
				},
				FallbackModel: workflowconfig.AgentModelConfig{Harness: "cursor", Model: "fallback"},
			},
		},
	}
	m.telegram.configWizard.modelTargets = []telegramConfigModelTarget{{agentName: "fallback_model"}}
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
	if !agent.Subagent.SameOfAgent {
		t.Fatalf("changing the parent model must reset the subagent to the parent model: %+v", agent.Subagent)
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

func TestTelegramConfigModelTargetsFollowActiveStagesAndScope(t *testing.T) {
	cfg := telegramConfigScopeRegressionConfig()
	got := telegramConfigModelTargets(cfg)
	want := []telegramConfigModelTarget{
		{agentName: "orchestration_agent"},
		{agentName: "orchestration_agent", subagent: true},
		{agentName: "context_agent"},
		{agentName: "context_agent", subagent: true},
		{agentName: "discover_agent"},
		{agentName: "discover_agent", subagent: true},
		{agentName: "planning_agent"},
		{agentName: "planning_agent", subagent: true},
		{agentName: "frontend_agent"},
		{agentName: "frontend_agent", subagent: true},
		{agentName: "qa_agent"},
		{agentName: "qa_agent", subagent: true},
		{agentName: "judge_agent"},
		{agentName: "judge_agent", subagent: true},
		{agentName: "fallback_model"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("targets=%v, want %v", got, want)
	}
	for _, target := range got {
		if target.agentName == "backend_agent" || target.agentName == "generic_agent" ||
			target.agentName == "browser_ui_agent" || target.agentName == "end2end_qa_agent" {
			t.Fatalf("out-of-scope agent was queued: %+v", target)
		}
	}
}

func TestTelegramConfigSummaryHidesOutOfScopeAgentBlocks(t *testing.T) {
	summary := formatTelegramConfig(12, telegramConfigScopeRegressionConfig(), true)
	if !strings.Contains(summary, "Frontend Agent") {
		t.Fatalf("summary must include the selected frontend agent:\n%s", summary)
	}
	for _, name := range []string{"Backend Agent", "Generic Agent", "Browser Ui Agent", "End2end Qa Agent"} {
		if strings.Contains(summary, name) {
			t.Fatalf("summary must hide %q when it is outside the active scope/stages:\n%s", name, summary)
		}
	}
}

func TestTelegramConfigModelReviewAsksForEachActiveAgentSubagent(t *testing.T) {
	var outbound []string
	m := NewTestModel(nil)
	m.telegram = &telegramState{
		connected:      true,
		recordOutbound: func(text string) { outbound = append(outbound, text) },
		configWizard: &telegramConfigWizard{
			address: "proj",
			draft: workflowconfig.ManagedConfig{
				Stages: map[string]workflowconfig.ManagedStage{
					"research": {Enabled: true},
				},
				Agents: map[string]workflowconfig.AgentModelConfig{
					"orchestration_agent": {Harness: "cursor", Model: "model-a", Subagent: workflowconfig.SubagentConfig{SameOfAgent: true}},
					"context_agent":       {Harness: "cursor", Model: "model-a", Subagent: workflowconfig.SubagentConfig{SameOfAgent: true}},
					"discover_agent":      {Harness: "cursor", Model: "model-a", Subagent: workflowconfig.SubagentConfig{SameOfAgent: true}},
				},
			},
		},
	}

	next, _ := m.beginTelegramConfigModelReview()
	if next.telegram.configWizard.step != telegramConfigModelChoice {
		t.Fatalf("initial step=%q, want model choice", next.telegram.configWizard.step)
	}
	next, _ = next.handleTelegramConfigInput("proj", "1")
	if next.telegram.configWizard.step != telegramConfigSubagentChoice {
		t.Fatalf("after keeping parent model step=%q, want subagent choice", next.telegram.configWizard.step)
	}
	if !strings.Contains(outbound[len(outbound)-1], "Subagent de Orchestration Agent") {
		t.Fatalf("subagent prompt=%q", outbound[len(outbound)-1])
	}

	next, _ = next.handleTelegramConfigInput("proj", "2")
	if next.telegram.configWizard.step != telegramConfigModelChoice {
		t.Fatalf("after keeping subagent step=%q, want next parent model choice", next.telegram.configWizard.step)
	}
	if !strings.Contains(outbound[len(outbound)-1], "Modelo de Context Agent") {
		t.Fatalf("next parent prompt=%q", outbound[len(outbound)-1])
	}
}

func TestTelegramConfigSubagentSelectionUsesParentHarnessAndDraft(t *testing.T) {
	m, dir := newPickerTestModel(t)
	var outbound []string
	m.telegram = &telegramState{
		connected:      true,
		recordOutbound: func(text string) { outbound = append(outbound, text) },
		configWizard: &telegramConfigWizard{
			address: "proj",
			draft: workflowconfig.ManagedConfig{
				Agents: map[string]workflowconfig.AgentModelConfig{
					"orchestration_agent": {
						Harness: "cursor",
						Model:   "parent/model",
						Subagent: workflowconfig.SubagentConfig{
							SameOfAgent: true,
						},
					},
				},
			},
		},
	}
	m.telegram.configWizard.modelTargets = []telegramConfigModelTarget{{agentName: "orchestration_agent", subagent: true}}
	m.telegram.configWizard.modelIndex = 0

	next, _ := m.startTelegramCycleSubagentModelSelection("proj", "orchestration_agent")
	selection := next.telegram.modelSelection
	if selection == nil || !selection.configSubagent {
		t.Fatal("selection must be marked as a subagent selection")
	}
	if !reflect.DeepEqual(selection.harnesses, []string{"cursor"}) {
		t.Fatalf("subagent harnesses=%v, want only parent harness", selection.harnesses)
	}
	if len(outbound) == 0 || !strings.Contains(outbound[0], "subagent de Orchestration Agent") || strings.Contains(outbound[0], "OpenCode") {
		t.Fatalf("harness prompt=%q", outbound)
	}

	next.telegram.modelSelection = &telegramModelSelection{
		address:        "proj",
		configAgent:    "orchestration_agent",
		configSubagent: true,
		harnessID:      "cursor",
		modelSlug:      "full/model",
		properties: map[string]string{
			harness.PropertyFast:   "true",
			harness.PropertyThink:  "max",
			harness.PropertyEffort: "high",
		},
	}
	next, _ = next.commitTelegramModelSelection()
	agent := next.telegram.configWizard.draft.Agents["orchestration_agent"]
	if agent.Subagent.SameOfAgent || agent.Subagent.Model != "full/model" || !agent.Subagent.EnableFastModel || agent.Subagent.Thinking != "max" || agent.Subagent.ReasoningEffort != "high" {
		t.Fatalf("updated subagent=%+v", agent.Subagent)
	}
	if next.telegram.modelSelection != nil {
		t.Fatal("subagent model selection must be cleared after updating the draft")
	}
	hero, err := os.ReadFile(filepath.Join(dir, ".workflow-hero", "config", "hero.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(hero), "full/model") {
		t.Fatal("cycle subagent selection must not mutate free-chat hero.json")
	}
}

func telegramConfigScopeRegressionConfig() workflowconfig.ManagedConfig {
	return workflowconfig.ManagedConfig{
		Scope: workflowconfig.Scope{Frontend: true},
		Stages: map[string]workflowconfig.ManagedStage{
			"research":              {Enabled: true},
			"planning":              {Enabled: true},
			"implementation":        {Enabled: true},
			"qa":                    {Enabled: true},
			"judge":                 {Enabled: true},
			"browser_ui_validation": {Enabled: false},
			"qa_end_to_end":         {Enabled: false},
		},
		Agents: map[string]workflowconfig.AgentModelConfig{
			"orchestration_agent": {},
			"context_agent":       {},
			"discover_agent":      {},
			"planning_agent":      {},
			"frontend_agent":      {},
			"qa_agent":            {},
			"judge_agent":         {},
			"backend_agent":       {},
			"generic_agent":       {},
			"browser_ui_agent":    {},
			"end2end_qa_agent":    {},
		},
		FallbackModel: workflowconfig.AgentModelConfig{Harness: "cursor", Model: "fallback"},
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

func TestTelegramConfigWizardModelListMergesGrokFamily(t *testing.T) {
	m, _ := newPickerTestModel(t)
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
					"orchestration_agent": {Harness: "cursor", Model: "composer-2.5"},
				},
			},
			modelTargets: []telegramConfigModelTarget{{agentName: "orchestration_agent"}},
		},
	}
	m.propsSvc.Catalog = propsCatalog(map[string]map[string]modelprops.CatalogProperty{
		"cursor-grok-4.5": {
			"fs": {Available: true, Values: []string{"true", "false"}, Default: "false"},
		},
	})

	prev := listModelsForHarnessFn
	listModelsForHarnessFn = func(_ context.Context, _ model, harnessID string) ([]string, error) {
		return []string{"composer-2.5", "cursor-grok-4.5-high", "cursor-grok-4.5-low"}, nil
	}
	t.Cleanup(func() { listModelsForHarnessFn = prev })

	next, cmd := m.startTelegramCycleModelSelection("proj", "orchestration_agent")
	after, cmd := next.Update(telegramInboundMsg{text: "1", address: "proj"})
	_ = flushTeaCmds(after.(model), cmd)

	prompt := outbound[len(outbound)-1]
	for _, want := range []string{"cursor-grok-4.5", "cursor-grok-4.5-high", "cursor-grok-4.5-low"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("hero-config model prompt missing %q:\n%s", want, prompt)
		}
	}
}
