package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
)

func TestResolveHarnessModelSlug(t *testing.T) {
	got := ResolveHarnessModelSlug(HarnessConfig{Model: "composer-2.5", EnableFastModel: false})
	if got != "composer-2.5" {
		t.Fatalf("slug=%q", got)
	}
	got = ResolveHarnessModelSlug(HarnessConfig{Model: "composer-2.5", EnableFastModel: true})
	if got != "composer-2.5-fast" {
		t.Fatalf("fast slug=%q", got)
	}
	got = ResolveHarnessModelSlug(HarnessConfig{Model: "composer-2.5-fast", EnableFastModel: true})
	if got != "composer-2.5-fast" {
		t.Fatalf("already-fast slug=%q", got)
	}
	got = ResolveHarnessModelSlug(HarnessConfig{})
	if got != "" {
		t.Fatalf("empty model slug=%q", got)
	}
}

func TestEnsureHarnessDefaults(t *testing.T) {
	var hero HeroJSON
	if !EnsureHarnessDefaults(&hero) {
		t.Fatal("expected modified")
	}
	cfg, ok := hero.Harnesses["cursor"]
	if !ok || cfg.Model != "" || cfg.EnableFastModel {
		t.Fatalf("cursor defaults=%+v", cfg)
	}
	if EnsureHarnessDefaults(&hero) {
		t.Fatal("expected no change on second call")
	}
	hero.Harnesses["cursor"] = HarnessConfig{Model: "", EnableFastModel: true}
	if EnsureHarnessDefaults(&hero) {
		t.Fatal("expected no change when model empty but harness exists")
	}
	if hero.Harnesses["cursor"].Model != "" {
		t.Fatalf("model should stay empty, got=%q", hero.Harnesses["cursor"].Model)
	}
	if !hero.Harnesses["cursor"].EnableFastModel {
		t.Fatal("expected enable_fast_model preserved")
	}
}

func TestHarnessModelSlugForProject(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, filepath.Dir(cursoradapter.HeroJSONPath))
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := HeroJSON{
		CLI: CLIInfo{Version: "1.0.0", Tools: []string{"cursor"}},
		Harnesses: map[string]HarnessConfig{
			"cursor": {Model: "composer-2.5", EnableFastModel: true},
		},
	}
	data, _ := json.MarshalIndent(hero, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, cursoradapter.HeroJSONPath), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := HarnessModelSlugForProject(dir, "cursor"); got != "composer-2.5-fast" {
		t.Fatalf("slug=%q", got)
	}
	if got := HarnessModelSlugForProject(t.TempDir(), "cursor"); got != "" {
		t.Fatalf("missing file default=%q", got)
	}
}

func TestHasDefaultHarnessModel(t *testing.T) {
	dir := t.TempDir()
	if HasDefaultHarnessModel(dir, "cursor") {
		t.Fatal("expected false without hero.json")
	}
	cfgDir := filepath.Join(dir, filepath.Dir(cursoradapter.HeroJSONPath))
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := HeroJSON{
		CLI:       CLIInfo{Version: "1.0.0", Tools: []string{"cursor"}},
		Harnesses: DefaultHarnesses(),
	}
	data, _ := json.MarshalIndent(hero, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, cursoradapter.HeroJSONPath), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if HasDefaultHarnessModel(dir, "cursor") {
		t.Fatal("expected false with empty model")
	}
	if err := SaveHarnessModel(dir, "cursor", "composer-2.5"); err != nil {
		t.Fatal(err)
	}
	if !HasDefaultHarnessModel(dir, "cursor") {
		t.Fatal("expected true after SaveHarnessModel")
	}
}

func TestSaveHarnessModel(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, filepath.Dir(cursoradapter.HeroJSONPath))
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := HeroJSON{
		CLI: CLIInfo{Version: "1.0.0", Tools: []string{"cursor"}},
		Harnesses: map[string]HarnessConfig{
			"cursor": {Model: "composer-2.5", EnableFastModel: true},
		},
	}
	data, _ := json.MarshalIndent(hero, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, cursoradapter.HeroJSONPath), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveHarnessModel(dir, "cursor", "cursor-grok-4.5-high"); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := loaded.Harnesses["cursor"]
	if cfg.Model != "cursor-grok-4.5-high" {
		t.Fatalf("model=%q", cfg.Model)
	}
	if cfg.EnableFastModel {
		t.Fatal("enable_fast_model should be false after SaveHarnessModel")
	}
	if got := HarnessModelSlugForProject(dir, "cursor"); got != "cursor-grok-4.5-high" {
		t.Fatalf("slug=%q", got)
	}
}

func TestSaveHarnessModel_RequiresSlug(t *testing.T) {
	if err := SaveHarnessModel(t.TempDir(), "cursor", "  "); err == nil {
		t.Fatal("expected error")
	}
}
