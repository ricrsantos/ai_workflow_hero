package plugin

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/telegram"
)

func TestManifestLoadSaveRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "telegram")
	m := Manifest{
		Name:            telegram.PluginName,
		Version:         "2.9.2",
		ProtocolVersion: 1,
		DaemonPath:      filepath.Join(dir, "hero-telegram-daemon"),
		InstalledAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if err := Save(dir, m); err != nil {
		t.Fatal(err)
	}
	if !IsInstalled(dir) {
		t.Fatal("expected installed")
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != telegram.PluginName || got.Version != "2.9.2" {
		t.Fatalf("unexpected manifest: %+v", got)
	}
}

func TestListSkipsNonPluginDirs(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "telegram"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Save(filepath.Join(base, "telegram"), Manifest{Name: "telegram", Version: "1.0.0", ProtocolVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "not-a-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	list, err := List(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "telegram" {
		t.Fatalf("unexpected list: %+v", list)
	}
}

func TestInstallTelegramCopiesDaemonAndWritesManifest(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "daemon-src")
	if err := os.WriteFile(src, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	pluginDir := filepath.Join(base, "plugins", "telegram")

	m, err := InstallTelegram(pluginDir, src, "2.9.2", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != "2.9.2" {
		t.Fatalf("version=%q", m.Version)
	}
	dst := filepath.Join(pluginDir, telegram.DaemonBinaryName)
	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("daemon binary missing: %v", err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Fatal("daemon binary not executable")
	}
	if !IsInstalled(pluginDir) {
		t.Fatal("plugin not recorded as installed")
	}
}

func TestInstallTelegramMissingSourceFailsClosed(t *testing.T) {
	pluginDir := filepath.Join(t.TempDir(), "plugins", "telegram")
	if _, err := InstallTelegram(pluginDir, filepath.Join(t.TempDir(), "missing"), "2.9.2", time.Now()); err == nil {
		t.Fatal("expected error for missing source")
	}
	if IsInstalled(pluginDir) {
		t.Fatal("partial plugin state must not persist on failure")
	}
}

func TestUninstallTelegramRemovesOnlyPluginDir(t *testing.T) {
	base := t.TempDir()
	pluginDir := filepath.Join(base, "plugins", "telegram")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(base, "keep.txt")
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UninstallTelegram(pluginDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Fatal("plugin dir still present")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("unrelated state removed")
	}
}
