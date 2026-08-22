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
	if !strings.Contains(stripANSI(view), "> Chat") {
		t.Fatalf("expected active Chat marker: %q", view)
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
