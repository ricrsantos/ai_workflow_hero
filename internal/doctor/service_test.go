package doctor_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/doctor"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
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

func TestDoctor_TrackedSecrets_Warns(t *testing.T) {
	dir := makeInstalledDir(t, "1.0.0")

	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	add := exec.Command("git", "-C", dir, "add", "-f", ".env")
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.0.0",
	})

	found := false
	for _, c := range report.Checks {
		if c.Name == "secrets-tracked" && c.Status == "warn" && strings.Contains(c.Message, ".env") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected secrets-tracked warn when .env is tracked")
	}
	if !report.OK {
		t.Error("secrets warn must not fail the doctor report")
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

func TestDoctor_MissingDB_AutoCreates(t *testing.T) {
	dir := makeInstalledDir(t, "1.0.0")

	dbPath := filepath.Join(dir, store.RelativeDBPath)
	if err := os.Remove(dbPath); err != nil {
		t.Fatalf("remove hero.db: %v", err)
	}

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.0.0",
	})

	if !report.OK {
		t.Fatalf("expected doctor OK after auto-creating hero.db; checks=%+v", report.Checks)
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected hero.db to be created: %v", err)
	}

	found := false
	for _, c := range report.Checks {
		if c.Name == "operational-store" && c.Status == "ok" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected operational-store ok check after auto-create")
	}
}

func TestDoctor_UnsupportedHarnessMarker_Warns(t *testing.T) {
	dir := makeInstalledDir(t, "1.0.0")

	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.0.0",
	})

	found := false
	for _, c := range report.Checks {
		if c.Name == "harness-marker:claude" && c.Status == "warn" {
			found = true
			if !strings.Contains(c.Message, "⚠ Detected .claude/ but cli.tools does not include it") {
				t.Errorf("unexpected message: %q", c.Message)
			}
			if !strings.Contains(c.Message, "unsupported in this Hero version") {
				t.Errorf("expected unsupported wording in: %q", c.Message)
			}
			break
		}
	}
	if !found {
		t.Error("expected harness-marker:claude warn check")
	}
	if !report.OK {
		t.Error("harness marker warn must not fail the doctor report")
	}
}

func TestDoctor_SupportedHarnessMarker_Ok(t *testing.T) {
	dir := makeInstalledDir(t, "1.0.0")

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.0.0",
	})

	found := false
	for _, c := range report.Checks {
		if c.Name == "harness-markers" && c.Status == "ok" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected harness-markers ok when only cursor is present")
	}
}

func TestDoctor_CursorCLIMissing_Warns(t *testing.T) {
	dir := makeInstalledDir(t, "1.0.0")

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.0.0",
		CursorCLIProbe: func(context.Context, string) error {
			return errors.New("cursor agent CLI not found on PATH (tried cursor-agent and cursor agent); harness unavailable")
		},
	})

	found := false
	for _, c := range report.Checks {
		if c.Name == "cursor-cli" && c.Status == "warn" && strings.Contains(c.Message, "not found on PATH") {
			found = true
			if !strings.Contains(c.Message, "hero` TUI") {
				t.Errorf("expected TUI hint in message: %q", c.Message)
			}
			break
		}
	}
	if !found {
		t.Error("expected cursor-cli warn when CLI missing")
	}
	if !report.OK {
		t.Error("cursor-cli warn must not fail the doctor report")
	}
}

func TestDoctor_CursorCLIAuth_WarnsWithLoginHint(t *testing.T) {
	dir := makeInstalledDir(t, "1.0.0")

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.0.0",
		CursorCLIProbe: func(context.Context, string) error {
			return &cursoradapter.AuthError{Detail: "not logged in"}
		},
	})

	found := false
	for _, c := range report.Checks {
		if c.Name == "cursor-cli-auth" && c.Status == "warn" && strings.Contains(c.Message, cursoradapter.LoginHint) {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cursor-cli-auth warn with login hint")
	}
	if !report.OK {
		t.Error("cursor-cli-auth warn must not fail the doctor report")
	}
}

func TestDoctor_CursorCLIOk(t *testing.T) {
	dir := makeInstalledDir(t, "1.0.0")

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.0.0",
		CursorCLIProbe: func(context.Context, string) error { return nil },
	})

	found := false
	for _, c := range report.Checks {
		if c.Name == "cursor-cli" && c.Status == "ok" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cursor-cli ok check")
	}
}

func TestDoctor_CodexCLIMissing_WarnsWhenEnabled(t *testing.T) {
	dir := makeInstalledDir(t, "1.0.0")
	if err := install.EnableHarnessWithProjection(dir, "codex", assets.FS); err != nil {
		t.Fatalf("enable codex: %v", err)
	}
	t.Setenv("PATH", t.TempDir()) // no codex binary

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.0.0",
		CursorCLIProbe: func(context.Context, string) error { return nil },
	})

	found := false
	for _, c := range report.Checks {
		if c.Name == "codex-cli" && c.Status == "warn" {
			found = true
			if !strings.Contains(c.Message, "Codex CLI not on PATH") {
				t.Errorf("unexpected message: %q", c.Message)
			}
			if !strings.Contains(c.Message, "unavailable until installed") {
				t.Errorf("expected unavailable guidance in: %q", c.Message)
			}
			break
		}
	}
	if !found {
		t.Error("expected codex-cli warn when Codex enabled and CLI missing")
	}
	if !report.OK {
		t.Error("codex-cli warn must not fail the doctor report")
	}

	// Cursor doctor line still present (UI-C06-001 §8: OpenCode/Cursor unchanged).
	cursorOK := false
	for _, c := range report.Checks {
		if c.Name == "cursor-cli" && c.Status == "ok" {
			cursorOK = true
			break
		}
	}
	if !cursorOK {
		t.Error("expected cursor-cli ok to remain when Codex warn is present")
	}
}

func TestDoctor_CodexCLI_NoCheckWhenDisabled(t *testing.T) {
	dir := makeInstalledDir(t, "1.0.0") // Codex disabled by default
	t.Setenv("PATH", t.TempDir())

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.0.0",
		CursorCLIProbe: func(context.Context, string) error { return nil },
	})

	for _, c := range report.Checks {
		if c.Name == "codex-cli" {
			t.Fatalf("codex-cli check must not run when Codex disabled: %+v", c)
		}
	}
	if !report.OK {
		t.Error("doctor must stay OK when Codex is disabled")
	}

	cursorOK := false
	for _, c := range report.Checks {
		if c.Name == "cursor-cli" && c.Status == "ok" {
			cursorOK = true
			break
		}
	}
	if !cursorOK {
		t.Error("expected cursor-cli ok when Codex disabled")
	}
}

func TestDoctor_CodexCLIOk_WhenOnPATH(t *testing.T) {
	dir := makeInstalledDir(t, "1.0.0")
	if err := install.EnableHarnessWithProjection(dir, "codex", assets.FS); err != nil {
		t.Fatalf("enable codex: %v", err)
	}

	binDir := t.TempDir()
	stub := filepath.Join(binDir, "codex")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.0.0",
		CursorCLIProbe: func(context.Context, string) error { return nil },
	})

	found := false
	for _, c := range report.Checks {
		if c.Name == "codex-cli" && c.Status == "ok" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected codex-cli ok when codex is on PATH")
	}
	if !report.OK {
		t.Error("expected doctor OK when Codex CLI is available")
	}
}

func TestDoctor_OpenCodeCLI_UnchangedWithCodexDisabled(t *testing.T) {
	dir := makeInstalledDir(t, "1.0.0")
	if err := install.EnableHarnessWithProjection(dir, "opencode", assets.FS); err != nil {
		t.Fatalf("enable opencode: %v", err)
	}
	t.Setenv("PATH", t.TempDir()) // no opencode/codex binaries

	report := doctor.Run(doctor.Options{
		ProjectDir:    dir,
		BinaryVersion: "1.0.0",
		CursorCLIProbe: func(context.Context, string) error { return nil },
	})

	opencodeWarn := false
	for _, c := range report.Checks {
		if c.Name == "codex-cli" {
			t.Fatalf("codex-cli must not appear when Codex disabled: %+v", c)
		}
		if c.Name == "opencode-cli" && c.Status == "warn" {
			opencodeWarn = true
			if !strings.Contains(c.Message, "opencode CLI not on PATH") {
				t.Errorf("unexpected opencode-cli message: %q", c.Message)
			}
		}
	}
	if !opencodeWarn {
		t.Error("expected opencode-cli warn (unchanged) when OpenCode enabled and CLI missing")
	}
	if !report.OK {
		t.Error("opencode-cli warn must not fail the doctor report")
	}
}
