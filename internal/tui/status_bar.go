package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type statusKind int

const (
	statusIdle statusKind = iota
	statusRunning
	statusOK
	statusErr
	// statusWarn is the C5 yellow warning state (missing catalog, stale cache,
	// invalidated values) — distinct from red execution errors (UI-C05-001 §5).
	statusWarn
)

// Fixed chrome for the footer status area. A altura normal é 2 linhas e
// expande automaticamente até 6 para perguntas/permissões longas (com quebra
// automática e scroll via Alt+↑↓), voltando ao normal ao concluir/cancelar.
const (
	statusBarMinLines = 2
	statusBarMaxLines = 6
	// statusBarMinContentReserve preserva linhas mínimas para o conteúdo/chat
	// ao calcular o teto da status bar a partir da altura da janela.
	statusBarMinContentReserve = 4
)

func (m model) closePalette() model {
	wasPicking := m.pickingModel || m.pickingHarness || m.pickingClaudeContext || m.pickingHarnessReset || m.harnessResetAwaitingOpen || m.pickingProps
	m.screen = m.prevScreen
	m.paletteFilter = ""
	m.paletteIndex = 0
	m.paletteOffset = 0
	if wasPicking {
		m.pickingModel = false
		m.pickingHarness = false
		m.pickingClaudeContext = false
		m.pickingHarnessReset = false
		m.harnessResetAwaitingOpen = false
		m.modelPickerHarness = ""
		m.harnessDraft = nil
		m.harnessPermissionDraft = nil
		m.pickingProps = false
		m.propsValueList = false
		m.propsValueKey = ""
		m = m.reloadPaletteItems()
	}
	if m.screen == screenConversation {
		m.chatInputFocused = true
	}
	return m
}

// openPaletteOverlay shows the palette without losing the screen underneath when
// already on a palette layer (e.g. /hero-model opened from the slash menu).
func (m model) openPaletteOverlay() model {
	if m.screen != screenPalette {
		m.prevScreen = m.screen
		m.screen = screenPalette
	}
	return m
}

func (m model) setStatusRunning(label string) model {
	m.statusKind = statusRunning
	m.statusLabel = label
	m.statusText = "running…"
	m.statusScrollOffset = 0
	m.actionBusy = true
	return m
}

func (m model) clearStatus() model {
	m.statusKind = statusIdle
	m.statusLabel = ""
	m.statusText = ""
	m.statusScrollOffset = 0
	m.actionBusy = false
	return m
}

func (m model) setStatusResult(ok bool, label, text string) model {
	m.actionBusy = false
	if label != "" {
		m.statusLabel = label
	}
	m.statusText = strings.TrimSpace(text)
	m.statusScrollOffset = 0
	if ok {
		m.statusKind = statusOK
	} else {
		m.statusKind = statusErr
	}
	return m
}

func busyExecuteCompletedText(label string) string {
	if strings.TrimSpace(label) == "/hero-start" {
		return "orchestration turn completed"
	}
	return "turn completed"
}

// completeBusyExecuteStatus clears a running palette/slash action after Execute
// finishes. Follow-up chat turns leave the footer unchanged when actionBusy is
// already false. /hero-start handoff skips this so discover keeps the timer.
func (m model) completeBusyExecuteStatus(ok bool, text string) model {
	if !m.actionBusy {
		return m
	}
	label := strings.TrimSpace(m.statusLabel)
	if label == "" {
		label = "execute"
	}
	return m.setStatusResult(ok, label, text)
}

// setStatusWarning shows a yellow warning in the footer status area (UI §2.1).
func (m model) setStatusWarning(label, text string) model {
	m.actionBusy = false
	if label != "" {
		m.statusLabel = label
	}
	m.statusText = strings.TrimSpace(text)
	m.statusScrollOffset = 0
	m.statusKind = statusWarn
	return m
}

func (m model) setStatusBusyBlocked() model {
	busy := m.statusLabel
	if busy == "" {
		busy = "previous action"
	}
	m.statusKind = statusErr
	m.statusText = fmt.Sprintf("busy — wait for %s to finish", busy)
	return m
}

func (m model) statusBarWidth() int {
	width := m.width
	if width < 20 {
		width = 20
	}
	return width
}

// statusBarMaxVisibleLines é o teto dinâmico (até 6) considerando a altura da
// janela: reserva 1 linha para a borda + footer + conteúdo mínimo.
func (m model) statusBarMaxVisibleLines() int {
	maxLines := statusBarMaxLines
	if m.height > 0 {
		maxAllowed := m.height - (1 + m.footerLineCount() + statusBarMinContentReserve)
		if maxAllowed < statusBarMinLines {
			maxAllowed = statusBarMinLines
		}
		if maxAllowed < maxLines {
			maxLines = maxAllowed
		}
	}
	if maxLines < statusBarMinLines {
		maxLines = statusBarMinLines
	}
	return maxLines
}

func (m model) statusBarLineCount() int {
	return len(m.statusBarDisplayLines(m.statusBarWidth()))
}

func (m model) renderStatusBar() string {
	width := m.statusBarWidth()
	lines := m.statusBarDisplayLines(width)
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	return b.String()
}

func (m model) statusBarDisplayLines(width int) []string {
	if width < 1 {
		width = 1
	}
	m = m.clampStatusScroll(width)
	all := m.statusBarAllLines(width)
	total := len(all)
	desired := total
	if desired < statusBarMinLines {
		desired = statusBarMinLines
	}
	if maxLines := m.statusBarMaxVisibleLines(); desired > maxLines {
		desired = maxLines
	}
	offset := m.statusScrollOffset
	if offset < 0 {
		offset = 0
	}
	if maxOff := total - desired; maxOff < 0 {
		offset = 0
	} else if offset > maxOff {
		offset = maxOff
	}
	visible := make([]string, 0, desired)
	for i := 0; i < desired; i++ {
		idx := offset + i
		if idx < total {
			visible = append(visible, all[idx])
		} else {
			visible = append(visible, "")
		}
	}
	return withStatusScrollHint(visible, offset, total, width)
}

func (m model) statusBarAllLines(width int) []string {
	if m.harnessPermissionPending {
		return wrapStatusPlain(m.harnessPermissionMsg, warnStyle, width)
	}
	if m.harnessQuestionPending {
		return wrapStatusPlain(m.harnessQuestionMsg, warnStyle, width)
	}
	if m.confirmPending {
		return wrapStatusPlain(m.confirmMsg, warnStyle, width)
	}
	switch m.statusKind {
	case statusRunning:
		label := m.statusLabel
		if label == "" {
			label = "action"
		}
		head := fmt.Sprintf("● %s  running", label)
		return []string{infoStyle.Render(head)}
	case statusOK:
		return wrapStatusMessageAll("✓", m.statusLabel, m.statusText, successStyle, width)
	case statusErr:
		return wrapStatusMessageAll("✗", m.statusLabel, m.statusText, errorStyle, width)
	case statusWarn:
		return wrapStatusMessageAll("⚠", m.statusLabel, m.statusText, warnStyle, width)
	default:
		lines := []string{mutedStyle.Render("ready")}
		if hint := m.conversationStatusHint(); hint != "" {
			lines = append(lines, wrapStatusPlain(hint, mutedStyle, width)...)
		}
		return lines
	}
}

// withStatusScrollHint indica scroll pendente sem consumir linha extra: o
// sufixo "[▲a ▼b Alt+↑↓]" é acomodado na última linha visível.
func withStatusScrollHint(visible []string, offset, total, width int) []string {
	above := offset
	below := total - (offset + len(visible))
	// Conta apenas linhas reais além da janela; padding vazio não gera hint.
	if above < 0 {
		above = 0
	}
	if below < 0 {
		below = 0
	}
	if above == 0 && below == 0 || len(visible) == 0 {
		return visible
	}
	var hint string
	switch {
	case above > 0 && below > 0:
		hint = fmt.Sprintf(" [▲%d ▼%d Alt+↑↓]", above, below)
	case above > 0:
		hint = fmt.Sprintf(" [▲%d Alt+↑]", above)
	default:
		hint = fmt.Sprintf(" [▼%d Alt+↓]", below)
	}
	last := len(visible) - 1
	if strings.TrimSpace(visible[last]) == "" {
		visible[last] = mutedStyle.Render(strings.TrimSpace(hint))
		return visible
	}
	visible[last] = appendStatusHint(visible[last], hint, width)
	return visible
}

func appendStatusHint(line, hint string, width int) string {
	if width < 1 {
		width = 1
	}
	if lipgloss.Width(line)+lipgloss.Width(hint) <= width {
		return line + mutedStyle.Render(hint)
	}
	budget := width - lipgloss.Width(hint)
	if budget < 1 {
		return truncateDisplayWidth(mutedStyle.Render(hint), width)
	}
	return truncateDisplayWidth(line, budget) + mutedStyle.Render(hint)
}

// conversationStatusHint is the second ready-line on the Chat screen (frees header space).
func (m model) conversationStatusHint() string {
	if m.screen != screenConversation || m.freeChatMode {
		return ""
	}
	if m.conversationStage == "" {
		return "No active stage — chatting with harness defaults. /hero-new for new cycle."
	}
	return fmt.Sprintf("Etapa: %s", m.conversationStage)
}

func wrapStatusMessage(icon, label, text string, style lipgloss.Style, width int) []string {
	return wrapStatusMessageAll(icon, label, text, style, width)
}

// wrapStatusMessageAll quebra automaticamente sem truncar: o scroll + teto de
// 6 linhas vivem em statusBarDisplayLines.
func wrapStatusMessageAll(icon, label, text string, style lipgloss.Style, width int) []string {
	prefix := icon + " "
	if label != "" {
		prefix += label + " — "
	}
	body := text
	if body == "" {
		body = "(no message)"
	}
	raw := prefix + body
	wrapped := splitOutputLines(raw, width)
	out := make([]string, len(wrapped))
	for i, line := range wrapped {
		out[i] = style.Render(line)
	}
	return out
}

// wrapStatusPlain quebra mensagens multlinha (pergunta/permissão/confirmação)
// preservando parágrafos e linhas vazias.
func wrapStatusPlain(text string, style lipgloss.Style, width int) []string {
	if strings.TrimSpace(text) == "" {
		return []string{style.Render("(no message)")}
	}
	wrapped := splitOutputLines(text, width)
	out := make([]string, len(wrapped))
	for i, line := range wrapped {
		if strings.TrimSpace(line) == "" {
			out[i] = ""
			continue
		}
		out[i] = style.Render(line)
	}
	return out
}

// statusScrollTotal conta as linhas reais (sem padding) para o clamp.
func (m model) statusScrollTotal(width int) int {
	return len(m.statusBarAllLines(width))
}

func (m model) clampStatusScroll(width int) model {
	if width < 1 {
		width = 1
	}
	total := len(m.statusBarAllLines(width))
	desired := total
	if desired < statusBarMinLines {
		desired = statusBarMinLines
	}
	if maxLines := m.statusBarMaxVisibleLines(); desired > maxLines {
		desired = maxLines
	}
	maxOff := total - desired
	if maxOff < 0 {
		maxOff = 0
	}
	if m.statusScrollOffset < 0 {
		m.statusScrollOffset = 0
	}
	if m.statusScrollOffset > maxOff {
		m.statusScrollOffset = maxOff
	}
	return m
}

func (m model) scrollStatus(delta int) model {
	m.statusScrollOffset += delta
	return m.clampStatusScroll(m.statusBarWidth())
}

// isStatusScrollKey indica as combinações Alt dedicadas à janela de status.
// ↑↓/PgUp/PgDn do transcript/composer permanecem intactos.
func isStatusScrollKey(s string) bool {
	switch s {
	case "alt+up", "alt+down", "alt+pgup", "alt+pgdown", "alt+home", "alt+end":
		return true
	default:
		return false
	}
}

// handleStatusScrollKey consome Alt+↑↓/PgUp/PgDn/Home/End quando a status bar
// tem o que rolar. Retorna handled=false se não há overflow.
func (m model) handleStatusScrollKey(s string) (model, bool) {
	if !isStatusScrollKey(s) {
		return m, false
	}
	width := m.statusBarWidth()
	total := m.statusScrollTotal(width)
	desired := total
	if desired < statusBarMinLines {
		desired = statusBarMinLines
	}
	if maxLines := m.statusBarMaxVisibleLines(); desired > maxLines {
		desired = maxLines
	}
	if total <= desired {
		return m, false
	}
	switch s {
	case "alt+up":
		m = m.scrollStatus(-1)
	case "alt+down":
		m = m.scrollStatus(1)
	case "alt+pgup":
		m = m.scrollStatus(-desired)
	case "alt+pgdown":
		m = m.scrollStatus(desired)
	case "alt+home":
		m.statusScrollOffset = 0
	case "alt+end":
		m.statusScrollOffset = total - desired
		m = m.clampStatusScroll(width)
	}
	return m, true
}

func statusResultOpensPanel(text string, width int) bool {
	return shouldOpenOutputPanel(text, width)
}

func firstStatusLine(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return strings.TrimSpace(text[:i])
	}
	return text
}
