package cycle

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
)

func TestFindProjectRoot_SkipsUserHomeWorkflowHero(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, cursoradapter.HeroDir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(home, "Downloads", "scratch")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	prev := lookupUserHomeDir
	lookupUserHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { lookupUserHomeDir = prev })

	if _, err := FindProjectRoot(nested); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("from nested under home: got %v, want ErrNotInstalled", err)
	}
	if _, err := FindProjectRoot(home); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("from home itself: got %v, want ErrNotInstalled", err)
	}
}

func TestFindProjectRoot_FindsInstalledProjectUnderHome(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, cursoradapter.HeroDir), 0o755); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(home, "code", "app")
	if err := os.MkdirAll(filepath.Join(project, cursoradapter.HeroDir), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(project, "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	prev := lookupUserHomeDir
	lookupUserHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { lookupUserHomeDir = prev })

	root, err := FindProjectRoot(nested)
	if err != nil {
		t.Fatalf("FindProjectRoot: %v", err)
	}
	want, err := filepath.Abs(project)
	if err != nil {
		t.Fatal(err)
	}
	if root != want {
		t.Fatalf("root=%q want %q", root, want)
	}
}
