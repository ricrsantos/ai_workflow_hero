package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/testaccess"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

func TestTestUsersEditorAsyncLoadAndSaveMessagesReachEditor(t *testing.T) {
	doc, err := testaccess.ParseDotenv([]byte("HERO_TEST_USERS=operator\nHERO_TEST_USER_OPERATOR_LOGIN=fixture\nHERO_TEST_USER_OPERATOR_PASSWORD=sentinel-password\nHERO_TEST_USER_OPERATOR_PROFILE=operator\n"))
	if err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(nil)
	m.width, m.height = 120, 40
	m.config.testUsers = configTestUsersScreen{open: true, loading: true, enabled: true}
	updated, _ := m.Update(configTestUsersLoadedMsg{draft: testaccess.Draft{Document: doc}})
	m = updated.(model)
	if m.config.testUsers.loading || len(m.config.testUsers.accounts) != 1 {
		t.Fatal("async load did not update the editor")
	}
	if strings.Contains(m.renderConfigTestUsers(), "sentinel-password") {
		t.Fatal("password revealed by loaded editor")
	}
	m.config.testUsers.saving = true
	m.config.testUsers.dirty = true
	updated, _ = m.Update(configTestUsersSavedMsg{draft: testaccess.Draft{Document: doc}})
	m = updated.(model)
	if m.config.testUsers.saving || m.config.testUsers.dirty {
		t.Fatal("async save did not clear pending state")
	}
}

func TestTestUsersEditorPasswordInputMaskedAndCancelDiscards(t *testing.T) {
	m := NewTestModel(nil)
	m.width, m.height = 120, 40
	m.config.testUsers = configTestUsersScreen{open: true, enabled: true,
		form: configTestUserForm{open: true, isNew: true, focus: testUserPasswordField, editing: true}}
	updated, _ := m.handleConfigTestUsersKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("sentinel-secret")})
	m = updated.(model)
	if strings.Contains(m.renderConfigTestUserForm(), "sentinel-secret") {
		t.Fatal("password input appeared in form")
	}
	updated, cmd := m.handleConfigTestUsersKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)
	if cmd != nil || m.config.testUsers.form.editBuffer != "" || m.config.testUsers.form.editing {
		t.Fatal("cancel did not discard the password edit")
	}
}

func TestStaleReloadGateRefusesSaveAndOffersReload(t *testing.T) {
	m := NewTestModel(nil)
	m.config.testUsers = configTestUsersScreen{open: true, saving: true, dirty: true}
	updated, _ := m.Update(configTestUsersSavedMsg{err: testaccess.ErrStaleDraft})
	m = updated.(model)
	if !m.config.testUsers.stale || !strings.Contains(m.config.testUsers.err, "Reload") {
		t.Fatal("stale save failed to require Reload")
	}
	m, cmd := m.beginConfigTestUsersSave()
	if cmd != nil || m.config.testUsers.saving {
		t.Fatal("stale draft attempted another write")
	}
}

func TestTestUsersEditorOptInStagesAndDiscardsManagedConfigChange(t *testing.T) {
	m := NewTestModel(nil)
	m.config.baseline = workflowconfig.ManagedConfig{}
	m.config.draft = workflowconfig.ManagedConfig{}
	m.config.testUsers = configTestUsersScreen{open: true}

	updated, _ := m.handleConfigTestUsersKey(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(model)
	if !m.config.draft.TestAccess.Enabled || !m.config.dirty || !m.config.testUsers.enabled {
		t.Fatal("Test users toggle did not stage the managed workflow setting")
	}
	if paths := workflowconfig.ManagedDiff(m.config.baseline, m.config.draft); len(paths) != 1 || paths[0] != "test_access.enabled" {
		t.Fatalf("managed diff=%v, want only test_access.enabled", paths)
	}
	view := stripANSI(m.renderConfigTestUsers())
	if !strings.Contains(view, "return and press alt+s") {
		t.Fatalf("editor does not explain how to persist its workflow setting:\n%s", view)
	}

	updated, _ = m.handleConfigTestUsersKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)
	if m.config.testUsers.open || m.config.draft.TestAccess.Enabled || m.config.dirty {
		t.Fatal("Cancel should return without persisting the staged Test users setting")
	}
}

func TestTestUsersEditorOffExplainsPersistentConfigSave(t *testing.T) {
	m := NewTestModel(nil)
	m.width, m.height = 120, 40
	m.config.baseline = workflowconfig.ManagedConfig{}
	m.config.draft = workflowconfig.ManagedConfig{}
	m.config.testUsers = configTestUsersScreen{open: true}

	view := stripANSI(m.renderConfigTestUsers())
	if strings.Contains(view, "session-only") || strings.Contains(view, "persistent opt-in storage is pending") {
		t.Fatalf("editor still describes the opt-in as session-only:\n%s", view)
	}
	if !strings.Contains(view, "saved in workflow-config.yml") || !strings.Contains(view, "alt+s") {
		t.Fatalf("editor does not explain persistent Config save:\n%s", view)
	}
}

func TestTestUsersEditorConfigRowsExposePersistentToggleAndAccountEditor(t *testing.T) {
	m := NewTestModel(nil)
	m.config.draft = workflowconfig.ManagedConfig{}
	fields := m.configFields()
	foundToggle, foundEditor := false, false
	for _, field := range fields {
		if field.path == "test_access.enabled" && field.kind == "bool" {
			foundToggle = true
		}
		if field.path == "test_access.editor" && field.kind == "action" {
			foundEditor = true
		}
	}
	if !foundToggle || !foundEditor {
		t.Fatalf("Test users Config rows missing: toggle=%t editor=%t", foundToggle, foundEditor)
	}
}

func TestTestUsersEditorReadOnlyGuardPreventsMutation(t *testing.T) {
	m := NewTestModel(nil)
	m.streaming = true
	m.config.testUsers = configTestUsersScreen{open: true, enabled: true}
	updated, cmd := m.handleConfigTestUsersKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd != nil || updated.(model).config.testUsers.form.open {
		t.Fatal("busy editor allowed a new account")
	}
}
