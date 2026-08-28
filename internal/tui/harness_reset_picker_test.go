package tui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/tui"
)

func TestHarnessResetPickerShowsEnabledHarnessesOnly(t *testing.T) {
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

	next := tui.RunHarnessResetPickerForTest(tui.NewTestModel(svc))
	if !tui.PickingHarnessResetForTest(next) {
		t.Fatal("expected harness reset picker")
	}
	view := tui.ViewForTest(next)
	if !strings.Contains(view, "Cursor") {
		t.Fatalf("expected Cursor in picker: %q", view)
	}
	if strings.Contains(view, "OpenCode") {
		t.Fatalf("disabled OpenCode should not appear: %q", view)
	}
	if !strings.Contains(view, "enter reset") {
		t.Fatalf("missing reset hint: %q", view)
	}
}

func TestHarnessResetShowsLoadingAnimation(t *testing.T) {
	dir := t.TempDir()
	writeHeroJSON(t, dir, []byte(`{
  "harnesses": {"cursor": {"enabled": true}}
}
`))
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	m, _ := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/harness-reset")
	if !tui.HarnessResetAwaitingOpenForTest(m) {
		t.Fatal("expected loading state")
	}
	view := tui.ViewForTest(m)
	if !strings.Contains(view, "Preparing harness list") {
		t.Fatalf("missing loading copy: %q", view)
	}
}

func TestHarnessResetIgnoresEnterWhileLoading(t *testing.T) {
	dir := t.TempDir()
	writeHeroJSON(t, dir, []byte(`{
  "harnesses": {"cursor": {"enabled": true}}
}
`))
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	m, cmd := tui.RunPaletteItemForTest(tui.NewTestModel(svc), "/harness-reset")
	if !tui.HarnessResetAwaitingOpenForTest(m) {
		t.Fatal("expected loading state")
	}
	m, _ = tui.HandleTestKey(m, "enter")
	if tui.PickingHarnessResetForTest(m) {
		t.Fatal("enter must not select a harness before the picker is ready")
	}
	if tui.StatusTextForTest(m) != "" {
		t.Fatalf("unexpected status during load: %q", tui.StatusTextForTest(m))
	}
	m = tui.CompleteHarnessResetPickerForTest(m, cmd)
	if !tui.PickingHarnessResetForTest(m) {
		t.Fatal("picker should open after load completes")
	}
}

func TestHarnessResetOpenCodeNotStarted(t *testing.T) {
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

	m := tui.RunHarnessResetPickerForTest(tui.NewTestModel(svc))
	items := tui.PaletteItemsForTest(m)
	idx := paletteIndexByLabel(items, "OpenCode")
	if idx < 0 {
		t.Fatal("OpenCode not in reset picker")
	}
	m = tui.SetPaletteIndexForTest(m, idx)
	m, _ = tui.HandleTestKey(m, "enter")

	if tui.StatusTextForTest(m) != "OpenCode has not been started by Hero yet." {
		t.Fatalf("status=%q", tui.StatusTextForTest(m))
	}
	if tui.StatusKindForTest(m) != "warn" {
		t.Fatalf("status kind=%q want warn", tui.StatusKindForTest(m))
	}
}

func TestHarnessResetCodexNotStarted(t *testing.T) {
	dir := t.TempDir()
	writeHeroJSON(t, dir, []byte(`{
  "harnesses": {"cursor": {"enabled": true}, "codex": {"enabled": true}}
}
`))
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	m := tui.RunHarnessResetPickerForTest(tui.NewTestModel(svc))
	items := tui.PaletteItemsForTest(m)
	idx := paletteIndexByLabel(items, "Codex")
	if idx < 0 {
		t.Fatal("Codex not in reset picker")
	}
	m = tui.SetPaletteIndexForTest(m, idx)
	m, _ = tui.HandleTestKey(m, "enter")

	if tui.StatusTextForTest(m) != "Codex has not been started by Hero yet." {
		t.Fatalf("status=%q", tui.StatusTextForTest(m))
	}
	if tui.StatusKindForTest(m) != "warn" {
		t.Fatalf("status kind=%q want warn", tui.StatusKindForTest(m))
	}
}

func TestHarnessResetCursorNothingRunning(t *testing.T) {
	dir := t.TempDir()
	writeHeroJSON(t, dir, []byte(`{
  "harnesses": {"cursor": {"enabled": true}}
}
`))
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	m := tui.RunHarnessResetPickerForTest(tui.NewTestModel(svc))
	m, _ = tui.HandleTestKey(m, "enter")

	if tui.StatusTextForTest(m) != "No Cursor agent process is running." {
		t.Fatalf("status=%q", tui.StatusTextForTest(m))
	}
	if tui.StatusKindForTest(m) != "warn" {
		t.Fatalf("status kind=%q want warn", tui.StatusKindForTest(m))
	}
}

func TestHarnessResetOpenCodeStopsManagedServe(t *testing.T) {
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

	if _, err := svc.Store.InsertServeRegistry(store.ServeRegistryEntry{
		Harness:   "opencode",
		PID:       4242,
		Port:      45123,
		URL:       "http://127.0.0.1:45123",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}

	m := tui.RunHarnessResetPickerForTest(tui.SetContextUsedTokensForTest(tui.NewTestModel(svc), 1234))
	items := tui.PaletteItemsForTest(m)
	idx := paletteIndexByLabel(items, "OpenCode")
	m = tui.SetPaletteIndexForTest(m, idx)
	m, _ = tui.HandleTestKey(m, "enter")

	if !strings.Contains(tui.StatusTextForTest(m), "OpenCode serve stopped") {
		t.Fatalf("unexpected status: %q", tui.StatusTextForTest(m))
	}
	entries, err := svc.Store.ListServeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected serve registry cleared, got %d entries", len(entries))
	}
	if got := tui.ContextUsedTokensForTest(m); got != 0 {
		t.Fatalf("context used=%d want 0 after OpenCode reset", got)
	}
}

func TestHarnessResetOpenCodeKeepsSessionID(t *testing.T) {
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

	if _, err := svc.Store.InsertServeRegistry(store.ServeRegistryEntry{
		Harness:   "opencode",
		PID:       4242,
		Port:      45123,
		URL:       "http://127.0.0.1:45123",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}

	m := tui.SetContextUsedTokensForTest(tui.SetChatHarnessIDForTest(tui.NewTestModel(svc), "opencode"), 1234)
	m = tui.SetHarnessSessionIDForTest(m, "keep-sess")
	m = tui.SetHarnessSessionHarnessIDForTest(m, "opencode")
	m = tui.RunHarnessResetPickerForTest(m)
	items := tui.PaletteItemsForTest(m)
	idx := paletteIndexByLabel(items, "OpenCode")
	if idx < 0 {
		t.Fatal("OpenCode not in reset picker")
	}
	m = tui.SetPaletteIndexForTest(m, idx)
	m, _ = tui.HandleTestKey(m, "enter")

	if tui.HarnessSessionIDForTest(m) != "keep-sess" {
		t.Fatalf("OpenCode reset must keep session id, got %q", tui.HarnessSessionIDForTest(m))
	}
	if tui.HarnessSessionHarnessIDForTest(m) != "opencode" {
		t.Fatalf("harness binding = %q", tui.HarnessSessionHarnessIDForTest(m))
	}
	if got := tui.ContextUsedTokensForTest(m); got != 0 {
		t.Fatalf("context used=%d want 0 after OpenCode reset", got)
	}
}

func TestHarnessResetCodexStopsManagedAppServer(t *testing.T) {
	dir := t.TempDir()
	writeHeroJSON(t, dir, []byte(`{
  "harnesses": {"cursor": {"enabled": true}, "codex": {"enabled": true}}
}
`))
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	if _, err := svc.Store.InsertCodexServeRegistry(dir, 4242); err != nil {
		t.Fatal(err)
	}

	m := tui.RunHarnessResetPickerForTest(tui.SetContextUsedTokensForTest(tui.NewTestModel(svc), 1234))
	items := tui.PaletteItemsForTest(m)
	idx := paletteIndexByLabel(items, "Codex")
	if idx < 0 {
		t.Fatal("Codex not in reset picker")
	}
	m = tui.SetPaletteIndexForTest(m, idx)
	m, _ = tui.HandleTestKey(m, "enter")

	if !strings.Contains(tui.StatusTextForTest(m), "Codex app-server stopped") {
		t.Fatalf("unexpected status: %q", tui.StatusTextForTest(m))
	}
	if tui.StatusKindForTest(m) != "ok" {
		t.Fatalf("status kind=%q want ok", tui.StatusKindForTest(m))
	}
	entries, err := svc.Store.ListCodexServeRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected codex registry cleared, got %d entries", len(entries))
	}
	if got := tui.ContextUsedTokensForTest(m); got != 0 {
		t.Fatalf("context used=%d want 0 after Codex reset", got)
	}
}
