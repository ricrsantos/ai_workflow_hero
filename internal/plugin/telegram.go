package plugin

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/telegram"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

// ErrUnsupportedPlugin is returned for unknown plugin names.
type ErrUnsupportedPlugin struct{ Name string }

func (e ErrUnsupportedPlugin) Error() string {
	return fmt.Sprintf("unsupported plugin %q", e.Name)
}

// InstallTelegram installs the Telegram plugin: it copies the daemon binary from
// daemonSrc into pluginDir (0755) and writes the manifest with the matching Hero
// version and protocol version (ADR-059). Both artifacts are staged before the
// pair is made visible, and the daemon is restored if manifest installation
// fails.
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
	dst, err := os.CreateTemp(pluginDir, ".hero-telegram-daemon-*")
	if err != nil {
		return Manifest{}, fmt.Errorf("create temporary daemon binary: %w", err)
	}
	tmpPath := dst.Name()
	defer os.Remove(tmpPath)
	if err := dst.Chmod(0o755); err != nil {
		_ = dst.Close()
		return Manifest{}, fmt.Errorf("chmod temporary daemon binary: %w", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return Manifest{}, fmt.Errorf("copy daemon binary: %w", err)
	}
	if err := dst.Sync(); err != nil {
		_ = dst.Close()
		return Manifest{}, fmt.Errorf("sync daemon binary: %w", err)
	}
	if err := dst.Close(); err != nil {
		return Manifest{}, fmt.Errorf("close daemon binary: %w", err)
	}
	m := Manifest{
		Name:            telegram.PluginName,
		Version:         version,
		ProtocolVersion: ipc.ProtocolVersion,
		DaemonPath:      dstPath,
		InstalledAt:     now.UTC().Format(time.RFC3339),
	}
	manifestData, err := marshalManifest(m)
	if err != nil {
		return Manifest{}, fmt.Errorf("encode plugin manifest: %w", err)
	}
	manifestPath := filepath.Join(pluginDir, ManifestFileName)
	manifestTmp, err := stageFile(pluginDir, ".hero-manifest-*", manifestData, 0o644)
	if err != nil {
		return Manifest{}, fmt.Errorf("stage plugin manifest: %w", err)
	}
	defer os.Remove(manifestTmp)

	var oldDaemonTmp string
	if info, statErr := os.Stat(dstPath); statErr == nil {
		oldData, readErr := os.ReadFile(dstPath)
		if readErr != nil {
			return Manifest{}, fmt.Errorf("backup existing daemon binary: %w", readErr)
		}
		oldDaemonTmp, err = stageFile(pluginDir, ".hero-telegram-daemon-old-*", oldData, info.Mode().Perm())
		if err != nil {
			return Manifest{}, fmt.Errorf("stage existing daemon backup: %w", err)
		}
		defer os.Remove(oldDaemonTmp)
	} else if !os.IsNotExist(statErr) {
		return Manifest{}, fmt.Errorf("inspect existing daemon binary: %w", statErr)
	}

	if err := os.Rename(tmpPath, dstPath); err != nil {
		return Manifest{}, fmt.Errorf("install daemon binary: %w", err)
	}
	if err := os.Rename(manifestTmp, manifestPath); err != nil {
		if oldDaemonTmp != "" {
			if rollbackErr := os.Rename(oldDaemonTmp, dstPath); rollbackErr != nil {
				return Manifest{}, fmt.Errorf("install plugin manifest: %w (rollback daemon: %v)", err, rollbackErr)
			}
			oldDaemonTmp = ""
		} else if rollbackErr := os.Remove(dstPath); rollbackErr != nil && !os.IsNotExist(rollbackErr) {
			return Manifest{}, fmt.Errorf("install plugin manifest: %w (remove daemon: %v)", err, rollbackErr)
		}
		return Manifest{}, fmt.Errorf("install plugin manifest: %w", err)
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
