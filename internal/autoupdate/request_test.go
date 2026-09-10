package autoupdate

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequestCommitsChangesAndArmsUpdater(t *testing.T) {
	dir := newGitRepo(t)
	installDir := filepath.Join(t.TempDir(), "install")
	prepareUpdateFiles(t, dir, installDir)

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Request(context.Background(), Config{
		SourceDir:     dir,
		InstallDir:    installDir,
		StateFile:     filepath.Join(installDir, "needs-update.txt"),
		BuildScript:   filepath.Join(dir, "scripts", "build_update.sh"),
		UpdaterScript: filepath.Join(installDir, "hero-update.sh"),
		HeroBinary:    filepath.Join(installDir, "hero"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Commit == "" {
		t.Fatal("commit hash is empty")
	}
	flag, err := os.ReadFile(result.StateFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(flag) != "true\n" {
		t.Fatalf("flag=%q want true", flag)
	}
	log := gitOutput(t, dir, "log", "-1", "--pretty=%s")
	if strings.TrimSpace(log) != DefaultCommitMessage {
		t.Fatalf("commit message=%q", log)
	}
}

func TestRequestRefusesSensitiveChanges(t *testing.T) {
	dir := newGitRepo(t)
	installDir := filepath.Join(t.TempDir(), "install")
	prepareUpdateFiles(t, dir, installDir)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("TOKEN=secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Request(context.Background(), Config{
		SourceDir:     dir,
		InstallDir:    installDir,
		StateFile:     filepath.Join(installDir, "needs-update.txt"),
		BuildScript:   filepath.Join(dir, "scripts", "build_update.sh"),
		UpdaterScript: filepath.Join(installDir, "hero-update.sh"),
		HeroBinary:    filepath.Join(installDir, "hero"),
	})
	if err == nil || !strings.Contains(err.Error(), "sensitive file") {
		t.Fatalf("err=%v want sensitive-file refusal", err)
	}
	if _, statErr := os.Stat(filepath.Join(installDir, "needs-update.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("state file exists after refusal: %v", statErr)
	}
}

func TestRequestReportsNoChanges(t *testing.T) {
	dir := newGitRepo(t)
	installDir := filepath.Join(t.TempDir(), "install")
	prepareUpdateFiles(t, dir, installDir)
	gitOutput(t, dir, "add", "--", ".")
	gitOutput(t, dir, "commit", "-qm", "update helper")

	_, err := Request(context.Background(), Config{
		SourceDir:     dir,
		InstallDir:    installDir,
		StateFile:     filepath.Join(installDir, "needs-update.txt"),
		BuildScript:   filepath.Join(dir, "scripts", "build_update.sh"),
		UpdaterScript: filepath.Join(installDir, "hero-update.sh"),
		HeroBinary:    filepath.Join(installDir, "hero"),
	})
	if !errors.Is(err, ErrNoChanges) {
		t.Fatalf("err=%v want ErrNoChanges", err)
	}
}

func newGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitOutput(t, dir, "init", "-q")
	gitOutput(t, dir, "config", "user.email", "hero-test@example.invalid")
	gitOutput(t, dir, "config", "user.name", "Hero Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("initial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, dir, "add", "--", ".")
	gitOutput(t, dir, "commit", "-qm", "initial")
	return dir
}

func prepareUpdateFiles(t *testing.T, source, install string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(source, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	build := filepath.Join(source, "scripts", "build_update.sh")
	if err := os.WriteFile(build, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(install, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(install, "hero-update.sh"), filepath.Join(install, "hero")} {
		if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
