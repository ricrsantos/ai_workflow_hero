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
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encode plugin manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestFileName), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write plugin manifest: %w", err)
	}
	return nil
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
