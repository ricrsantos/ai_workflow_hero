package todos_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/todos"
)

func TestParse_sampleFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "current-state_sample.md"))
	if err != nil {
		t.Fatal(err)
	}
	items, err := todos.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 5 {
		t.Fatalf("got %d items, want 5: %#v", len(items), items)
	}
	for _, item := range items {
		if item.Section != "Pending Features" {
			t.Errorf("item %q has section %q, want Pending Features", item.Text, item.Section)
		}
	}
	if !strings.Contains(items[2].Text, "Tag/publish GitHub Release") {
		t.Errorf("third item = %q", items[2].Text)
	}
}

func TestParse_emptyPendingSection(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "current-state_empty_pending.md"))
	if err != nil {
		t.Fatal(err)
	}
	items, err := todos.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("got %d items, want 0: %#v", len(items), items)
	}
}

func TestParse_missingPendingSection(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "current-state_no_pending_section.md"))
	if err != nil {
		t.Fatal(err)
	}
	items, err := todos.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("got %d items, want 0", len(items))
	}
}

func TestParse_ignoresNextSteps(t *testing.T) {
	content := []byte(`## Pending Features

- only pending item

## Next Steps

1. numbered step must not appear
`)
	items, err := todos.Parse(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Text != "only pending item" {
		t.Fatalf("got %#v", items)
	}
}

func TestReadProject(t *testing.T) {
	dir := t.TempDir()
	ctxDir := filepath.Join(dir, "context")
	if err := os.MkdirAll(ctxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join("testdata", "current-state_sample.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctxDir, "current-state.md"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	items, err := todos.ReadProject(dir)
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}
	if len(items) != 5 {
		t.Fatalf("got %d items, want 5", len(items))
	}
}

func TestReadProject_missingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := todos.ReadProject(dir)
	if err == nil {
		t.Fatal("expected error for missing current-state.md")
	}
}

func TestFormatLines_matchesUISpec(t *testing.T) {
	items := []todos.Item{
		{Section: "Pending Features", Text: "Tag/publish GitHub Release v1.0.0 when ready"},
		{Section: "Pending Features", Text: "Post-1.0 deferred D1–D13"},
	}
	lines := todos.FormatLines(items)
	if lines[0] != todos.HeaderLine() {
		t.Errorf("header = %q, want %q", lines[0], todos.HeaderLine())
	}
	if lines[2] != "• Tag/publish GitHub Release v1.0.0 when ready" {
		t.Errorf("first bullet = %q", lines[2])
	}
	last := lines[len(lines)-1]
	if last != todos.SyncNoticeLine() {
		t.Errorf("sync notice = %q, want %q", last, todos.SyncNoticeLine())
	}
}

func TestFormat_emptyItemsStillShowsNotice(t *testing.T) {
	out := todos.Format(nil)
	if !strings.Contains(out, todos.HeaderLine()) {
		t.Error("missing header")
	}
	if !strings.Contains(out, todos.SyncNoticeLine()) {
		t.Error("missing sync notice")
	}
	if strings.Contains(out, "•") {
		t.Error("unexpected bullet for empty items")
	}
}

func TestSyncNoticeText(t *testing.T) {
	want := "/hero-sync then /hero-todos"
	if !strings.Contains(todos.SyncNoticeText, want) {
		t.Errorf("SyncNoticeText = %q, want substring %q", todos.SyncNoticeText, want)
	}
}

func TestParse_numberedListInPendingSection(t *testing.T) {
	content := []byte(`## Pending Features

1. first numbered
2. second numbered
`)
	items, err := todos.Parse(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %#v", items)
	}
}

func TestParse_realRepoCurrentState(t *testing.T) {
	path := filepath.Join("..", "..", "context", "current-state.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip("repo context/current-state.md not available")
	}
	items, err := todos.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 3 {
		t.Fatalf("expected at least 3 pending items from repo file, got %d", len(items))
	}
}
