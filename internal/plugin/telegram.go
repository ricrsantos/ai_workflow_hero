package plugin

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/telegram"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

// ErrUnsupportedPlugin is returned for unknown plugin names.
type ErrUnsupportedPlugin struct{ Name string }

func (e ErrUnsupportedPlugin) Error() string {
	return fmt.Sprintf("unsupported plugin %q", e.Name)
}

// DaemonSource locates the platform daemon binary shipped alongside the running
// hero executable (release layout). It fails when the artifact is missing so
// install fails closed (telegram-plugin R1).
func DaemonSource() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve hero executable: %w", err)
	}
	name := telegram.DaemonBinaryName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	candidate := filepath.Join(filepath.Dir(exe), name)
	if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
		return candidate, nil
	}
	return "", fmt.Errorf("telegram daemon artifact %q not found next to hero executable; reinstall a matching Hero release", name)
}

// InstallTelegram installs the Telegram plugin: it copies the daemon binary from
// daemonSrc into pluginDir (0755) and writes the manifest with the matching Hero
// version and protocol version (ADR-059). It fails without partial state when the
// source is missing.
func InstallTelegram(pluginDir, daemonSrc, version string, now time.Time) (Manifest, error) {
	src, err := os.Open(daemonSrc)
	if err != nil {
		return Manifest{}, fmt.Errorf("open daemon artifact: %w", err)
	}
	defer src.Close()

	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("create plugin dir: %w", err)
	}
	dstPath := filepath.Join(pluginDir, telegram.DaemonBinaryName)
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return Manifest{}, fmt.Errorf("create daemon binary: %w", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return Manifest{}, fmt.Errorf("copy daemon binary: %w", err)
	}
	if err := dst.Close(); err != nil {
		return Manifest{}, fmt.Errorf("close daemon binary: %w", err)
	}
	if err := os.Chmod(dstPath, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("chmod daemon binary: %w", err)
	}

	m := Manifest{
		Name:            telegram.PluginName,
		Version:         version,
		ProtocolVersion: ipc.ProtocolVersion,
		DaemonPath:      dstPath,
		InstalledAt:     now.UTC().Format(time.RFC3339),
	}
	if err := Save(pluginDir, m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// UninstallTelegram removes the Telegram plugin directory (metadata + daemon
// binary). It leaves unrelated Hero state untouched (cli-deterministic-command-
// suite R1).
func UninstallTelegram(pluginDir string) error {
	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("remove plugin dir: %w", err)
	}
	return nil
}
