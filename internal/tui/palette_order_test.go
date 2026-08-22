package tui

import "testing"

func TestDefaultHeroPaletteOrder(t *testing.T) {
	items := defaultHeroPaletteItems()
	want := []string{
		"/new-chat",
		"/hero-model",
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
		"/hero-harness",
		"/harness-reset",
		"/hero-help",
		"Go to - Chat",
		"Go to - Status",
		"Go to - Artifacts",
		"Go to - Costs",
		"Go to - Events",
		"Refresh",
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
