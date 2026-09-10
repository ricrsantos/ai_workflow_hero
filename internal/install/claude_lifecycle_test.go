package install_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

func TestRun_ClaudeOnlyInstallProjectsAssetsAndState(t *testing.T) {
	dir := makeGitRepo(t)
	var out strings.Builder
	if err := install.Run(install.Options{
		ProjectDir: dir,
		Name:       "Claude project",
		Summary:    "projection test",
		Tools:      []string{"claude"},
		Version:    "3.1.1",
		AssetsFS:   assets.FS,
	}, &out, &out); err != nil {
		t.Fatalf("install: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".claude", "agents", "generic_agent.md")); err != nil {
		t.Fatalf("Claude projection missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".workflow-hero", "models", "claude.yml")); err != nil {
		t.Fatalf("Claude catalog missing: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, cursoradapter.HeroJSONPath))
	if err != nil {
		t.Fatal(err)
	}
	var hero install.HeroJSON
	if err := json.Unmarshal(data, &hero); err != nil {
		t.Fatal(err)
	}
	if !install.IsHarnessEnabled(hero, "claude") {
		t.Fatal("Claude should be enabled after Claude-only install")
	}
	if install.IsHarnessEnabled(hero, "cursor") {
		t.Fatal("Cursor should remain disabled after Claude-only install")
	}
	checksums, err := install.LoadChecksums(dir)
	if err != nil {
		t.Fatal(err)
	}
	if checksums[filepath.ToSlash(filepath.Join(".claude", "agents", "generic_agent.md"))] == "" {
		t.Fatal("Claude projected asset checksum missing")
	}
}

func TestRun_ClaudeInstallAppliesManagedContextDecision(t *testing.T) {
	dir := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Project instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := install.Run(install.Options{
		ProjectDir:    dir,
		Name:          "Claude context",
		Summary:       "managed context test",
		Tools:         []string{"claude"},
		ClaudeContext: install.ClaudeContextInsertOrUpdate,
		Version:       "3.1.1",
		AssetsFS:      assets.FS,
	}, &out, &out); err != nil {
		t.Fatalf("install: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("CLAUDE.md missing after Claude install: %v", err)
	}
	if !strings.Contains(string(content), "@AGENTS.md") {
		t.Fatalf("CLAUDE.md missing @AGENTS.md import: %q", content)
	}
}

func TestRun_ClaudeInstallLeaveUnchangedDoesNotCreateClaudeMd(t *testing.T) {
	dir := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Project instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := install.Run(install.Options{
		ProjectDir:    dir,
		Name:          "Claude no context",
		Tools:         []string{"claude"},
		ClaudeContext: install.ClaudeContextLeaveUnchanged,
		Version:       "3.1.1",
		AssetsFS:      assets.FS,
	}, &out, &out); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatal("CLAUDE.md must not be created for leave-unchanged")
	}
}

func TestEnableClaudeProjectsAndDisableKeepsFiles(t *testing.T) {
	dir := makeGitRepo(t)
	var out strings.Builder
	if err := install.Run(install.Options{
		ProjectDir: dir,
		Name:       "Enable Claude",
		Tools:      []string{"cursor"},
		Version:    "3.1.1",
		AssetsFS:   assets.FS,
	}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if err := install.EnableHarnessWithProjection(dir, "claude", assets.FS); err != nil {
		t.Fatalf("enable Claude: %v", err)
	}
	managed := filepath.Join(dir, ".claude", "agents", "orchestration_agent.md")
	if _, err := os.Stat(managed); err != nil {
		t.Fatalf("managed Claude file missing: %v", err)
	}
	if err := install.SetHarnessEnabled(dir, "claude", false); err != nil {
		t.Fatalf("disable Claude: %v", err)
	}
	if _, err := os.Stat(managed); err != nil {
		t.Fatalf("disable must keep projected Claude files: %v", err)
	}
}
