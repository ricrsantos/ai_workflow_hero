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

func TestSetHarnessEnabled_LastHarnessGuard(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := []byte(`{
  "harnesses": {"cursor": {"enabled": true}, "opencode": {"enabled": false}}
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
