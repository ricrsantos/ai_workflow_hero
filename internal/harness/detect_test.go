package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectMarkers(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".cursor"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)

	res, err := DetectMarkers(dir, []string{"cursor"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Present) != 2 {
		t.Fatalf("present = %+v", res.Present)
	}
	if len(res.UnsupportedPresent) != 1 || res.UnsupportedPresent[0].ToolID != "claude" {
		t.Fatalf("unsupported = %+v", res.UnsupportedPresent)
	}
	if len(res.MissingConfigured) != 0 {
		t.Fatalf("missing = %v", res.MissingConfigured)
	}
}

func TestDetectMarkersAllKnownDirs(t *testing.T) {
	dir := t.TempDir()
	for _, m := range KnownMarkers {
		if err := os.MkdirAll(filepath.Join(dir, m.Dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	res, err := DetectMarkers(dir, []string{"cursor"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Present) != len(KnownMarkers) {
		t.Fatalf("present = %d want %d", len(res.Present), len(KnownMarkers))
	}
	if len(res.UnsupportedPresent) != 2 {
		t.Fatalf("unsupported = %+v", res.UnsupportedPresent)
	}
}

func TestDetectMarkersMissingConfigured(t *testing.T) {
	dir := t.TempDir()
	res, err := DetectMarkers(dir, []string{"cursor"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.MissingConfigured) != 1 || res.MissingConfigured[0] != "cursor" {
		t.Fatalf("missing = %v", res.MissingConfigured)
	}
}
