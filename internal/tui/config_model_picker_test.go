package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

func TestConfigModelPickerListsModelsAlphabetically(t *testing.T) {
	m := newConfigModelPickerTestModel()
	m = focusConfigPath(t, m, "agents.orchestration_agent.model")

	next, _ := m.handleConfigKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	if !got.config.modelPicker {
		t.Fatal("enter on a model field must open the model picker")
	}
	items := got.config.pickerItems
	if len(items) < 2 {
		t.Fatalf("picker items=%v, want a catalog to sort", items)
	}
	for i := 1; i < len(items); i++ {
		if strings.ToLower(items[i-1]) > strings.ToLower(items[i]) {
			t.Fatalf("picker items are not alphabetical at %d: %q then %q", i, items[i-1], items[i])
		}
	}
}

func TestConfigEnterOpensModelPickerInsteadOfCycling(t *testing.T) {
	m := newConfigModelPickerTestModel()
	m = focusConfigPath(t, m, "agents.orchestration_agent.model")
	before := m.config.draft.Agents["orchestration_agent"].Model

	next, _ := m.handleConfigKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)

	if !got.config.modelPicker {
		t.Fatal("enter on a model field must open the model picker")
	}
	if got.config.draft.Agents["orchestration_agent"].Model != before {
		t.Fatal("opening the picker must not change the configured model")
	}
	if len(got.config.pickerItems) < 2 {
		t.Fatalf("picker items=%v, want the harness catalog", got.config.pickerItems)
	}
	if got.config.pickerItems[got.config.pickerIndex] != before {
		t.Fatalf("picker cursor=%q, want current model %q", got.config.pickerItems[got.config.pickerIndex], before)
	}
	view := stripANSI(got.renderConfig())
	for _, want := range []string{before, "▸ ", "Codex"} {
		if !strings.Contains(view, want) {
			t.Fatalf("picker view missing %q:\n%s", want, view)
		}
	}
}

func TestConfigModelPickerNavigatesAndSelectsWithEnter(t *testing.T) {
	m := newConfigModelPickerTestModel()
	m = focusConfigPath(t, m, "agents.orchestration_agent.model")
	next, _ := m.handleConfigKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)

	move := tea.KeyMsg{Type: tea.KeyDown}
	if got.config.pickerIndex >= len(got.config.pickerItems)-1 {
		move = tea.KeyMsg{Type: tea.KeyUp}
	}
	next, _ = got.handleConfigKey(move)
	got = next.(model)
	selected := got.config.pickerItems[got.config.pickerIndex]
	if selected == "gpt-5.4" {
		t.Fatal("arrow key must move the picker off the current model")
	}

	next, _ = got.handleConfigKey(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(model)
	if got.config.modelPicker {
		t.Fatal("enter must close the model picker")
	}
	if model := got.config.draft.Agents["orchestration_agent"].Model; model != selected {
		t.Fatalf("model=%q, want selected %q", model, selected)
	}
	if !got.config.dirty {
		t.Fatal("selecting a different model must dirty the Config draft")
	}
}

func TestConfigModelPickerEscapeKeepsCurrentModel(t *testing.T) {
	m := newConfigModelPickerTestModel()
	m = focusConfigPath(t, m, "agents.orchestration_agent.model")
	next, _ := m.handleConfigKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	next, _ = got.handleConfigKey(tea.KeyMsg{Type: tea.KeyDown})
	got = next.(model)

	next, _ = got.handleConfigKey(tea.KeyMsg{Type: tea.KeyEsc})
	got = next.(model)
	if got.config.modelPicker {
		t.Fatal("escape must close the model picker")
	}
	if got.config.draft.Agents["orchestration_agent"].Model != "gpt-5.4" {
		t.Fatal("escape must keep the configured model")
	}
	if got.config.dirty {
		t.Fatal("canceling the picker must not dirty the draft")
	}
}

func TestConfigModelPickerScrollsLongCatalog(t *testing.T) {
	m := newConfigModelPickerTestModel()
	m = focusConfigPath(t, m, "agents.orchestration_agent.model")
	next, _ := m.handleConfigKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	if len(got.config.pickerItems) <= configModelPickerMax {
		t.Skip("embedded Codex catalog is not long enough to scroll")
	}
	got.config.pickerIndex = 0
	got.config.pickerOffset = 0
	for i := 0; i < configModelPickerMax+1; i++ {
		next, _ = got.handleConfigKey(tea.KeyMsg{Type: tea.KeyDown})
		got = next.(model)
	}
	if got.config.pickerOffset == 0 {
		t.Fatal("moving past the visible window must scroll the picker")
	}
	view := stripANSI(got.renderConfig())
	if !strings.Contains(view, "more above") {
		t.Fatalf("scrolled picker must show items above the window:\n%s", view)
	}
}

func TestConfigHarnessEnterStillCycles(t *testing.T) {
	m := newConfigModelPickerTestModel()
	m.config.draft.Agents["orchestration_agent"] = workflowconfig.AgentModelConfig{
		Harness: "codex", Model: "gpt-5.4",
	}
	m = focusConfigPath(t, m, "agents.orchestration_agent.harness")

	next, _ := m.handleConfigKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	if got.config.modelPicker {
		t.Fatal("enter on a harness field must not open the model picker")
	}
	if got.config.editing {
		t.Fatal("enter on a harness field must not start text editing")
	}
}

func TestConfigSubagentModelOpensPicker(t *testing.T) {
	m := newConfigModelPickerTestModel()
	agent := m.config.draft.Agents["orchestration_agent"]
	agent.Subagent = workflowconfig.SubagentConfig{SameOfAgent: false, Model: "gpt-5.4"}
	m.config.draft.Agents["orchestration_agent"] = agent
	m = focusConfigPath(t, m, "agents.orchestration_agent.subagent.model")

	next, _ := m.handleConfigKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	if !got.config.modelPicker || got.config.pickerField.path != "agents.orchestration_agent.subagent.model" {
		t.Fatal("enter on a subagent model must open the picker for that field")
	}
}

func TestConfigModelPickerTabClosesAndFocusesNavbar(t *testing.T) {
	m := newConfigModelPickerTestModel()
	m.status.CycleNumber = 7
	m = focusConfigPath(t, m, "agents.orchestration_agent.model")
	next, _ := m.handleConfigKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if !m.config.modelPicker {
		t.Fatal("expected the model picker to be open")
	}

	got, _ := HandleTestKey(m, "tab")
	if got.config.modelPicker {
		t.Fatal("tab must close the model picker")
	}
	if got.shellFocus != shellFocusNavbar {
		t.Fatal("tab should focus the navbar")
	}
	if got.config.draft.Agents["orchestration_agent"].Model != "gpt-5.4" {
		t.Fatal("tab must not apply a picker selection")
	}
}

func TestConfigModelPickerFooterHints(t *testing.T) {
	m := newConfigModelPickerTestModel()
	m.config.modelPicker = true

	if got := m.footerHints(); !strings.Contains(got, "enter select") || !strings.Contains(got, "esc cancel") {
		t.Fatalf("picker footer=%q", got)
	}
}

func newConfigModelPickerTestModel() model {
	m := NewTestModel(nil)
	m.width, m.height = 120, 40
	m.screen = screenConfig
	m.modelOptions = []harnessmgr.ModelOption{{Harness: "cursor", Model: "composer-2.5"}}
	m.config.doc = &workflowconfig.Document{}
	m.config.draft = workflowconfig.ManagedConfig{
		Agents: map[string]workflowconfig.AgentModelConfig{
			"orchestration_agent": {Harness: "codex", Model: "gpt-5.4"},
			"context_agent":       {Harness: "cursor", Model: "composer-2.5"},
		},
		FallbackModel: workflowconfig.AgentModelConfig{Harness: "cursor", Model: "composer-2.5"},
	}
	return m
}

func focusConfigPath(t *testing.T, m model, path string) model {
	t.Helper()
	for i, field := range m.configFields() {
		if field.path == path {
			m.config.focus = i
			return m
		}
	}
	t.Fatalf("config field %q not found", path)
	return m
}
