package install

import (
	"os"
	"path/filepath"
	"testing"
)

func heroJSONForSelectionTest(t *testing.T, dir string) string {
	t.Helper()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{
  "cli": {"version": "2.0.0", "tools": ["cursor"]},
  "harnesses": {
    "cursor": {"enabled": true, "model": "old-model", "enable_fast_model": false},
    "opencode": {"enabled": true, "model": "", "enable_fast_model": false}
  },
  "freechat_default": {"harness": "cursor", "model": "old-model"}
}`
	path := filepath.Join(cfgDir, "hero.json")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCommitModelSelectionWritesCompleteDraftAtomically(t *testing.T) {
	dir := t.TempDir()
	heroJSONForSelectionTest(t, dir)
	// A workflow-config.yml sitting nearby must never be touched.
	yamlPath := filepath.Join(dir, ".workflow-hero", "cycles", "current", "workflow-config.yml")
	if err := os.MkdirAll(filepath.Dir(yamlPath), 0o755); err != nil {
		t.Fatal(err)
	}
	yamlBefore := []byte("title: keep\nagents:\n  qa_agent:\n    harness: cursor\n    model: grok-4.6\n")
	if err := os.WriteFile(yamlPath, yamlBefore, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CommitModelSelection(dir, "opencode", "opencode-go/deepseek-v4-pro",
		map[string]string{"fs": "true", "th": "max", "ef": "high"}); err != nil {
		t.Fatal(err)
	}
	hero, err := LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hero.FreechatDefault.Harness != "opencode" || hero.FreechatDefault.Model != "opencode-go/deepseek-v4-pro" {
		t.Fatalf("freechat_default: %+v", hero.FreechatDefault)
	}
	if hero.Harnesses["opencode"].Model != "opencode-go/deepseek-v4-pro" {
		t.Fatalf("harness model: %+v", hero.Harnesses["opencode"])
	}
	if hero.Harnesses["opencode"].EnableFastModel {
		t.Fatal("legacy fast flag must be cleared on C5 commits")
	}
	props := PairProperties(hero, "opencode", "opencode-go/deepseek-v4-pro")
	if props["fs"] != "true" || props["th"] != "max" || props["ef"] != "high" {
		t.Fatalf("committed properties: %v", props)
	}
	yamlAfter, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(yamlBefore) != string(yamlAfter) {
		t.Fatal("workflow YAML must be byte-for-byte unchanged")
	}
}

func TestCommitModelSelectionWithoutPropertiesSkipsSubmenu(t *testing.T) {
	dir := t.TempDir()
	heroJSONForSelectionTest(t, dir)
	if err := CommitModelSelection(dir, "cursor", "composer-2.5", nil); err != nil {
		t.Fatal(err)
	}
	hero, err := LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hero.FreechatDefault.Model != "composer-2.5" {
		t.Fatalf("pair not committed: %+v", hero.FreechatDefault)
	}
	if props := PairProperties(hero, "cursor", "composer-2.5"); len(props) != 0 {
		t.Fatalf("no properties expected for skip path: %v", props)
	}
}

func TestEscapeCancellationLeavesDiskUntouched(t *testing.T) {
	dir := t.TempDir()
	path := heroJSONForSelectionTest(t, dir)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Escape cancellation means the commit helper is never called: the disk
	// state and the prior pair remain exactly as they were.
	hero, err := LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hero.FreechatDefault.Model != "old-model" {
		t.Fatalf("prior pair must be intact: %+v", hero.FreechatDefault)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("disk must be unchanged after Escape (no commit call)")
	}
}

func TestCommitModelSelectionReplacesPairEntryOnly(t *testing.T) {
	dir := t.TempDir()
	heroJSONForSelectionTest(t, dir)
	if err := CommitModelSelection(dir, "cursor", "a-model", map[string]string{"fs": "true"}); err != nil {
		t.Fatal(err)
	}
	if err := CommitModelSelection(dir, "cursor", "b-model", map[string]string{"fs": "false"}); err != nil {
		t.Fatal(err)
	}
	hero, err := LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hero.FreechatDefault.Model != "b-model" {
		t.Fatalf("latest commit must win: %+v", hero.FreechatDefault)
	}
	// Independent pair entries survive a later commit.
	if PairProperties(hero, "cursor", "a-model")["fs"] != "true" {
		t.Fatal("a-model properties must survive the b-model commit")
	}
	if PairProperties(hero, "cursor", "b-model")["fs"] != "false" {
		t.Fatal("b-model properties must be present")
	}
}

func TestCommitModelSelectionValidation(t *testing.T) {
	dir := t.TempDir()
	heroJSONForSelectionTest(t, dir)
	if err := CommitModelSelection(dir, "", "m", nil); err == nil {
		t.Fatal("empty harness must fail")
	}
	if err := CommitModelSelection(dir, "cursor", "", nil); err == nil {
		t.Fatal("empty model must fail")
	}
	if err := CommitModelSelection(dir, "claude", "m", nil); err == nil {
		t.Fatal("unsupported harness must fail")
	}
}
