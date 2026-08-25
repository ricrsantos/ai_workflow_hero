package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

func TestConfigRendersMissingDocumentError(t *testing.T) {
	m := NewTestModel(nil)
	m.status = cycle.StatusView{CycleNumber: 7}
	m.screen = screenConfig
	m.config.err = "read workflow-config.yml: no such file"
	view := stripANSI(m.renderConfig())
	for _, want := range []string{"workflow-config.yml", "Correct the workflow-config.yml file manually"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestConfigFieldsProgressivelyHideDisabledStage(t *testing.T) {
	m := NewTestModel(nil)
	m.config.draft = workflowconfig.ManagedConfig{
		Stages: map[string]workflowconfig.ManagedStage{
			"research":       {Enabled: false, Purpose: "retained", MaxIterations: 2, TimeoutMinutes: 5},
			"implementation": {Enabled: true, Purpose: "build", MaxIterations: 2, TimeoutMinutes: 5},
		},
		Agents: map[string]workflowconfig.AgentModelConfig{},
	}
	fields := m.configFields()
	for _, field := range fields {
		if strings.HasPrefix(field.path, "stages.research.") && !strings.HasSuffix(field.path, ".enabled") {
			t.Fatalf("disabled stage exposed %q", field.path)
		}
	}
}

func TestConfigCompletedStageIsProtected(t *testing.T) {
	m := NewTestModel(nil)
	m.status = cycle.StatusView{Stages: []cycle.StatusStage{{Name: "Research", Status: "Completed"}}}
	if !m.stageProtected("research") {
		t.Fatal("completed stage must be protected")
	}
}

func TestConfigCompletedStageProtectsItsAgentFields(t *testing.T) {
	m := NewTestModel(nil)
	m.status = cycle.StatusView{Stages: []cycle.StatusStage{{Name: "QA", Status: "Completed"}}}
	m.config.draft = workflowconfig.ManagedConfig{
		Stages: map[string]workflowconfig.ManagedStage{"qa": {Enabled: true, MaxIterations: 1, TimeoutMinutes: 1}},
		Agents: map[string]workflowconfig.AgentModelConfig{
			"orchestration_agent": {Harness: "cursor", Model: "composer"},
			"context_agent":       {Harness: "cursor", Model: "composer"},
			"qa_agent":            {Harness: "cursor", Model: "composer"},
		},
	}
	for _, field := range m.configFields() {
		if field.agent == "qa_agent" && !m.stageProtected(field.stage) {
			t.Fatalf("QA agent field %q is not associated with completed QA stage", field.path)
		}
	}
}

func TestConfigValidationKeepsDraftAndRendersFieldError(t *testing.T) {
	m := NewTestModel(nil)
	m.width, m.height = 120, 40
	m.screen = screenConfig
	m.config.doc = &workflowconfig.Document{}
	m.config.draft = workflowconfig.ManagedConfig{
		Title:     "",
		Objective: "objective",
		WorkflowConfig: workflowconfig.WorkflowPreferences{
			UserPreferredLanguage: "EN",
		},
		Stages: map[string]workflowconfig.ManagedStage{},
		Agents: map[string]workflowconfig.AgentModelConfig{
			"orchestration_agent": {Harness: "cursor", Model: "composer"},
			"context_agent":       {Harness: "cursor", Model: "composer"},
		},
		FallbackModel: workflowconfig.AgentModelConfig{Harness: "cursor", Model: "composer"},
	}
	before := m.config.draft
	got, cmd := m.beginConfigSave(false)
	if cmd != nil {
		t.Fatal("validation must not start a save command")
	}
	if !reflect.DeepEqual(got.config.draft, before) {
		t.Fatal("validation must keep the user's draft")
	}
	view := stripANSI(got.renderConfig())
	for _, want := range []string{"Title:", "title is required"} {
		if !strings.Contains(view, want) {
			t.Fatalf("validation view missing %q:\n%s", want, view)
		}
	}
}

func TestConfigSameOfAgentToggleRevealsSubagentControls(t *testing.T) {
	m := NewTestModel(nil)
	m.config.draft = workflowconfig.ManagedConfig{
		Stages: map[string]workflowconfig.ManagedStage{"research": {Enabled: true, MaxIterations: 1, TimeoutMinutes: 1}},
		Agents: map[string]workflowconfig.AgentModelConfig{
			"orchestration_agent": {Harness: "cursor", Model: "composer"},
			"context_agent":       {Harness: "cursor", Model: "composer"},
			"discover_agent": {
				Harness: "cursor", Model: "composer",
				Subagent: workflowconfig.SubagentConfig{SameOfAgent: true, Model: "sub"},
			},
		},
	}
	var toggle configField
	for _, field := range m.configFields() {
		if field.path == "agents.discover_agent.subagent.same_of_agent" {
			toggle = field
		}
		if field.path == "agents.discover_agent.subagent.model" {
			t.Fatal("same_of_agent=true must hide subagent model")
		}
	}
	if toggle.path == "" {
		t.Fatal("same_of_agent toggle missing")
	}
	m = m.toggleConfigField(toggle)
	found := false
	for _, field := range m.configFields() {
		if field.path == "agents.discover_agent.subagent.model" {
			found = true
		}
	}
	if !found {
		t.Fatal("same_of_agent=false must reveal subagent controls")
	}
}

func TestConfigRetryOnlyMatchesChangedFailedStage(t *testing.T) {
	if configChangedStage([]string{"stages.qa.max_iterations"}, "research") {
		t.Fatal("QA configuration must not enable a Research retry")
	}
	if !configChangedStage([]string{"stages.qa.max_iterations"}, "qa") {
		t.Fatal("QA configuration must enable only QA retry")
	}
	if !configChangedStage([]string{"agents.qa_agent.model"}, "qa") {
		t.Fatal("QA agent configuration must enable QA retry")
	}
}

func TestConfigFocusScrollsIntoViewport(t *testing.T) {
	m := NewTestModel(nil)
	m.width, m.height = 120, 12
	m.screen = screenConfig
	m.config.doc = &workflowconfig.Document{}
	m.config.draft = workflowconfig.ManagedConfig{
		Stages: map[string]workflowconfig.ManagedStage{
			"research":       {Enabled: true, Purpose: "research", MaxIterations: 1, TimeoutMinutes: 1},
			"implementation": {Enabled: true, Purpose: "implementation", MaxIterations: 1, TimeoutMinutes: 1},
		},
		Agents: map[string]workflowconfig.AgentModelConfig{
			"orchestration_agent": {Harness: "cursor", Model: "composer"},
			"context_agent":       {Harness: "cursor", Model: "composer"},
			"discover_agent":      {Harness: "cursor", Model: "composer"},
			"backend_agent":       {Harness: "cursor", Model: "composer"},
		},
		Scope: workflowconfig.Scope{Backend: true},
	}
	fields := m.configFields()
	m.config.focus = len(fields) - 1
	m = m.configEnsureFocusVisible()
	if m.contentOffset == 0 {
		t.Fatal("focused lower form field must scroll into the viewport")
	}
	_, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyTab})
}

func TestConfigDirtyExitCompletesDiscardAndSaveNavigation(t *testing.T) {
	m := NewTestModel(nil)
	m.screen = screenConfig
	m.config.dirty = true
	next, _ := m.handleConfigKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3"), Alt: true})
	got := next.(model)
	if !got.config.leaveDialog || got.config.leaveScreen != screenStatus {
		t.Fatalf("dirty exit dialog=%t target=%v", got.config.leaveDialog, got.config.leaveScreen)
	}
	got.config.baseline = workflowconfig.ManagedConfig{}
	got.config.draft = workflowconfig.ManagedConfig{Title: "draft"}
	next, _ = got.handleConfigKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	got = next.(model)
	if got.screen != screenStatus {
		t.Fatalf("discard must complete requested navigation, got %v", got.screen)
	}

	m = NewTestModel(nil)
	m.screen = screenConfig
	m.status.CycleNumber = 7
	m.config.dirty = true
	m.config.leaveDialog = true
	m.config.leaveScreen = screenConversation
	saved := &workflowconfig.Document{Config: workflowconfig.ManagedConfig{Title: "saved"}}
	next, _ = m.handleConfigMsg(configSavedMsg{doc: saved})
	updated := next.(model)
	if updated.screen != screenConversation {
		t.Fatalf("save must complete requested navigation, got %v", updated.screen)
	}
}

func TestConfigBusyRendersDraftReadOnly(t *testing.T) {
	m := NewTestModel(nil)
	m.width, m.height = 120, 40
	m.screen = screenConfig
	m.actionBusy = true
	m.config.doc = &workflowconfig.Document{}
	m.config.draft = workflowconfig.ManagedConfig{
		Title: "draft", Objective: "objective",
		WorkflowConfig: workflowconfig.WorkflowPreferences{UserPreferredLanguage: "EN"},
		Stages:         map[string]workflowconfig.ManagedStage{},
		Agents:         map[string]workflowconfig.AgentModelConfig{},
	}
	view := stripANSI(m.renderConfig())
	if !strings.Contains(view, "Editing is available when execution/preflight finishes.") || !strings.Contains(view, "Title: draft") {
		t.Fatalf("busy Config must remain visible and read-only:\n%s", view)
	}
}
