package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/testaccess"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

const (
	testUserIDField = iota
	testUserProfileField
	testUserLoginField
	testUserPasswordField
)

const testUserPasswordMask = "••••••••"

var configTestUserKeys = struct {
	Add     key.Binding
	Remove  key.Binding
	Enable  key.Binding
	Commit  key.Binding
	Confirm key.Binding
	Decline key.Binding
}{
	Add:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add user")),
	Remove:  key.NewBinding(key.WithKeys("x", "delete"), key.WithHelp("x", "remove user")),
	Enable:  key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle setup")),
	Commit:  key.NewBinding(key.WithKeys("alt+enter"), key.WithHelp("alt+enter", "finish account")),
	Confirm: key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "confirm remove")),
	Decline: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "keep user")),
}

type configTestUsersScreen struct {
	open            bool
	loading         bool
	saving          bool
	enabled         bool // mirrors the managed workflow-config draft while this editor is open
	originalEnabled bool
	accounts        []testaccess.Account
	baseline        []testaccess.Account
	draft           testaccess.Draft
	cursor          int
	offset          int
	dirty           bool
	stale           bool
	removePending   bool
	form            configTestUserForm
	err             string
	message         string
}

func (configTestUsersScreen) String() string   { return "Config test-users editor (redacted)" }
func (configTestUsersScreen) GoString() string { return "Config test-users editor (redacted)" }
func (configTestUsersScreen) Format(state fmt.State, verb rune) {
	_, _ = io.WriteString(state, "Config test-users editor (redacted)")
}

type configTestUserForm struct {
	open          bool
	isNew         bool
	originalIndex int
	original      testaccess.Account
	values        [4]string
	focus         int
	editing       bool
	editBuffer    string
	editCursor    int
}

func (configTestUserForm) String() string   { return "test-user form (redacted)" }
func (configTestUserForm) GoString() string { return "test-user form (redacted)" }
func (configTestUserForm) Format(state fmt.State, verb rune) {
	_, _ = io.WriteString(state, "test-user form (redacted)")
}

type configTestUsersLoadedMsg struct {
	draft testaccess.Draft
	err   error
}

type configTestUsersSavedMsg struct {
	draft testaccess.Draft
	err   error
}

func (configTestUsersLoadedMsg) String() string   { return "test-users load result (redacted)" }
func (configTestUsersLoadedMsg) GoString() string { return "test-users load result (redacted)" }
func (configTestUsersLoadedMsg) Format(state fmt.State, verb rune) {
	_, _ = io.WriteString(state, "test-users load result (redacted)")
}
func (configTestUsersSavedMsg) String() string   { return "test-users save result (redacted)" }
func (configTestUsersSavedMsg) GoString() string { return "test-users save result (redacted)" }
func (configTestUsersSavedMsg) Format(state fmt.State, verb rune) {
	_, _ = io.WriteString(state, "test-users save result (redacted)")
}

func (m model) openConfigTestUsers() (model, tea.Cmd) {
	if m.configReadOnly() {
		m.config.message = "Test users are read-only while execution/preflight is active."
		return m, nil
	}
	if m.svc == nil {
		m.config.message = "Test users require an active project."
		return m, nil
	}
	m.config.testUsers = configTestUsersScreen{
		open: true, loading: true, enabled: m.config.draft.TestAccess.Enabled,
		originalEnabled: m.config.draft.TestAccess.Enabled,
	}
	return m, configTestUsersLoadCmd(m.svc.ProjectDir)
}

func configTestUsersLoadCmd(projectDir string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		access, err := testaccess.OpenSafeStore(projectDir, slog.Default())
		if err != nil {
			slog.Error("test-users editor open failed", "error", err)
			return configTestUsersLoadedMsg{err: err}
		}
		draft, loadErr := access.Load(ctx)
		closeErr := access.Close()
		if closeErr != nil {
			slog.Error("test-users editor ownership release failed", "error", closeErr)
		}
		if loadErr != nil {
			return configTestUsersLoadedMsg{err: errors.Join(loadErr, closeErr)}
		}
		if closeErr != nil {
			return configTestUsersLoadedMsg{err: errors.New("test-access ownership could not be released")}
		}
		return configTestUsersLoadedMsg{draft: draft}
	}
}

func configTestUsersSaveCmd(projectDir string, draft testaccess.Draft) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		access, err := testaccess.OpenSafeStore(projectDir, slog.Default())
		if err != nil {
			slog.Error("test-users editor open for save failed", "error", err)
			return configTestUsersSavedMsg{err: err}
		}
		saved, saveErr := access.Save(ctx, draft)
		closeErr := access.Close()
		if closeErr != nil {
			slog.Error("test-users editor ownership release failed", "error", closeErr)
		}
		if saveErr != nil {
			return configTestUsersSavedMsg{err: errors.Join(saveErr, closeErr)}
		}
		if closeErr != nil {
			return configTestUsersSavedMsg{err: errors.New("test-access ownership could not be released after saving")}
		}
		return configTestUsersSavedMsg{draft: saved}
	}
}

func (m model) handleConfigTestUsersMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.config.testUsers.open {
		return m, nil
	}
	switch msg := msg.(type) {
	case configTestUsersLoadedMsg:
		m.config.testUsers.loading = false
		if msg.err != nil {
			m.config.testUsers.err = msg.err.Error()
			slog.Error("test-users editor load failed", "error", msg.err)
			return m, nil
		}
		enabled := m.config.testUsers.enabled
		originalEnabled := m.config.testUsers.originalEnabled
		accounts := msg.draft.Document.Accounts()
		m.config.testUsers = configTestUsersScreen{
			open:            true,
			enabled:         enabled,
			originalEnabled: originalEnabled,
			draft:           msg.draft,
			accounts:        accounts,
			baseline:        append([]testaccess.Account(nil), accounts...),
			message:         "Test-user file reloaded.",
		}
		return m, nil
	case configTestUsersSavedMsg:
		m.config.testUsers.saving = false
		if msg.err != nil {
			if errors.Is(msg.err, testaccess.ErrStaleDraft) {
				m.config.testUsers.stale = true
				m.config.testUsers.err = "File changed outside Config. Reload before saving."
			} else {
				m.config.testUsers.err = msg.err.Error()
			}
			slog.Error("test-users editor save failed", "error", msg.err)
			return m, nil
		}
		accounts := msg.draft.Document.Accounts()
		m.config.testUsers.draft = msg.draft
		m.config.testUsers.accounts = accounts
		m.config.testUsers.baseline = append([]testaccess.Account(nil), accounts...)
		m.config.testUsers.dirty = false
		m.config.testUsers.stale = false
		m.config.testUsers.err = ""
		m.config.testUsers.message = "Test-user file saved atomically."
		if m.config.dirty {
			m.config.testUsers.message += " Return to Config and press alt+s to persist the Test users setting."
		}
		return m, nil
	}
	return m, nil
}

func (m model) handleConfigTestUsersKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	t := &m.config.testUsers
	if msg.String() == "ctrl+c" {
		return m.handleKey(msg)
	}
	if t.saving {
		return m, nil
	}
	if key.Matches(msg, configKeys.Cancel) {
		if t.form.open {
			if t.form.editing {
				t.form.editing = false
				t.form.editBuffer = ""
				t.form.editCursor = 0
			} else {
				t.form = configTestUserForm{}
			}
			t.removePending = false
			return m, nil
		}
		m.config.draft.TestAccess.Enabled = t.originalEnabled
		m.config.dirty = len(workflowconfig.ManagedDiff(m.config.baseline, m.config.draft)) > 0
		m.config.testUsers = configTestUsersScreen{}
		m.config.message = "Returned to Config without writing Test users."
		return m, nil
	}
	if key.Matches(msg, configKeys.Reload) {
		if t.loading || m.svc == nil {
			return m, nil
		}
		enabled := t.enabled
		originalEnabled := t.originalEnabled
		*t = configTestUsersScreen{open: true, loading: true, enabled: enabled, originalEnabled: originalEnabled}
		return m, configTestUsersLoadCmd(m.svc.ProjectDir)
	}
	if t.loading {
		return m, nil
	}
	if t.form.open {
		return m.handleConfigTestUserFormKey(msg)
	}
	if t.removePending {
		if m.configReadOnly() {
			return m.configTestUsersReadOnly()
		}
		if key.Matches(msg, configTestUserKeys.Confirm) || key.Matches(msg, configKeys.Edit) {
			return m.confirmConfigTestUserRemove()
		}
		if key.Matches(msg, configTestUserKeys.Decline) || key.Matches(msg, configKeys.Cancel) {
			t.removePending = false
			return m, nil
		}
		return m, nil
	}
	if key.Matches(msg, configKeys.Save) {
		return m.beginConfigTestUsersSave()
	}
	if key.Matches(msg, configKeys.Discard) {
		if m.configReadOnly() {
			return m.configTestUsersReadOnly()
		}
		t.accounts = append([]testaccess.Account(nil), t.baseline...)
		t.dirty = false
		t.stale = false
		t.err = ""
		t.form = configTestUserForm{}
		t.removePending = false
		t.enabled = t.originalEnabled
		m.config.draft.TestAccess.Enabled = t.enabled
		m.config.dirty = len(workflowconfig.ManagedDiff(m.config.baseline, m.config.draft)) > 0
		t.cursor = 0
		t.offset = 0
		t.message = "Test-user changes discarded."
		return m, nil
	}
	if key.Matches(msg, configTestUserKeys.Enable) || key.Matches(msg, configKeys.Edit) && t.cursor == 0 {
		if m.configReadOnly() {
			return m.configTestUsersReadOnly()
		}
		t.enabled = !t.enabled
		m.config.draft.TestAccess.Enabled = t.enabled
		m.config.dirty = len(workflowconfig.ManagedDiff(m.config.baseline, m.config.draft)) > 0
		if !t.enabled {
			t.cursor = 0
		}
		t.message = "Test users opt-in changed in the Config draft. Return to Config and press alt+s to persist it."
		return m, nil
	}
	if key.Matches(msg, configKeys.Next) {
		t.cursor = min(t.cursor+1, t.testUserCursorMax())
		return m.ensureConfigTestUsersCursorVisible(), nil
	}
	if key.Matches(msg, configKeys.Previous) {
		t.cursor = max(0, t.cursor-1)
		return m.ensureConfigTestUsersCursorVisible(), nil
	}
	if key.Matches(msg, configTestUserKeys.Add) {
		if m.configReadOnly() {
			return m.configTestUsersReadOnly()
		}
		if !t.enabled {
			t.message = "Enable Test users before adding accounts. Required-login tests remain blocked while setup is Off."
			return m, nil
		}
		t.form = configTestUserForm{open: true, isNew: true, originalIndex: -1}
		t.err = ""
		t.message = ""
		return m, nil
	}
	if key.Matches(msg, configTestUserKeys.Remove) {
		if m.configReadOnly() {
			return m.configTestUsersReadOnly()
		}
		if !t.enabled || t.cursor == 0 || t.cursor > len(t.accounts) {
			t.message = "Select a configured user to remove."
			return m, nil
		}
		t.removePending = true
		t.message = "Remove this test user? Press [y] to confirm or [n] to keep it."
		return m, nil
	}
	if key.Matches(msg, configKeys.Edit) {
		if !t.enabled || t.cursor == 0 || t.cursor > len(t.accounts) {
			return m, nil
		}
		if m.configReadOnly() {
			return m.configTestUsersReadOnly()
		}
		account := t.accounts[t.cursor-1]
		t.form = configTestUserForm{
			open:          true,
			originalIndex: t.cursor - 1,
			original:      account,
			values:        [4]string{account.ID(), account.Profile(), account.Login(), ""},
		}
		t.err = ""
		t.message = ""
		return m, nil
	}
	return m, nil
}

func (m model) configTestUsersReadOnly() (model, tea.Cmd) {
	m.config.testUsers.message = "Test users are read-only while execution/preflight is active."
	return m, nil
}

func (t *configTestUsersScreen) testUserCursorMax() int {
	if !t.enabled {
		return 0
	}
	return len(t.accounts)
}

func (m model) ensureConfigTestUsersCursorVisible() model {
	t := &m.config.testUsers
	if t.cursor == 0 {
		t.offset = 0
		return m
	}
	visible := max(1, m.height-13)
	row := t.cursor - 1
	if row < t.offset {
		t.offset = row
	}
	if row >= t.offset+visible {
		t.offset = row - visible + 1
	}
	return m
}

func (m model) confirmConfigTestUserRemove() (model, tea.Cmd) {
	t := &m.config.testUsers
	index := t.cursor - 1
	if index < 0 || index >= len(t.accounts) {
		t.removePending = false
		return m, nil
	}
	t.accounts = append(t.accounts[:index], t.accounts[index+1:]...)
	t.dirty = !sameTestAccounts(t.accounts, t.baseline)
	t.removePending = false
	t.err = ""
	if t.cursor > len(t.accounts) {
		t.cursor = len(t.accounts)
	}
	t.message = "Test user removed from the draft. Save to write .env.hero."
	return m.ensureConfigTestUsersCursorVisible(), nil
}

func sameTestAccounts(a, b []testaccess.Account) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID() != b[i].ID() || a[i].Profile() != b[i].Profile() || a[i].Login() != b[i].Login() || a[i].Password() != b[i].Password() {
			return false
		}
	}
	return true
}

func (m model) handleConfigTestUserFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	t := &m.config.testUsers
	f := &t.form
	if key.Matches(msg, configKeys.Cancel) {
		if f.editing {
			f.editing = false
			f.editBuffer = ""
			f.editCursor = 0
		} else {
			*f = configTestUserForm{}
		}
		return m, nil
	}
	if m.configReadOnly() {
		return m.configTestUsersReadOnly()
	}
	if f.editing {
		return m.updateConfigTestUserInput(msg)
	}
	if key.Matches(msg, configTestUserKeys.Commit) {
		return m.commitConfigTestUserForm()
	}
	if key.Matches(msg, configKeys.Next) {
		f.focus = (f.focus + 1) % 4
		return m, nil
	}
	if key.Matches(msg, configKeys.Previous) {
		f.focus = (f.focus + 3) % 4
		return m, nil
	}
	if key.Matches(msg, configKeys.Edit) {
		f.editing = true
		f.editBuffer = f.values[f.focus]
		if f.focus == testUserPasswordField {
			// Existing password material is never loaded into the edit buffer.
			f.editBuffer = ""
		}
		f.editCursor = runeLen(f.editBuffer)
	}
	return m, nil
}

func (m model) updateConfigTestUserInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := &m.config.testUsers.form
	if key.Matches(msg, configKeys.Edit) {
		f.values[f.focus] = f.editBuffer
		f.editBuffer = ""
		f.editCursor = 0
		f.editing = false
		return m, nil
	}
	switch msg.String() {
	case "left":
		if f.editCursor > 0 {
			f.editCursor--
		}
	case "right":
		if f.editCursor < runeLen(f.editBuffer) {
			f.editCursor++
		}
	case "home":
		f.editCursor = 0
	case "end":
		f.editCursor = runeLen(f.editBuffer)
	case "backspace":
		if f.editCursor > 0 {
			runes := []rune(f.editBuffer)
			f.editBuffer = string(append(runes[:f.editCursor-1], runes[f.editCursor:]...))
			f.editCursor--
		}
	case "delete":
		runes := []rune(f.editBuffer)
		if f.editCursor < len(runes) {
			f.editBuffer = string(append(runes[:f.editCursor], runes[f.editCursor+1:]...))
		}
	default:
		if len(msg.Runes) > 0 && !msg.Alt {
			runes := []rune(f.editBuffer)
			cursor := min(max(f.editCursor, 0), len(runes))
			out := make([]rune, 0, len(runes)+len(msg.Runes))
			out = append(out, runes[:cursor]...)
			out = append(out, msg.Runes...)
			out = append(out, runes[cursor:]...)
			f.editBuffer = string(out)
			f.editCursor = cursor + len(msg.Runes)
		}
	}
	return m, nil
}

func (m model) commitConfigTestUserForm() (model, tea.Cmd) {
	t := &m.config.testUsers
	f := t.form
	password := f.values[testUserPasswordField]
	if !f.isNew && password == "" {
		password = f.original.Password()
	}
	account, err := testaccess.NewAccount(f.values[testUserIDField], f.values[testUserLoginField], password, f.values[testUserProfileField])
	if err != nil {
		t.err = err.Error()
		return m, nil
	}
	updated := append([]testaccess.Account(nil), t.accounts...)
	if f.isNew {
		updated = append(updated, account)
	} else if f.originalIndex >= 0 && f.originalIndex < len(updated) {
		updated[f.originalIndex] = account
	} else {
		t.err = "The selected test user is no longer available. Reload before saving."
		return m, nil
	}
	doc := t.draft.Document
	if err := doc.SetAccounts(updated); err != nil {
		t.err = err.Error()
		return m, nil
	}
	t.accounts = updated
	t.dirty = !sameTestAccounts(t.accounts, t.baseline)
	t.form = configTestUserForm{}
	t.err = ""
	t.message = "Test-user draft updated. Save to write .env.hero."
	if f.isNew {
		t.cursor = len(t.accounts)
	}
	return m.ensureConfigTestUsersCursorVisible(), nil
}

func (m model) beginConfigTestUsersSave() (model, tea.Cmd) {
	t := &m.config.testUsers
	if t.loading || t.saving || t.stale {
		if t.stale {
			t.err = "File changed outside Config. Reload before saving."
		}
		return m, nil
	}
	if m.configReadOnly() {
		return m.configTestUsersReadOnly()
	}
	if !t.dirty {
		if m.config.dirty {
			t.message = "No account-file changes. Return to Config and press alt+s to save the pending workflow setting."
		} else {
			t.message = "No Test users changes to save."
		}
		return m, nil
	}
	doc := t.draft.Document
	if err := doc.SetAccounts(t.accounts); err != nil {
		t.err = err.Error()
		return m, nil
	}
	draft := t.draft
	draft.Document = doc
	if m.svc == nil {
		t.err = "Project unavailable; Test users were not saved."
		return m, nil
	}
	t.saving = true
	t.err = ""
	t.message = ""
	return m, configTestUsersSaveCmd(m.svc.ProjectDir, draft)
}

func (m model) renderConfigTestUsers() string {
	t := m.config.testUsers
	if m.width < 50 || m.height < 12 {
		return errorStyle.Render("window too small\nResize the terminal to edit Test users.")
	}
	var b strings.Builder
	b.WriteString(headerStyle.Render("Test users · .env.hero"))
	b.WriteByte('\n')
	if t.loading {
		b.WriteString(infoStyle.Render("→ Loading test-user file…"))
		return b.String()
	}
	if t.saving {
		b.WriteString(infoStyle.Render("→ Saving test-user file…"))
		b.WriteByte('\n')
	}
	if t.err != "" {
		b.WriteString(errorStyle.Render("✗ " + t.err))
		b.WriteByte('\n')
	}
	if m.configReadOnly() {
		b.WriteString(mutedStyle.Render("Test users are read-only while execution/preflight is active."))
		b.WriteByte('\n')
	}
	if t.message != "" {
		b.WriteString(mutedStyle.Render(t.message))
		b.WriteByte('\n')
	}
	if !t.enabled {
		b.WriteString(m.renderConfigField("Enable Test users", "Off", t.cursor == 0, false))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("Protected tests that require login remain blocked while setup is Off."))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("Press space to enable setup. You may also edit the project-root .env.hero manually."))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("This setting is saved in workflow-config.yml when you return to Config and press alt+s."))
		b.WriteByte('\n')
	} else {
		toggleLabel := "On"
		if m.config.draft.TestAccess.Enabled != m.config.baseline.TestAccess.Enabled {
			toggleLabel = "On · unsaved Config change"
		}
		b.WriteString(m.renderConfigField("Enable Test users", toggleLabel, t.cursor == 0, false))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("Saved users are configured, not access-verified. Passwords are never shown, copied, or exported."))
		b.WriteByte('\n')
		if t.form.open {
			b.WriteByte('\n')
			b.WriteString(m.renderConfigTestUserForm())
		} else if t.removePending {
			b.WriteByte('\n')
			b.WriteString(warnStyle.Render(t.message))
		} else if len(t.accounts) == 0 {
			b.WriteByte('\n')
			b.WriteString(mutedStyle.Render("No test users configured. Press [a] to add one."))
		} else {
			b.WriteByte('\n')
			visible := max(1, m.height-13)
			start := min(t.offset, len(t.accounts)-1)
			end := min(len(t.accounts), start+visible)
			for i := start; i < end; i++ {
				account := t.accounts[i]
				line := fmt.Sprintf("%s · profile %s · login %s · password %s",
					safeTestUserDisplay(account.ID()),
					safeTestUserDisplay(account.Profile()),
					safeTestUserDisplay(account.Login()),
					maskedTestUserPassword(account.Password() != ""),
				)
				prefix := "  "
				if t.cursor == i+1 {
					prefix = "▸ "
				}
				for lineIndex, wrapped := range wrapOutputLine(line, max(20, m.configValueWidth())) {
					if lineIndex == 0 {
						b.WriteString(prefix)
					} else {
						b.WriteString("  ")
					}
					b.WriteString(configValueStyle.Render(wrapped))
					b.WriteByte('\n')
				}
			}
		}
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("[↑/↓] select  [space] toggle  [a] add  [enter] edit  [x] remove  [alt+s] save accounts  [d] discard  [alt+r] reload  [esc] return"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("The opt-in toggle is saved with Config: return and press alt+s. Account changes save separately to .env.hero."))
		if t.form.open {
			b.WriteString("\n")
			b.WriteString(mutedStyle.Render("[↑/↓] field  [enter] edit field  [alt+enter] finish account  [esc] cancel account"))
		}
	}
	return b.String()
}

func (m model) renderConfigTestUserForm() string {
	f := m.config.testUsers.form
	title := "Edit test user"
	if f.isNew {
		title = "Add test user"
	}
	var b strings.Builder
	b.WriteString(headerStyle.Render(title))
	b.WriteByte('\n')
	labels := [...]string{"Stable ID", "Profile", "Login", "Password"}
	for i, label := range labels {
		value := f.values[i]
		editing := f.editing && f.focus == i
		if i == testUserPasswordField {
			value = maskedTestUserPassword(f.values[i] != "" || !f.isNew && f.original.Password() != "")
			if value == "" {
				value = "Not set"
			} else if f.values[i] == "" && !f.isNew {
				value += " (stored; blank keeps current)"
			}
			// A password edit is always a fixed mask. Do not expose its length or caret.
		} else if editing {
			value = configValueWithCaret(f.editBuffer, f.editCursor)
		}
		line := m.renderConfigField(label, value, f.focus == i, false)
		if editing && i != testUserPasswordField {
			line = m.renderConfigField(label, value, f.focus == i, false)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func maskedTestUserPassword(hasPassword bool) string {
	if !hasPassword {
		return ""
	}
	return testUserPasswordMask
}

func safeTestUserDisplay(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, value)
}
