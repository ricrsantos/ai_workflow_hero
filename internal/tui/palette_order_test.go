package tui

import "testing"

func TestDefaultHeroPaletteOrder(t *testing.T) {
	items := defaultHeroPaletteItems()
	want := []string{
		"/new-chat",
		"/model",
		"/hero-new",
		"/hero-start",
		"/hero-approve",
		"/hero-continue",
		"/hero-reject",
		"/hero-cancel",
		"/hero-back",
		"/hero-resume",
		"/hero-finish",
		"/hero-archive",
		"/hero-status",
		"/hero-cycles",
		"/hero-todos",
		"/hero-sync",
		"/hero-config-update",
		"/harness",
		"/harness-reset",
		"/hero-help",
		"/hero-refresh",
		"Go to - Chat",
		"Go to - Status",
		"Go to - Artifacts",
		"Go to - Costs",
		"Go to - Events",
		"Quit",
	}
	if len(items) != len(want) {
		t.Fatalf("len=%d want %d", len(items), len(want))
	}
	for i, label := range want {
		if items[i].label != label {
			t.Fatalf("index %d: got %q want %q", i, items[i].label, label)
		}
	}
}

func TestFreeChatPaletteFilter(t *testing.T) {
	items := filterFreeChatPaletteItems(defaultHeroPaletteItems())
	want := []string{"/new-chat", "/model", "/harness", "/harness-reset", "Quit"}
	if len(items) != len(want) {
		t.Fatalf("len=%d want %d labels=%v", len(items), len(want), labelsOf(items))
	}
	for i, label := range want {
		if items[i].label != label {
			t.Fatalf("index %d: got %q want %q", i, items[i].label, label)
		}
	}
}

func labelsOf(items []paletteItem) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.label
	}
	return out
}
