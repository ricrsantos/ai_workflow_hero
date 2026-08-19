// Package integration contains lightweight end-to-end tests that exercise
// the compiled feature logic against real temp directories (ADR-009).
package integration_test

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
	"github.com/ricrsantos/ai_workflow_hero/internal/doctor"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/uninstall"
	"github.com/ricrsantos/ai_workflow_hero/internal/upgrade"
)

func makeGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return dir
}

func doInstall(t *testing.T, dir, version string) {
	t.Helper()
	var sb strings.Builder
	if err := install.Run(install.Options{
		ProjectDir: dir,
		Name:       "Integration Test",
		Summary:    "Integration test project",
		Tools:      []string{"cursor"},
		Version:    version,
		GitInit:    false,
		AssetsFS:   assets.FS,
	}, &sb, &sb); err != nil {
		t.Fatalf("install: %v", err)
	}
}

// TestIntegration_InstallThenDoctor verifies that a fresh install passes all doctor checks.
func TestIntegration_InstallThenDoctor(t *testing.T) {
	dir := makeGitRepo(t)
	doInstall(t, dir, "1.0.0")

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.0.0",
	})

	if !report.OK {
		var fails []string
		for _, c := range report.Checks {
			if c.Status == "fail" {
				fails = append(fails, c.Name+": "+c.Message)
			}
		}
		t.Errorf("doctor failed after install:\n%s", strings.Join(fails, "\n"))
	}
}

// TestIntegration_InstallUpgradeDoctor verifies upgrade preserves installation integrity.
func TestIntegration_InstallUpgradeDoctor(t *testing.T) {
	dir := makeGitRepo(t)
	doInstall(t, dir, "1.0.0")

	var sb strings.Builder
	if _, err := upgrade.Run(upgrade.Options{
		ProjectDir: dir,
		Version:    "1.1.0",
		AssetsFS:   assets.FS,
	}, &sb, &sb); err != nil {
		t.Fatalf("upgrade: %v", err)
	}

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.1.0",
	})

	if !report.OK {
		var fails []string
		for _, c := range report.Checks {
			if c.Status == "fail" {
				fails = append(fails, c.Name+": "+c.Message)
			}
		}
		t.Errorf("doctor failed after upgrade:\n%s", strings.Join(fails, "\n"))
	}

	// Verify hero.json was updated.
	heroData, _ := os.ReadFile(filepath.Join(dir, cursoradapter.HeroJSONPath))
	var heroJSON install.HeroJSON
	_ = json.Unmarshal(heroData, &heroJSON)
	if heroJSON.CLI.Version != "1.1.0" {
		t.Errorf("expected cli.version=1.1.0 after upgrade, got %q", heroJSON.CLI.Version)
	}
}

// TestIntegration_InstallUninstallPreservesArtifacts verifies uninstall preserves project artifacts.
func TestIntegration_InstallUninstallPreservesArtifacts(t *testing.T) {
	dir := makeGitRepo(t)
	doInstall(t, dir, "1.0.0")

	// Create project artifacts.
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Preserved"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "context"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "context", "current-state.md"), []byte("state"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "docs"), 0o755)

	var sb strings.Builder
	if err := uninstall.Run(uninstall.Options{ProjectDir: dir}, &sb, &sb); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	// Hero-owned dirs removed.
	for _, p := range []string{cursoradapter.AgentsDir, cursoradapter.HeroDir} {
		if _, err := os.Stat(filepath.Join(dir, p)); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed", p)
		}
	}

	// Project artifacts preserved.
	for _, p := range []string{"AGENTS.md", "context", "docs"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("expected %s to be preserved: %v", p, err)
		}
	}
}

// TestIntegration_DoctorMissingCommand verifies doctor catches a missing command file.
func TestIntegration_DoctorMissingCommand(t *testing.T) {
	dir := makeGitRepo(t)
	doInstall(t, dir, "1.0.0")

	// Remove a command file.
	missing := filepath.Join(dir, cursoradapter.CommandsDir, "hero-start.md")
	_ = os.Remove(missing)

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.0.0",
	})

	if report.OK {
		t.Error("expected doctor to fail when hero-start.md is missing")
	}
}

// TestIntegration_UpgradeReplacesCustomizedFileWithBackup verifies conflict replacement.
func TestIntegration_UpgradeReplacesCustomizedFileWithBackup(t *testing.T) {
	dir := makeGitRepo(t)
	doInstall(t, dir, "1.0.0")

	// Customize an agent file.
	agentFile := filepath.Join(dir, cursoradapter.AgentsDir, "qa_agent.md")
	orig, _ := os.ReadFile(agentFile)
	origHash := sha256Sum(orig)
	customized := append(orig, []byte("\n# Custom addition\n")...)
	_ = os.WriteFile(agentFile, customized, 0o644)

	var sb strings.Builder
	result, err := upgrade.Run(upgrade.Options{
		ProjectDir: dir,
		Version:    "1.1.0",
		AssetsFS:   assets.FS,
	}, &sb, &sb)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}

	targetRel := filepath.Join(cursoradapter.AgentsDir, "qa_agent.md")
	found := false
	for _, s := range result.Replaced {
		if s == targetRel {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected qa_agent.md in replaced list; got: %v", result.Replaced)
	}

	// File should contain the original embedded content.
	after, _ := os.ReadFile(agentFile)
	if sha256Sum(after) != origHash {
		t.Error("customized file was not replaced with embedded asset content")
	}

	entries, _ := os.ReadDir(filepath.Dir(agentFile))
	var backupFound bool
	for _, e := range entries {
		if strings.Contains(e.Name(), ".conflict") && strings.HasPrefix(e.Name(), "qa_agent.md_") {
			backupFound = true
		}
	}
	if !backupFound {
		t.Error("conflict backup file not found")
	}
}

func sha256Sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
