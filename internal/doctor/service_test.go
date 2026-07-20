package doctor_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/doctor"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

func makeInstalledDir(t *testing.T, version string) string {
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
		Version:    version,
		GitInit:    false,
		AssetsFS:   assets.FS,
	}, &sb, &sb); err != nil {
		t.Fatalf("install: %v", err)
	}
	return dir
}

func TestDoctor_AllChecksPass(t *testing.T) {
	dir := makeInstalledDir(t, "1.0.0")

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
		t.Errorf("doctor failed:\n%s", strings.Join(fails, "\n"))
	}
}

func TestDoctor_MissingFile_Fails(t *testing.T) {
	dir := makeInstalledDir(t, "1.0.0")

	// Remove an agent file.
	agentFile := filepath.Join(dir, cursoradapter.AgentsDir, "backend_agent.md")
	if err := os.Remove(agentFile); err != nil {
		t.Fatalf("remove file: %v", err)
	}

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.0.0",
	})

	if report.OK {
		t.Error("expected doctor to fail when a file is missing")
	}

	found := false
	for _, c := range report.Checks {
		if c.Status == "fail" && strings.Contains(c.Name, "backend_agent") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected failure check for backend_agent.md")
	}
}

func TestDoctor_VersionMismatch_Warns(t *testing.T) {
	dir := makeInstalledDir(t, "1.0.0")

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.1.0", // different from installed 1.0.0
	})

	found := false
	for _, c := range report.Checks {
		if c.Name == "version-match" && c.Status == "warn" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected version-match warn when binary version differs from installed version")
	}
}

func TestDoctor_NotGitRepo_Fails(t *testing.T) {
	dir := t.TempDir()

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.0.0",
	})

	found := false
	for _, c := range report.Checks {
		if c.Name == "git-repo" && c.Status == "fail" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected git-repo fail check")
	}
}
