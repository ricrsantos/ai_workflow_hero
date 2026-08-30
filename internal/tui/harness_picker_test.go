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
  "harnesses": {"cursor": {"enabled": true}, "opencode": {"enabled": false}, "codex": {"enabled": false}}
}

`))
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	next, _ := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/harness")
	if !tui.PickingHarnessForTest(next) {
		t.Fatal("expected harness picker")
	}
	view := tui.ViewForTest(next)
	if !strings.Contains(view, "[x] Cursor") || !strings.Contains(view, "[ ] OpenCode") || !strings.Contains(view, "[ ] Codex") {
		t.Fatalf("missing checkboxes: %q", view)
	}
	if !strings.Contains(view, "Cursor (") || !strings.Contains(view, "OpenCode (") || !strings.Contains(view, "Codex (") {
		t.Fatalf("missing availability parens: %q", view)
	}
	if !strings.Contains(view, "available") && !strings.Contains(view, "unavailable") {
		t.Fatalf("missing availability label: %q", view)
	}
	if !strings.Contains(view, "space toggle") {
		t.Fatalf("missing checkbox hint: %q", view)
	}
}

func TestHarnessPickerPersistsAutoProjectPermissionProfileInline(t *testing.T) {
	dir := t.TempDir()
	writeHeroJSON(t, dir, []byte(`{
  "harnesses": {"cursor": {"enabled": true}, "opencode": {"enabled": false}, "codex": {"enabled": false}}
}
`))
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	next, _ := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/harness")
	view := tui.ViewForTest(next)
	if !strings.Contains(view, "[x] Cursor") || strings.Count(view, "Permissions:") != 3 || !strings.Contains(view, "Ask every time") || !strings.Contains(view, "Auto approve in project") || !strings.Contains(view, "Auto approve every time (Yolo)") {
		t.Fatalf("expected inline permission controls: %q", view)
	}

	// Cursor is the first Harness; its three permission options follow the
	// non-selectable Permissions heading.
	next = tui.SetPaletteIndexForTest(next, 3)
	next, _ = tui.HandleTestKey(next, " ")
	next, _ = tui.HandleTestKey(next, "enter")
	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := install.HarnessPermissionProfile(hero, "cursor"); got != "auto-project" {
		t.Fatalf("profile=%q", got)
	}
}

func TestHarnessPickerDisabledPermissionRowsDoNotChangeAndNoSelectionDefaultsToAsk(t *testing.T) {
	dir := t.TempDir()
	writeHeroJSON(t, dir, []byte(`{
  "harnesses": {
    "cursor": {"enabled": true, "permission_profile": "auto-project"},
    "opencode": {"enabled": false, "permission_profile": "auto-project"},
    "codex": {"enabled": false}
  }
}
`))
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	next, _ := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/harness")
	view := tui.ViewForTest(next)
	if !strings.Contains(view, "[ ] OpenCode") || !strings.Contains(view, "Auto approve in project") {
		t.Fatalf("expected disabled Harness and its permission rows: %q", view)
	}

	// OpenCode's Automatic option is row 9. Its Harness is disabled, so Space
	// must be a no-op and its persisted profile must remain untouched.
	next = tui.SetPaletteIndexForTest(next, 9)
	next, _ = tui.HandleTestKey(next, " ")

	// Cursor's Automatic option is row 3. Selecting Ask restores the conservative
	// profile after a previously automatic selection.
	// must persist as the conservative Ask profile.
	next = tui.SetPaletteIndexForTest(next, 2)
	next, _ = tui.HandleTestKey(next, " ")
	next, _ = tui.HandleTestKey(next, "enter")

	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := install.HarnessPermissionProfile(hero, "cursor"); got != "ask" {
		t.Fatalf("empty selection profile=%q, want ask", got)
	}
	if got := install.HarnessPermissionProfile(hero, "opencode"); got != "auto-project" {
		t.Fatalf("disabled Harness profile=%q, want unchanged auto-project", got)
	}
}

func TestHarnessPickerPersistsYoloPermissionProfile(t *testing.T) {
	dir := t.TempDir()
	writeHeroJSON(t, dir, []byte(`{
  "harnesses": {"cursor": {"enabled": true}, "opencode": {"enabled": false}, "codex": {"enabled": false}}
}
`))
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	next, _ := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/harness")
	next = tui.SetPaletteIndexForTest(next, 4)
	next, _ = tui.HandleTestKey(next, " ")
	next, _ = tui.HandleTestKey(next, "enter")
	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := install.HarnessPermissionProfile(hero, "cursor"); got != "auto-all" {
		t.Fatalf("profile=%q want auto-all", got)
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

	next, _ := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/harness")
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

func TestHarnessPickerEnableCodexSuccessLine(t *testing.T) {
	dir := t.TempDir()
	writeHeroJSON(t, dir, []byte(`{
  "harnesses": {"cursor": {"enabled": true}, "opencode": {"enabled": false}, "codex": {"enabled": false}}
}
`))
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	next, _ := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/harness")
	if !tui.PickingHarnessForTest(next) {
		t.Fatal("expected harness picker")
	}
	items := tui.FilteredPalette(next)
	idx := paletteIndexByLabel(items, "Codex")
	if idx < 0 {
		t.Fatalf("Codex missing from picker: %v", items)
	}
	next = tui.SetPaletteIndexForTest(next, idx)
	next, _ = tui.HandleTestKey(next, " ")
	view := tui.ViewForTest(next)
	if !strings.Contains(view, "[x] Codex") {
		t.Fatalf("space should check Codex: %q", view)
	}
	next, _ = tui.HandleTestKey(next, "enter")
	if tui.StatusKindForTest(next) != "ok" {
		t.Fatalf("status=%s text=%q", tui.StatusKindForTest(next), tui.StatusTextForTest(next))
	}
	if !strings.Contains(tui.StatusTextForTest(next), "Codex enabled (projected .codex/)") {
		t.Fatalf("text=%q", tui.StatusTextForTest(next))
	}
	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !install.IsHarnessEnabled(hero, "codex") {
		t.Fatal("codex not enabled")
	}
	if _, err := os.Stat(filepath.Join(dir, ".codex", "agents")); err != nil {
		t.Fatalf(".codex/ not projected on enable: %v", err)
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

	next, _ := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/harness")
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

func TestHarnessPickerDisableCodexStopsAppServer(t *testing.T) {
	dir := t.TempDir()
	writeHeroJSON(t, dir, []byte(`{
  "harnesses": {"cursor": {"enabled": true}, "opencode": {"enabled": false}, "codex": {"enabled": true}}
}
`))
	// Seed projected files so disable can assert they are kept.
	codexAgents := filepath.Join(dir, ".codex", "agents")
	if err := os.MkdirAll(codexAgents, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(codexAgents, "keep-me.md")
	if err := os.WriteFile(marker, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	stopCalled := false
	prev := tui.StopCodexAppServerFnForTest()
	tui.SetStopCodexAppServerFnForTest(func(ctx context.Context, projectDir string, st *store.Store, _ harnessmgr.Registry) error {
		stopCalled = true
		if projectDir != dir {
			t.Fatalf("projectDir=%q", projectDir)
		}
		return nil
	})
	t.Cleanup(func() { tui.SetStopCodexAppServerFnForTest(prev) })

	next, _ := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/harness")
	items := tui.FilteredPalette(next)
	idx := paletteIndexByLabel(items, "Codex")
	if idx < 0 {
		t.Fatalf("Codex missing from picker: %v", items)
	}
	next = tui.SetPaletteIndexForTest(next, idx)
	next, _ = tui.HandleTestKey(next, " ")
	next, _ = tui.HandleTestKey(next, "enter")
	if tui.StatusKindForTest(next) != "ok" {
		t.Fatalf("status=%s text=%q", tui.StatusKindForTest(next), tui.StatusTextForTest(next))
	}
	if !strings.Contains(tui.StatusTextForTest(next), "Codex disabled (files kept)") {
		t.Fatalf("text=%q", tui.StatusTextForTest(next))
	}
	if !stopCalled {
		t.Fatal("expected stopCodexAppServer on codex disable")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("disable must keep .codex/ files: %v", err)
	}
	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if install.IsHarnessEnabled(hero, "codex") {
		t.Fatal("codex should be disabled")
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

	next, _ := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/harness")
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

func TestHarnessPickerLastHarnessError_CodexOnly(t *testing.T) {
	dir := t.TempDir()
	writeHeroJSON(t, dir, []byte(`{
  "harnesses": {"cursor": {"enabled": false}, "opencode": {"enabled": false}, "codex": {"enabled": true}}
}
`))
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	next, _ := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/harness")
	items := tui.FilteredPalette(next)
	idx := paletteIndexByLabel(items, "Codex")
	if idx < 0 {
		t.Fatalf("Codex missing from picker: %v", items)
	}
	next = tui.SetPaletteIndexForTest(next, idx)
	next, _ = tui.HandleTestKey(next, " ")
	next, _ = tui.HandleTestKey(next, "enter")
	if tui.StatusKindForTest(next) != "err" {
		t.Fatalf("status=%s text=%q", tui.StatusKindForTest(next), tui.StatusTextForTest(next))
	}
	if !strings.Contains(tui.StatusTextForTest(next), "Select at least one harness") {
		t.Fatalf("text=%q", tui.StatusTextForTest(next))
	}
	if !tui.PickingHarnessForTest(next) {
		t.Fatal("picker should stay open after empty selection")
	}
	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !install.IsHarnessEnabled(hero, "codex") {
		t.Fatal("codex must remain enabled when last-harness guard rejects")
	}
}
