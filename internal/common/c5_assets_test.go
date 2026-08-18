package common

import (
	"os"
	"path/filepath"
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
	// Existing pricing/context-window entries remain valid (no properties block).
	if !installed.HasModel("composer-2.5") {
		t.Fatal("projected cursor pricing model missing")
	}
	if p, ok := installed.CatalogValues("composer-2.5", "fs"); ok && p.HasProperty {
		t.Fatal("pricing-only entry must not gain property metadata")
	}
}
