package userpath

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLookPathUsesLookPathFirst(t *testing.T) {
	got, err := LookPath("openspec", func(name string) (string, error) {
		if name != "openspec" {
			t.Fatalf("name=%q", name)
		}
		return "/usr/bin/openspec", nil
	}, []string{"/unused"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "/usr/bin/openspec" {
		t.Fatalf("got=%q", got)
	}
}

func TestLookPathFallsBackToExtraDir(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "openspec")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := LookPath("openspec", func(string) (string, error) {
		return "", exec.ErrNotFound
	}, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if got != bin {
		t.Fatalf("got=%q want %q", got, bin)
	}
}

func TestLookPathSkipsNonExecutable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "openspec"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LookPath("openspec", func(string) (string, error) {
		return "", exec.ErrNotFound
	}, []string{dir})
	if err == nil {
		t.Fatal("expected error for non-executable")
	}
}

func TestAugmentPATHPrependsAndDedupes(t *testing.T) {
	env := []string{"FOO=bar", "PATH=/usr/bin:/bin"}
	got := AugmentPATH(env, "/home/u/.nvm/versions/node/v22/bin", "/usr/bin", "/home/u/.local/bin")
	path := ""
	for _, e := range got {
		if strings.HasPrefix(e, "PATH=") {
			path = strings.TrimPrefix(e, "PATH=")
		}
	}
	wantPrefix := "/home/u/.nvm/versions/node/v22/bin" + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/home/u/.local/bin"
	if !strings.HasPrefix(path, wantPrefix) {
		t.Fatalf("PATH=%q want prefix %q", path, wantPrefix)
	}
	if strings.Count(path, "/usr/bin") != 1 {
		t.Fatalf("expected /usr/bin once: %q", path)
	}
}

func TestExtraBinDirsIncludesNVMBin(t *testing.T) {
	t.Setenv("NVM_BIN", "/tmp/hero-nvm-bin")
	dirs := ExtraBinDirs()
	found := false
	for _, d := range dirs {
		if d == "/tmp/hero-nvm-bin" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("NVM_BIN missing from ExtraBinDirs: %v", dirs)
	}
}
