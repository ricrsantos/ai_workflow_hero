package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

func TestScreenshotToggleDefaultsIndependentlyAndRejectsHTTPOnlyE2E(t *testing.T) {
	m := NewTestModel(nil)
	m.width, m.height = 120, 60
	m.config.doc = &workflowconfig.Document{}
	m.config.draft = workflowconfig.ManagedConfig{
		Scope: workflowconfig.Scope{Frontend: true},
		Stages: map[string]workflowconfig.ManagedStage{
			"browser_ui_validation": {Enabled: true},
			"qa_end_to_end":         {Enabled: true, UsePlaywright: false},
		},
	}

	var browser, e2e configField
	for _, field := range m.configFields() {
		switch field.path {
		case "stages.browser_ui_validation.screenshots.enabled":
			browser = field
		case "stages.qa_end_to_end.screenshots.enabled":
			e2e = field
		}
	}
	if browser.path == "" || e2e.path == "" {
		t.Fatalf("screenshot fields missing: browser=%q e2e=%q", browser.path, e2e.path)
	}
	if configBoolValue(m.config.draft, browser) || configBoolValue(m.config.draft, e2e) {
		t.Fatal("screenshot toggles must default Off")
	}
	if !m.configFieldUnavailable(e2e) {
		t.Fatal("HTTP-only E2E screenshot toggle must be disabled")
	}

	m = m.toggleConfigField(browser)
	m = m.toggleConfigField(e2e)
	if !configBoolValue(m.config.draft, browser) {
		t.Fatal("Browser UI screenshots must toggle independently")
	}
	if configBoolValue(m.config.draft, e2e) {
		t.Fatal("HTTP-only E2E must not enable browser screenshots")
	}
	view := stripANSI(m.renderConfig())
	for _, want := range []string{"Screenshots: On", "Screenshots: Off · HTTP-only E2E", "browser screenshots require Playwright"} {
		if !strings.Contains(view, want) {
			t.Fatalf("Config view missing %q:\n%s", want, view)
		}
	}
}

func TestScreenshotToggleReadOnlyDuringExecution(t *testing.T) {
	m := NewTestModel(nil)
	m.actionBusy = true
	m.config.draft = workflowconfig.ManagedConfig{
		Scope: workflowconfig.Scope{Frontend: true},
		Stages: map[string]workflowconfig.ManagedStage{
			"browser_ui_validation": {Enabled: true},
			"qa_end_to_end":         {Enabled: true, UsePlaywright: true},
		},
	}
	for _, field := range m.configFields() {
		if field.path != "stages.browser_ui_validation.screenshots.enabled" {
			continue
		}
		m.config.focus = indexConfigPath(m.configFields(), field.path)
		updated, _ := m.handleConfigKey(teaKeySpace())
		got := updated.(model)
		if configBoolValue(got.config.draft, field) {
			t.Fatal("screenshot toggle changed while execution guard was active")
		}
		return
	}
	t.Fatal("Browser UI screenshot field missing")
}

func indexConfigPath(fields []configField, path string) int {
	for index, field := range fields {
		if field.path == path {
			return index
		}
	}
	return 0
}

func teaKeySpace() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
}
