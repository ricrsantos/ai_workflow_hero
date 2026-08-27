package tui

import (
	"strings"
	"testing"
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
	if !strings.Contains(plain, "alt+1-5") {
		t.Fatalf("expected five-screen shortcut label: %q", plain)
	}
	if strings.Contains(plain, "alt+n") {
		t.Fatalf("legacy alt+n label must not be rendered: %q", plain)
	}
}

func TestNavSidebarShowsConfigLastWithSixShortcuts(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 100)
	m = SetHeight(m, 24)
	m.status.CycleNumber = 1
	plain := stripANSI(ViewForTest(m))

	labels := []string{"Chat", "Status", "Artifacts", "Costs", "Events", "Config"}
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
	if !strings.Contains(plain, "alt+1-6") {
		t.Fatalf("expected six-screen shortcut label: %q", plain)
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
