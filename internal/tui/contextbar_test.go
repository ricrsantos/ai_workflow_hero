package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var ansiEscapeRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestContextWindowLookup(t *testing.T) {
	cat := loadContextWindowCatalog("")
	if cat.lookup("composer-2.5") != 200000 {
		t.Fatalf("composer-2.5 window=%d", cat.lookup("composer-2.5"))
	}
	if cat.lookup("cursor-grok-4.6-high") != 500000 {
		t.Fatalf("exact high slug window=%d", cat.lookup("cursor-grok-4.6-high"))
	}
	if cat.lookup("gpt-5.3-codex-medium") != 400000 {
		t.Fatalf("suffix strip window=%d", cat.lookup("gpt-5.3-codex-medium"))
	}
	if cat.lookup("composer-2.5-thinking-max") != 200000 {
		t.Fatalf("combined property suffix strip window=%d", cat.lookup("composer-2.5-thinking-max"))
	}
	if cat.lookup("unknown-model") != 0 {
		t.Fatalf("unknown slug should be 0, got %d", cat.lookup("unknown-model"))
	}
	if cat.lookup("anthropic/claude-sonnet-4") != 200000 {
		t.Fatalf("opencode id window=%d", cat.lookup("anthropic/claude-sonnet-4"))
	}
	if cat.lookup("") != 0 {
		t.Fatal("empty slug should be 0")
	}
}

func TestContextWindowLookupProjectOverlay(t *testing.T) {
	dir := t.TempDir()
	modelsDir := filepath.Join(dir, ".workflow-hero", "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "provider: test\nmodels:\n  overlay-model:\n    context_window: 12345\n  composer-2.5:\n    context_window: 111000\n"
	if err := os.WriteFile(filepath.Join(modelsDir, "test.yml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := loadContextWindowCatalog(dir)
	if cat.lookup("overlay-model") != 12345 {
		t.Fatalf("overlay=%d", cat.lookup("overlay-model"))
	}
	if cat.lookup("composer-2.5") != 111000 {
		t.Fatalf("project should override embed, got %d", cat.lookup("composer-2.5"))
	}
	if cat.lookup("grok-4.6") != 500000 {
		t.Fatalf("embed fallback missing grok-4.6=%d", cat.lookup("grok-4.6"))
	}
}

func TestFormatTokenCount(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{500, "500"},
		{180000, "180k"},
		{256000, "256k"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
	}
	for _, tc := range cases {
		if got := formatTokenCount(tc.n); got != tc.want {
			t.Errorf("formatTokenCount(%d)=%q want %q", tc.n, got, tc.want)
		}
	}
}

func TestRenderContextBar(t *testing.T) {
	if renderContextBar(0, 0) != "" {
		t.Fatal("expected empty render without max")
	}
	if renderContextBar(10, 0) != "" {
		t.Fatal("expected empty render without max even with used")
	}

	empty := stripANSI(renderContextBar(0, 256000))
	if !strings.Contains(empty, "0/256k") {
		t.Fatalf("empty bar label: %q", empty)
	}
	if strings.Count(empty, contextBarEmptyChar) != contextBarCells {
		t.Fatalf("empty bar should be all empty cells: %q", empty)
	}
	if strings.Contains(empty, contextBarFillChar) {
		t.Fatalf("empty bar should have no fill: %q", empty)
	}

	partial := stripANSI(renderContextBar(180000, 256000))
	if !strings.Contains(partial, "180k/256k") {
		t.Fatalf("partial label: %q", partial)
	}
	if !strings.Contains(partial, contextBarFillChar) || !strings.Contains(partial, contextBarEmptyChar) {
		t.Fatalf("partial bar should mix fill and empty: %q", partial)
	}

	full := stripANSI(renderContextBar(256000, 256000))
	if strings.Contains(full, contextBarEmptyChar) {
		t.Fatalf("full bar should have no empty cells: %q", full)
	}
	if strings.Count(full, contextBarFillChar) != contextBarCells {
		t.Fatalf("full bar fill count: %q", full)
	}

	million := stripANSI(renderContextBar(500000, 1000000))
	if !strings.Contains(million, "500k/1.0M") {
		t.Fatalf("million label: %q", million)
	}
}

func stripANSI(s string) string {
	return ansiEscapeRE.ReplaceAllString(s, "")
}
