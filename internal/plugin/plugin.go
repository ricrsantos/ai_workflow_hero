// Package plugin manages optional official Hero plugins and their manifests.
// A plugin lives under ~/.workflow-hero/plugins/<name>/ with a manifest.json
// describing its version, protocol version, and daemon path (ADR-059). Plugin
// install is explicit (`hero plugin install telegram`); a normal `hero install`
// never enables a plugin.
package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ManifestFileName is the plugin metadata file name within a plugin directory.
const ManifestFileName = "manifest.json"

// Manifest is the on-disk plugin metadata (no secrets).
type Manifest struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	ProtocolVersion int    `json:"protocol_version"`
	DaemonPath      string `json:"daemon_path,omitempty"`
	InstalledAt     string `json:"installed_at"`
}

// Load reads the manifest for a plugin directory. It returns os.ErrNotExist when
// the plugin is not installed.
func Load(dir string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, ManifestFileName))
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse plugin manifest: %w", err)
	}
	return m, nil
}

// Save writes the manifest, creating the directory when needed.
func Save(dir string, m Manifest) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create plugin dir: %w", err)
	}
	data, err := marshalManifest(m)
	if err != nil {
		return fmt.Errorf("encode plugin manifest: %w", err)
	}
	if err := atomicWriteFile(filepath.Join(dir, ManifestFileName), data, 0o644); err != nil {
		return fmt.Errorf("write plugin manifest: %w", err)
	}
	return nil
}

func marshalManifest(m Manifest) ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func stageFile(dir, pattern string, data []byte, mode os.FileMode) (string, error) {
	tmp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	if err := tmp.Chmod(mode); err != nil {
		cleanup()
		return "", err
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

// atomicWriteFile writes a complete file beside its destination before making
// it visible. This prevents readers from observing a truncated manifest during
// plugin upgrades.
func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	tmpPath, err := stageFile(filepath.Dir(path), ".hero-atomic-*", data, mode)
	if err != nil {
		return err
	}
	defer os.Remove(tmpPath)
	return os.Rename(tmpPath, path)
}

// IsInstalled reports whether a manifest exists under dir.
func IsInstalled(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ManifestFileName))
	return err == nil
}

// List returns the manifests of all installed plugins under baseDir, sorted by
// name.
func List(baseDir string) ([]Manifest, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read plugins dir: %w", err)
	}
	var out []Manifest
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, err := Load(filepath.Join(baseDir, e.Name()))
		if err != nil {
			continue // not an installed plugin
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
