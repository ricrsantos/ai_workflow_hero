package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

type todoControlPhase int

const (
	todoPhaseNone todoControlPhase = iota
	todoPhaseAddSelect
	todoPhaseAddReviewPartial
	todoPhaseAddReviewDeferAll
	todoPhaseAddTypeDefer
	todoPhaseCompleteSelect
	todoPhaseCompleteNote
	todoPhaseCompleteReview
	todoPhaseFinishTypeFinish
)

const (
	addTodoGateMessage      = "✗ /hero-add-todo is available only for open findings in an Escalated loop."
	addTodoSelectNoneHint   = "Select at least one finding or press Esc to cancel."
	addTodoProjectionErrMsg = "✗ ToDo projection was not completed; the cycle remains Escalated.\n  No duplicate was created. Run /hero-add-todo again to retry reconciliation."
	typedDeferConfirm       = "DEFER"
	typedFinishConfirm      = "FINISH"
	completeTodoSecretWarn  = "✗ Resolution note looks like a secret or credential. Remove it before continuing."
)

type addFindingTodosResultMsg struct {
	result cycle.AddFindingTodosResult
	err    error
}

type completeManualTodosResultMsg struct {
	todoIDs []string
	note    string
	err     error
}

func (m model) todoControlActive() bool {
	return m.todoPhase != todoPhaseNone
}

func (m model) todoControlUsesComposer() bool {
	switch m.todoPhase {
	case todoPhaseCompleteNote, todoPhaseAddTypeDefer, todoPhaseFinishTypeFinish:
		return true
	default:
		return false
	}
}

func (m model) clearTodoControl() model {
	m.todoPhase = todoPhaseNone
	m.todoCtrlFindings = nil
	m.todoCtrlSelected = nil
	m.todoCtrlCursor = 0
	m.todoCtrlTodoIDs = nil
	m.todoCtrlNote = ""
	m.todoCtrlFinishIDs = nil
	m.todoCtrlBusy = false
	return m
}

func findingOwnerLabel(owner string) string {
	switch strings.TrimSpace(owner) {
	case store.FindingOwnerBackend:
		return "BACK"
	case store.FindingOwnerFrontend:
		return "FRNT"
	case store.FindingOwnerGeneric:
		return "GEN"
	default:
		o := strings.TrimSpace(owner)
		if o == "" {
			return "?"
		}
		if len(o) > 4 {
			return strings.ToUpper(o[:4])
		}
		return strings.ToUpper(o)
	}
}

func oneLineIssue(text string) string {
	s := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if len(s) > 64 {
		return s[:61] + "..."
	}
	return s
}

func (m model) beginHeroAddTodo(preselect []string) (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	if m.svc == nil {
		m = m.setStatusResult(false, "/hero-add-todo", "cycle service unavailable")
		return m, nil
	}
	st, err := m.svc.Status()
	if err != nil || !statusViewIsActive(st) {
		m = m.setStatusResult(false, "/hero-add-todo", noActiveCycleForStartMessage())
		return m, nil
	}
	if escalatedStage(st) == "" {
		m, _ = m.enterConversation()
		m.convError = addTodoGateMessage
		return m, nil
	}
	findings, err := m.svc.ListActionableFindings("")
	if err != nil {
		m = m.setStatusResult(false, "/hero-add-todo", err.Error())
		return m, nil
	}
	if len(findings) == 0 {
		m, _ = m.enterConversation()
		m.convError = addTodoGateMessage
		return m, nil
	}
	if len(preselect) > 0 {
		if errMsg := validateAddTodoPreselect(m.svc, findings, preselect); errMsg != "" {
			m, _ = m.enterConversation()
			m.convError = errMsg
			return m, nil
		}
	}
	selected := make(map[string]bool, len(preselect))
	for _, id := range preselect {
		selected[id] = true
	}
	m, _ = m.enterConversation()
	m = m.clearTodoControl()
	m.todoPhase = todoPhaseAddSelect
	m.todoCtrlFindings = findings
	m.todoCtrlSelected = selected
	m.todoCtrlCursor = 0
	m.convError = ""
	m.chatInputFocused = false
	return m, nil
}

func validateAddTodoPreselect(svc *cycle.Service, actionable []store.Finding, ids []string) string {
	allowed := make(map[string]store.Finding, len(actionable))
	for _, f := range actionable {
		allowed[f.ID] = f
	}
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := allowed[id]; !ok {
			f, err := lookupFindingForAddTodo(svc, id)
			if err != nil {
				return err.Error()
			}
			switch f.Status {
			case store.FindingStatusOpen, store.FindingStatusReopened:
				return fmt.Sprintf("✗ %s is not actionable in this Escalated loop.", id)
			case store.FindingStatusDeferredTodo:
				return fmt.Sprintf("✗ %s is already deferred to a project ToDo.", id)
			default:
				return fmt.Sprintf("✗ %s is %s and cannot be deferred.", id, f.Status)
			}
		}
	}
	return ""
}

func lookupFindingForAddTodo(svc *cycle.Service, id string) (store.Finding, error) {
	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		return store.Finding{}, err
	}
	f, err := svc.Store.GetFinding(c.ID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Finding{}, fmt.Errorf("unknown finding %q", id)
		}
		return store.Finding{}, err
	}
	if f.CycleID != c.ID {
		return store.Finding{}, fmt.Errorf("finding %q belongs to another cycle", id)
	}
	return f, nil
}

func (m model) selectedFindingIDs() []string {
	var out []string
	for _, f := range m.todoCtrlFindings {
		if m.todoCtrlSelected[f.ID] {
			out = append(out, f.ID)
		}
	}
	return out
}

func (m model) handleTodoControlKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.todoCtrlBusy {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		switch m.todoPhase {
		case todoPhaseAddReviewPartial, todoPhaseAddReviewDeferAll:
			m.todoPhase = todoPhaseAddSelect
			m.convError = ""
			return m, nil
		case todoPhaseCompleteReview:
			m.todoPhase = todoPhaseCompleteNote
			m = m.clearChatInput()
			m.chatInputFocused = true
			return m, nil
		default:
			m = m.clearTodoControl()
			return m, nil
		}
	}
	switch m.todoPhase {
	case todoPhaseAddSelect:
		return m.handleAddTodoSelectKey(msg)
	case todoPhaseAddReviewPartial, todoPhaseAddReviewDeferAll:
		if msg.String() == "enter" {
			if m.todoPhase == todoPhaseAddReviewDeferAll {
				m.todoPhase = todoPhaseAddTypeDefer
				m = m.clearChatInput()
				m.chatInputFocused = true
				return m, nil
			}
			return m.runAddFindingTodos(m.selectedFindingIDs())
		}
		if msg.String() == "backspace" || msg.String() == "left" {
			m.todoPhase = todoPhaseAddSelect
			return m, nil
		}
	case todoPhaseCompleteSelect:
		return m.handleCompleteTodoSelectKey(msg)
	case todoPhaseCompleteReview:
		if msg.String() == "enter" {
			return m.runCompleteManualTodos(m.todoCtrlTodoIDs, m.todoCtrlNote)
		}
		if msg.String() == "backspace" || msg.String() == "left" {
			m.todoPhase = todoPhaseCompleteNote
			m = m.clearChatInput()
			m.chatInputFocused = true
			return m, nil
		}
	}
	return m, nil
}

func (m model) handleAddTodoSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case " ":
		if len(m.todoCtrlFindings) == 0 {
			return m, nil
		}
		id := m.todoCtrlFindings[m.todoCtrlCursor].ID
		sel := m.todoCtrlSelected[id]
		m.todoCtrlSelected[id] = !sel
		return m, nil
	case "a", "A":
		for _, f := range m.todoCtrlFindings {
			m.todoCtrlSelected[f.ID] = true
		}
		return m, nil
	case "n", "N":
		m.todoCtrlSelected = make(map[string]bool)
		return m, nil
	case "up", "k":
		if m.todoCtrlCursor > 0 {
			m.todoCtrlCursor--
		}
		return m, nil
	case "down", "j":
		if m.todoCtrlCursor < len(m.todoCtrlFindings)-1 {
			m.todoCtrlCursor++
		}
		return m, nil
	case "enter":
		selected := m.selectedFindingIDs()
		if len(selected) == 0 {
			m.convError = addTodoSelectNoneHint
			return m, nil
		}
		m.convError = ""
		if len(selected) == len(m.todoCtrlFindings) {
			m.todoPhase = todoPhaseAddReviewDeferAll
		} else {
			m.todoPhase = todoPhaseAddReviewPartial
		}
		return m, nil
	}
	return m, nil
}

func (m model) handleCompleteTodoSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case " ":
		if len(m.todoCtrlTodoIDs) == 0 {
			return m, nil
		}
		id := m.todoCtrlTodoIDs[m.todoCtrlCursor]
		sel := m.todoCtrlSelected[id]
		m.todoCtrlSelected[id] = !sel
		return m, nil
	case "up", "k":
		if m.todoCtrlCursor > 0 {
			m.todoCtrlCursor--
		}
		return m, nil
	case "down", "j":
		if m.todoCtrlCursor < len(m.todoCtrlTodoIDs)-1 {
			m.todoCtrlCursor++
		}
		return m, nil
	case "enter":
		var selected []string
		for _, id := range m.todoCtrlTodoIDs {
			if m.todoCtrlSelected[id] {
				selected = append(selected, id)
			}
		}
		if len(selected) == 0 {
			m.convError = "Select at least one pending ToDo or press Esc to cancel."
			return m, nil
		}
		m.todoCtrlTodoIDs = selected
		m.todoPhase = todoPhaseCompleteNote
		m = m.clearChatInput()
		m.chatInputFocused = true
		m.convError = ""
		return m, nil
	}
	return m, nil
}

func (m model) submitTodoControlComposer() (model, tea.Cmd) {
	text := strings.TrimSpace(m.input)
	switch m.todoPhase {
	case todoPhaseCompleteNote:
		if text == "" {
			m.convError = "Resolution note is required."
			return m, nil
		}
		if redact.HasToken(text) {
			m.convError = completeTodoSecretWarn
			return m, nil
		}
		m.todoCtrlNote = text
		m = m.clearChatInput()
		m.todoPhase = todoPhaseCompleteReview
		m.chatInputFocused = false
		m.convError = ""
		return m, nil
	case todoPhaseAddTypeDefer:
		if !strings.EqualFold(text, typedDeferConfirm) {
			m.convError = fmt.Sprintf("Type %s to confirm or press Esc to cancel.", typedDeferConfirm)
			return m, nil
		}
		m = m.clearChatInput()
		return m.runAddFindingTodos(m.selectedFindingIDs())
	case todoPhaseFinishTypeFinish:
		if !strings.EqualFold(text, typedFinishConfirm) {
			m.convError = fmt.Sprintf("Type %s to confirm or press Esc to cancel.", typedFinishConfirm)
			return m, nil
		}
		m = m.clearTodoControl()
		m = m.clearChatInput()
		return m.beginHeroFinishExecute()
	}
	return m, nil
}

func (m model) runAddFindingTodos(ids []string) (model, tea.Cmd) {
	m.todoCtrlBusy = true
	svc := m.svc
	return m, func() tea.Msg {
		res, err := svc.AddFindingTodos(ids, "")
		return addFindingTodosResultMsg{result: res, err: err}
	}
}

func (m model) runCompleteManualTodos(ids []string, note string) (model, tea.Cmd) {
	m.todoCtrlBusy = true
	svc := m.svc
	return m, func() tea.Msg {
		err := svc.CompleteManualTodos(ids, note, "")
		return completeManualTodosResultMsg{todoIDs: ids, note: note, err: err}
	}
}

func (m model) handleAddFindingTodosResult(msg addFindingTodosResultMsg) (model, tea.Cmd) {
	m.todoCtrlBusy = false
	m = m.clearTodoControl()
	if msg.err != nil {
		m = m.appendChatControlTurn("/hero-add-todo", formatAddTodoFailure(msg.err))
		return m, m.refreshCmd()
	}
	body := formatAddTodoSuccess(m.svc, msg.result)
	m = m.appendChatControlTurn("/hero-add-todo", body)
	return m, m.refreshCmd()
}

func (m model) handleCompleteManualTodosResult(msg completeManualTodosResultMsg) (model, tea.Cmd) {
	m.todoCtrlBusy = false
	m = m.clearTodoControl()
	if msg.err != nil {
		m = m.appendChatControlTurn("/hero-complete-todo", formatCompleteTodoFailure(m.svc, msg.todoIDs, msg.err))
		return m, m.refreshCmd()
	}
	body := formatCompleteTodoSuccess(m.svc, msg.todoIDs)
	m = m.appendChatControlTurn("/hero-complete-todo", body)
	return m, m.refreshCmd()
}

func formatAddTodoFailure(err error) string {
	if err == nil {
		return ""
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "projection") || strings.Contains(lower, "current-state") {
		return addTodoProjectionErrMsg
	}
	return "✗ " + err.Error()
}

func formatAddTodoSuccess(svc *cycle.Service, res cycle.AddFindingTodosResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "✓ Added %d finding(s) to project ToDos: %s\n", len(res.TodoIDs), strings.Join(res.TodoIDs, ", "))
	if res.CycleCompleted {
		c, err := svc.Store.GetActiveCycle()
		if err == nil {
			b.WriteString(fmt.Sprintf("✓ Cycle C%d completed with deferred ToDos\n", c.Number))
		} else {
			b.WriteString("✓ Cycle completed with deferred ToDos\n")
		}
		b.WriteString("  disposition: completed_with_deferred_todos\n")
		fmt.Fprintf(&b, "  deferred: %d · %s\n", len(res.TodoIDs), strings.Join(res.TodoIDs, ", "))
		return strings.TrimRight(b.String(), "\n")
	}
	if res.PartialDeferral {
		remaining, _ := svc.ListActionableFindings("")
		fmt.Fprintf(&b, "⚠ Implementation remains Escalated with %d open finding(s).\n", len(remaining))
		b.WriteString("  Use /hero-continue [N], /hero-add-todo, /hero-cancel, or /hero-finish.")
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatCompleteTodoFailure(svc *cycle.Service, ids []string, err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.Contains(msg, "adopted by the active cycle") && len(ids) > 0 {
		c, cErr := svc.Store.GetActiveCycle()
		if cErr == nil {
			return fmt.Sprintf("✗ %s is adopted by active cycle C%d.\n  Complete that cycle's validation or cancel it to return the ToDo to pending.", ids[0], c.Number)
		}
	}
	if strings.Contains(msg, "forbidden token") {
		return completeTodoSecretWarn
	}
	if errors.Is(err, store.ErrTodoResolutionNoteConflict) {
		return "✗ A different resolution note was already recorded for this ToDo."
	}
	return "✗ " + msg
}

func formatCompleteTodoSuccess(svc *cycle.Service, ids []string) string {
	var b strings.Builder
	for _, id := range ids {
		t, err := svc.Store.GetTodo(id)
		if err != nil {
			fmt.Fprintf(&b, "✓ ToDo resolved: %s\n", id)
			continue
		}
		if t.ResolvedAt != "" {
			fmt.Fprintf(&b, "✓ ToDo resolved: %s\n  Removed from Pending; audit retained in Events.\n  (resolved %s)\n", id, t.ResolvedAt)
		} else {
			fmt.Fprintf(&b, "✓ ToDo resolved: %s\n  Removed from Pending; audit retained in Events.\n", id)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m model) beginHeroCompleteTodo(preselect []string) (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	if m.svc == nil {
		m = m.setStatusResult(false, "/hero-complete-todo", "cycle service unavailable")
		return m, nil
	}
	// complete-todo is project-scoped; an active cycle is not required.
	if len(preselect) > 0 {
		if errMsg := validateCompleteTodoPreselect(m.svc, preselect); errMsg != "" {
			m, _ = m.enterConversation()
			m.convError = errMsg
			return m, nil
		}
		m, _ = m.enterConversation()
		m = m.clearTodoControl()
		m.todoCtrlTodoIDs = preselect
		m.todoPhase = todoPhaseCompleteNote
		m.chatInputFocused = true
		m.convError = ""
		return m, nil
	}
	pending, err := m.svc.ListPendingTodos()
	if err != nil {
		m = m.setStatusResult(false, "/hero-complete-todo", err.Error())
		return m, nil
	}
	if len(pending) == 0 {
		m, _ = m.enterConversation()
		m.convError = "✗ No pending project ToDos."
		return m, nil
	}
	ids := make([]string, 0, len(pending))
	for _, t := range pending {
		ids = append(ids, t.ID)
	}
	m, _ = m.enterConversation()
	m = m.clearTodoControl()
	m.todoPhase = todoPhaseCompleteSelect
	m.todoCtrlTodoIDs = ids
	m.todoCtrlSelected = make(map[string]bool)
	m.todoCtrlCursor = 0
	m.convError = ""
	m.chatInputFocused = false
	return m, nil
}

func validateCompleteTodoPreselect(svc *cycle.Service, ids []string) string {
	var activeCycleID int64
	var activeCycleNum int
	if c, err := svc.Store.GetActiveCycle(); err == nil {
		activeCycleID = c.ID
		activeCycleNum = c.Number
	} else if !errors.Is(err, store.ErrNoActiveCycle) {
		return err.Error()
	}
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		t, err := svc.Store.GetTodo(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				// Allow legacy Pending Features prose; CompleteManualTodos promotes on submit.
				continue
			}
			return err.Error()
		}
		if t.Status == store.TodoStatusAdopted {
			if activeCycleID != 0 && t.AdoptedCycleID == activeCycleID {
				return fmt.Sprintf("✗ %s is adopted by active cycle C%d.\n  Complete that cycle's validation or cancel it to return the ToDo to pending.", id, activeCycleNum)
			}
			return fmt.Sprintf("✗ %s is adopted and cannot be completed manually.", id)
		}
		if t.Status == store.TodoStatusResolved {
			if resolved := strings.TrimSpace(t.ResolvedAt); resolved != "" {
				return fmt.Sprintf("✓ ToDo %s was already resolved at %s.", id, resolved)
			}
			return fmt.Sprintf("✓ ToDo %s was already resolved.", id)
		}
		if t.Status != store.TodoStatusPending {
			return fmt.Sprintf("✗ %s is %s; only pending ToDos can be completed manually.", id, t.Status)
		}
	}
	return ""
}

func (m model) listOpenFindingsForFinish() ([]store.Finding, error) {
	if m.svc == nil || m.svc.Store == nil {
		return nil, errors.New("cycle service unavailable")
	}
	c, err := m.svc.Store.GetActiveCycle()
	if err != nil {
		return nil, err
	}
	return m.svc.Store.ListFindingsByStatus(c.ID, store.FindingStatusOpen, store.FindingStatusReopened)
}

func (m model) beginHeroFinishWarning(findings []store.Finding) (model, tea.Cmd) {
	ids := make([]string, 0, len(findings))
	for _, f := range findings {
		ids = append(ids, f.ID)
	}
	m, _ = m.enterConversation()
	m = m.clearTodoControl()
	m.todoPhase = todoPhaseFinishTypeFinish
	m.todoCtrlFinishIDs = ids
	m = m.clearChatInput()
	m.chatInputFocused = true
	m.convError = ""
	return m, nil
}

func (m model) appendChatControlTurn(command, body string) model {
	m.transcript = append(m.transcript, convMessage{role: convRoleUser, content: command})
	m.transcript = append(m.transcript, convMessage{role: convRoleAgent, content: body})
	m.transcriptFollowBottom = true
	m.convError = ""
	return m
}

func (m model) renderTodoControlModal() string {
	if !m.todoControlActive() || m.todoControlUsesComposer() {
		return ""
	}
	var body strings.Builder
	switch m.todoPhase {
	case todoPhaseAddSelect:
		body.WriteString("Defer findings to project ToDos\n\n")
		for i, f := range m.todoCtrlFindings {
			mark := "[ ]"
			if m.todoCtrlSelected[f.ID] {
				mark = "[x]"
			}
			prefix := "  "
			if i == m.todoCtrlCursor {
				prefix = "▸ "
			}
			fmt.Fprintf(&body, "%s%s %s · %s · %s\n", prefix, mark, f.ID, findingOwnerLabel(f.Owner), oneLineIssue(f.Issue))
		}
		body.WriteString("\nspace toggle   a select all   n select none   enter review   esc cancel")
	case todoPhaseAddReviewPartial:
		selected := m.selectedFindingIDs()
		var remain []string
		for _, f := range m.todoCtrlFindings {
			if !m.todoCtrlSelected[f.ID] {
				remain = append(remain, f.ID)
			}
		}
		body.WriteString("Review escalation decision\n\n")
		fmt.Fprintf(&body, "Move to ToDos (%d)\n", len(selected))
		for _, f := range m.todoCtrlFindings {
			if m.todoCtrlSelected[f.ID] {
				fmt.Fprintf(&body, "  %s · %s\n", f.ID, oneLineIssue(f.Issue))
			}
		}
		fmt.Fprintf(&body, "\nResolve in this cycle (%d)\n  %s\n\n", len(remain), strings.Join(remain, ", "))
		body.WriteString("The stage will remain Escalated.\nRun /hero-continue after this operation to grant more iterations.\n\nenter confirm   esc back")
	case todoPhaseAddReviewDeferAll:
		selected := m.selectedFindingIDs()
		body.WriteString("Complete cycle with deferred ToDos?\n\n")
		fmt.Fprintf(&body, "Move to ToDos (%d)\n  %s\n\n", len(selected), strings.Join(selected, ", "))
		body.WriteString("No OpenSpec tasks or other findings will remain.\nThe cycle will close now. No empty Implementation wave will run.\nRemaining enabled validation stages will not run.\n\ntype DEFER to confirm   esc cancel")
	case todoPhaseCompleteSelect:
		body.WriteString("Complete project ToDos manually\n\nPending\n")
		for i, id := range m.todoCtrlTodoIDs {
			t, _ := m.svc.Store.GetTodo(id)
			summary := ""
			if t.ID != "" {
				summary = oneLineIssue(t.Summary)
			}
			mark := "[ ]"
			if m.todoCtrlSelected[id] {
				mark = "[x]"
			}
			prefix := "  "
			if i == m.todoCtrlCursor {
				prefix = "▸ "
			}
			fmt.Fprintf(&body, "%s%s %s · %s\n", prefix, mark, id, summary)
		}
		body.WriteString("\nspace toggle   enter continue   esc cancel")
	case todoPhaseCompleteReview:
		body.WriteString(fmt.Sprintf("Mark %d ToDo(s) resolved?\n  %s\n  note: %s\n\nenter confirm   esc back", len(m.todoCtrlTodoIDs), strings.Join(m.todoCtrlTodoIDs, ", "), m.todoCtrlNote))
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBlue).
		Padding(1, 2).
		Width(minInt(88, max(40, m.width-8))).
		Render(body.String())
	if m.todoCtrlBusy {
		busy := lipgloss.NewStyle().Foreground(colorAccentFast).Render("Reconciling project ToDos and current-state.md…")
		box += "\n" + busy
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box, lipgloss.WithWhitespaceBackground(colorBgSurface))
}

func (m model) renderTodoControlComposerHint() string {
	if !m.todoControlUsesComposer() {
		return ""
	}
	var hint strings.Builder
	switch m.todoPhase {
	case todoPhaseCompleteNote:
		hint.WriteString("Complete project ToDos manually\n\nSelected\n")
		for _, id := range m.todoCtrlTodoIDs {
			t, _ := m.svc.Store.GetTodo(id)
			fmt.Fprintf(&hint, "  %s · %s\n", id, oneLineIssue(t.Summary))
		}
		hint.WriteString("\nResolution note (required):\nenter review   esc cancel")
	case todoPhaseAddTypeDefer:
		hint.WriteString("Complete cycle with deferred ToDos?\n\n")
		fmt.Fprintf(&hint, "type %s to confirm   esc cancel", typedDeferConfirm)
	case todoPhaseFinishTypeFinish:
		n := len(m.todoCtrlFinishIDs)
		hint.WriteString(fmt.Sprintf("Emergency finish with %d open finding(s)?\n\n", n))
		hint.WriteString(formatFinishFindingList(m.todoCtrlFinishIDs))
		hint.WriteString("\n\nwill NOT be added to project ToDos.\nUse /hero-add-todo if you want to preserve them as deferred work.\n\n")
		fmt.Fprintf(&hint, "type %s to confirm   esc cancel", typedFinishConfirm)
	}
	return hint.String()
}

func formatFinishFindingList(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	if len(ids) <= 3 {
		return strings.Join(ids, " and ") + " will NOT be added to project ToDos."
	}
	return strings.Join(ids[:3], ", ") + fmt.Sprintf(" and %d more will NOT be added to project ToDos.", len(ids)-3)
}
