package install_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

func TestToolsFlagRemovedMessage(t *testing.T) {
	if !strings.Contains(install.ToolsFlagRemovedMsg, "--tools is not supported") {
		t.Fatalf("msg=%q", install.ToolsFlagRemovedMsg)
	}
	if !strings.Contains(install.ToolsFlagRemovedSuggestion, "/harness") {
		t.Fatalf("suggestion=%q", install.ToolsFlagRemovedSuggestion)
	}
}

func TestMigrateHarnessStateFromLegacyTools(t *testing.T) {
	hero := install.HeroJSON{
		CLI: install.CLIInfo{Tools: []string{"cursor"}},
	}
	if !install.MigrateHarnessState(&hero) {
		t.Fatal("expected migration")
	}
	if !install.IsHarnessEnabled(hero, "cursor") {
		t.Fatal("cursor should be enabled")
	}
	if install.IsHarnessEnabled(hero, "opencode") {
		t.Fatal("opencode should stay disabled on 1.x upgrade")
	}
	if hero.FreechatDefault.Model != "" {
		t.Fatalf("migration must not invent a default model, got %q", hero.FreechatDefault.Model)
	}
}

func TestGetFreechatDefaultDoesNotInventModel(t *testing.T) {
	hero := install.HeroJSON{
		Harnesses: install.HarnessesFromSelection([]string{"cursor"}),
	}
	h, m := install.GetFreechatDefault(hero)
	if h != "cursor" {
		t.Fatalf("harness=%q", h)
	}
	if m != "" {
		t.Fatalf("model=%q want empty until /hero-model", m)
	}
}

func TestListEnabledHarnesses(t *testing.T) {
	hero := install.HeroJSON{
		Harnesses: install.HarnessesFromSelection([]string{"cursor", "opencode"}),
	}
	got := install.ListEnabledHarnesses(hero)
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}

// hero-json-harness-state: Codex is a supported id and defaults to disabled on a
// fresh install even when the user never selects it (PRD-C06-001 §4.10; ADR-048).
func TestSupportedHarnessIDsIncludesCodex(t *testing.T) {
	got := install.SupportedHarnessIDs
	if !slicesContains(got, "cursor") || !slicesContains(got, "opencode") || !slicesContains(got, "codex") {
		t.Fatalf("SupportedHarnessIDs = %v, want cursor+opencode+codex", got)
	}
}

func TestHarnessesFromSelection_CodexDisabledByDefault(t *testing.T) {
	harnesses := install.HarnessesFromSelection([]string{"cursor"})
	cfg, ok := harnesses["codex"]
	if !ok {
		t.Fatal("harnesses.codex must be present even when not selected")
	}
	if cfg.Enabled {
		t.Fatal("harnesses.codex.enabled must be false when only Cursor is selected")
	}
	if cfg.Model != "" {
		t.Fatalf("harnesses.codex.model must stay empty, got %q", cfg.Model)
	}
	if !harnesses["cursor"].Enabled {
		t.Fatal("cursor must stay enabled")
	}
	def := install.DefaultFreechatDefault(harnesses)
	if def.Harness == "codex" {
		t.Fatal("freechat default must not point to a disabled codex harness")
	}
}

// hero-json-harness-state: upgrade from a 2.4.x hero.json adds harnesses.codex
// disabled and preserves existing Cursor/OpenCode settings (ADR-048).
func TestMigrateHarnessState_AddsCodexDisabledFrom24x(t *testing.T) {
	hero := install.HeroJSON{
		CLI: install.CLIInfo{Tools: []string{"cursor", "opencode"}},
		Harnesses: map[string]install.HarnessConfig{
			"cursor":   {Enabled: true, Model: "composer-2.5"},
			"opencode": {Enabled: true, Model: "opencode-go/deepseek-v4-flash"},
		},
	}
	if !install.MigrateHarnessState(&hero) {
		t.Fatal("expected migration to add codex")
	}
	if !install.IsHarnessEnabled(hero, "cursor") || !install.IsHarnessEnabled(hero, "opencode") {
		t.Fatal("existing enabled harnesses must be preserved")
	}
	codex, ok := hero.Harnesses["codex"]
	if !ok {
		t.Fatal("harnesses.codex key must be present after 2.4.x → 2.5.0 migration")
	}
	if codex.Enabled {
		t.Fatal("harnesses.codex.enabled must be false on 2.4.x → 2.5.0 migration")
	}
	if codex.Model != "" {
		t.Fatalf("harnesses.codex.model must stay empty, got %q", codex.Model)
	}
	if hero.Harnesses["cursor"].Model != "composer-2.5" || hero.Harnesses["opencode"].Model != "opencode-go/deepseek-v4-flash" {
		t.Fatalf("existing models must be preserved: %+v", hero.Harnesses)
	}
}

// hero-json-harness-state: freechat_default may select a Codex pair with a native
// model id (PRD-C06-001 §4.8).
func TestSetFreechatDefault_CodexPair(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := install.SetFreechatDefault(dir, "codex", "gpt-5.4-codex"); err != nil {
		t.Fatal(err)
	}
	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hero.FreechatDefault.Harness != "codex" || hero.FreechatDefault.Model != "gpt-5.4-codex" {
		t.Fatalf("freechat_default = %+v", hero.FreechatDefault)
	}
	if hero.Harnesses["codex"].Model != "gpt-5.4-codex" {
		t.Fatalf("harnesses.codex.model = %+v", hero.Harnesses["codex"])
	}
}

// hero-json-harness-state: C5 model_properties persist per harness codex + native
// model id, consistent with existing Cursor/OpenCode entries (PRD-C06-001 §4.10).
func TestCommitModelSelection_CodexPersistsPairAndProperties(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	props := map[string]string{"fs": "true", "th": "max", "ef": "na"}
	if err := install.CommitModelSelection(dir, "codex", "gpt-5.4-codex", props); err != nil {
		t.Fatal(err)
	}
	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hero.FreechatDefault.Harness != "codex" || hero.FreechatDefault.Model != "gpt-5.4-codex" {
		t.Fatalf("freechat_default = %+v", hero.FreechatDefault)
	}
	got := install.PairProperties(hero, "codex", "gpt-5.4-codex")
	if got["fs"] != "true" || got["th"] != "max" {
		t.Fatalf("model_properties.codex = %v", got)
	}
	if _, hasNA := got["ef"]; hasNA {
		t.Fatalf("'na' sentinel must not be persisted: %v", got)
	}
	// Reject a harness outside the supported set, codex included.
	if err := install.CommitModelSelection(dir, "claude", "m", nil); err == nil {
		t.Fatal("unsupported harness must be rejected")
	}
	// Persisted JSON round-trips through the schema.
	data, err := os.ReadFile(filepath.Join(cfgDir, "hero.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["model_properties"]; !ok {
		t.Fatal("hero.json must carry model_properties for codex")
	}
}

func slicesContains(ids []string, id string) bool {
	for _, s := range ids {
		if s == id {
			return true
		}
	}
	return false
}
