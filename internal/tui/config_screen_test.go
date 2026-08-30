package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

func TestConfigActionBindingsUseAlt(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pressed string
		binding key.Binding
	}{
		{name: "save", pressed: "alt+s", binding: configKeys.Save},
		{name: "save and start", pressed: "alt+enter", binding: configKeys.SaveStart},
		{name: "reload", pressed: "alt+r", binding: configKeys.Reload},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !key.Matches(parseTestKey(tc.pressed), tc.binding) {
				t.Fatalf("%s does not match %s", tc.pressed, tc.name)
			}
		})
	}
}

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

func TestConfigModelChoiceUsesHarnessCatalogBeyondBootModels(t *testing.T) {
	m := NewTestModel(nil)
	// TUI boot intentionally has no managed Codex/OpenCode rows. Config must
	// still use its local catalog/cache to make this agent's model editable.
	m.modelOptions = []harnessmgr.ModelOption{{Harness: "cursor", Model: "composer-2.5"}}
	m.config.draft = workflowconfig.ManagedConfig{
		Agents: map[string]workflowconfig.AgentModelConfig{
			"orchestration_agent": {Harness: "codex", Model: "gpt-5.4"},
		},
	}
	field := configField{kind: "model", agent: "orchestration_agent"}

	choices := m.configModelChoices("codex", "gpt-5.4")
	if len(choices) < 2 {
		t.Fatalf("Codex catalog choices=%v, want multiple models", choices)
	}
	next := m.cycleConfigChoice(field)
	if got := next.config.draft.Agents["orchestration_agent"].Model; got == "gpt-5.4" {
		t.Fatalf("Config model did not change; choices=%v", choices)
	}
	if !next.config.dirty {
		t.Fatal("changing an agent model must dirty the Config draft")
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
	_, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyDown})
}

func TestConfigTabCommitsEditAndFocusesNavbar(t *testing.T) {
	m := NewTestModel(nil)
	m.status.CycleNumber = 7
	m.screen = screenConfig
	m.config.draft = workflowconfig.ManagedConfig{Title: "before"}
	m.config.editing = true
	m.config.editBuffer = "after"
	m.config.editCursor = runeLen(m.config.editBuffer)

	next, _ := HandleTestKey(m, "tab")

	if next.config.editing {
		t.Fatal("Tab should finish the active field edit")
	}
	if next.config.draft.Title != "after" {
		t.Fatalf("title=%q want after", next.config.draft.Title)
	}
	if next.shellFocus != shellFocusNavbar {
		t.Fatal("Tab should focus the navbar")
	}
}

func TestDirtyConfigNavbarSelectionShowsLeaveDialog(t *testing.T) {
	m := NewTestModel(nil)
	m.status.CycleNumber = 7
	m.screen = screenConfig
	m.config.dirty = true

	m, _ = HandleTestKey(m, "tab")
	m, _ = HandleTestKey(m, "up")
	m, _ = HandleTestKey(m, "enter")

	if !m.config.leaveDialog || m.config.leaveScreen != screenSettings {
		t.Fatalf("leave dialog=%t target=%v, want Settings", m.config.leaveDialog, m.config.leaveScreen)
	}
	if m.shellFocus != shellFocusContent {
		t.Fatal("leave dialog should return focus to Config content")
	}
}

func TestConfigDirtyExitCompletesDiscardAndSaveNavigation(t *testing.T) {
	m := NewTestModel(nil)
	m.screen = screenConfig
	m.config.dirty = true
	next, _ := m.handleConfigKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2"), Alt: true})
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

func TestConfigFocusedProtectedFieldKeepsSelectionHighlight(t *testing.T) {
	forceColorProfile(t, termenv.ANSI)
	m := NewTestModel(nil)
	m.width, m.height = 120, 40

	focused := m.renderConfigField("QA purpose", "value", true, true)
	muted := m.renderConfigField("QA purpose", "value", false, true)

	if !strings.Contains(focused, "▸ ") {
		t.Fatal("focused protected field must show the cursor marker")
	}
	normalized := strings.Replace(focused, "▸ ", "  ", 1)
	if normalized == muted {
		t.Fatal("focused protected field must keep the selection highlight instead of rendering fully muted")
	}
}

func TestConfigEditingRendersBufferAndCaretInFocusedField(t *testing.T) {
	m := NewTestModel(nil)
	m.width, m.height = 120, 40
	m.config.editing = true
	m.config.editBuffer = "edited title"
	m.config.editCursor = runeLen(m.config.editBuffer)

	view := stripANSI(m.renderConfigField("Title", "old title", true, false))
	if !strings.Contains(view, "edited title") {
		t.Fatalf("editing must render the live buffer in the field: %q", view)
	}
	if !strings.Contains(view, "▸ Title:") {
		t.Fatalf("editing field must retain its focus marker: %q", view)
	}
}

func TestConfigFocusedFieldHighlightsOnlyItsLabel(t *testing.T) {
	forceColorProfile(t, termenv.ANSI)
	m := NewTestModel(nil)
	m.width, m.height = 120, 40

	view := m.renderConfigField("Title", "value", true, false)
	if !strings.Contains(view, configSelectedLabelStyle.Render("Title: ")) {
		t.Fatalf("focused field must highlight the label: %q", view)
	}
	if strings.Contains(view, configSelectedLabelStyle.Render("value")) {
		t.Fatalf("focused field must not apply the label highlight to its value: %q", view)
	}
}

func TestConfigEditKeysUpdateBufferAtCaret(t *testing.T) {
	m := NewTestModel(nil)
	m.screen = screenConfig
	m.config.editing = true
	m.config.editBuffer = "ac"
	m.config.editCursor = 1

	next, _ := m.handleConfigEditKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	got := next.(model)
	if got.config.editBuffer != "abc" || got.config.editCursor != 2 {
		t.Fatalf("insert buffer=%q cursor=%d, want abc/2", got.config.editBuffer, got.config.editCursor)
	}
	next, _ = got.handleConfigEditKey(tea.KeyMsg{Type: tea.KeyBackspace})
	got = next.(model)
	if got.config.editBuffer != "ac" || got.config.editCursor != 1 {
		t.Fatalf("backspace buffer=%q cursor=%d, want ac/1", got.config.editBuffer, got.config.editCursor)
	}
}

func TestConfigDirtyLeaveDialogIsVisibleAndAcceptsEnterToSave(t *testing.T) {
	m := NewTestModel(nil)
	m.width, m.height = 100, 16
	m.screen = screenConfig
	m.config.doc = &workflowconfig.Document{}
	m.config.dirty = true

	next, _ := m.handleConfigKey(tea.KeyMsg{Type: tea.KeyEsc})
	got := next.(model)
	if !got.config.leaveDialog {
		t.Fatal("escape on a dirty form must open the leave dialog")
	}
	view := stripANSI(got.renderFrame())
	for _, want := range []string{"Unsaved configuration changes", "[enter] Save", "[d] Discard", "[esc] Cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("visible leave dialog missing %q:\n%s", want, view)
		}
	}
	next, cmd := got.handleConfigKey(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(model)
	if cmd != nil || got.config.saving {
		t.Fatal("Enter must route to normal validation before a save command")
	}
	if got.config.err == "" {
		t.Fatal("Enter must attempt to save instead of leaving the dialog inert")
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
