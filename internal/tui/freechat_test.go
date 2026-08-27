package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

func TestEnsureFreeChatHomeCreatesHeroJSON(t *testing.T) {
	home := t.TempDir()
	if err := ensureFreeChatHome(home); err != nil {
		t.Fatalf("ensureFreeChatHome: %v", err)
	}
	path := filepath.Join(home, cursoradapter.HeroJSONPath)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("hero.json missing: %v", err)
	}
	hero, err := install.LoadHeroJSON(home)
	if err != nil {
		t.Fatalf("LoadHeroJSON: %v", err)
	}
	if !install.IsHarnessEnabled(hero, "cursor") {
		t.Fatal("cursor must be enabled by default")
	}
}

func TestEnsureFreeChatHomeIdempotent(t *testing.T) {
	home := t.TempDir()
	if err := ensureFreeChatHome(home); err != nil {
		t.Fatal(err)
	}
	if err := ensureFreeChatHome(home); err != nil {
		t.Fatalf("second ensure: %v", err)
	}
}

func TestFreeChatModeHidesEtapaHintAndNav(t *testing.T) {
	m := NewTestModel(nil)
	m.freeChatMode = true
	m = m.reloadPaletteItems()
	m.width = 100
	m.height = 40

	if hint := m.conversationStatusHint(); hint != "" {
		t.Fatalf("free chat must hide etapa hint, got %q", hint)
	}

	labels := map[string]bool{}
	for _, item := range PaletteItemsForTest(m) {
		labels[item.Label] = true
		if strings.HasPrefix(item.Label, "/hero") {
			t.Fatalf("free chat palette must not include %q", item.Label)
		}
		if strings.HasPrefix(item.Label, "Go to -") {
			t.Fatalf("free chat palette must not include %q", item.Label)
		}
	}
	for _, want := range []string{"/new-chat", "/model", "/harness", "/harness-reset", "Quit"} {
		if !labels[want] {
			t.Fatalf("missing free-chat label %q", want)
		}
	}

	view := ViewForTest(m)
	if strings.Contains(view, "  Status") || strings.Contains(view, "> Status") {
		t.Fatalf("nav should only show Chat in free chat:\n%s", view)
	}
	if strings.Contains(view, "Artifacts") {
		t.Fatalf("nav should not show Artifacts in free chat:\n%s", view)
	}
	if !strings.Contains(stripANSI(view), "alt+1-5") {
		t.Fatalf("free chat navbar should use the numbered range label:\n%s", view)
	}

	next, _ := HandleTestKey(m, "alt+2")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("alt+2 must stay on Chat in free chat, got %v", CurrentScreen(next))
	}
}

func TestFreeChatBlocksHeroSlash(t *testing.T) {
	m := NewTestModel(nil)
	m.freeChatMode = true
	next, _, ok := m.dispatchExactHeroSlash("/hero-new")
	if !ok {
		t.Fatal("expected free-chat block for /hero-new")
	}
	if !strings.Contains(StatusTextForTest(next), "not available in free chat") {
		t.Fatalf("status=%q", StatusTextForTest(next))
	}
}
