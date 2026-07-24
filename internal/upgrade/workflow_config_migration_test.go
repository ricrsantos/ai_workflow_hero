package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
)

func TestMigrateLegacyWorkflowConfigs_RenamesScalarGenericModel(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, cursoradapter.HeroCyclesDir, "current", "workflow-config.yml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}

	legacy := `title: Test
objective: Test objective.

generic_model: claude-sonnet-5

scope:
  backend: true
`
	if err := os.WriteFile(configPath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr strings.Builder
	migrated, err := migrateLegacyWorkflowConfigs(dir, &stderr)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(migrated) != 1 {
		t.Fatalf("migrated = %v, want one file", migrated)
	}

	updated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(updated)
	for _, want := range []string{
		"fallback_model:",
		"model: claude-sonnet-5",
		"reasoning_effort: medium",
		"enable_fast_model: false",
		"thinking: na",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("updated config missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "generic_model:") {
		t.Errorf("generic_model was not removed:\n%s", content)
	}
}

func TestMigrateLegacyWorkflowConfigs_WarnsWhenBothKeysPresent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, cursoradapter.HeroTemplatesDir, "workflow-config.yml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}

	content := `generic_model: old-model
fallback_model:
  model: new-model
  reasoning_effort: medium
  enable_fast_model: false
  thinking: na
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr strings.Builder
	migrated, err := migrateLegacyWorkflowConfigs(dir, &stderr)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(migrated) != 0 {
		t.Fatalf("expected no migration, got %v", migrated)
	}
	if !strings.Contains(stderr.String(), "merge manually") {
		t.Fatalf("expected manual merge warning, got: %s", stderr.String())
	}

	after, _ := os.ReadFile(configPath)
	if string(after) != content {
		t.Fatal("file should not have been modified")
	}
}
