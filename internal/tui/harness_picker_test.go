package tui_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
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

func paletteIndexByLabel(items []tui.PaletteItemView, label string) int {
	for i, item := range items {
		if strings.EqualFold(item.Label, label) {
			return i
		}
	}
	return -1
}

func TestHarnessPickerCheckboxesShowAvailability(t *testing.T) {
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
	next, _ := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/hero-harness")
	if !tui.PickingHarnessForTest(next) {
		t.Fatal("expected harness picker")
	}
	view := tui.ViewForTest(next)
	if !strings.Contains(view, "[x] Cursor") || !strings.Contains(view, "[ ] OpenCode") {
		t.Fatalf("missing checkboxes: %q", view)
	}
	if !strings.Contains(view, "Cursor (") || !strings.Contains(view, "OpenCode (") {
		t.Fatalf("missing availability parens: %q", view)
	}
	if !strings.Contains(view, "available") && !strings.Contains(view, "unavailable") {
		t.Fatalf("missing availability label: %q", view)
	}
	if !strings.Contains(view, "space toggle") {
		t.Fatalf("missing checkbox hint: %q", view)
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

	next, _ := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/hero-harness")
	if !tui.PickingHarnessForTest(next) {
		t.Fatal("expected harness picker")
	}
	items := tui.FilteredPalette(next)
	idx := paletteIndexByLabel(items, "OpenCode")
	if idx < 0 {
		t.Fatalf("items=%v", items)
	}
	next = tui.SetPaletteIndexForTest(next, idx)
	next, _ = tui.HandleTestKey(next, " ")
	view := tui.ViewForTest(next)
	if !strings.Contains(view, "[x] OpenCode") {
		t.Fatalf("space should check OpenCode: %q", view)
	}
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
	tui.SetStopOpenCodeServeFnForTest(func(ctx context.Context, projectDir string, st *store.Store, _ harnessmgr.Registry) error {
		stopCalled = true
		if projectDir != dir {
			t.Fatalf("projectDir=%q", projectDir)
		}
		return nil
	})
	t.Cleanup(func() { tui.SetStopOpenCodeServeFnForTest(prev) })

	next, _ := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/hero-harness")
	items := tui.FilteredPalette(next)
	idx := paletteIndexByLabel(items, "OpenCode")
	next = tui.SetPaletteIndexForTest(next, idx)
	next, _ = tui.HandleTestKey(next, " ")
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

	next, _ := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/hero-harness")
	items := tui.FilteredPalette(next)
	idx := paletteIndexByLabel(items, "Cursor")
	next = tui.SetPaletteIndexForTest(next, idx)
	next, _ = tui.HandleTestKey(next, " ")
	next, _ = tui.HandleTestKey(next, "enter")
	if tui.StatusKindForTest(next) != "err" {
		t.Fatalf("status=%s", tui.StatusKindForTest(next))
	}
	if !strings.Contains(tui.StatusTextForTest(next), "Select at least one harness") {
		t.Fatalf("text=%q", tui.StatusTextForTest(next))
	}
	if !tui.PickingHarnessForTest(next) {
		t.Fatal("picker should stay open after empty selection")
	}
}
