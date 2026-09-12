package tui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/rivo/uniseg"
)

// UI-C16-001 §4 copy (exact strings).
const (
	historyCopyEmptyActive1     = "No saved sessions yet."
	historyCopyEmptyActive2     = "→ Send a message in Chat to create one."
	historyCopyEmptyArchived    = "No archived sessions."
	historyCopyBusy1            = "⚠ Session is open in another Hero TUI."
	historyCopyBusy2            = "→ Retry after that instance releases it."
	historyCopyLegacyTranscript = "Local transcript unavailable"
	historyCopyLegacyHint       = "→ Native resume may still be available."
	historyCopyInterrupted1     = "⚠ This session was interrupted during a response."
	historyCopyInterrupted2     = "→ Checking the harness execution before continuing…"
)

const (
	historyMinContentWidth  = 28
	historyMinContentHeight = 10
	historyWideMinWidth     = 72
	historyStackedMinWidth  = 44
	historyNarrowMaxHeight  = 14
)

var historyKeys = struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Search  key.Binding
	Open    key.Binding
	Rename  key.Binding
	Archive key.Binding
	Delete  key.Binding
	Details key.Binding
	Cancel  key.Binding
	Confirm key.Binding
	DialogL key.Binding
	DialogR key.Binding
}{
	Up:      key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "navigate")),
	Down:    key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "navigate")),
	Left:    key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "active/archived")),
	Right:   key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "active/archived")),
	Search:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Open:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
	Rename:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
	Archive: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive")),
	Delete:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Details: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
	Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	DialogL: key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←", "select")),
	DialogR: key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→", "select")),
}

type historyDialog int

const (
	historyDialogNone historyDialog = iota
	historyDialogDelete
	historyDialogArchiveCurrent
	historyDialogFork
	historyDialogImport
)

type historyLayout int

const (
	historyLayoutWide historyLayout = iota
	historyLayoutStacked
	historyLayoutNarrow
)

type historyScreen struct {
	archivedView bool
	sessions     []store.Session
	cursor       int
	listOffset   int

	loading   bool
	mutating  bool
	loadErr   string
	actionErr string

	searchActive bool
	searchQuery  string

	renameActive bool
	renameBuffer string
	renameErr    string
	renameID     string

	dialog      historyDialog
	dialogFocus int // 0 = safe/default (Cancel)
	dialogBusy  bool

	narrowDetail bool
	detailBusy   bool // lease busy detail (UI stub)

	forkHarness         string
	forkModel           string
	forkSourceID        string
	importHarness       string
	importSessionID     string
	pendingOpenRestored bool
}

type historyLoadedMsg struct {
	sessions []store.Session
	err      error
}

type historyRenameMsg struct {
	session store.Session
	err     error
}

type historyArchiveMsg struct {
	session store.Session
	err     error
}

type historyRestoreMsg struct {
	session store.Session
	err     error
}

type historyDeleteMsg struct {
	sessionID     string
	remoteWarning string
	err           error
}

type historyOpenMsg struct {
	sessionID     string
	restored      bool
	err           error
	busy          bool
	needFork      bool
	forkHarness   string
	forkModel     string
	forkSource    string
	offerImport   bool
	importHarness string
}

func (m model) openHistory() (model, tea.Cmd) {
	m.chatInputFocused = false
	m.screen = screenHistory
	m.contentOffset = 0
	m.history.narrowDetail = false
	m.history.detailBusy = false
	m.history.actionErr = ""
	m.history.loading = true
	m.history.loadErr = ""
	return m, tea.Batch(m.historyLoadCmd(), convWaitTickCmd())
}

func (m model) historyLoadCmd() tea.Cmd {
	svc := m.sessionService
	archived := m.history.archivedView
	query := m.history.searchQuery
	return func() tea.Msg {
		if svc == nil {
			return historyLoadedMsg{}
		}
		ctx := context.Background()
		rows, err := svc.ListSessions(ctx, store.ListSessionsFilter{
			Archived: archived,
			Query:    query,
		})
		return historyLoadedMsg{sessions: rows, err: err}
	}
}

func (m model) handleHistoryMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case historyLoadedMsg:
		m.history.loading = false
		if msg.err != nil {
			m.history.loadErr = msg.err.Error()
			slog.Error("history list failed", "error", redact.Error(msg.err))
			return m, nil
		}
		m.history.sessions = msg.sessions
		m.history.loadErr = ""
		m = m.clampHistoryCursor()
		return m, nil

	case historyRenameMsg:
		m.history.mutating = false
		if msg.err != nil {
			if errors.Is(msg.err, store.ErrEmptySessionTitle) {
				m.history.renameErr = "Name cannot be empty."
				return m, nil
			}
			m.history.actionErr = msg.err.Error()
			slog.Error("history rename failed", "error", redact.Error(msg.err))
			return m, nil
		}
		m.history.renameActive = false
		m.history.renameErr = ""
		m = m.historyReplaceSession(msg.session)
		m = m.setStatusResult(true, "history", "Session renamed.")
		return m, tea.Batch(m.historyLoadCmd())

	case historyArchiveMsg:
		m.history.mutating = false
		m.history.dialog = historyDialogNone
		if msg.err != nil {
			m.history.actionErr = msg.err.Error()
			slog.Error("history archive failed", "error", redact.Error(msg.err))
			return m, nil
		}
		m = m.setStatusResult(true, "history", "✓ Session archived.")
		archivedID := strings.TrimSpace(msg.session.ID)
		openID := strings.TrimSpace(m.heroChatSessionID)
		if archivedID != "" && archivedID == openID && !m.streaming {
			return m.emptyChatAfterCurrentSessionMutation()
		}
		return m, tea.Batch(m.historyLoadCmd())

	case historyRestoreMsg:
		m.history.mutating = false
		if msg.err != nil {
			m.history.actionErr = msg.err.Error()
			slog.Error("history restore failed", "error", redact.Error(msg.err))
			return m, nil
		}
		m = m.setStatusResult(true, "history", "✓ Session restored.")
		return m, tea.Batch(m.historyLoadCmd())

	case historyDeleteMsg:
		m.history.mutating = false
		m.history.dialog = historyDialogNone
		m.history.dialogBusy = false
		if msg.err != nil {
			m.history.actionErr = msg.err.Error()
			slog.Error("history delete failed", "error", redact.Error(msg.err))
			return m, nil
		}
		m.history.sessions = removeHistorySession(m.history.sessions, msg.sessionID)
		m = m.clampHistoryCursor()
		msgText := "✓ Session deleted."
		if strings.TrimSpace(msg.remoteWarning) != "" {
			msgText += " " + msg.remoteWarning
		}
		m = m.setStatusResult(true, "history", msgText)
		deletedID := strings.TrimSpace(msg.sessionID)
		openID := strings.TrimSpace(m.heroChatSessionID)
		if deletedID != "" && deletedID == openID && !m.streaming {
			return m.emptyChatAfterCurrentSessionMutation()
		}
		return m, m.historyLoadCmd()

	case historyForkDoneMsg:
		m.history.mutating = false
		m.history.dialog = historyDialogNone
		m.history.forkSourceID = ""
		if msg.err != nil {
			m.history.actionErr = msg.err.Error()
			slog.Error("history fork failed", "error", redact.Error(msg.err))
			return m, nil
		}
		return m.finishHistoryOpen(msg.sessionID, false)

	case historyOpenMsg:
		m.history.mutating = false
		if msg.busy {
			m.history.detailBusy = true
			return m, nil
		}
		if msg.err != nil {
			m.history.actionErr = msg.err.Error()
			slog.Error("history open failed", "error", redact.Error(msg.err))
			return m, nil
		}
		if msg.needFork {
			m.history.forkHarness = msg.forkHarness
			m.history.forkModel = msg.forkModel
			m.history.forkSourceID = msg.forkSource
			m.history.dialog = historyDialogFork
			m.history.dialogFocus = 0
			return m, nil
		}
		if msg.offerImport {
			m.history.dialog = historyDialogImport
			m.history.dialogFocus = 0
			m.history.importHarness = msg.importHarness
			m.history.importSessionID = msg.sessionID
			m.history.pendingOpenRestored = msg.restored
			return m, nil
		}
		return m.finishHistoryOpen(msg.sessionID, msg.restored)

	case historyImportMsg:
		m.history.dialogBusy = false
		m.history.mutating = false
		m.history.dialog = historyDialogNone
		sessionID := strings.TrimSpace(m.history.importSessionID)
		if sessionID == "" {
			sessionID = strings.TrimSpace(msg.sessionID)
		}
		restored := m.history.pendingOpenRestored
		m.history.importSessionID = ""
		m.history.pendingOpenRestored = false
		if msg.err != nil {
			m.history.actionErr = msg.err.Error()
			slog.Error("history remote import failed", "error", redact.Error(msg.err))
			var cmds []tea.Cmd
			if sessionID != "" {
				// Release the provisional lease acquired for import so retries
				// and navigation are not blocked by an orphan owner (find-qa-30).
				cmds = append(cmds, m.releaseHeroChatLeaseCmd(sessionID))
				if strings.TrimSpace(m.heroLeasedSessionID) == sessionID {
					m.heroLeasedSessionID = ""
				}
				if strings.TrimSpace(m.heroChatSessionID) == sessionID {
					m.heroChatSessionID = ""
				}
			}
			return m, tea.Batch(cmds...)
		}
		if msg.imported > 0 {
			m = m.setStatusResult(true, "history", fmt.Sprintf("✓ Imported %d events.", msg.imported))
		}
		return m.finishHistoryOpen(sessionID, restored)
	}
	return m, nil
}

func removeHistorySession(rows []store.Session, id string) []store.Session {
	out := make([]store.Session, 0, len(rows))
	for _, row := range rows {
		if row.ID != id {
			out = append(out, row)
		}
	}
	return out
}

func (m model) historyReplaceSession(sess store.Session) model {
	for i, row := range m.history.sessions {
		if row.ID == sess.ID {
			m.history.sessions[i] = sess
			return m
		}
	}
	return m
}

func (m model) clampHistoryCursor() model {
	n := len(m.history.sessions)
	if n == 0 {
		m.history.cursor = 0
		m.history.listOffset = 0
		return m
	}
	if m.history.cursor >= n {
		m.history.cursor = n - 1
	}
	if m.history.cursor < 0 {
		m.history.cursor = 0
	}
	return m
}

func (m model) historySelected() (store.Session, bool) {
	if m.history.cursor < 0 || m.history.cursor >= len(m.history.sessions) {
		return store.Session{}, false
	}
	return m.history.sessions[m.history.cursor], true
}

func (m model) historyUnsafeActions() bool {
	return m.streaming || m.heroStartBootstrapping || m.heroStartPreparing || m.actionBusy
}

func (m model) handleHistoryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.history.dialog != historyDialogNone {
		return m.handleHistoryDialogKey(msg)
	}
	if m.history.renameActive {
		return m.handleHistoryRenameKey(msg)
	}
	if m.history.searchActive {
		return m.handleHistorySearchKey(msg)
	}
	if m.history.narrowDetail && m.historyLayout() == historyLayoutNarrow {
		return m.handleHistoryNarrowDetailKey(msg)
	}

	switch {
	case key.Matches(msg, historyKeys.Search):
		m.history.searchActive = true
		return m, nil
	case key.Matches(msg, historyKeys.Left):
		if !m.history.archivedView {
			return m, nil
		}
		m.history.archivedView = false
		m.history.loading = true
		m.history.detailBusy = false
		return m, tea.Batch(m.historyLoadCmd(), convWaitTickCmd())
	case key.Matches(msg, historyKeys.Right):
		if m.history.archivedView {
			return m, nil
		}
		m.history.archivedView = true
		m.history.loading = true
		m.history.detailBusy = false
		return m, tea.Batch(m.historyLoadCmd(), convWaitTickCmd())
	case key.Matches(msg, historyKeys.Up):
		if len(m.history.sessions) == 0 {
			return m, nil
		}
		m.history.cursor = (m.history.cursor - 1 + len(m.history.sessions)) % len(m.history.sessions)
		m.history.detailBusy = false
		m = m.ensureHistoryListOffset()
		return m, nil
	case key.Matches(msg, historyKeys.Down):
		if len(m.history.sessions) == 0 {
			return m, nil
		}
		m.history.cursor = (m.history.cursor + 1) % len(m.history.sessions)
		m.history.detailBusy = false
		m = m.ensureHistoryListOffset()
		return m, nil
	case key.Matches(msg, historyKeys.Rename):
		return m.beginHistoryRename()
	case key.Matches(msg, historyKeys.Archive):
		return m.beginHistoryArchiveToggle()
	case key.Matches(msg, historyKeys.Delete):
		return m.beginHistoryDelete()
	case key.Matches(msg, historyKeys.Open):
		if m.historyLayout() == historyLayoutNarrow {
			m.history.narrowDetail = true
			return m, nil
		}
		return m.beginHistoryOpen()
	case key.Matches(msg, historyKeys.Cancel):
		if m.history.searchQuery != "" {
			m.history.searchQuery = ""
			m.history.loading = true
			return m, tea.Batch(m.historyLoadCmd(), convWaitTickCmd())
		}
		return m.focusShellNavbar()
	default:
		return m.handleKey(msg)
	}
}

func (m model) handleHistoryNarrowDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, historyKeys.Cancel):
		m.history.narrowDetail = false
		return m, nil
	case key.Matches(msg, historyKeys.Open):
		return m.beginHistoryOpen()
	default:
		return m, nil
	}
}

func (m model) handleHistorySearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.history.searchActive = false
		if m.history.searchQuery != "" {
			m.history.searchQuery = ""
			m.history.loading = true
			return m, tea.Batch(m.historyLoadCmd(), convWaitTickCmd())
		}
		return m, nil
	case tea.KeyEnter:
		m.history.searchActive = false
		m.history.loading = true
		return m, tea.Batch(m.historyLoadCmd(), convWaitTickCmd())
	case tea.KeyBackspace:
		m.history.searchQuery = deleteLastGrapheme(m.history.searchQuery)
		return m, nil
	default:
		if len(msg.Runes) > 0 {
			m.history.searchQuery += string(msg.Runes)
		}
		return m, nil
	}
}

func (m model) handleHistoryRenameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.history.renameActive = false
		m.history.renameErr = ""
		return m, nil
	case tea.KeyEnter:
		if m.history.mutating {
			return m, nil
		}
		title := strings.TrimSpace(m.history.renameBuffer)
		if title == "" {
			m.history.renameErr = "Name cannot be empty."
			return m, nil
		}
		m.history.mutating = true
		return m, m.historyRenameCmd(m.history.renameID, title)
	case tea.KeyBackspace:
		m.history.renameBuffer = deleteLastGrapheme(m.history.renameBuffer)
		return m, nil
	default:
		if len(msg.Runes) > 0 {
			m.history.renameBuffer += string(msg.Runes)
		}
		return m, nil
	}
}

func (m model) beginHistoryRename() (model, tea.Cmd) {
	if m.historyUnsafeActions() || m.history.mutating || m.history.loading {
		return m, nil
	}
	sess, ok := m.historySelected()
	if !ok {
		return m, nil
	}
	m.history.renameActive = true
	m.history.renameBuffer = sess.Title
	m.history.renameErr = ""
	m.history.renameID = sess.ID
	return m, nil
}

func (m model) historyRenameCmd(id, title string) tea.Cmd {
	svc := m.sessionService
	return func() tea.Msg {
		if svc == nil {
			return historyRenameMsg{err: fmt.Errorf("sessions unavailable")}
		}
		sess, err := svc.RenameSession(context.Background(), id, title)
		return historyRenameMsg{session: sess, err: err}
	}
}

func (m model) historyOpenIdleSessionID() string {
	if m.streaming || m.historyUnsafeActions() {
		return ""
	}
	return strings.TrimSpace(m.heroChatSessionID)
}

func (m model) beginHistoryArchiveToggle() (model, tea.Cmd) {
	if m.historyUnsafeActions() || m.history.mutating || m.history.loading {
		return m, nil
	}
	sess, ok := m.historySelected()
	if !ok {
		return m, nil
	}
	if m.history.archivedView {
		m.history.mutating = true
		return m, m.historyRestoreCmd(sess.ID)
	}
	if strings.TrimSpace(sess.ID) == m.historyOpenIdleSessionID() {
		m.history.dialog = historyDialogArchiveCurrent
		m.history.dialogFocus = 0
		return m, nil
	}
	m.history.mutating = true
	return m, m.historyArchiveCmd(sess.ID)
}

func (m model) historyArchiveCmd(id string) tea.Cmd {
	svc := m.sessionService
	return func() tea.Msg {
		if svc == nil {
			return historyArchiveMsg{err: fmt.Errorf("sessions unavailable")}
		}
		sess, err := svc.ArchiveSession(context.Background(), id)
		return historyArchiveMsg{session: sess, err: err}
	}
}

func (m model) historyRestoreCmd(id string) tea.Cmd {
	svc := m.sessionService
	return func() tea.Msg {
		if svc == nil {
			return historyRestoreMsg{err: fmt.Errorf("sessions unavailable")}
		}
		sess, err := svc.RestoreSession(context.Background(), id)
		return historyRestoreMsg{session: sess, err: err}
	}
}

func (m model) beginHistoryDelete() (model, tea.Cmd) {
	if m.historyUnsafeActions() || m.history.mutating || m.history.loading {
		return m, nil
	}
	if _, ok := m.historySelected(); !ok {
		return m, nil
	}
	m.history.dialog = historyDialogDelete
	m.history.dialogFocus = 0
	return m, nil
}

func (m model) beginHistoryOpen() (model, tea.Cmd) {
	if m.historyUnsafeActions() || m.history.mutating || m.history.loading {
		return m, nil
	}
	sess, ok := m.historySelected()
	if !ok {
		return m, nil
	}
	restore := m.history.archivedView
	m.history.mutating = true
	return m, m.historyOpenCmd(sess.ID, restore, m.priorHeroLeaseSessionID())
}

func (m model) handleHistoryDialogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.history.dialog {
	case historyDialogDelete:
		return m.handleHistoryDeleteDialogKey(msg)
	case historyDialogArchiveCurrent:
		return m.handleHistoryArchiveCurrentDialogKey(msg)
	case historyDialogFork:
		return m.handleHistoryForkDialogKey(msg)
	case historyDialogImport:
		return m.handleHistoryImportDialogKey(msg)
	default:
		m.history.dialog = historyDialogNone
		return m, nil
	}
}

func (m model) handleHistoryDeleteDialogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, historyKeys.DialogL), key.Matches(msg, historyKeys.DialogR):
		m.history.dialogFocus = 1 - m.history.dialogFocus
		return m, nil
	case key.Matches(msg, historyKeys.Cancel):
		m.history.dialog = historyDialogNone
		return m, nil
	case key.Matches(msg, historyKeys.Confirm):
		if m.history.dialogFocus != 1 || m.history.dialogBusy {
			m.history.dialog = historyDialogNone
			return m, nil
		}
		sess, ok := m.historySelected()
		if !ok {
			m.history.dialog = historyDialogNone
			return m, nil
		}
		m.history.dialogBusy = true
		m.history.mutating = true
		return m, m.historyDeleteCompleteCmd(sess.ID)
	default:
		return m, nil
	}
}

func (m model) handleHistoryArchiveCurrentDialogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, historyKeys.DialogL), key.Matches(msg, historyKeys.DialogR):
		m.history.dialogFocus = 1 - m.history.dialogFocus
		return m, nil
	case key.Matches(msg, historyKeys.Cancel):
		m.history.dialog = historyDialogNone
		return m, nil
	case key.Matches(msg, historyKeys.Confirm):
		if m.history.dialogFocus != 1 {
			m.history.dialog = historyDialogNone
			return m, nil
		}
		sess, ok := m.historySelected()
		if !ok {
			m.history.dialog = historyDialogNone
			return m, nil
		}
		m.history.dialog = historyDialogNone
		m.history.mutating = true
		return m, m.historyArchiveCmd(sess.ID)
	default:
		return m, nil
	}
}

func (m model) handleHistoryForkDialogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, historyKeys.Cancel):
		m.history.dialog = historyDialogNone
		return m, nil
	case key.Matches(msg, historyKeys.DialogL), key.Matches(msg, historyKeys.DialogR):
		m.history.dialogFocus = 1 - m.history.dialogFocus
		return m, nil
	case key.Matches(msg, historyKeys.Confirm):
		if m.history.dialogFocus == 1 {
			sourceID := strings.TrimSpace(m.history.forkSourceID)
			if sourceID == "" {
				sess, ok := m.historySelected()
				if ok {
					sourceID = sess.ID
				}
			}
			if sourceID == "" {
				m.history.dialog = historyDialogNone
				return m, nil
			}
			m.history.mutating = true
			m.history.dialogBusy = true
			return m, m.historyForkCmd(sourceID)
		}
		m.history.dialog = historyDialogNone
		return m, nil
	default:
		return m, nil
	}
}

func (m model) handleHistoryImportDialogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, historyKeys.Cancel):
		m.history.dialog = historyDialogNone
		sessionID := strings.TrimSpace(m.history.importSessionID)
		restored := m.history.pendingOpenRestored
		m.history.importSessionID = ""
		m.history.pendingOpenRestored = false
		if sessionID != "" {
			return m.finishHistoryOpen(sessionID, restored)
		}
		return m, nil
	case key.Matches(msg, historyKeys.DialogL), key.Matches(msg, historyKeys.DialogR):
		m.history.dialogFocus = 1 - m.history.dialogFocus
		return m, nil
	case key.Matches(msg, historyKeys.Confirm):
		if m.history.dialogFocus != 1 || m.history.dialogBusy {
			m.history.dialog = historyDialogNone
			sessionID := strings.TrimSpace(m.history.importSessionID)
			restored := m.history.pendingOpenRestored
			m.history.importSessionID = ""
			m.history.pendingOpenRestored = false
			if sessionID != "" {
				return m.finishHistoryOpen(sessionID, restored)
			}
			return m, nil
		}
		sessionID := strings.TrimSpace(m.history.importSessionID)
		if sessionID == "" {
			m.history.dialog = historyDialogNone
			return m, nil
		}
		m.history.dialogBusy = true
		m.history.mutating = true
		return m, m.historyImportCmd(sessionID)
	default:
		return m, nil
	}
}

func (m model) historyLayout() historyLayout {
	w := m.contentWidth()
	h := m.frameContentHeight()
	if w < historyMinContentWidth || h < historyMinContentHeight {
		return historyLayoutNarrow
	}
	if w < historyStackedMinWidth || h < historyNarrowMaxHeight {
		return historyLayoutNarrow
	}
	if w < historyWideMinWidth {
		return historyLayoutStacked
	}
	return historyLayoutWide
}

func (m model) ensureHistoryListOffset() model {
	layout := m.historyLayout()
	visible := m.historyListVisibleRows(layout)
	if visible < 1 {
		visible = 1
	}
	if m.history.cursor < m.history.listOffset {
		m.history.listOffset = m.history.cursor
	}
	if m.history.cursor >= m.history.listOffset+visible {
		m.history.listOffset = m.history.cursor - visible + 1
	}
	if m.history.listOffset < 0 {
		m.history.listOffset = 0
	}
	return m
}

func (m model) historyListVisibleRows(layout historyLayout) int {
	h := m.frameContentHeight()
	switch layout {
	case historyLayoutWide, historyLayoutStacked:
		return max(3, h-8)
	default:
		return max(3, h-6)
	}
}

func (m model) renderHistory() string {
	layout := m.historyLayout()
	w := m.contentWidth()
	h := m.frameContentHeight()
	if w < historyMinContentWidth || h < historyMinContentHeight {
		return errorStyle.Render("window too small\nResize the terminal to use History.")
	}
	if m.history.narrowDetail && layout == historyLayoutNarrow {
		return m.renderHistoryDetailPanel(w)
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render("History"))
	b.WriteByte('\n')
	b.WriteString(m.renderHistoryTabs(w))
	b.WriteByte('\n')
	if m.history.searchActive || m.history.searchQuery != "" {
		b.WriteString(m.renderHistorySearchLine(w))
		b.WriteByte('\n')
	}
	if m.history.loading {
		frame := waitAnimFrames[m.waitAnimFrame%len(waitAnimFrames)]
		b.WriteString(mutedStyle.Render(frame + " Loading sessions…"))
		b.WriteByte('\n')
	}
	if m.history.loadErr != "" {
		b.WriteString(errorStyle.Render(m.history.loadErr))
		b.WriteByte('\n')
	}
	if m.history.actionErr != "" {
		b.WriteString(errorStyle.Render(m.history.actionErr))
		b.WriteByte('\n')
	}
	if !m.history.loading && len(m.history.sessions) == 0 && m.history.loadErr == "" {
		b.WriteString(m.renderHistoryEmpty())
		b.WriteByte('\n')
	}
	if len(m.history.sessions) > 0 && !m.history.loading {
		b.WriteString(m.renderHistoryList(layout, w))
	}
	if layout == historyLayoutWide && len(m.history.sessions) > 0 {
		b.WriteByte('\n')
		b.WriteString(m.renderHistoryDetailPanel(w))
	} else if layout == historyLayoutStacked && len(m.history.sessions) > 0 {
		b.WriteByte('\n')
		b.WriteString(m.renderHistoryDetailPanel(w))
	}
	if m.history.renameActive {
		b.WriteByte('\n')
		b.WriteString(m.renderHistoryRenameLine(w))
	}
	if m.history.dialog != historyDialogNone {
		b.WriteByte('\n')
		b.WriteString(m.renderHistoryDialog(w))
	}
	return b.String()
}

func (m model) renderHistoryTabs(width int) string {
	active := "Active"
	archived := "Archived"
	if m.history.archivedView {
		active = mutedStyle.Render("Active")
		archived = headerStyle.Render("Archived")
	} else {
		active = headerStyle.Render("Active")
		archived = mutedStyle.Render("Archived")
	}
	line := active + "  " + archived
	return truncateHistoryText(line, width)
}

func (m model) renderHistorySearchLine(width int) string {
	prefix := "Search by name: "
	if m.history.searchActive {
		prefix = infoStyle.Render(prefix)
	}
	line := prefix + m.history.searchQuery
	if m.history.searchActive {
		line += "▌"
	}
	return truncateHistoryText(line, width)
}

func (m model) renderHistoryEmpty() string {
	if m.history.archivedView {
		return mutedStyle.Render(historyCopyEmptyArchived)
	}
	return mutedStyle.Render(historyCopyEmptyActive1) + "\n" + mutedStyle.Render(historyCopyEmptyActive2)
}

func (m model) renderHistoryList(layout historyLayout, width int) string {
	visible := m.historyListVisibleRows(layout)
	start := m.history.listOffset
	end := start + visible
	if end > len(m.history.sessions) {
		end = len(m.history.sessions)
	}
	var b strings.Builder
	for i := start; i < end; i++ {
		sess := m.history.sessions[i]
		marker := "  "
		if i == m.history.cursor {
			marker = "› "
		}
		title := truncateHistoryText(sess.Title, historyListTitleWidth(width, layout))
		meta := historyListMeta(sess, width, layout)
		ts := formatHistoryActivityTime(sess.LastActivityAt)
		titleCol := padDisplayWidth(title, historyListTitleWidth(width, layout))
		line := marker + titleCol + " " + meta + " " + ts
		if i == m.history.cursor {
			b.WriteString(infoStyle.Render(truncateHistoryText(line, width)))
		} else {
			b.WriteString(truncateHistoryText(line, width))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func historyListTitleWidth(width int, layout historyLayout) int {
	switch layout {
	case historyLayoutNarrow:
		return max(12, width-4)
	default:
		return max(16, width/3)
	}
}

func historyListMeta(sess store.Session, width int, layout historyLayout) string {
	kind := historySessionKindLabel(sess)
	harnessModel := historyHarnessModel(sess)
	switch layout {
	case historyLayoutNarrow:
		return ""
	case historyLayoutStacked:
		if width < 56 {
			return kind
		}
		return truncateHistoryText(kind+" · "+harnessModel, width/2)
	default:
		return truncateHistoryText(kind+" · "+harnessModel, width/3)
	}
}

func (m model) renderHistoryDetailPanel(width int) string {
	sess, ok := m.historySelected()
	if !ok {
		return ""
	}
	var b strings.Builder
	if layout := m.historyLayout(); layout == historyLayoutStacked {
		b.WriteString(mutedStyle.Render("│"))
		b.WriteByte('\n')
	}
	rows := historyDetailRows(sess, m.history.detailBusy)
	for _, row := range rows {
		line := fmt.Sprintf("│ %-14s %s", row.label, row.value)
		b.WriteString(truncateHistoryText(line, width))
		b.WriteByte('\n')
	}
	m.writeHistoryDetailStateLines(&b, sess, width)
	return b.String()
}

func historyDetailRows(sess store.Session, busy bool) []struct {
	label string
	value string
} {
	cycle := "—"
	if sess.CycleID.Valid && sess.CycleID.Int64 > 0 {
		cycle = fmt.Sprintf("C%d", sess.CycleID.Int64)
	}
	stageAgent := "—"
	if sess.StageName != "" || sess.AgentName != "" {
		stageAgent = conversation.StageDisplayName(sess.StageName)
		if label := conversation.AgentShortLabel(sess.AgentName); label != "" {
			stageAgent += " / " + label
		}
	}
	state := historyLifecycleLabel(sess.Lifecycle)
	transcript := "Available locally"
	if sess.TranscriptState == store.TranscriptUnavailableLegacy {
		transcript = historyCopyLegacyTranscript
	}
	return []struct {
		label string
		value string
	}{
		{"Name", sess.Title},
		{"Type", historySessionTypeLabel(sess)},
		{"Cycle", cycle},
		{"Stage/Agent", stageAgent},
		{"Harness", sess.HarnessID},
		{"Model", sess.Model},
		{"Created", formatHistoryDetailTime(sess.CreatedAt)},
		{"Last activity", formatHistoryDetailTime(sess.LastActivityAt)},
		{"State", state},
		{"Transcript", transcript},
	}
}

func (m model) writeHistoryDetailStateLines(b *strings.Builder, sess store.Session, width int) {
	if m.history.detailBusy {
		b.WriteString(mutedStyle.Render(historyCopyBusy1))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render(historyCopyBusy2))
		b.WriteByte('\n')
		return
	}
	if sess.TranscriptState == store.TranscriptUnavailableLegacy {
		b.WriteString(mutedStyle.Render(historyCopyLegacyHint))
		b.WriteByte('\n')
	}
	if sess.Lifecycle == store.SessionLifecycleInterrupted {
		b.WriteString(warningStyle.Render(historyCopyInterrupted1))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render(historyCopyInterrupted2))
		b.WriteByte('\n')
	}
}

func (m model) renderHistoryRenameLine(width int) string {
	line := "Rename: " + m.history.renameBuffer
	if m.history.renameActive {
		line += "▌"
	}
	if m.history.renameErr != "" {
		line += "\n" + errorStyle.Render(m.history.renameErr)
	}
	return truncateHistoryText(line, width)
}

func (m model) renderHistoryDialog(width int) string {
	switch m.history.dialog {
	case historyDialogArchiveCurrent:
		sess, ok := m.historySelected()
		if !ok {
			return ""
		}
		var b strings.Builder
		b.WriteString(headerStyle.Render(fmt.Sprintf("Archive “%s” while it is open in Chat?", sess.Title)))
		b.WriteByte('\n')
		b.WriteString(m.renderHistoryDialogChoices("Archive", "Cancel", width))
		return b.String()
	case historyDialogDelete:
		sess, ok := m.historySelected()
		if !ok {
			return ""
		}
		var b strings.Builder
		b.WriteString(headerStyle.Render(fmt.Sprintf("Delete “%s” permanently?", sess.Title)))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("This removes the local transcript and Hero-managed asset copies."))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("Original files are never deleted. Provider-side data may remain if the"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("harness cannot delete it."))
		b.WriteByte('\n')
		b.WriteString(m.renderHistoryDialogChoices("Delete permanently", "Cancel", width))
		return b.String()
	case historyDialogFork:
		h := m.history.forkHarness
		mod := m.history.forkModel
		if h == "" {
			h = "harness"
		}
		if mod == "" {
			mod = "model"
		}
		var b strings.Builder
		b.WriteString(errorStyle.Render(fmt.Sprintf("✗ The original %s/%s session cannot be resumed.", h, mod)))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("→ Create a new session using the available local transcript as context?"))
		b.WriteByte('\n')
		b.WriteString(m.renderHistoryDialogChoices("Create fork", "Cancel", width))
		return b.String()
	case historyDialogImport:
		h := m.history.importHarness
		if h == "" {
			h = "harness"
		}
		var b strings.Builder
		b.WriteString(headerStyle.Render(fmt.Sprintf("Import transcript from %s?", h)))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("The imported conversation will be stored locally in this project."))
		b.WriteByte('\n')
		b.WriteString(m.renderHistoryDialogChoices("Import", "Cancel", width))
		return b.String()
	default:
		return ""
	}
}

func (m model) renderHistoryDialogChoices(primary, secondary string, width int) string {
	left := secondary
	right := primary
	if m.history.dialogFocus == 1 {
		left = mutedStyle.Render(secondary)
		right = infoStyle.Render(primary)
	} else {
		left = infoStyle.Render(secondary)
		right = mutedStyle.Render(primary)
	}
	return truncateHistoryText(left+"  "+right, width)
}

func historyLifecycleLabel(lifecycle string) string {
	switch strings.ToLower(strings.TrimSpace(lifecycle)) {
	case store.SessionLifecycleArchived:
		return "Archived"
	case store.SessionLifecycleInterrupted:
		return "Interrupted"
	case store.SessionLifecycleDeleting:
		return "Deleting"
	default:
		return "Active"
	}
}

func historySessionTypeLabel(sess store.Session) string {
	switch sess.Kind {
	case store.SessionKindFreechat:
		return "Free Chat"
	case store.SessionKindOrchestration:
		return "Orchestration"
	case store.SessionKindResearch:
		return "Research"
	case store.SessionKindStageAgent:
		return "Stage"
	default:
		return "Session"
	}
}

func historySessionKindLabel(sess store.Session) string {
	switch sess.Kind {
	case store.SessionKindFreechat:
		return "Free Chat"
	case store.SessionKindStageAgent:
		return conversation.StageDisplayName(sess.StageName)
	case store.SessionKindResearch:
		return "Research"
	case store.SessionKindOrchestration:
		return "Orchestration"
	default:
		return historySessionTypeLabel(sess)
	}
}

func historyHarnessModel(sess store.Session) string {
	h := strings.TrimSpace(sess.HarnessID)
	m := strings.TrimSpace(sess.Model)
	if h == "" && m == "" {
		return "—"
	}
	if h == "" {
		return m
	}
	if m == "" {
		return h
	}
	return h + "/" + m
}

func formatHistoryActivityTime(ts string) string {
	t, ok := parseHistoryTime(ts)
	if !ok {
		return truncateTS(ts)
	}
	return t.Local().Format("2 Jan 15:04")
}

func formatHistoryDetailTime(ts string) string {
	t, ok := parseHistoryTime(ts)
	if !ok {
		return truncateTS(ts)
	}
	return t.Local().Format("2 Jan 2006 15:04")
}

func parseHistoryTime(ts string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return time.Time{}, false
		}
	}
	return t, true
}

// deleteLastGrapheme removes the trailing grapheme cluster (UTF-8 safe).
func deleteLastGrapheme(s string) string {
	if s == "" {
		return ""
	}
	gr := uniseg.NewGraphemes(s)
	var clusters []string
	for gr.Next() {
		clusters = append(clusters, gr.Str())
	}
	if len(clusters) <= 1 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(clusters)-1; i++ {
		b.WriteString(clusters[i])
	}
	return b.String()
}

// truncateHistoryText shortens s using ANSI-safe display-cell clipping (preserves escape sequences).
func truncateHistoryText(s string, width int) string {
	return truncateDisplayWidth(s, width)
}

var warningStyle = lipgloss.NewStyle().Foreground(colorAccentFast)

func (m model) historyFooterHints() string {
	if m.history.dialog != historyDialogNone {
		return "←→ select · enter confirm · esc cancel"
	}
	if m.history.renameActive {
		return "enter save · esc cancel"
	}
	if m.history.searchActive {
		return "/ search name (not command palette) · enter apply · esc cancel"
	}
	if m.history.narrowDetail && m.historyLayout() == historyLayoutNarrow {
		return "enter open · esc list · " + m.historyActionFooter()
	}
	return "↑↓ navigate · ←→ active/archived · / search · " + m.historyActionFooter() + " · esc navbar"
}

func (m model) historyActionFooter() string {
	parts := []string{"enter open", "r rename"}
	if !m.historyUnsafeActions() && !m.history.mutating {
		if m.history.archivedView {
			parts = append(parts, "a restore")
		} else {
			parts = append(parts, "a archive")
		}
		parts = append(parts, "d delete")
	}
	return strings.Join(parts, " · ")
}
