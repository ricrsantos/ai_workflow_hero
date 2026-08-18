package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func mkdirAll(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

func TestHeroJSONLoadsWithoutModelProperties(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := mkdirAll(cfgDir); err != nil {
		t.Fatal(err)
	}
	raw := `{
  "cli": {"version": "2.0.0", "tools": ["cursor"]},
  "harnesses": {
    "cursor": {"enabled": true, "model": "composer-2.5", "enable_fast_model": false},
    "opencode": {"enabled": true}
  },
  "freechat_default": {"harness": "cursor", "model": "composer-2.5"}
}`
	if err := writeFile(filepath.Join(cfgDir, "hero.json"), []byte(raw)); err != nil {
		t.Fatal(err)
	}
	hero, err := LoadHeroJSON(dir)
	if err != nil {
		t.Fatalf("existing C4 hero.json must load without model_properties: %v", err)
	}
	if hero.ModelProperties != nil {
		t.Fatal("missing model_properties must unmarshal as empty/nil")
	}
	h, m := GetFreechatDefault(hero)
	if h != "cursor" || m != "composer-2.5" {
		t.Fatalf("pair lost: %s/%s", h, m)
	}
}

func TestModelPropertiesJSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := mkdirAll(cfgDir); err != nil {
		t.Fatal(err)
	}
	raw := `{
  "harnesses": {
    "cursor": {"enabled": true, "model": "composer-2.5"},
    "opencode": {"enabled": true, "model": "opencode-go/deepseek-v4-pro"}
  },
  "freechat_default": {"harness": "opencode", "model": "opencode-go/deepseek-v4-pro"},
  "model_properties": {
    "opencode": {
      "opencode-go/deepseek-v4-pro": {"fs": "true", "th": "max", "ef": "high"}
    },
    "cursor": {
      "composer-2.5": {"fs": "false"}
    }
  }
}`
	if err := writeFile(filepath.Join(cfgDir, "hero.json"), []byte(raw)); err != nil {
		t.Fatal(err)
	}
	hero, err := LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	props := PairProperties(hero, "opencode", "opencode-go/deepseek-v4-pro")
	if props["fs"] != "true" || props["th"] != "max" || props["ef"] != "high" {
		t.Fatalf("opencode pair props=%v", props)
	}
	cursorProps := PairProperties(hero, "cursor", "composer-2.5")
	if cursorProps["fs"] != "false" || len(cursorProps) != 1 {
		t.Fatalf("cursor pair props=%v", cursorProps)
	}
	// Independent pairs never leak values across harnesses/models.
	if p := PairProperties(hero, "cursor", "opencode-go/deepseek-v4-pro"); len(p) != 0 {
		t.Fatalf("cross-pair leak: %v", p)
	}
}

func TestPairPropertiesCloneIsDeep(t *testing.T) {
	hero := HeroJSON{}
	SetPairProperties(&hero, "opencode", "m1", map[string]string{"fs": "true"})
	props := PairProperties(hero, "opencode", "m1")
	props["fs"] = "false"
	if PairProperties(hero, "opencode", "m1")["fs"] != "true" {
		t.Fatal("read helper must return a copy")
	}
}

func TestSetPairPropertiesRemovesEmptyEntries(t *testing.T) {
	hero := HeroJSON{}
	SetPairProperties(&hero, "opencode", "m1", map[string]string{"fs": "true"})
	SetPairProperties(&hero, "opencode", "m2", map[string]string{"th": "max"})
	SetPairProperties(&hero, "opencode", "m1", nil)
	if len(hero.ModelProperties["opencode"]) != 1 {
		t.Fatalf("m1 entry must be removed: %v", hero.ModelProperties)
	}
	SetPairProperties(&hero, "opencode", "m2", nil)
	if hero.ModelProperties != nil {
		t.Fatalf("empty harness map must be removed: %v", hero.ModelProperties)
	}
}

func TestEffectivePairPropertiesSeedsLegacyFast(t *testing.T) {
	hero := HeroJSON{Harnesses: map[string]HarnessConfig{
		"cursor": {Enabled: true, Model: "composer-2.5", EnableFastModel: true},
	}}
	// Legacy fast seeds fs=true when no C5 entry exists.
	props := EffectivePairProperties(hero, "cursor", "composer-2.5")
	if props[harness.PropertyFast] != "true" {
		t.Fatalf("legacy fast must seed fs=true: %v", props)
	}
	// A different model does not inherit the legacy fast flag.
	if props := EffectivePairProperties(hero, "cursor", "other-model"); len(props) != 0 {
		t.Fatalf("unrelated model must not inherit legacy fast: %v", props)
	}
	// A new C5 entry wins over the legacy flag.
	SetPairProperties(&hero, "cursor", "composer-2.5", map[string]string{"fs": "false"})
	props = EffectivePairProperties(hero, "cursor", "composer-2.5")
	if props["fs"] != "false" {
		t.Fatalf("explicit entry must win: %v", props)
	}
}

func TestFutureKeysPreservedInJSON(t *testing.T) {
	hero := HeroJSON{}
	SetPairProperties(&hero, "opencode", "m1", map[string]string{
		"fs": "true", "future_key": "future_value",
	})
	encoded, err := json.Marshal(hero)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"future_key":"future_value"`) && !strings.Contains(string(encoded), `"future_key": "future_value"`) {
		t.Fatalf("future keys must remain stored: %s", encoded)
	}
}

func TestModelPropertiesDoesNotDisturbC4Fields(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := mkdirAll(cfgDir); err != nil {
		t.Fatal(err)
	}
	raw := `{
  "cli": {"version": "2.0.0", "tools": ["cursor", "opencode"]},
  "assets": {"version": "2.0.0"},
  "harnesses": {
    "cursor": {"enabled": true, "model": "composer-2.5", "enable_fast_model": true},
    "opencode": {"enabled": true, "model": ""}
  },
  "freechat_default": {"harness": "cursor", "model": "composer-2.5"}
}`
	path := filepath.Join(cfgDir, "hero.json")
	if err := writeFile(path, []byte(raw)); err != nil {
		t.Fatal(err)
	}
	hero, err := LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	SetPairProperties(&hero, "cursor", "composer-2.5", map[string]string{"fs": "false"})
	if err := saveHeroJSON(dir, hero); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.CLI.Tools) != 2 || reloaded.CLI.Version != "2.0.0" {
		t.Fatalf("cli fields disturbed: %+v", reloaded.CLI)
	}
	if !reloaded.Harnesses["cursor"].EnableFastModel {
		t.Fatal("legacy enable_fast_model must remain readable")
	}
	if reloaded.Harnesses["opencode"].Model != "" {
		t.Fatal("opencode model must remain empty")
	}
	if PairProperties(reloaded, "cursor", "composer-2.5")["fs"] != "false" {
		t.Fatal("committed pair property lost")
	}
}
