package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestHelpOpensFromChatAndRestoresComposer(t *testing.T) {
	m := SetHeight(SetWidth(NewTestModel(nil), 70), 18)
	m = EnterConversationForTest(m)
	m.input = "unsent message"
	m.inputCursor = len([]rune(m.input))

	next, _ := m.Update(parseTestKey("f1"))
	if next.(model).helpOpen {
		t.Fatal("plain F1 should not open keyboard help")
	}
	next, _ = m.Update(parseTestKey("alt+f1"))
	opened := next.(model)
	if !opened.helpOpen || !strings.Contains(stripANSI(opened.View()), "KEYBOARD SHORTCUTS") {
		t.Fatal("Alt+F1 should open keyboard help")
	}
	for _, want := range []string{"Alt+Q", "Alt+M", "Alt+R / I", "Alt+A"} {
		if !strings.Contains(strings.Join(helpActions(opened.helpGroups()), " "), want) {
			t.Fatalf("help missing %q", want)
		}
	}

	next, _ = opened.Update(parseTestKey("esc"))
	closed := next.(model)
	if closed.helpOpen || closed.screen != screenConversation || closed.input != "unsent message" || closed.inputCursor != m.inputCursor {
		t.Fatal("closing help should restore the existing Chat state")
	}
	next, _ = closed.Update(parseTestKey("alt+f1"))
	next, _ = next.(model).Update(parseTestKey("alt+f1"))
	if next.(model).helpOpen {
		t.Fatal("Alt+F1 should also close keyboard help")
	}
}

func TestHelpScrollsAndFitsTerminal(t *testing.T) {
	m := SetHeight(SetWidth(NewTestModel(nil), 40), 12)
	next, _ := m.Update(parseTestKey("alt+f1"))
	m = next.(model)
	first := stripANSI(m.View())
	next, _ = m.Update(parseTestKey("pgdown"))
	m = next.(model)
	if m.helpOffset == 0 || stripANSI(m.View()) == first {
		t.Fatal("help should scroll by page")
	}
	for _, line := range strings.Split(stripANSI(m.View()), "\n") {
		if lipgloss.Width(line) > m.width {
			t.Fatalf("line exceeds width: %q", line)
		}
	}
	if got := len(strings.Split(stripANSI(m.View()), "\n")); got != m.height {
		t.Fatalf("help height=%d want %d", got, m.height)
	}
}

func TestHelpCommandOpensLocalHelpFromChatAndPalette(t *testing.T) {
	m := SetHeight(SetWidth(NewTestModel(nil), 80), 24)
	m = EnterConversationForTest(m)
	m.input = "/help"
	m.inputCursor = len([]rune(m.input))
	next, cmd := m.Update(parseTestKey("enter"))
	chat := next.(model)
	if cmd != nil || !chat.helpOpen || chat.screen != screenConversation || chat.input != "" {
		t.Fatalf("/help in Chat should open local help without a harness turn: help=%v screen=%v input=%q cmd=%v", chat.helpOpen, chat.screen, chat.input, cmd)
	}
	dismissed := m
	dismissed.slashOverlayDismissed = true
	next, cmd = dismissed.Update(parseTestKey("enter"))
	if got := next.(model); cmd != nil || !got.helpOpen || got.input != "" {
		t.Fatalf("/help should work with autocomplete dismissed: help=%v input=%q cmd=%v", got.helpOpen, got.input, cmd)
	}

	m = SetScreen(m, screenStatus)
	m.prevScreen = screenStatus
	m.screen = screenPalette
	for _, item := range m.paletteItems {
		if item.label == "/help" {
			m, cmd = m.runPaletteAction(item)
			if cmd != nil || !m.helpOpen || m.screen != screenStatus {
				t.Fatalf("/help in palette should overlay Status: help=%v screen=%v cmd=%v", m.helpOpen, m.screen, cmd)
			}
			return
		}
	}
	t.Fatal("/help missing from command palette")
}

func helpActions(groups []helpGroup) []string {
	var actions []string
	for _, group := range groups {
		for _, entry := range group.entries {
			actions = append(actions, entry.keys+" "+entry.action)
		}
	}
	return actions
}
