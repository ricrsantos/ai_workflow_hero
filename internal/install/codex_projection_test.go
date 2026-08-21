package install_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

func TestProvisionCodex_ProjectsMirroredFamilies(t *testing.T) {
	dir := t.TempDir()
	checksums := make(install.Checksums)

	if err := install.ProvisionCodex(dir, assets.FS, checksums); err != nil {
		t.Fatalf("ProvisionCodex: %v", err)
	}

	wantAgents := []string{
		"backend_agent.md",
		"browser_ui_agent.md",
		"context_agent.md",
		"discover_agent.md",
		"end2end_qa_agent.md",
		"frontend_agent.md",
		"generic_agent.md",
		"judge_agent.md",
		"orchestration_agent.md",
		"planning_agent.md",
		"qa_agent.md",
	}
	for _, name := range wantAgents {
		path := filepath.Join(dir, ".codex", "agents", name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing agent %s: %v", name, err)
		}
		rel := filepath.ToSlash(filepath.Join(".codex", "agents", name))
		if checksums[rel] == "" {
			t.Fatalf("checksum missing for %s", rel)
		}
	}

	if _, err := os.Stat(filepath.Join(dir, ".codex", "commands", "hero-start.md")); err != nil {
		t.Fatalf("commands not projected: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".codex", "skills", "workflow-hero", "SKILL.md")); err != nil {
		t.Fatalf("workflow-hero skill missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".codex", "skills", "grilling", "SKILL.md")); err != nil {
		t.Fatalf("grilling skill missing: %v", err)
	}
}

// golden: projected .codex/ layout — no root AGENTS.md, no adapter config file (ADR-046 §5.3).
func TestProvisionCodex_LayoutGoldenNoAgentsMdNoConfig(t *testing.T) {
	dir := t.TempDir()
	checksums := make(install.Checksums)
	if err := install.ProvisionCodex(dir, assets.FS, checksums); err != nil {
		t.Fatalf("ProvisionCodex: %v", err)
	}

	var got []string
	root := filepath.Join(dir, ".codex")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		got = append(got, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)

	goldenPath := filepath.Join("testdata", "codex_projection_layout.golden")
	wantBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	want := strings.Split(strings.TrimSpace(string(wantBytes)), "\n")
	if !slices.Equal(got, want) {
		t.Fatalf("projection layout mismatch\ngot:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	for _, rel := range got {
		base := filepath.Base(rel)
		if strings.EqualFold(base, "AGENTS.md") {
			t.Fatal("root AGENTS.md must not be copied into .codex/")
		}
		if strings.EqualFold(base, "config.toml") || strings.EqualFold(base, "config.json") {
			t.Fatal("Codex adapter does not require a minimal config template; none should be projected")
		}
	}
}

func TestProvisionCodex_DisableKeepsFiles(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := []byte(`{
  "cli": {"version": "2.5.0", "installedAt": "2026-01-01T00:00:00Z"},
  "assets": {"version": "2.5.0", "installedAt": "2026-01-01T00:00:00Z"},
  "harnesses": {
    "cursor": {"enabled": true},
    "opencode": {"enabled": false},
    "codex": {"enabled": false}
  }
}
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), hero, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "checksums.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := install.EnableHarnessWithProjection(dir, "codex", assets.FS); err != nil {
		t.Fatal(err)
	}
	agentPath := filepath.Join(dir, ".codex", "agents", "orchestration_agent.md")
	if _, err := os.Stat(agentPath); err != nil {
		t.Fatalf("expected projection before disable: %v", err)
	}

	if err := install.SetHarnessEnabled(dir, "codex", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	loaded, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if install.IsHarnessEnabled(loaded, "codex") {
		t.Fatal("codex should be disabled")
	}
	if _, err := os.Stat(agentPath); err != nil {
		t.Fatalf("disable must keep .codex/ files: %v", err)
	}
}

func TestAssetsFS_EmbedsCodexFamilies(t *testing.T) {
	entries, err := fs.ReadDir(assets.FS, "codex/agents")
	if err != nil {
		t.Fatalf("codex/agents missing from embed: %v", err)
	}
	if len(entries) < 11 {
		t.Fatalf("want full agent family (≥11), got %d", len(entries))
	}
	if _, err := fs.Stat(assets.FS, "codex/commands/hero-start.md"); err != nil {
		t.Fatalf("codex commands not embedded: %v", err)
	}
	if _, err := fs.Stat(assets.FS, "codex/skills/workflow-hero/SKILL.md"); err != nil {
		t.Fatalf("codex skills not embedded: %v", err)
	}
	if _, err := fs.Stat(assets.FS, "codex/agents/STUB.md"); err == nil {
		t.Fatal("STUB.md must be removed after §5 full projection assets")
	}
}
