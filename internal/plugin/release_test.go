package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/telegram"
)

func TestDaemonArtifactFileName(t *testing.T) {
	got := DaemonArtifactFileName("3.0.0", "linux", "amd64")
	want := "hero-telegram-daemon_v3.0.0_linux_amd64"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDaemonReleaseURL(t *testing.T) {
	got := DaemonReleaseURL(DefaultReleaseRepo, "3.0.0", "linux", "amd64")
	want := "https://github.com/ricrsantos/ai_workflow_hero/releases/download/v3.0.0/hero-telegram-daemon_v3.0.0_linux_amd64"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDownloadDaemonArtifactSuccess(t *testing.T) {
	const body = "#!/bin/sh\nexit 0\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dl/v3.0.0/hero-telegram-daemon_v3.0.0_linux_amd64" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	path, cleanup, err := DownloadDaemonArtifact(context.Background(), DownloadOptions{
		Version:        "3.0.0",
		GOOS:           "linux",
		GOARCH:         "amd64",
		ReleaseBaseURL: srv.URL + "/dl",
		HTTPClient:     srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != body {
		t.Fatalf("unexpected body: %q", data)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Fatal("expected executable temp file")
	}
}

func TestDownloadDaemonArtifactHTTP404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, _, err := DownloadDaemonArtifact(context.Background(), DownloadOptions{
		Version:        "3.0.0",
		GOOS:           "linux",
		GOARCH:         "amd64",
		ReleaseBaseURL: srv.URL + "/dl",
		HTTPClient:     srv.Client(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDownloadDaemonArtifactUnsupportedPlatform(t *testing.T) {
	_, _, err := DownloadDaemonArtifact(context.Background(), DownloadOptions{
		Version: "3.0.0",
		GOOS:    "windows",
		GOARCH:  "amd64",
	})
	if err == nil {
		t.Fatal("expected error for unsupported platform")
	}
}

func TestInstallTelegramFromRelease(t *testing.T) {
	const body = "#!/bin/sh\necho daemon\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)

	pluginDir, err := telegram.PluginDir(telegram.PluginName)
	if err != nil {
		t.Fatal(err)
	}

	src, cleanup, err := DownloadDaemonArtifact(context.Background(), DownloadOptions{
		Version:        "3.0.0",
		GOOS:           "linux",
		GOARCH:         "amd64",
		ReleaseBaseURL: srv.URL + "/dl",
		HTTPClient:     srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	m, err := InstallTelegram(pluginDir, src, "3.0.0", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != "3.0.0" {
		t.Fatalf("version=%q", m.Version)
	}
	if _, err := os.Stat(filepath.Join(pluginDir, telegram.DaemonBinaryName)); err != nil {
		t.Fatalf("daemon not installed: %v", err)
	}
}
