package assetconflict_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/assetconflict"
)

func TestBackupPath(t *testing.T) {
	now := time.Date(2026, 8, 19, 15, 41, 0, 0, time.UTC)
	got := assetconflict.BackupPath("/proj/.cursor/agents/backend_agent.md", now)
	want := filepath.Join("/proj/.cursor/agents", "backend_agent.md_20260819_154100.conflict")
	if got != want {
		t.Fatalf("BackupPath = %q, want %q", got, want)
	}
}

func TestIsCustomized(t *testing.T) {
	data := []byte("hello")
	hash := assetconflict.SHA256Hex(data)

	if assetconflict.IsCustomized(data, "") {
		t.Fatal("empty original checksum should not be customized")
	}
	if assetconflict.IsCustomized(data, hash) {
		t.Fatal("matching checksum should not be customized")
	}
	if !assetconflict.IsCustomized(append(data, '!'), hash) {
		t.Fatal("different content should be customized")
	}
}

func TestReplace_CreatesBackupAndWritesNewFile(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "openai.yml")
	existing := []byte("provider: old\n")
	newData := []byte("provider: new\n")
	if err := os.WriteFile(dst, existing, 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 19, 15, 41, 0, 0, time.UTC)
	var stderr strings.Builder
	backupPath, err := assetconflict.Replace(dst, existing, newData, ".workflow-hero/models/openai.yml", &stderr, now)
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}

	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backupData) != string(existing) {
		t.Fatalf("backup content = %q, want %q", backupData, existing)
	}

	written, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(newData) {
		t.Fatalf("dst content = %q, want %q", written, newData)
	}
	if !strings.Contains(stderr.String(), "replaced with the new file due to conflicts") {
		t.Fatalf("expected warning in stderr, got: %s", stderr.String())
	}
}
