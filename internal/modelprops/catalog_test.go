package modelprops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ricrsantos/ai_workflow_hero/assets"
)

func TestLoadCatalogFromEmbeddedAssetsPreservesPricingOnlyEntries(t *testing.T) {
	cat := LoadCatalogFromFS(assets.FS, "models")
	if !cat.HasModel("composer-2.5") {
		t.Fatal("embedded pricing-only cursor model must remain selectable")
	}
	if !cat.HasModel("openai/gpt-5.3-codex") {
		t.Fatal("embedded opencode-native model row must remain selectable")
	}
	if !cat.HasModel("gpt-5-mini") {
		t.Fatal("embedded openai model row must remain selectable")
	}
}

const completeCatalogYAML = `models:
  acme/complete:
    properties:
      fs:
        available: true
        values: ["true", "false"]
        default: "false"
      th:
        available: true
        values: ["off", "max", "vendor-custom"]
        default: "max"
      ef:
        available: true
        values: ["low", "medium", "high"]
        default: "medium"
`

const partialCatalogYAML = `models:
  acme/partial:
    input: 1.0
    output: 2.0
    context_window: 128000
    properties:
      th:
        available: true
        values: ["off", "max"]
        default: "off"
      ef:
        available: false
`

const absentCatalogYAML = `models:
  acme/absent:
    input: 1.0
    output: 2.0
    context_window: 128000
`

func testCatalogFromFS(t *testing.T, files map[string]string) Catalog {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, content := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return LoadCatalogFromFS(fsys, "models")
}

func TestCatalogCompleteFixture(t *testing.T) {
	cat := testCatalogFromFS(t, map[string]string{"models/complete.yml": completeCatalogYAML})
	if !cat.HasModel("acme/complete") {
		t.Fatal("complete model missing")
	}
	th, ok := cat.CatalogValues("acme/complete", "th")
	if !ok || !th.Available {
		t.Fatalf("th block missing: %+v", th)
	}
	wantValues := []string{"off", "max", "vendor-custom"}
	if len(th.Values) != len(wantValues) {
		t.Fatalf("th values=%v want %v (dynamic order must be preserved)", th.Values, wantValues)
	}
	for i, v := range wantValues {
		if th.Values[i] != v {
			t.Fatalf("th values[%d]=%q want %q", i, th.Values[i], v)
		}
	}
	if !th.HasDefault || th.Default != "max" {
		t.Fatalf("th default=%q hasDefault=%v", th.Default, th.HasDefault)
	}
	fs, ok := cat.CatalogValues("acme/complete", "fs")
	if !ok || !fs.Available || fs.Default != "false" {
		t.Fatalf("fs block unexpected: %+v", fs)
	}
	ef, ok := cat.CatalogValues("acme/complete", "ef")
	if !ok || !ef.Available || len(ef.Values) != 3 {
		t.Fatalf("ef block unexpected: %+v", ef)
	}
}

func TestCatalogPartialAndAbsentFixtures(t *testing.T) {
	cat := testCatalogFromFS(t, map[string]string{
		"models/partial.yml": partialCatalogYAML,
		"models/absent.yml":  absentCatalogYAML,
	})
	th, ok := cat.CatalogValues("acme/partial", "th")
	if !ok || !th.Available {
		t.Fatalf("partial th missing: %+v", th)
	}
	ef, ok := cat.CatalogValues("acme/partial", "ef")
	if !ok || ef.Available {
		t.Fatalf("partial ef must be unavailable: %+v", ef)
	}
	if _, ok := cat.CatalogValues("acme/partial", "fs"); ok {
		t.Fatal("partial fs must be absent")
	}
	if !cat.HasModel("acme/absent") {
		t.Fatal("pricing-only entry must remain selectable")
	}
	if p, ok := cat.CatalogValues("acme/absent", "fs"); ok && p.HasProperty {
		t.Fatal("absent model must expose no property metadata")
	}
}

func TestCatalogUnknownKeysIgnoredWithoutPanic(t *testing.T) {
	yaml := `models:
  acme/weird:
    properties:
      fs:
        available: true
        values: ["true"]
        default: "false"
        vendor_future_field: {nested: [1,2,3]}
      future_key:
        available: true
        values: ["a"]
  not-a-map-entry:
`
	cat := testCatalogFromFS(t, map[string]string{"models/weird.yml": yaml})
	fs, ok := cat.CatalogValues("acme/weird", "fs")
	if !ok || !fs.Available || len(fs.Values) != 1 || fs.Values[0] != "true" {
		t.Fatalf("fs block unexpected: %+v", fs)
	}
	if !cat.HasModel("acme/weird") {
		t.Fatal("weird model must still load")
	}
	// Unknown future keys may be retained in metadata but are not C5 keys.
	keys := CatalogPropertyKeys(cat["acme/weird"])
	if len(keys) == 0 {
		t.Fatal("keys empty")
	}
	if keys[0] != "fs" {
		t.Fatalf("first key=%q want fs", keys[0])
	}
}

func TestCatalogMalformedFilesIgnored(t *testing.T) {
	cat := testCatalogFromFS(t, map[string]string{
		"models/bad.yml":     "this is: [not valid yaml {{",
		"models/good.yml":    completeCatalogYAML,
		"models/note.txt":    "not yaml",
		"models/noext_md.md": "models:\n  x:\n",
	})
	if !cat.HasModel("acme/complete") {
		t.Fatal("good file must load even when a sibling file is malformed")
	}
	if cat.HasModel("x") {
		t.Fatal("non-.yml files must be skipped")
	}
}

func TestCatalogInstalledOverlayWins(t *testing.T) {
	dir := t.TempDir()
	modelsDir := filepath.Join(dir, ".workflow-hero", "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Overlay adds a property to an embedded model row.
	overlay := `models:
  composer-2.5:
    properties:
      fs:
        available: true
        values: ["true", "false"]
        default: "false"
`
	if err := os.WriteFile(filepath.Join(modelsDir, "cursor.yml"), []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := LoadCatalog(assets.FS, dir)
	if !cat.HasModel("composer-2.5") {
		t.Fatal("embedded row lost")
	}
	fs, ok := cat.CatalogValues("composer-2.5", "fs")
	if !ok || !fs.Available {
		t.Fatalf("installed overlay must win for composer-2.5: %+v", fs)
	}
	if !cat.HasModel("gpt-5-mini") {
		t.Fatal("non-overlaid embedded rows must survive")
	}
}

func TestCatalogPropertyKeysDeterministicOrder(t *testing.T) {
	m := CatalogModel{Properties: map[string]CatalogProperty{
		"future":  {},
		"ef":      {},
		"th":      {},
		"fs":      {},
		"another": {},
	}}
	keys := CatalogPropertyKeys(m)
	want := []string{"fs", "th", "ef", "another", "future"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("keys=%v want %v", keys, want)
	}
}
