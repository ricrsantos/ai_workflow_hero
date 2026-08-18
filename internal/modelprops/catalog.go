// Package modelprops owns the C5 model-property vertical slice: local catalog
// parsing, normalized capability snapshots, source resolution/reconciliation,
// and background refresh orchestration (PRD-C05-001; ADR-038/039).
package modelprops

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"gopkg.in/yaml.v3"
)

// CatalogProperty is one per-model property block in assets/models/*.yml.
// Accepted values are dynamic strings preserved in source order (no fixed enum).
type CatalogProperty struct {
	Available   bool     `yaml:"available"`
	Values      []string `yaml:"values"`
	Default     string   `yaml:"default"`
	HasDefault  bool
	HasProperty bool
}

// UnmarshalYAML keeps a strict, panic-free parse of the optional property block.
// Malformed or unknown entries are ignored without failing the catalog.
func (p *CatalogProperty) UnmarshalYAML(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	p.HasProperty = true
	var raw struct {
		Available bool     `yaml:"available"`
		Values    []string `yaml:"values"`
		Default   string   `yaml:"default"`
	}
	if err := node.Decode(&raw); err != nil {
		slog.Debug("modelprops catalog property malformed", "error", err)
		return nil
	}
	p.Available = raw.Available
	p.Values = raw.Values
	p.Default = strings.TrimSpace(raw.Default)
	p.HasDefault = nodeHasKey(node, "default")
	return nil
}

func nodeHasKey(node *yaml.Node, key string) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return true
		}
	}
	return false
}

// CatalogModel is one model row from a catalog file. Pricing fields are ignored
// by this parser so pricing-only entries keep loading unchanged.
type CatalogModel struct {
	// Provider is the catalog file's native provider marker.  It is used only
	// to keep catalog model rows scoped to the harness that owns them; the
	// native model ID remains the map key and is never rewritten.
	Provider   string
	Properties map[string]CatalogProperty
}

// Catalog maps native model IDs to catalog metadata.
type Catalog map[string]CatalogModel

type catalogFile struct {
	Provider string                  `yaml:"provider"`
	Models   map[string]catalogEntry `yaml:"models"`
}

type catalogEntry struct {
	Properties map[string]CatalogProperty `yaml:"properties"`
}

// CatalogPropertyKeys returns the property keys defined for a model in
// deterministic order (C5 keys first, then remaining keys sorted).
func CatalogPropertyKeys(m CatalogModel) []string {
	keys := make([]string, 0, len(m.Properties))
	for k := range m.Properties {
		keys = append(keys, k)
	}
	rank := func(k string) int {
		switch k {
		case "fs":
			return 0
		case "th":
			return 1
		case "ef":
			return 2
		default:
			return 3
		}
	}
	sort.SliceStable(keys, func(i, j int) bool {
		ri, rj := rank(keys[i]), rank(keys[j])
		if ri != rj {
			return ri < rj
		}
		return keys[i] < keys[j]
	})
	return keys
}

// LoadCatalogFromFS parses every .yml file under dir in fsys into a Catalog.
// Malformed files and unknown keys are ignored without panicking. Later files
// win per model ID, matching the installed-overlay semantics.
func LoadCatalogFromFS(fsys fs.FS, dir string) Catalog {
	cat := make(Catalog)
	if fsys == nil {
		return cat
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		slog.Debug("modelprops catalog read failed", "dir", dir, "error", err)
		return cat
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".yml") {
			continue
		}
		data, err := fs.ReadFile(fsys, dir+"/"+e.Name())
		if err != nil {
			slog.Debug("modelprops catalog file read failed", "file", e.Name(), "error", err)
			continue
		}
		mergeCatalogYAML(cat, data)
	}
	return cat
}

// LoadCatalogFromDir parses every .yml file in an installed directory.
func LoadCatalogFromDir(dir string) Catalog {
	cat := make(Catalog)
	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Debug("modelprops catalog dir read failed", "dir", dir, "error", err)
		return cat
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".yml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		mergeCatalogYAML(cat, data)
	}
	return cat
}

// LoadCatalog loads embedded assets.FS plus the installed project overlay.
func LoadCatalog(embedded fs.FS, projectDir string) Catalog {
	cat := LoadCatalogFromFS(embedded, "models")
	if strings.TrimSpace(projectDir) != "" {
		installed := LoadCatalogFromDir(filepath.Join(projectDir, cursor.HeroModelsDir))
		for id, model := range installed {
			cat[id] = model
		}
	}
	return cat
}

func mergeCatalogYAML(cat Catalog, data []byte) {
	var file catalogFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		slog.Debug("modelprops catalog yaml skipped", "error", err)
		return
	}
	for id, entry := range file.Models {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		model := CatalogModel{
			Provider:   strings.TrimSpace(file.Provider),
			Properties: map[string]CatalogProperty{},
		}
		for key, prop := range entry.Properties {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			model.Properties[key] = prop
		}
		// Installed overlays commonly contain only capability metadata. Keep
		// the embedded provider marker when the overlay omits it so model-row
		// fallback remains harness-scoped.
		if model.Provider == "" {
			if previous, ok := cat[id]; ok {
				model.Provider = previous.Provider
			}
		}
		cat[id] = model
	}
}

// CatalogValues returns the property block for a model/key, preserving order.
func (c Catalog) CatalogValues(modelID, key string) (CatalogProperty, bool) {
	m, ok := c[strings.TrimSpace(modelID)]
	if !ok {
		return CatalogProperty{}, false
	}
	p, ok := m.Properties[key]
	return p, ok
}

// HasModel reports whether the catalog supplies a row for the model ID.
func (c Catalog) HasModel(modelID string) bool {
	_, ok := c[strings.TrimSpace(modelID)]
	return ok
}

// ModelsForHarness returns native model IDs that can be offered for a
// harness when live listing is unavailable.  Provider markers are preferred;
// the ID-shape fallback keeps older pricing catalogs (which predate provider
// metadata in this package) useful for Cursor/OpenCode selection.
func (c Catalog) ModelsForHarness(harnessID string) []string {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if harnessID == "" {
		return nil
	}
	ids := make([]string, 0, len(c))
	for id, model := range c {
		provider := strings.TrimSpace(strings.ToLower(model.Provider))
		matches := false
		switch harnessID {
		case "cursor":
			matches = provider == "cursor" || provider == ""
		case "opencode":
			// OpenCode native IDs are provider/model pairs.  A catalog may
			// explicitly mark a provider as opencode or opencode-go, but
			// provider pricing files also contain native slash IDs.
			matches = provider == "opencode" || provider == "opencode-go" ||
				(provider != "cursor" && strings.Contains(id, "/"))
		default:
			matches = provider == harnessID
		}
		if matches {
			ids = append(ids, strings.TrimSpace(id))
		}
	}
	sort.Strings(ids)
	return ids
}
