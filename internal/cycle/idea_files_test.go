package cycle

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCycleIdeaFilesCommand_text(t *testing.T) {
	dir := t.TempDir()
	setupIdeaFilesProject(t, dir)

	cmd := newCycleIdeaFilesCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(nil)
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(out.String())
	want := "docs/idea/active.md"
	if got != want {
		t.Fatalf("output=%q want %q", got, want)
	}
}

func TestCycleIdeaFilesCommand_json(t *testing.T) {
	dir := t.TempDir()
	setupIdeaFilesProject(t, dir)

	cmd := newCycleIdeaFilesCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json"})
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var view ideaFilesView
	if err := json.Unmarshal(out.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Paths) != 1 || view.Paths[0] != "docs/idea/active.md" {
		t.Fatalf("paths=%v", view.Paths)
	}
}

func setupIdeaFilesProject(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".workflow-hero", "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs", "idea"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "idea", "active.md"), []byte("# active"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs", "idea", "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "idea", "archive", "old.md"), []byte("# old"), 0o644); err != nil {
		t.Fatal(err)
	}
}
