package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
)

// DefaultCursorModel is the V1 default Agent CLI model for the Cursor harness.
const DefaultCursorModel = "composer-2.5"

// HarnessConfig holds per-harness defaults in hero.json (ADR-030).
type HarnessConfig struct {
	Model           string `json:"model"`
	EnableFastModel bool   `json:"enable_fast_model"`
}

// DefaultHarnesses returns the install-time harness defaults (Cursor V1 only).
func DefaultHarnesses() map[string]HarnessConfig {
	return map[string]HarnessConfig{
		"cursor": {
			Model:           DefaultCursorModel,
			EnableFastModel: false,
		},
	}
}

// ResolveHarnessModelSlug builds the kebab model id for Cursor Agent CLI --model
// (ADR-005 / ADR-030): enable_fast_model → "<id>-fast"; otherwise bare id.
func ResolveHarnessModelSlug(cfg HarnessConfig) string {
	id := strings.TrimSpace(cfg.Model)
	if id == "" {
		id = DefaultCursorModel
	}
	if !cfg.EnableFastModel {
		return id
	}
	if strings.HasSuffix(id, "-fast") {
		return id
	}
	return id + "-fast"
}

// EnsureHarnessDefaults merges missing harness defaults into hero. Returns true if hero was modified.
func EnsureHarnessDefaults(hero *HeroJSON) bool {
	if hero == nil {
		return false
	}
	modified := false
	if hero.Harnesses == nil {
		hero.Harnesses = make(map[string]HarnessConfig)
		modified = true
	}
	for tool, def := range DefaultHarnesses() {
		cur, ok := hero.Harnesses[tool]
		if !ok {
			hero.Harnesses[tool] = def
			modified = true
			continue
		}
		if strings.TrimSpace(cur.Model) == "" {
			cur.Model = def.Model
			hero.Harnesses[tool] = cur
			modified = true
		}
	}
	return modified
}

// LoadHeroJSON reads and parses .workflow-hero/config/hero.json.
func LoadHeroJSON(projectDir string) (HeroJSON, error) {
	path := filepath.Join(projectDir, cursoradapter.HeroJSONPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return HeroJSON{}, err
	}
	var hero HeroJSON
	if err := json.Unmarshal(data, &hero); err != nil {
		return HeroJSON{}, err
	}
	return hero, nil
}

// HarnessConfigForTool returns the configured (or default) harness block for toolID.
func HarnessConfigForTool(hero HeroJSON, toolID string) HarnessConfig {
	toolID = strings.TrimSpace(toolID)
	if toolID == "" {
		toolID = "cursor"
	}
	if hero.Harnesses != nil {
		if cfg, ok := hero.Harnesses[toolID]; ok {
			if strings.TrimSpace(cfg.Model) == "" {
				cfg.Model = DefaultCursorModel
			}
			return cfg
		}
	}
	if def, ok := DefaultHarnesses()[toolID]; ok {
		return def
	}
	return HarnessConfig{Model: DefaultCursorModel, EnableFastModel: false}
}

// HarnessModelSlugForProject loads hero.json and resolves the CLI model slug for toolID.
// On read errors, returns the Cursor default slug.
func HarnessModelSlugForProject(projectDir, toolID string) string {
	hero, err := LoadHeroJSON(projectDir)
	if err != nil {
		return ResolveHarnessModelSlug(DefaultHarnesses()["cursor"])
	}
	return ResolveHarnessModelSlug(HarnessConfigForTool(hero, toolID))
}
