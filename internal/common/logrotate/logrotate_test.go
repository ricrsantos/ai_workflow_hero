package logrotate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeN(t *testing.T, w *Writer, line string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, err := w.Write([]byte(line)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
}

func TestWriterRotatesBySize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logs", "tui.log")
	w := New(path)
	w.MaxSize = 24 // tiny threshold to force rotation
	w.MaxFiles = 3
	w.Redactor = nil

	// Each line is 12 bytes ("line-XXXXXXX" + \n); writing several forces rotations.
	writeN(t, w, "line-000001\n", 1)
	writeN(t, w, "line-000002\n", 1)
	writeN(t, w, "line-000003\n", 1)
	writeN(t, w, "line-000004\n", 1)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// The active file must exist.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("active file missing: %v", err)
	}
	// With MaxFiles=3, backups .1 and .2 may exist but never .3 or beyond.
	if _, err := os.Stat(path + ".3"); !os.IsNotExist(err) {
		t.Fatalf("unexpected backup .3 exists")
	}
	// Every retained file must be under MaxSize.
	for _, p := range []string{path, path + ".1", path + ".2"} {
		fi, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatal(err)
		}
		if fi.Size() > 24 {
			t.Fatalf("%s size %d exceeds threshold", p, fi.Size())
		}
	}
}

func TestWriterRedacts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tui.log")
	w := New(path)
	w.Redactor = func(s string) string {
		return strings.ReplaceAll(s, "SECRET", "[REDACTED]")
	}
	writeN(t, w, "token=SECRET\n", 1)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "SECRET") {
		t.Fatalf("secret leaked: %q", string(data))
	}
}

func TestWriterReportsConsumedLength(t *testing.T) {
	dir := t.TempDir()
	w := New(filepath.Join(dir, "tui.log"))
	w.Redactor = func(s string) string { return strings.ReplaceAll(s, "SECRET", "[REDACTED]") }
	in := []byte("token=SECRET\n")
	n, err := w.Write(in)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(in) {
		t.Fatalf("consumed %d, want %d", n, len(in))
	}
	_ = w.Close()
}

func TestMigrateLegacy(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, ".workflow-hero", "tui.log")
	newPath := filepath.Join(dir, ".workflow-hero", "logs", "tui.log")

	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("legacy\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MigrateLegacy(legacy, newPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatal("legacy file still present after migration")
	}
	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "legacy\n" {
		t.Fatalf("unexpected migrated content: %q", data)
	}
}

func TestMigrateLegacyNoopWhenMissing(t *testing.T) {
	dir := t.TempDir()
	if err := MigrateLegacy(filepath.Join(dir, "missing"), filepath.Join(dir, "logs", "tui.log")); err != nil {
		t.Fatalf("expected noop, got %v", err)
	}
}

func TestMigrateLegacyKeepsExistingNew(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "legacy.log")
	newPath := filepath.Join(dir, "new.log")
	if err := os.WriteFile(legacy, []byte("legacy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MigrateLegacy(legacy, newPath); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(newPath)
	if string(data) != "new\n" {
		t.Fatalf("existing new file overwritten: %q", data)
	}
}
