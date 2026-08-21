package modelprops

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
)

// TestEmbeddedCodexCatalogNativeIDs covers PRD-C06-001 §4.8 / design D12:
// Codex-native ids ship with context-capable properties and provider:codex.
func TestEmbeddedCodexCatalogNativeIDs(t *testing.T) {
	cat := LoadCatalogFromFS(assets.FS, "models")
	for _, id := range []string{
		"gpt-5.4", "gpt-5.4-mini", "gpt-5.5", "gpt-5.6-terra", "gpt-5.6-luna",
		"gpt-5.3-codex", "gpt-5.3-codex-spark",
	} {
		if !cat.HasModel(id) {
			t.Fatalf("missing Codex-native catalog id %q", id)
		}
		m := cat[id]
		if !strings.EqualFold(m.Provider, "codex") {
			t.Fatalf("%s provider=%q want codex (codex.yml must win over openai.yml)", id, m.Provider)
		}
	}

	ef, ok := cat.CatalogValues("gpt-5.4", "ef")
	if !ok || !ef.Available || ef.Default != "medium" {
		t.Fatalf("gpt-5.4 ef: %+v ok=%v", ef, ok)
	}
	fs, ok := cat.CatalogValues("gpt-5.4", "fs")
	if !ok || fs.Available {
		t.Fatalf("gpt-5.4 fs must be na/unavailable: %+v", fs)
	}
	th, ok := cat.CatalogValues("gpt-5.4", "th")
	if !ok || !th.Available {
		t.Fatalf("gpt-5.4 th must be available for Codex summary mapping: %+v", th)
	}

	codexIDs := cat.ModelsForHarness("codex")
	joined := strings.Join(codexIDs, ",")
	for _, want := range []string{"gpt-5.4", "gpt-5.3-codex"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("ModelsForHarness(codex) missing %q: %v", want, codexIDs)
		}
	}
	// Cursor / OpenCode rows must remain resolvable.
	if !cat.HasModel("composer-2.5") {
		t.Fatal("Cursor catalog regress")
	}
	if !cat.HasModel("opencode-go/deepseek-v4-pro") {
		t.Fatal("OpenCode catalog regress")
	}
}

// TestUnknownCodexIDCostUnsetNoPanic verifies catalog miss leaves cost at zero
// with an explicit warning and never panics (PRD-C06-001 §4.8).
func TestUnknownCodexIDCostUnsetNoPanic(t *testing.T) {
	cat := LoadCatalogFromFS(assets.FS, "models")
	unknown := "codex-model-that-does-not-exist"
	if cat.HasModel(unknown) {
		t.Fatal("unknown id must not be present")
	}
	cost, warn := EstimateCatalogCostUSD(cat, unknown, 1000, 500)
	if cost != 0 {
		t.Fatalf("cost=%v want 0", cost)
	}
	if warn == "" || !strings.Contains(strings.ToLower(warn), "unknown") {
		t.Fatalf("expected unknown-model warning, got %q", warn)
	}

	// Known Codex id with zero ChatGPT rates → cost zero + subsidized warning.
	cost, warn = EstimateCatalogCostUSD(cat, "gpt-5.4", 1000, 500)
	if cost != 0 {
		t.Fatalf("gpt-5.4 ChatGPT rates unset: cost=%v", cost)
	}
	if warn == "" {
		t.Fatal("expected zero-rate warning for Codex catalog row")
	}
}

func TestCatalogModelsForHarnessCodex(t *testing.T) {
	cat := testCatalogFromFS(t, map[string]string{
		"models/openai.yml": "provider: openai\nmodels:\n  gpt-5.3-codex: {}\n  gpt-5-mini: {}\n",
		"models/codex.yml":  "provider: codex\nmodels:\n  gpt-5.4: {}\n  gpt-5.3-codex: {}\n",
	})
	got := cat.ModelsForHarness("codex")
	want := "gpt-5.3-codex,gpt-5.4"
	if strings.Join(got, ",") != want {
		t.Fatalf("codex rows=%v want %s (codex.yml must win provider)", got, want)
	}
	if !strings.EqualFold(cat["gpt-5.3-codex"].Provider, "codex") {
		t.Fatalf("shared id provider=%q", cat["gpt-5.3-codex"].Provider)
	}
}
