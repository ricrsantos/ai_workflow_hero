package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
)

func TestNavSidebarListsScreensInOrder(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 100)
	m = SetHeight(m, 24)
	view := ViewForTest(m)

	if !strings.Contains(view, "AI Hero") {
		t.Fatalf("expected AI Hero title in sidebar: %q", view)
	}
	chat := strings.Index(view, "Chat")
	status := strings.Index(view, "Status")
	artifacts := strings.Index(view, "Artifacts")
	costs := strings.Index(view, "Costs")
	events := strings.Index(view, "Events")
	if chat < 0 || status < 0 || artifacts < 0 || costs < 0 || events < 0 {
		t.Fatalf("missing nav labels in view: %q", view)
	}
	if !(chat < status && status < artifacts && artifacts < costs && costs < events) {
		t.Fatalf("expected Chat→Status→Artifacts→Costs→Events order: %q", view)
	}
	plain := stripANSI(view)
	if !strings.Contains(plain, "> Chat") {
		t.Fatalf("expected active Chat marker: %q", view)
	}
	if !strings.Contains(plain, "alt+1-6") {
		t.Fatalf("expected six-screen shortcut label: %q", plain)
	}
	if strings.Contains(plain, "alt+n") {
		t.Fatalf("legacy alt+n label must not be rendered: %q", plain)
	}
}

func TestNavSidebarShowsSettingsBeforeConfigWithSevenShortcuts(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 100)
	m = SetHeight(m, 24)
	m.status.CycleNumber = 1
	plain := stripANSI(ViewForTest(m))

	labels := []string{"Chat", "Status", "Artifacts", "Costs", "Events", "Settings", "Config"}
	previous := -1
	for _, label := range labels {
		index := strings.Index(plain, label)
		if index < 0 {
			t.Fatalf("missing %q in sidebar: %q", label, plain)
		}
		if index <= previous {
			t.Fatalf("expected %q after previous nav item: %q", label, plain)
		}
		previous = index
	}
	if !strings.Contains(plain, "alt+1-7") {
		t.Fatalf("expected seven-screen shortcut label: %q", plain)
	}
	if strings.Contains(plain, "alt+n") {
		t.Fatalf("legacy alt+n label must not be rendered: %q", plain)
	}
}

func TestNavSidebarHiddenWhenNarrow(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 70)
	m = SetHeight(m, 24)
	view := ViewForTest(m)
	plain := stripANSI(view)
	// Title only lived in the sidebar; narrow mode drops the left rail.
	if strings.Contains(plain, "AI Hero") && strings.Contains(plain, "> Chat") {
		t.Fatalf("sidebar should be hidden below %d cols: %q", navSidebarMinWidth, view)
	}
}

func TestNavSidebarHighlightsStatus(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 100)
	m = SetHeight(m, 24)
	m = SetScreen(m, ScreenStatus)
	plain := stripANSI(ViewForTest(m))
	if !strings.Contains(plain, "> Status") {
		t.Fatalf("expected active Status marker: %q", plain)
	}
	if strings.Contains(plain, "> Chat") {
		t.Fatalf("Chat must not stay active on Status screen: %q", plain)
	}
}

func TestTabFocusesNavbarAndEnterOpensHighlightedScreen(t *testing.T) {
	forceColorProfile(t, termenv.ANSI)
	m := NewTestModel(nil)
	m = SetWidth(m, 100)
	m = SetHeight(m, 24)

	next, _ := HandleTestKey(m, "tab")
	if next.shellFocus != shellFocusNavbar {
		t.Fatal("Tab should move focus from content to navbar")
	}
	if hints := next.footerHints(); !strings.Contains(hints, "↑↓ navbar") || !strings.Contains(hints, "enter open") {
		t.Fatalf("navbar help is incomplete: %q", hints)
	}
	if ChatInputFocusedForTest(next) {
		t.Fatal("chat composer must lose focus while navbar is focused")
	}

	next, _ = HandleTestKey(next, "down")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatal("arrow navigation must not open a screen before Enter")
	}
	lines := next.navSidebarNavigationLines(navSidebarWidth - navSidebarBoxStyle.GetHorizontalFrameSize())
	focusedStatus := navSidebarFocusedStyle.Render(truncateNavText("  Status", navSidebarWidth-navSidebarBoxStyle.GetHorizontalFrameSize()))
	if !containsLine(lines, focusedStatus) {
		t.Fatalf("Status row is not rendered with the focused background: %q", lines)
	}
	plain := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(plain, "> Chat") || strings.Contains(plain, "> Status") {
		t.Fatalf("active marker must remain on Chat before Enter: %q", plain)
	}

	next, _ = HandleTestKey(next, "enter")
	if CurrentScreen(next) != ScreenStatus {
		t.Fatalf("Enter opened %v, want Status", CurrentScreen(next))
	}
	if next.shellFocus != shellFocusNavbar {
		t.Fatal("navbar should retain focus after opening a screen")
	}
	if plain := stripANSI(ViewForTest(next)); !strings.Contains(plain, "> Status") {
		t.Fatalf("active marker did not move to Status: %q", plain)
	}

	next, _ = HandleTestKey(next, "tab")
	if next.shellFocus != shellFocusContent {
		t.Fatal("second Tab should return focus to screen content")
	}
}

func TestNavbarArrowNavigationWraps(t *testing.T) {
	m := SetWidth(NewTestModel(nil), 100)
	m, _ = HandleTestKey(m, "tab")
	m, _ = HandleTestKey(m, "up")

	if got, want := m.navCursor, len(m.visibleNavScreens())-1; got != want {
		t.Fatalf("nav cursor=%d want %d", got, want)
	}
}

func TestTabDoesNotFocusHiddenNavbar(t *testing.T) {
	m := SetWidth(NewTestModel(nil), navSidebarMinWidth-1)
	next, _ := HandleTestKey(m, "tab")

	if next.shellFocus != shellFocusContent {
		t.Fatal("hidden navbar must not receive focus")
	}
	if !ChatInputFocusedForTest(next) {
		t.Fatal("chat composer should keep focus when navbar is hidden")
	}
}

func TestResizeReturnsFocusWhenNavbarBecomesHidden(t *testing.T) {
	m := SetWidth(NewTestModel(nil), 100)
	m, _ = HandleTestKey(m, "tab")

	nextModel, _ := m.Update(tea.WindowSizeMsg{Width: navSidebarMinWidth - 1, Height: 24})
	next := nextModel.(model)
	if next.shellFocus != shellFocusContent {
		t.Fatal("content must regain focus when a resize hides the navbar")
	}
	if !ChatInputFocusedForTest(next) {
		t.Fatal("chat composer should regain focus after the navbar is hidden")
	}
}

func TestControlNavigationAliasIsRemoved(t *testing.T) {
	m := NewTestModel(nil)
	next, cmd := HandleTestKeyMsg(m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil || CurrentScreen(next) != ScreenConversation {
		t.Fatal("Ctrl+C must not trigger a TUI action")
	}
	next, _ = HandleTestKeyMsg(next, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if CurrentScreen(next) != ScreenConversation {
		t.Fatal("a non-Alt digit must not navigate")
	}
}

func containsLine(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}

func TestNavSidebarShowsAgentsSection(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 100)
	m = SetHeight(m, 24)
	m = EnterConversationForTest(m)
	m.liveAgents = []liveAgent{
		{Name: "orchestration_agent", Label: "ORCH"},
		{Name: "backend_agent", Label: "BACK"},
	}
	plain := stripANSI(ViewForTest(m))
	if !strings.Contains(plain, "agents: 2") {
		t.Fatalf("expected agents count in sidebar: %q", plain)
	}
	if !strings.Contains(plain, "ORCH") {
		t.Fatalf("expected agent labels in sidebar: %q", plain)
	}
	hero := strings.Index(plain, "AI Hero")
	agents := strings.Index(plain, "agents: 2")
	chatNav := strings.Index(plain, "> Chat")
	if hero < 0 || agents < 0 || chatNav < 0 || !(hero < agents && agents < chatNav) {
		t.Fatalf("expected AI Hero → agents → nav order: %q", plain)
	}
	// Separators (dim rules) between title/agents and agents/nav.
	if strings.Count(plain, strings.Repeat("─", 10)) < 2 {
		t.Fatalf("expected separator rules in sidebar: %q", plain)
	}
}

func TestNavSidebarAgentsHiddenFromChatHeader(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 100)
	m = SetHeight(m, 24)
	m = EnterConversationForTest(m)
	view := ViewForTest(m)
	plain := stripANSI(view)
	// Agents live in the sidebar box, not as a separate chat-header box.
	if strings.Count(plain, "agents: 0") != 1 {
		t.Fatalf("agents summary should appear once in sidebar: %q", plain)
	}
	if strings.Contains(plain, "Chat · harness") {
		t.Fatalf("chat harness header removed when sidebar visible: %q", plain)
	}
}

func TestNavSidebarHidesConfigAfterCycleArchived(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 100)
	m = SetHeight(m, 24)
	m.status.CycleNumber = 1
	m.screen = screenConfig
	m.config.dirty = true

	next, _ := m.Update(RefreshDataForTest(cycle.StatusView{CycleNumber: 0}))
	m = next.(model)
	plain := stripANSI(ViewForTest(m))

	if strings.Contains(plain, "Config") {
		t.Fatalf("Config nav item should be hidden after archive refresh: %q", plain)
	}
	if !strings.Contains(plain, "alt+1-6") {
		t.Fatalf("expected six-screen shortcut label after archive: %q", plain)
	}
	if strings.Contains(plain, "alt+1-7") {
		t.Fatalf("seven-screen label must not appear without active cycle: %q", plain)
	}
	if CurrentScreen(m) != ScreenConversation {
		t.Fatalf("screen=%v want conversation after archive", CurrentScreen(m))
	}
}

func TestSyncActiveCycleChromeClearsConfigState(t *testing.T) {
	m := NewTestModel(nil)
	m.status.CycleNumber = 0
	m.screen = screenConfig
	m.config.dirty = true
	m.config.editing = true

	m = SyncActiveCycleChromeForTest(m)
	if m.config.dirty || m.config.editing {
		t.Fatalf("config state should reset: dirty=%t editing=%t", m.config.dirty, m.config.editing)
	}
	if CurrentScreen(m) != ScreenConversation {
		t.Fatalf("screen=%v want conversation", CurrentScreen(m))
	}
}
