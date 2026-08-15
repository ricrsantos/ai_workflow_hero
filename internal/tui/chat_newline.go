package tui

// isComposerNewlineKey is true for keys that insert a line break in the Chat
// composer without submitting. Enter remains send.
//
// Alt+Enter is the only newline binding: Cursor's xterm.js sends ESC+CR,
// which Bubble Tea v1 reports as alt+enter. It does not conflict with
// alt+1–5 screen jumps. Do not enable the Kitty keyboard protocol on
// Bubble Tea v1 (ordinary keys become unknown CSI and the composer freezes).
func isComposerNewlineKey(s string) bool {
	return s == "alt+enter"
}

func (m model) insertComposerNewline() model {
	prev := chatSlashToken(m.input)
	m = m.insertRunesAtCursor([]rune{'\n'})
	m = m.afterChatInputEdit(prev)
	return m.ensureInputCaretVisible()
}
