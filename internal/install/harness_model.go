package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
)

// DefaultCursorModel is a documented example slug for tests and docs only.
// Runtime TUI/harness defaults are not pre-filled; users choose via /hero-model.
const DefaultCursorModel = "composer-2.5"

// HarnessConfig holds per-harness defaults in hero.json (ADR-030).
type HarnessConfig struct {
	Model           string `json:"model"`
	EnableFastModel bool   `json:"enable_fast_model"`
}

// DefaultHarnesses returns the install-time harness block shape (Cursor V1 only).
// Model is intentionally empty until the user selects /hero-model in the TUI.
func DefaultHarnesses() map[string]HarnessConfig {
	return map[string]HarnessConfig{
		"cursor": {
			Model:           "",
			EnableFastModel: false,
		},
	}
}

// ResolveHarnessModelSlug builds the kebab model id for Cursor Agent CLI --model
// (ADR-005 / ADR-030): enable_fast_model → "<id>-fast"; otherwise bare id.
func ResolveHarnessModelSlug(cfg HarnessConfig) string {
	id := strings.TrimSpace(cfg.Model)
	if id == "" {
		return ""
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
		if _, ok := hero.Harnesses[tool]; !ok {
			hero.Harnesses[tool] = def
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
			return cfg
		}
	}
	if def, ok := DefaultHarnesses()[toolID]; ok {
		return def
	}
	return HarnessConfig{EnableFastModel: false}
}

// HasDefaultHarnessModel reports whether the user selected a default model for toolID.
func HasDefaultHarnessModel(projectDir, toolID string) bool {
	hero, err := LoadHeroJSON(projectDir)
	if err != nil {
		return false
	}
	return strings.TrimSpace(HarnessConfigForTool(hero, toolID).Model) != ""
}

// HarnessModelSlugForProject loads hero.json and resolves the CLI model slug for toolID.
// Returns empty when the user has not chosen a default model yet.
func HarnessModelSlugForProject(projectDir, toolID string) string {
	hero, err := LoadHeroJSON(projectDir)
	if err != nil {
		return ""
	}
	return ResolveHarnessModelSlug(HarnessConfigForTool(hero, toolID))
}

// SaveHarnessModel updates harnesses.<toolID>.model in hero.json.
// The slug is stored as-is with enable_fast_model=false (slug is already resolved).
func SaveHarnessModel(projectDir, toolID, slug string) error {
	toolID = strings.TrimSpace(toolID)
	if toolID == "" {
		toolID = "cursor"
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return fmt.Errorf("model slug is required")
	}
	hero, err := LoadHeroJSON(projectDir)
	if err != nil {
		return err
	}
	if hero.Harnesses == nil {
		hero.Harnesses = make(map[string]HarnessConfig)
	}
	cfg := hero.Harnesses[toolID]
	cfg.Model = slug
	cfg.EnableFastModel = false
	hero.Harnesses[toolID] = cfg
	path := filepath.Join(projectDir, cursoradapter.HeroJSONPath)
	encoded, err := json.MarshalIndent(hero, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}
