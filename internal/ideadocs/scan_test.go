package ideadocs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListActive_missingDir(t *testing.T) {
	dir := t.TempDir()
	paths, err := ListActive(dir)
	if err != nil {
		t.Fatal(err)
	}
	if paths != nil {
		t.Fatalf("paths=%v want nil", paths)
	}
}

func TestListActive_emptyIdeaDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	paths, err := ListActive(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Fatalf("paths=%v want empty", paths)
	}
}

func TestListActive_includesRootAndNested(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, DirName, "root.md"), "# root")
	mustWrite(t, filepath.Join(dir, DirName, "feature", "note.md"), "# note")

	paths, err := ListActive(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{DirName + "/feature/note.md", DirName + "/root.md"}
	if strings.Join(paths, "|") != strings.Join(want, "|") {
		t.Fatalf("paths=%v want %v", paths, want)
	}
}

func TestListActive_excludesArchiveAndTobe(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, DirName, "active.md"), "# active")
	mustWrite(t, filepath.Join(dir, DirName, ExcludeArchive, "old.md"), "# old")
	mustWrite(t, filepath.Join(dir, DirName, ExcludeTobe, "future.md"), "# future")
	mustWrite(t, filepath.Join(dir, DirName, ExcludeTobe, "v3", "deep.md"), "# deep")

	paths, err := ListActive(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != DirName+"/active.md" {
		t.Fatalf("paths=%v want only active.md", paths)
	}
}

func TestListActive_ignoresDotfiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, DirName, ".hidden.md"), "# hidden")
	mustWrite(t, filepath.Join(dir, DirName, "visible.md"), "# visible")

	paths, err := ListActive(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != DirName+"/visible.md" {
		t.Fatalf("paths=%v want visible only", paths)
	}
}

func TestListActive_respectsCap(t *testing.T) {
	dir := t.TempDir()
	idea := filepath.Join(dir, DirName)
	if err := os.MkdirAll(idea, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxActiveFiles+5; i++ {
		name := filepath.Join(idea, fmt.Sprintf("file-%04d.md", i))
		mustWrite(t, name, "# x")
	}

	paths, err := ListActive(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != maxActiveFiles {
		t.Fatalf("len=%d want %d", len(paths), maxActiveFiles)
	}
}

func TestPromptSection_empty(t *testing.T) {
	if got := PromptSection(nil); got != "" {
		t.Fatalf("got %q want empty", got)
	}
}

func TestPromptSection_listsPaths(t *testing.T) {
	got := PromptSection([]string{DirName + "/foo.md"})
	if !strings.Contains(got, DirName+"/foo.md") {
		t.Fatalf("missing path: %q", got)
	}
	if !strings.Contains(got, "ADR-019") {
		t.Fatalf("missing ADR note: %q", got)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
