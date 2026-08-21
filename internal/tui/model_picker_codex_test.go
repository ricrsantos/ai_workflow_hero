package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/modelprops"
)

// writeCodexModelPickerFixture installs hero.json + a stage workflow-config for
// /hero-model Codex tests (UI-C06-001 §4; hero-model-pair).
func writeCodexModelPickerFixture(t *testing.T, heroJSON, workflowYAML string) (svc *cycle.Service, dir string) {
	t.Helper()
	dir = t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), []byte(heroJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	cycleDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow-config.yml"), []byte(workflowYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc, dir
}

const codexStageYAML = `title: Codex Model Picker
objective: keep stage YAML untouched
agents:
  orchestration_agent:
    harness: cursor
    model: composer-2.5
fallback_model:
  harness: cursor
  model: grok-4.6
`

func TestHeroModelPickerIncludesCodexWhenEnabled(t *testing.T) {
	svc, _ := writeCodexModelPickerFixture(t, `{
  "harnesses": {
    "cursor": {"enabled": true},
    "opencode": {"enabled": true},
    "codex": {"enabled": true}
  }
}
`, codexStageYAML)

	m := OpenHeroModelForTest(NewTestModel(svc))
	if ModelPickerHarnessForTest(m) != "" {
		t.Fatal("expected harness submenu when multiple harnesses enabled")
	}
	view := ViewForTest(m)
	if !strings.Contains(view, "/hero-model · select harness") {
		t.Fatalf("title missing: %q", view)
	}
	for _, name := range []string{"Cursor", "OpenCode", "Codex"} {
		if !strings.Contains(view, name) {
			t.Fatalf("step 1 must list %s when enabled: %q", name, view)
		}
	}
}

func TestHeroModelPickerOmitsCodexWhenDisabled(t *testing.T) {
	svc, _ := writeCodexModelPickerFixture(t, `{
  "harnesses": {
    "cursor": {"enabled": true},
    "opencode": {"enabled": true},
    "codex": {"enabled": false}
  }
}
`, codexStageYAML)

	m := OpenHeroModelForTest(NewTestModel(svc))
	view := ViewForTest(m)
	if strings.Contains(view, "Codex") {
		t.Fatalf("disabled Codex must not appear in step 1: %q", view)
	}
	if !strings.Contains(view, "Cursor") || !strings.Contains(view, "OpenCode") {
		t.Fatalf("enabled harnesses missing: %q", view)
	}
}

func TestHeroModelPickerListsCodexNativeModels(t *testing.T) {
	svc, _ := writeCodexModelPickerFixture(t, `{
  "harnesses": {
    "cursor": {"enabled": true},
    "codex": {"enabled": true}
  }
}
`, codexStageYAML)

	m := NewTestModel(svc)
	m = SetModelOptionsForTest(m, []harnessmgr.ModelOption{
		{Model: "composer-2.5", Harness: "cursor"},
		{Model: "gpt-5.4", Harness: "codex"},
		{Model: "gpt-5.3-codex", Harness: "codex"},
	})
	m = OpenHeroModelForTest(m)

	idx := -1
	for i, item := range FilteredPalette(m) {
		if item.Label == "Codex" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("Codex missing from step 1: %v", FilteredPalette(m))
	}
	m = SetPaletteIndexForTest(m, idx)
	m, _ = HandleTestKey(m, "enter")
	if ModelPickerHarnessForTest(m) != "codex" {
		t.Fatalf("harness=%q want codex", ModelPickerHarnessForTest(m))
	}
	view := ViewForTest(m)
	if !strings.Contains(view, "/hero-model · Codex") {
		t.Fatalf("step 2 title=%q", view)
	}
	if !strings.Contains(view, "gpt-5.4") || !strings.Contains(view, "gpt-5.3-codex") {
		t.Fatalf("native Codex ids missing: %q", view)
	}
	if strings.Contains(view, "composer-2.5") {
		t.Fatalf("cursor models leaked into Codex list: %q", view)
	}
}

func TestHeroModelPickerListsCodexViaListModels(t *testing.T) {
	svc, _ := writeCodexModelPickerFixture(t, `{
  "harnesses": {
    "cursor": {"enabled": true},
    "codex": {"enabled": true}
  }
}
`, codexStageYAML)

	var listedHarness string
	prev := listModelsForHarnessFn
	listModelsForHarnessFn = func(_ context.Context, _ model, harnessID string) ([]string, error) {
		listedHarness = harnessID
		return []string{"gpt-5.4", "o3"}, nil
	}
	t.Cleanup(func() { listModelsForHarnessFn = prev })

	m := OpenHeroModelForTest(NewTestModel(svc))
	// Force live ListModels path: no boot/cache/catalog short-circuit (UI-C06-001 §4).
	// Clearing Catalog alone is insufficient once assets/models/codex.yml exists —
	// Models() falls back to ModelsForHarness, and a prior refresh may have
	// persisted that list into the project store.
	if m.propsSvc != nil {
		m.propsSvc.Catalog = nil
		if m.propsSvc.Store != nil {
			_ = m.propsSvc.Store.UpsertModelList("codex", nil, "")
		}
	}
	m.modelOptions = nil
	m.availableModels = nil
	idx := -1
	for i, item := range FilteredPalette(m) {
		if item.Label == "Codex" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("Codex missing from step 1")
	}
	m = SetPaletteIndexForTest(m, idx)
	m, cmd := HandleTestKey(m, "enter")
	if cmd == nil {
		t.Fatal("expected ListModels cmd (may start app-server)")
	}
	msg := cmd()
	m, _ = HandleTestMsg(m, msg)
	if listedHarness != "codex" {
		t.Fatalf("ListModels harness=%q want codex", listedHarness)
	}
	if ModelPickerHarnessForTest(m) != "codex" {
		t.Fatalf("harness=%q", ModelPickerHarnessForTest(m))
	}
	view := ViewForTest(m)
	if !strings.Contains(view, "gpt-5.4") || !strings.Contains(view, "o3") {
		t.Fatalf("ListModels native ids missing: %q", view)
	}
}

func TestHeroModelPickerEscFromCodexReturnsToHarnessList(t *testing.T) {
	svc, _ := writeCodexModelPickerFixture(t, `{
  "harnesses": {
    "cursor": {"enabled": true},
    "opencode": {"enabled": true},
    "codex": {"enabled": true}
  }
}
`, codexStageYAML)

	m := NewTestModel(svc)
	m = SetModelOptionsForTest(m, []harnessmgr.ModelOption{
		{Model: "composer-2.5", Harness: "cursor"},
		{Model: "gpt-5.4", Harness: "codex"},
	})
	m = OpenHeroModelForTest(m)
	idx := -1
	for i, item := range FilteredPalette(m) {
		if item.Label == "Codex" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("Codex missing")
	}
	m = SetPaletteIndexForTest(m, idx)
	m, _ = HandleTestKey(m, "enter")
	if ModelPickerHarnessForTest(m) != "codex" {
		t.Fatalf("step 2 harness=%q", ModelPickerHarnessForTest(m))
	}

	m, _ = HandleTestKey(m, "esc")
	if ModelPickerHarnessForTest(m) != "" {
		t.Fatalf("Esc must return to harness list, got harness=%q", ModelPickerHarnessForTest(m))
	}
	if !PickingModelForTest(m) {
		t.Fatal("still in /hero-model after Esc")
	}
	view := ViewForTest(m)
	if !strings.Contains(view, "/hero-model · select harness") {
		t.Fatalf("expected harness list title: %q", view)
	}
	if !strings.Contains(view, "Codex") {
		t.Fatalf("Codex must remain on harness list when enabled: %q", view)
	}
}

func TestHeroModelPickerCodexPersistsPairLeavesStageYAML(t *testing.T) {
	svc, dir := writeCodexModelPickerFixture(t, `{
  "harnesses": {
    "cursor": {"enabled": true},
    "codex": {"enabled": true, "model": ""}
  },
  "freechat_default": {"harness": "cursor", "model": "old-model"}
}
`, codexStageYAML)
	yamlPath := filepath.Join(dir, ".workflow-hero", "cycles", "current", "workflow-config.yml")
	before, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}

	m := NewTestModel(svc)
	m = SetModelOptionsForTest(m, []harnessmgr.ModelOption{
		{Model: "composer-2.5", Harness: "cursor"},
		{Model: "gpt-5.4", Harness: "codex"},
	})
	m = OpenHeroModelForTest(m)
	idx := -1
	for i, item := range FilteredPalette(m) {
		if item.Label == "Codex" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("Codex missing")
	}
	m = SetPaletteIndexForTest(m, idx)
	m, _ = HandleTestKey(m, "enter")
	m = SetPaletteFilter(m, "gpt-5.4")
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")
	// Catalog may open C5 property submenu; save defaults so freechat pair commits.
	// Enter once may land on a property row — keep saving until the picker closes.
	for i := 0; i < 6 && m.pickingProps; i++ {
		m, _ = HandleTestKey(m, "enter")
	}

	if PickingModelForTest(m) || m.pickingProps {
		t.Fatal("picker should close after select")
	}
	if ChatModelSlugForTest(m) != "gpt-5.4" || ChatHarnessIDForTest(m) != "codex" {
		t.Fatalf("chat pair=%s · %s", ChatModelSlugForTest(m), ChatHarnessIDForTest(m))
	}

	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hero.FreechatDefault.Harness != "codex" || hero.FreechatDefault.Model != "gpt-5.4" {
		t.Fatalf("freechat_default=%+v", hero.FreechatDefault)
	}
	if hero.Harnesses["codex"].Model != "gpt-5.4" {
		t.Fatalf("harnesses.codex.model=%q", hero.Harnesses["codex"].Model)
	}

	after, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("stage YAML must be untouched\nbefore=%s\nafter=%s", before, after)
	}
}

func TestHeroModelPickerCodexPropertySubmenuUnchanged(t *testing.T) {
	svc, dir := writeCodexModelPickerFixture(t, `{
  "harnesses": {
    "cursor": {"enabled": true},
    "codex": {"enabled": true}
  }
}
`, codexStageYAML)

	m := NewTestModel(svc)
	m.propsSvc.Catalog = propsCatalog(map[string]map[string]modelprops.CatalogProperty{
		"gpt-5.4": {
			"fs": {Available: true, Values: []string{"true", "false"}, Default: "false"},
			"th": {Available: true, Values: []string{"off", "max"}, Default: "off"},
			"ef": {Available: true, Values: []string{"low", "medium", "high"}, Default: "medium"},
		},
	})
	m = OpenHeroModelForTest(m)
	m, _ = SelectChatModelPairForTest(m, "gpt-5.4", "codex")
	if !m.pickingProps {
		t.Fatal("C5 property submenu must open after Codex model select when properties are selectable")
	}
	view := ViewForTest(m)
	for _, want := range []string{
		"/hero-model · Codex · properties",
		"Fast Mode:",
		"Thinking:",
		"Reasoning effort:",
		"enter save",
		"esc cancel",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("property picker missing %q in %q", want, view)
		}
	}

	m, _ = HandleTestKey(m, "enter") // save defaults
	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hero.FreechatDefault.Harness != "codex" || hero.FreechatDefault.Model != "gpt-5.4" {
		t.Fatalf("freechat_default=%+v", hero.FreechatDefault)
	}
	if hero.Harnesses["codex"].Model != "gpt-5.4" {
		t.Fatalf("harnesses.codex.model=%q", hero.Harnesses["codex"].Model)
	}
	yamlPath := filepath.Join(dir, ".workflow-hero", "cycles", "current", "workflow-config.yml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "harness: cursor") || !strings.Contains(string(data), "model: composer-2.5") {
		t.Fatalf("stage YAML must remain authoritative: %s", data)
	}
}
