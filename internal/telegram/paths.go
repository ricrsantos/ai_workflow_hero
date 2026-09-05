// Package telegram holds the shared path layout and constants for the optional
// Telegram remote-interface plugin (PRD-C09-001; ADR-059). It deliberately
// contains no networking, IPC, or secret-handling logic: only path resolution
// under the per-OS-user Hero state directory (~/.workflow-hero).
package telegram

import (
	"fmt"
	"os"
	"path/filepath"
)

// PluginName is the canonical name of the official Telegram plugin.
const PluginName = "telegram"

// DaemonBinaryName is the per-platform daemon executable name.
const DaemonBinaryName = "hero-telegram-daemon"

// UserHeroDir resolves ~/.workflow-hero (the user-global Hero state directory).
func UserHeroDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if home == "" {
		return "", fmt.Errorf("home directory is empty")
	}
	return filepath.Join(home, ".workflow-hero"), nil
}

// PluginsDir resolves ~/.workflow-hero/plugins.
func PluginsDir() (string, error) {
	root, err := UserHeroDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "plugins"), nil
}

// PluginDir resolves the install directory for a named plugin
// (e.g. ~/.workflow-hero/plugins/telegram).
func PluginDir(name string) (string, error) {
	dir, err := PluginsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// LogsDir resolves the user-global rotating log directory (~/.workflow-hero/logs).
func LogsDir() (string, error) {
	root, err := UserHeroDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "logs"), nil
}

// RunDir resolves the OS-user-private IPC endpoint directory
// (~/.workflow-hero/run).
func RunDir() (string, error) {
	root, err := UserHeroDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "run"), nil
}

// SocketPath resolves the IPC socket path for a named endpoint.
func SocketPath(name string) (string, error) {
	dir, err := RunDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".sock"), nil
}

// DaemonLogPath resolves the daemon global log (~/.workflow-hero/logs/telegram-daemon.log).
func DaemonLogPath() (string, error) {
	dir, err := LogsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "telegram-daemon.log"), nil
}
