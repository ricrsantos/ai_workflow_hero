package uninstall_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/uninstall"
)

func makeInstalledDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	var sb strings.Builder
	if err := install.Run(install.Options{
		ProjectDir: dir,
		Name:       "Test",
		Summary:    "test",
		Tools:      []string{"cursor"},
		Version:    "1.0.0",
		GitInit:    false,
		AssetsFS:   assets.FS,
	}, &sb, &sb); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Create preserved project artifacts.
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# AGENTS"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "context"), 0o755); err != nil {
		t.Fatalf("mkdir context: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "context", "current-state.md"), []byte("# State"), 0o644); err != nil {
		t.Fatalf("write current-state.md: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "openspec"), 0o755); err != nil {
		t.Fatalf("mkdir openspec: %v", err)
	}

	return dir
}

func TestUninstall_RemovesHeroOwnedPaths(t *testing.T) {
	dir := makeInstalledDir(t)

	var out strings.Builder
	if err := uninstall.Run(uninstall.Options{ProjectDir: dir}, &out, &out); err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}

	// These should be removed.
	removed := []string{
		cursoradapter.AgentsDir,
		cursoradapter.WorkflowHeroSkillDir,
		cursoradapter.GrillingSkillDir,
		cursoradapter.HeroDir,
	}
	for _, p := range removed {
		if _, err := os.Stat(filepath.Join(dir, p)); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed after uninstall", p)
		}
	}

	// Hero commands should be removed.
	for _, cmdFile := range cursoradapter.RequiredCommandFiles() {
		if _, err := os.Stat(filepath.Join(dir, cmdFile)); !os.IsNotExist(err) {
			t.Errorf("expected command file %s to be removed", cmdFile)
		}
	}

	// hero.db removed with .workflow-hero/.
	if _, err := os.Stat(filepath.Join(dir, store.RelativeDBPath)); !os.IsNotExist(err) {
		t.Error("expected hero.db to be removed after uninstall")
	}
}

func TestUninstall_PreservesProjectArtifacts(t *testing.T) {
	dir := makeInstalledDir(t)

	var out strings.Builder
	if err := uninstall.Run(uninstall.Options{ProjectDir: dir}, &out, &out); err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}

	preserved := []string{
		"AGENTS.md",
		filepath.Join("context", "current-state.md"),
		"docs",
		"openspec",
	}
	for _, p := range preserved {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("expected %s to be preserved after uninstall: %v", p, err)
		}
	}
}
