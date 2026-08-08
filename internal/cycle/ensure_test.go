package cycle_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestEnsureOperationalStore_CreatesDB(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, cursoradapter.HeroDir), 0o755); err != nil {
		t.Fatal(err)
	}

	st, res, err := cycle.EnsureOperationalStore(dir)
	if err != nil {
		t.Fatalf("EnsureOperationalStore: %v", err)
	}
	defer st.Close()

	if res.LegacyImported {
		t.Fatal("expected no legacy import without workflow.md")
	}
	if _, err := os.Stat(filepath.Join(dir, store.RelativeDBPath)); err != nil {
		t.Fatalf("hero.db missing: %v", err)
	}
}

func TestOpenService_AutoCreatesDB(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, cursoradapter.HeroConfigDir), 0o755); err != nil {
		t.Fatal(err)
	}

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatalf("OpenService: %v", err)
	}
	defer svc.Close()

	if _, err := os.Stat(filepath.Join(dir, store.RelativeDBPath)); err != nil {
		t.Fatalf("hero.db missing after OpenService: %v", err)
	}
}

func TestFindProjectRoot_NotInstalled(t *testing.T) {
	dir := t.TempDir()
	_, err := cycle.FindProjectRoot(dir)
	if !errors.Is(err, cycle.ErrNotInstalled) {
		t.Fatalf("got %v, want ErrNotInstalled", err)
	}
}
