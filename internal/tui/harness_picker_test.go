package tui_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/tui"
)

func writeHeroJSON(t *testing.T, dir string, body []byte) {
	t.Helper()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "checksums.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHarnessPickerEnableOpenCodeSuccessLine(t *testing.T) {
	dir := t.TempDir()
	writeHeroJSON(t, dir, []byte(`{
  "harnesses": {"cursor": {"enabled": true}, "opencode": {"enabled": false}}
}
`))
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	m := tui.NewTestModel(svc)
	next, _ := tui.RunPaletteItemForTest(m, "/hero-harness")
	if !tui.PickingHarnessForTest(next) {
		t.Fatal("expected harness picker")
	}
	items := tui.FilteredPalette(next)
	var idx int = -1
	for i, item := range items {
		if strings.EqualFold(item.Label, "OpenCode") {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("items=%v", items)
	}
	next = tui.SetPaletteIndexForTest(next, idx)
	next, _ = tui.HandleTestKey(next, "enter")
	if tui.StatusKindForTest(next) != "ok" {
		t.Fatalf("status=%s text=%q", tui.StatusKindForTest(next), tui.StatusTextForTest(next))
	}
	if !strings.Contains(tui.StatusTextForTest(next), "OpenCode enabled (projected .opencode/)") {
		t.Fatalf("text=%q", tui.StatusTextForTest(next))
	}
	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !install.IsHarnessEnabled(hero, "opencode") {
		t.Fatal("opencode not enabled")
	}
}

func TestHarnessPickerDisableSuccessLine(t *testing.T) {
	dir := t.TempDir()
	writeHeroJSON(t, dir, []byte(`{
  "harnesses": {"cursor": {"enabled": true}, "opencode": {"enabled": true}}
}
`))
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	stopCalled := false
	prev := tui.StopOpenCodeServeFnForTest()
	tui.SetStopOpenCodeServeFnForTest(func(ctx context.Context, projectDir string, st *store.Store) error {
		stopCalled = true
		if projectDir != dir {
			t.Fatalf("projectDir=%q", projectDir)
		}
		return nil
	})
	t.Cleanup(func() { tui.SetStopOpenCodeServeFnForTest(prev) })

	m := tui.NewTestModel(svc)
	next, _ := tui.RunPaletteItemForTest(m, "/hero-harness")
	items := tui.FilteredPalette(next)
	idx := -1
	for i, item := range items {
		if strings.EqualFold(item.Label, "OpenCode") {
			idx = i
			break
		}
	}
	next = tui.SetPaletteIndexForTest(next, idx)
	next, _ = tui.HandleTestKey(next, "enter")
	if !strings.Contains(tui.StatusTextForTest(next), "OpenCode disabled (files kept)") {
		t.Fatalf("text=%q", tui.StatusTextForTest(next))
	}
	if !stopCalled {
		t.Fatal("expected stopOpenCodeServe on opencode disable")
	}
}

func TestHarnessPickerLastHarnessError(t *testing.T) {
	dir := t.TempDir()
	writeHeroJSON(t, dir, []byte(`{
  "harnesses": {"cursor": {"enabled": true}, "opencode": {"enabled": false}}
}
`))
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	m := tui.NewTestModel(svc)
	next, _ := tui.RunPaletteItemForTest(m, "/hero-harness")
	items := tui.FilteredPalette(next)
	idx := -1
	for i, item := range items {
		if strings.EqualFold(item.Label, "Cursor") {
			idx = i
			break
		}
	}
	next = tui.SetPaletteIndexForTest(next, idx)
	next, _ = tui.HandleTestKey(next, "enter")
	if tui.StatusKindForTest(next) != "err" {
		t.Fatalf("status=%s", tui.StatusKindForTest(next))
	}
	if !strings.Contains(tui.StatusTextForTest(next), "Cannot disable the last enabled harness") {
		t.Fatalf("text=%q", tui.StatusTextForTest(next))
	}
}
