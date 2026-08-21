package common

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/modelprops"
)

// TestEmbeddedOpenCodeFixtureLoadsCapabilities validates the primary C5 fixture
// in the embedded asset catalog (PRD-C05-001 §7): capability-only entries are
// allowed and must never carry invented pricing.
func TestEmbeddedOpenCodeFixtureLoadsCapabilities(t *testing.T) {
	cat := modelprops.LoadCatalogFromFS(assets.FS, "models")
	if !cat.HasModel("opencode-go/deepseek-v4-pro") {
		t.Fatal("embedded opencode fixture model id missing")
	}
	fs, ok := cat.CatalogValues("opencode-go/deepseek-v4-pro", "fs")
	if !ok || !fs.Available || len(fs.Values) != 2 || fs.Default != "false" {
		t.Fatalf("fs fixture: %+v", fs)
	}
	th, ok := cat.CatalogValues("opencode-go/deepseek-v4-pro", "th")
	if !ok || !th.Available || len(th.Values) != 2 {
		t.Fatalf("th fixture: %+v", th)
	}
	ef, ok := cat.CatalogValues("opencode-go/deepseek-v4-pro", "ef")
	if !ok || !ef.Available || len(ef.Values) != 3 || ef.Default != "medium" {
		t.Fatalf("ef fixture: %+v", ef)
	}
	lunaFS, ok := cat.CatalogValues("opencode-go/gpt-5.6-luna", "fs")
	if !ok || lunaFS.Available {
		t.Fatalf("luna must not expose fast mode: %+v", lunaFS)
	}
	lunaEF, ok := cat.CatalogValues("opencode-go/gpt-5.6-luna", "ef")
	if !ok || !lunaEF.Available || len(lunaEF.Values) != 6 || lunaEF.Values[0] != "none" || lunaEF.Values[5] != "max" {
		t.Fatalf("luna ef fixture: %+v", lunaEF)
	}
}

// TestEmbeddedCodexFixtureLoadsCapabilities validates Codex-native catalog rows
// (PRD-C06-001 §4.8): context/properties present; no invented ChatGPT USD rates.
func TestEmbeddedCodexFixtureLoadsCapabilities(t *testing.T) {
	cat := modelprops.LoadCatalogFromFS(assets.FS, "models")
	if !cat.HasModel("gpt-5.4") {
		t.Fatal("embedded Codex fixture gpt-5.4 missing")
	}
	ef, ok := cat.CatalogValues("gpt-5.4", "ef")
	if !ok || !ef.Available || ef.Default != "medium" {
		t.Fatalf("gpt-5.4 ef: %+v", ef)
	}
	cost, warn := modelprops.EstimateCatalogCostUSD(cat, "gpt-5.4", 100, 50)
	if cost != 0 || warn == "" {
		t.Fatalf("Codex ChatGPT rates must stay unset/zero with warning: cost=%v warn=%q", cost, warn)
	}
}

// TestEmbeddedCatalogPropertyMetadataSurvivesInstallProjection proves the
// optional property metadata is preserved when install copies assets/models/*
// into .workflow-hero/models/, while existing pricing/context-window entries
// remain valid (PRD-C05-001 §4.2.12; ADR-039).
func TestEmbeddedCatalogPropertyMetadataSurvivesInstallProjection(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectDir, ".workflow-hero"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := install.Options{ProjectDir: projectDir, AssetsFS: assets.FS}
	if err := install.CopyCoreAssets(opts, install.Checksums{}); err != nil {
		t.Fatalf("copy core assets: %v", err)
	}

	installed := modelprops.LoadCatalogFromDir(filepath.Join(projectDir, ".workflow-hero", "models"))
	if !installed.HasModel("opencode-go/deepseek-v4-pro") {
		t.Fatal("projected catalog lost the opencode fixture")
	}
	if p, ok := installed.CatalogValues("opencode-go/deepseek-v4-pro", "ef"); !ok || !p.Available {
		t.Fatalf("projected fixture lost property metadata: %+v", p)
	}
	// C5 base rows may carry property metadata; variant pricing rows stay block-free.
	if p, ok := installed.CatalogValues("composer-2.5", "fs"); !ok || !p.Available {
		t.Fatalf("composer-2.5 must expose fs catalog metadata: %+v", p)
	}
	if m, ok := installed["cursor-grok-4.6-high"]; ok && len(m.Properties) > 0 {
		t.Fatal("pricing-only variant entry must not embed property blocks in YAML")
	}
}

// TestModelPropertyHelpAssetContract keeps the installed Runtime guidance in
// sync with the native picker contract.  The same help is embedded for Cursor
// and OpenCode, while workflow YAML remains explicitly separate from freechat
// model_properties.
func TestModelPropertyHelpAssetContract(t *testing.T) {
	for _, path := range []string{
		"cursor/commands/hero-model.md",
		"opencode/commands/hero-model.md",
		"cursor/commands/hero-help.md",
		"opencode/commands/hero-help.md",
	} {
		data, err := fs.ReadFile(assets.FS, path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		body := string(data)
		for _, want := range []string{"fs", "th", "ef", "model_properties", "workflow-config.yml", "yellow"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s missing model-property help keyword %q", path, want)
			}
		}
	}
	help, err := fs.ReadFile(assets.FS, "docs/workflow-help.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"model_properties", "[fs-<value>]", "[fs-<valor>]", "stale cache", "cache antigo"} {
		if !strings.Contains(string(help), want) {
			t.Errorf("workflow-help.md missing C5 keyword %q", want)
		}
	}
}
