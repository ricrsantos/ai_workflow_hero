package plugin

import (
	"os"

	"github.com/ricrsantos/ai_workflow_hero/internal/telegram"
)

// Health describes the Telegram plugin install state for doctor/status
// (telegram-plugin R2). It carries no secrets: the manifest stores only the
// install version, protocol version, and daemon path (ADR-059/062).
type Health struct {
	Installed       bool   `json:"installed"`
	DaemonPath      string `json:"daemon_path,omitempty"`
	DaemonExists    bool   `json:"daemon_exists"`
	Version         string `json:"version,omitempty"`
	ProtocolVersion int    `json:"protocol_version,omitempty"`
	// VersionMatches reports whether the installed plugin version matches the
	// running hero binary version.
	VersionMatches bool `json:"version_matches"`
}

// CheckTelegramHealth resolves the Telegram plugin health for doctor/status.
// An uninstalled plugin yields Health{} with no error (Telegram is optional).
func CheckTelegramHealth(binaryVersion string) (Health, error) {
	dir, err := telegram.PluginDir(telegram.PluginName)
	if err != nil {
		return Health{}, err
	}
	if !IsInstalled(dir) {
		return Health{}, nil
	}
	m, err := Load(dir)
	if err != nil {
		// Manifest unreadable: still report installed so doctor can warn.
		return Health{Installed: true}, nil
	}
	h := Health{
		Installed:       true,
		DaemonPath:      m.DaemonPath,
		Version:         m.Version,
		ProtocolVersion: m.ProtocolVersion,
		VersionMatches:  m.Version == binaryVersion,
	}
	if h.DaemonPath != "" {
		if _, err := os.Stat(h.DaemonPath); err == nil {
			h.DaemonExists = true
		}
	}
	return h, nil
}
