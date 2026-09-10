package install_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

func TestEnableHarnessWithProjection_OpenCode(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := []byte(`{
  "cli": {"version": "2.0.0", "installedAt": "2026-01-01T00:00:00Z"},
  "assets": {"version": "2.0.0", "installedAt": "2026-01-01T00:00:00Z"},
  "harnesses": {"cursor": {"enabled": true}, "opencode": {"enabled": false}}
}
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), hero, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "checksums.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := install.EnableHarnessWithProjection(dir, "opencode", assets.FS); err != nil {
		t.Fatal(err)
	}
	loaded, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !install.IsHarnessEnabled(loaded, "opencode") {
		t.Fatal("opencode should be enabled")
	}
	if _, err := os.Stat(filepath.Join(dir, ".opencode", "opencode.json")); err != nil {
		t.Fatalf(".opencode projection missing: %v", err)
	}
}

func TestEnableHarnessWithProjection_Codex(t *testing.T) {
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
	loaded, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !install.IsHarnessEnabled(loaded, "codex") {
		t.Fatal("codex should be enabled")
	}
	if _, err := os.Stat(filepath.Join(dir, ".codex", "agents")); err != nil {
		t.Fatalf(".codex projection missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".codex", "skills", "workflow-hero", "SKILL.md")); err != nil {
		t.Fatalf(".codex skill missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".codex", "agents", "orchestration_agent.md")); err != nil {
		t.Fatalf(".codex agent missing: %v", err)
	}
}

func TestEnableClaudeHarness_CreatesManagedClaudeContext(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := []byte(`{
  "harnesses": {"cursor": {"enabled": true}, "opencode": {"enabled": false}, "codex": {"enabled": false}, "claude": {"enabled": false}}
}
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), hero, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "checksums.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Project instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := install.EnableClaudeHarness(dir, assets.FS, install.ClaudeContextInsertOrUpdate)
	if err != nil {
		t.Fatalf("EnableClaudeHarness: %v", err)
	}
	if !res.Created {
		t.Fatal("expected CLAUDE.md to be created")
	}
	loaded, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !install.IsHarnessEnabled(loaded, "claude") {
		t.Fatal("claude should be enabled")
	}
	content, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("CLAUDE.md missing: %v", err)
	}
	if !strings.Contains(string(content), "@AGENTS.md") {
		t.Fatalf("CLAUDE.md missing @AGENTS.md import: %q", content)
	}
}

func TestEnableClaudeHarness_LeaveUnchangedDoesNotCreate(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := []byte(`{
  "harnesses": {"cursor": {"enabled": true}, "opencode": {"enabled": false}, "codex": {"enabled": false}, "claude": {"enabled": false}}
}
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), hero, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "checksums.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Project instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := install.EnableClaudeHarness(dir, assets.FS, install.ClaudeContextLeaveUnchanged)
	if err != nil {
		t.Fatalf("EnableClaudeHarness: %v", err)
	}
	if res.Changed || res.Created {
		t.Fatalf("leave-unchanged must not touch CLAUDE.md: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatal("CLAUDE.md must not be created for leave-unchanged")
	}
}

func TestSetHarnessEnabled_LastHarnessGuard(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := []byte(`{
  "harnesses": {"cursor": {"enabled": true}, "opencode": {"enabled": false}, "codex": {"enabled": false}}
}
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), hero, 0o644); err != nil {
		t.Fatal(err)
	}
	err := install.SetHarnessEnabled(dir, "cursor", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "last enabled harness") {
		t.Fatalf("err=%v", err)
	}
}

func TestSetHarnessEnabled_LastHarnessGuard_CodexOnly(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := []byte(`{
  "harnesses": {
    "cursor": {"enabled": false},
    "opencode": {"enabled": false},
    "codex": {"enabled": true}
  }
}
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), hero, 0o644); err != nil {
		t.Fatal(err)
	}
	err := install.SetHarnessEnabled(dir, "codex", false)
	if err == nil {
		t.Fatal("expected error when disabling sole Codex harness")
	}
	if !strings.Contains(err.Error(), "last enabled harness") {
		t.Fatalf("err=%v", err)
	}
}
