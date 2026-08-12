package upgrade_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/upgrade"
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
	return dir
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func TestUpgrade_UnmodifiedFilesAreUpdated(t *testing.T) {
	dir := makeInstalledDir(t)

	var out strings.Builder
	result, err := upgrade.Run(upgrade.Options{
		ProjectDir: dir,
		Version:    "1.1.0",
		AssetsFS:   assets.FS,
	}, &out, &out)
	if err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}

	if len(result.Updated) == 0 {
		t.Error("expected at least one updated file")
	}
	if len(result.Skipped) != 0 {
		t.Errorf("unexpected skipped files: %v", result.Skipped)
	}

	// hero.json version should be updated.
	heroData, _ := os.ReadFile(filepath.Join(dir, cursoradapter.HeroJSONPath))
	var heroJSON install.HeroJSON
	_ = json.Unmarshal(heroData, &heroJSON)
	if heroJSON.CLI.Version != "1.1.0" {
		t.Errorf("hero.json cli.version = %q, want %q", heroJSON.CLI.Version, "1.1.0")
	}
	if _, ok := heroJSON.Harnesses["cursor"]; !ok {
		t.Fatal("upgrade should ensure harnesses.cursor defaults")
	}
}

func TestUpgrade_MigratesMissingHarnessDefaults(t *testing.T) {
	dir := makeInstalledDir(t)
	heroPath := filepath.Join(dir, cursoradapter.HeroJSONPath)
	// Strip harnesses to simulate pre-ADR-030 install.
	legacy := install.HeroJSON{
		CLI:    install.CLIInfo{Version: "1.0.0", InstalledAt: "2026-01-01T00:00:00Z", Tools: []string{"cursor"}},
		Assets: install.AssetsInfo{Version: "1.0.0", InstalledAt: "2026-01-01T00:00:00Z"},
	}
	data, _ := json.MarshalIndent(legacy, "", "  ")
	if err := os.WriteFile(heroPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if _, err := upgrade.Run(upgrade.Options{
		ProjectDir: dir,
		Version:    "1.1.0",
		AssetsFS:   assets.FS,
	}, &out, &out); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	heroData, _ := os.ReadFile(heroPath)
	var heroJSON install.HeroJSON
	_ = json.Unmarshal(heroData, &heroJSON)
	cfg, ok := heroJSON.Harnesses["cursor"]
	if !ok || cfg.Model != "" || cfg.EnableFastModel {
		t.Fatalf("harnesses.cursor=%+v", cfg)
	}
}

func TestUpgrade_MigratesLegacyGenericModelInCycleConfig(t *testing.T) {
	dir := makeInstalledDir(t)

	configPath := filepath.Join(dir, cursoradapter.HeroCurrentCycleDir, "workflow-config.yml")
	legacy := `title: Test
objective: Test objective.

generic_model: gpt-5.3-codex

scope:
  backend: true
`
	if err := os.WriteFile(configPath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	result, err := upgrade.Run(upgrade.Options{
		ProjectDir: dir,
		Version:    "1.1.0",
		AssetsFS:   assets.FS,
	}, &out, &out)
	if err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}

	if len(result.Migrated) != 1 {
		t.Fatalf("expected one migrated workflow-config, got %v", result.Migrated)
	}

	updated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(updated)
	if strings.Contains(content, "generic_model:") {
		t.Errorf("generic_model was not migrated:\n%s", content)
	}
	if !strings.Contains(content, "fallback_model:") {
		t.Errorf("fallback_model block missing:\n%s", content)
	}
}

func TestUpgrade_CustomizedFileIsSkipped(t *testing.T) {
	dir := makeInstalledDir(t)

	// Customize backend_agent.md.
	targetRel := filepath.Join(cursoradapter.AgentsDir, "backend_agent.md")
	targetAbs := filepath.Join(dir, targetRel)
	original, _ := os.ReadFile(targetAbs)
	customized := append(original, []byte("\n# User customization\n")...)
	if err := os.WriteFile(targetAbs, customized, 0o644); err != nil {
		t.Fatalf("write customized file: %v", err)
	}

	var out strings.Builder
	result, err := upgrade.Run(upgrade.Options{
		ProjectDir: dir,
		Version:    "1.1.0",
		AssetsFS:   assets.FS,
	}, &out, &out)
	if err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}

	found := false
	for _, s := range result.Skipped {
		if s == targetRel {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q in skipped, got: %v", targetRel, result.Skipped)
	}

	// The customized file should NOT have been overwritten.
	afterData, _ := os.ReadFile(targetAbs)
	afterHash := sha256hex(afterData)
	customizedHash := sha256hex(customized)
	if afterHash != customizedHash {
		t.Error("customized file was overwritten by upgrade")
	}
}

func TestUpgrade_ImportsLegacyCycleFrom09LikeFixture(t *testing.T) {
	dir := makeInstalledDir(t)

	cycleDir := filepath.Join(dir, cursoradapter.HeroCurrentCycleDir)
	workflow := `# Workflow — Cycle C2

**Title**: Legacy Upgrade
**Objective**: Import on upgrade
**Status**: In Progress
**Started**: 2026-07-01
**Completed**:

## Stages

| Stage | Status | Iteration | Human Approval | Extra Iterations Granted |
|-------|--------|-----------|----------------|--------------------------|
| Research | Completed | 1/50 | Auto | +0 |
| Planning | In Progress | 1/3 | N/A | +0 |
`
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow.md"), []byte(workflow), 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate 0.9.x: workflow.md present but no hero.db.
	dbPath := filepath.Join(dir, store.RelativeDBPath)
	if err := os.Remove(dbPath); err != nil {
		t.Fatalf("remove hero.db: %v", err)
	}

	var out strings.Builder
	result, err := upgrade.Run(upgrade.Options{
		ProjectDir: dir,
		Version:    "1.0.0",
		AssetsFS:   assets.FS,
	}, &out, &out)
	if err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}
	if !result.LegacyImported {
		t.Fatal("expected legacy cycle import")
	}
	if result.LegacyCycleNum != 2 {
		t.Errorf("legacy cycle number = %d, want 2", result.LegacyCycleNum)
	}
	if !strings.Contains(out.String(), "no longer canonical") {
		t.Error("expected legacy markdown warning in output")
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("hero.db not created after upgrade: %v", err)
	}

	s, err := store.OpenProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	cycles, err := s.ListCycles()
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) != 1 {
		t.Fatalf("cycles len = %d, want 1", len(cycles))
	}
	if cycles[0].Title != "Legacy Upgrade" {
		t.Errorf("cycle title = %q, want Legacy Upgrade", cycles[0].Title)
	}
}
