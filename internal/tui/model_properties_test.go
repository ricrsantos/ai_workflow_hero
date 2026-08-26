package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/modelprops"
)

// forceColorProfile pins the lipgloss renderer profile for color-semantic tests.
// go test runs with a non-TTY stdout, which termenv detects as Ascii; without
// forcing a profile the ANSI codes asserted below would never be emitted.
func forceColorProfile(t *testing.T, profile termenv.Profile) {
	t.Helper()
	lipgloss.SetColorProfile(profile)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
}

// propsCatalog builds an in-memory catalog for picker tests.
func propsCatalog(models map[string]map[string]modelprops.CatalogProperty) modelprops.Catalog {
	cat := make(modelprops.Catalog, len(models))
	for id, props := range models {
		for k, p := range props {
			p.HasProperty = true
			props[k] = p
		}
		cat[id] = modelprops.CatalogModel{Properties: props}
	}
	return cat
}

func fullCapabilityModel() map[string]map[string]modelprops.CatalogProperty {
	return map[string]map[string]modelprops.CatalogProperty{
		"full/model": {
			"fs": {Available: true, Values: []string{"true", "false"}, Default: "false"},
			"th": {Available: true, Values: []string{"off", "max"}, Default: "off"},
			"ef": {Available: true, Values: []string{"low", "medium", "high"}, Default: "medium"},
		},
		"partial/model": {
			"th": {Available: true, Values: []string{"off", "max"}, Default: "off"},
		},
	}
}

// newPickerTestModel builds a TUI model with a temp project, store, and catalog.
func newPickerTestModel(t *testing.T) (model, string) {
	t.Helper()
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := []byte(`{
  "harnesses": {
    "cursor": {"enabled": true, "model": "", "enable_fast_model": false},
    "opencode": {"enabled": true}
  },
  "freechat_default": {"harness": "cursor", "model": "old-model"}
}
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), hero, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".workflow-hero", "cycles", "current"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`title: t
objective: t
agents:
  orchestration_agent:
    harness: cursor
    model: composer-2.5
    reasoning_effort: high
    enable_fast_model: false
fallback_model:
  harness: cursor
  model: grok-4.6
`)
	if err := os.WriteFile(filepath.Join(dir, ".workflow-hero", "cycles", "current", "workflow-config.yml"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	m := NewTestModel(svc)
	m = SetAvailableModelsForTest(m, []string{"full/model", "partial/model", "pricing-only"})
	m.propsSvc.Catalog = propsCatalog(fullCapabilityModel())
	return m, dir
}

func TestPropertyPickerOpensWhenSelectable(t *testing.T) {
	m, _ := newPickerTestModel(t)
	m = OpenHeroModelForTest(m) // cursor is the default pair harness
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter") // select cursor harness
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter") // select full/model

	if !m.pickingProps {
		t.Fatal("property picker must open for a full-capability model")
	}
	view := ViewForTest(m)
	for _, want := range []string{
		"/model · Cursor · properties",
		"Fast Mode:",
		"Thinking:",
		"Reasoning effort:",
		"enter save",
		"esc cancel",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestPropertyPickerSkipsAndCommitsWithoutMetadata(t *testing.T) {
	m, dir := newPickerTestModel(t)
	m = OpenHeroModelForTest(m)
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")
	// Select the model with no property metadata (pricing-only).
	m = SetPaletteFilter(m, "pricing")
	items := FilteredPalette(m)
	if len(items) != 1 || items[0].Label != "pricing-only" {
		t.Fatalf("filtered=%v", items)
	}
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")

	if m.pickingProps {
		t.Fatal("picker must not open without selectable properties")
	}
	if PickingModelForTest(m) {
		t.Fatal("picker must close after immediate save")
	}
	data, err := os.ReadFile(filepath.Join(dir, ".workflow-hero", "config", "hero.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"model": "pricing-only"`) {
		t.Fatalf("pair must be committed: %s", data)
	}
	if StatusKindForTest(m) != "warn" {
		t.Fatalf("missing catalog warning expected, got %s", StatusKindForTest(m))
	}
	if !strings.Contains(StatusTextForTest(m), "No catalog is available") {
		t.Fatalf("warning text: %q", StatusTextForTest(m))
	}
}

func TestPropertyPickerSpaceTogglesFastWithoutPersisting(t *testing.T) {
	m, dir := newPickerTestModel(t)
	m = OpenHeroModelForTest(m)
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")

	before, err := os.ReadFile(filepath.Join(dir, ".workflow-hero", "config", "hero.json"))
	if err != nil {
		t.Fatal(err)
	}

	// fs is row 0. Draft starts at the catalog default "false".
	if m.propsDraft["fs"] != "false" {
		t.Fatalf("initial fs draft=%q", m.propsDraft["fs"])
	}
	m, _ = HandleTestKey(m, "space")
	if m.propsDraft["fs"] != "true" {
		t.Fatalf("space must toggle fs to true, got %q", m.propsDraft["fs"])
	}
	m, _ = HandleTestKey(m, "space")
	if m.propsDraft["fs"] != "false" {
		t.Fatalf("second space must toggle fs back, got %q", m.propsDraft["fs"])
	}

	after, err := os.ReadFile(filepath.Join(dir, ".workflow-hero", "config", "hero.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("row edits must not persist any partial state")
	}
}

func TestPropertyPickerSpaceCyclesAndEnterSaves(t *testing.T) {
	m, dir := newPickerTestModel(t)
	m = OpenHeroModelForTest(m)
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")

	// Toggle fs on.
	m, _ = HandleTestKey(m, "space")
	// Row 1 = th: space cycles off → max.
	m = SetPaletteIndexForTest(m, 1)
	m, _ = HandleTestKey(m, "space")
	if m.propsDraft["th"] != "max" {
		t.Fatalf("th draft=%q", m.propsDraft["th"])
	}
	// Row 2 = ef: space cycles medium → high.
	m = SetPaletteIndexForTest(m, 2)
	m, _ = HandleTestKey(m, "space")
	if m.propsDraft["ef"] != "high" {
		t.Fatalf("ef draft=%q", m.propsDraft["ef"])
	}

	m, _ = HandleTestKey(m, "enter")
	if m.pickingProps || PickingModelForTest(m) {
		t.Fatal("picker must close after save")
	}
	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hero.FreechatDefault.Model != "full/model" {
		t.Fatalf("pair: %+v", hero.FreechatDefault)
	}
	props := install.PairProperties(hero, "cursor", "full/model")
	if props["fs"] != "true" || props["th"] != "max" || props["ef"] != "high" {
		t.Fatalf("committed props: %v", props)
	}
	if StatusKindForTest(m) != "ok" {
		t.Fatalf("status=%s", StatusKindForTest(m))
	}
}

func TestPropertyPickerEscapeCancelsCompleteSelection(t *testing.T) {
	m, dir := newPickerTestModel(t)
	m = OpenHeroModelForTest(m)
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")

	m, _ = HandleTestKey(m, "space") // fs=true (draft only)
	m, _ = HandleTestKey(m, "esc")

	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hero.FreechatDefault.Model != "old-model" {
		t.Fatalf("escape must keep the prior pair: %+v", hero.FreechatDefault)
	}
	if props := install.PairProperties(hero, "cursor", "full/model"); len(props) != 0 {
		t.Fatalf("escape must not persist the draft: %v", props)
	}
	if m.chatModelSlug != "old-model" {
		t.Fatalf("in-memory pair must be restored: %s", m.chatModelSlug)
	}
	if m.pickingProps {
		t.Fatal("picker must close after escape")
	}
}

func TestPropertyPickerRestoresPerModelChoices(t *testing.T) {
	m, dir := newPickerTestModel(t)
	openPicker := func(m model, modelID string) model {
		m = OpenHeroModelForTest(m)
		m = SetPaletteIndexForTest(m, 0)
		m, _ = HandleTestKey(m, "enter") // cursor harness
		m = SetPaletteFilter(m, modelID)
		m = SetPaletteIndexForTest(m, 0)
		m, _ = HandleTestKey(m, "enter") // first filtered row is the requested model
		return m
	}
	// Save fs=true for full/model.
	m = openPicker(m, "full/model")
	m, _ = HandleTestKey(m, "space")
	m, _ = HandleTestKey(m, "enter")
	// Save th=max for partial/model (its fs is unavailable → stays na).
	m = openPicker(m, "partial/model")
	if m.propsDraft["fs"] != "na" {
		t.Fatalf("partial model fs must be na, got %q", m.propsDraft["fs"])
	}
	m = SetPaletteIndexForTest(m, 1)
	m, _ = HandleTestKey(m, "space")
	m, _ = HandleTestKey(m, "enter")

	// Reopen full/model: fs=true must be restored independently.
	m = openPicker(m, "full/model")
	if m.propsDraft["fs"] != "true" {
		t.Fatalf("full/model fs must restore to true, got %q", m.propsDraft["fs"])
	}
	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if install.PairProperties(hero, "cursor", "partial/model")["th"] != "max" {
		t.Fatal("partial/model th=max must survive")
	}
}

func TestPropertyPickerDisabledRowIsInert(t *testing.T) {
	m, _ := newPickerTestModel(t)
	m = OpenHeroModelForTest(m)
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")
	m = SetPaletteFilter(m, "partial")
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")
	// Row 2 (ef) is unavailable for partial/model: space/enter must be inert.
	m = SetPaletteIndexForTest(m, 2)
	m, _ = HandleTestKey(m, "space")
	if m.propsDraft["ef"] != "na" {
		t.Fatalf("unavailable ef must stay na: %q", m.propsDraft["ef"])
	}
	m, _ = HandleTestKey(m, "enter")
	if m.pickingProps {
		t.Fatal("enter must save and close the picker")
	}
}

func TestStatusLineShowsPropertyLabels(t *testing.T) {
	forceColorProfile(t, termenv.ANSI)
	m, _ := newPickerTestModel(t)
	m.freechatSnapshot = modelprops.Snapshot{Properties: map[string]harness.PropertyCapability{
		"fs": {Key: "fs", AcceptedValues: []string{"true", "false"}, DefaultValue: "false", Available: true},
		"th": {Key: "th", AcceptedValues: []string{"off", "max"}, DefaultValue: "off", Available: true},
		"ef": {Key: "ef", AcceptedValues: []string{"low", "high"}, DefaultValue: "low", Available: true},
	}}
	m.freechatProps = map[string]string{"fs": "true", "th": "max", "ef": "high"}
	m = EnterConversationForTest(m)

	view := ViewForTest(m)
	for _, want := range []string{"[fast-true]", "[thinking-max]", "[effort-high]"} {
		if !strings.Contains(view, want) {
			t.Fatalf("status line missing %q:\n%s", want, view)
		}
	}
	// Labels are green when validated: ANSI-16 may map the palette green to
	// either standard (32) or bright (92) green depending on the hex token.
	if !strings.Contains(view, "\x1b[32m") && !strings.Contains(view, "\x1b[92m") {
		t.Fatalf("validated labels must be green:\n%s", view)
	}
}

func TestStatusLineFastOffAndNaAreGrayAndTextual(t *testing.T) {
	forceColorProfile(t, termenv.ANSI)
	m, _ := newPickerTestModel(t)
	m.freechatProps = map[string]string{"fs": "false", "th": "na", "ef": "na"}
	m = EnterConversationForTest(m)
	view := ViewForTest(m)
	for _, want := range []string{"[fast-false]", "[thinking-na]", "[effort-na]"} {
		if !strings.Contains(view, want) {
			t.Fatalf("status line missing %q", want)
		}
	}
	if strings.Contains(view, "\x1b[32m[fast-false]") || strings.Contains(view, "\x1b[92m[fast-false]") {
		t.Fatalf("fast=false must not be green:\n%s", view)
	}
}

func TestStatusLineVisibleInEmptyChat(t *testing.T) {
	m, _ := newPickerTestModel(t)
	m.freechatProps = map[string]string{"fs": "true", "th": "max", "ef": "high"}
	m.freechatSnapshot = modelprops.Snapshot{Properties: map[string]harness.PropertyCapability{
		"fs": {Key: "fs", AcceptedValues: []string{"true", "false"}, Available: true},
		"th": {Key: "th", AcceptedValues: []string{"max"}, Available: true},
		"ef": {Key: "ef", AcceptedValues: []string{"high"}, Available: true},
	}}
	m = EnterConversationForTest(m)
	view := ViewForTest(m)
	if !strings.Contains(view, "Submit a message to start an interação") {
		t.Skip("empty chat placeholder changed")
	}
	if !strings.Contains(view, "[fast-true]") {
		t.Fatalf("empty chat must still show property labels:\n%s", view)
	}
}

func TestStatusLineNarrowWrapsWithoutHiding(t *testing.T) {
	forceColorProfile(t, termenv.Ascii) // deterministic rune-safe wrapping math
	m, _ := newPickerTestModel(t)
	m.freechatProps = map[string]string{"fs": "true", "th": "max", "ef": "high"}
	m.freechatSnapshot = modelprops.Snapshot{Properties: map[string]harness.PropertyCapability{
		"fs": {Key: "fs", AcceptedValues: []string{"true", "false"}, Available: true},
		"th": {Key: "th", AcceptedValues: []string{"max"}, Available: true},
		"ef": {Key: "ef", AcceptedValues: []string{"high"}, Available: true},
	}}
	m = SetWidth(m, 24)
	m = EnterConversationForTest(m)
	view := ViewForTest(m)
	for _, want := range []string{"[fast-true]", "[thinking-max]", "[effort-high]"} {
		if !strings.Contains(view, want) {
			t.Fatalf("narrow terminal must not hide %q:\n%s", want, view)
		}
	}
	lines := m.renderChatStatusLines(24)
	if len(lines) < 2 {
		t.Fatalf("narrow width must wrap onto multiple lines: %v", lines)
	}
}

func TestStatusLineMultiByteContentDoesNotPanic(t *testing.T) {
	forceColorProfile(t, termenv.Ascii) // deterministic rune-safe wrapping math
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("multi-byte wrapping panicked: %v", r)
		}
	}()
	m, _ := newPickerTestModel(t)
	m.freechatProps = map[string]string{"fs": "true", "th": "max", "ef": "high"}
	m.chatModelSlug = "モデル🚀/model-with-unicode"
	m.contextUsedTokens = 1234
	m.contextWindows = contextWindowCatalog{"モデル🚀/model-with-unicode": 200000}
	for _, w := range []int{20, 24, 30, 42, 80} {
		_ = m.renderChatStatusLines(w)
	}
}

func TestWorkflowProjectionShowsUnvalidatedValues(t *testing.T) {
	forceColorProfile(t, termenv.ANSI)
	m, _ := newPickerTestModel(t)
	// Orchestrator YAML has reasoning_effort high; fast disabled.
	m = m.withRuntimeAgent(agentOrchestration)
	if !m.workflowAgentActive() {
		t.Fatal("workflow projection must be active")
	}
	if m.workflowProps["ef"] != "high" || m.workflowProps["fs"] != "false" {
		t.Fatalf("workflow props: %v", m.workflowProps)
	}
	m = EnterConversationForTest(m)
	view := ViewForTest(m)
	if !strings.Contains(view, "[effort-high]") {
		t.Fatalf("workflow ef must appear on the status line:\n%s", view)
	}
	// Unvalidated workflow values stay gray (never green).
	if strings.Contains(view, "\x1b[32m[effort-high]") || strings.Contains(view, "\x1b[92m[effort-high]") {
		t.Fatalf("unvalidated workflow ef must be gray:\n%s", view)
	}
}

func TestWarningClearsOnNextUserAction(t *testing.T) {
	m, _ := newPickerTestModel(t)
	m = m.setPropsWarning(modelprops.WarningMissingCatalog)
	if StatusKindForTest(m) != "warn" {
		t.Fatalf("status=%s", StatusKindForTest(m))
	}
	view := ViewForTest(m)
	if !strings.Contains(view, "No catalog is available") {
		t.Fatalf("warning must render: %s", view)
	}
	// Any user key clears the warning.
	m, _ = HandleTestKey(m, "down")
	if StatusKindForTest(m) == "warn" {
		t.Fatal("warning must clear on the next user action")
	}
}

func TestExecutionSendsFreechatProperties(t *testing.T) {
	svc, h := newConversationTestService(t)
	m := NewTestModel(svc)
	m.freechatProps = map[string]string{"fs": "true", "ef": "high"}
	m.chatHarnessID = "cursor"
	m.chatModelSlug = "composer-2.5"

	m = SetConversationInput(m, "hello")
	m, cmd := SubmitConversationForTest(m)
	if cmd == nil {
		t.Fatal("expected execute cmd")
	}
	// Drain the stream channel synchronously.
	msg := RunCmdForTest(cmd)
	_ = msg
	// The goroutine may still be running; apply stream messages until done.
	for i := 0; i < 50 && IsConversationStreaming(m); i++ {
		done := ExecuteDoneResultForTest(nil, nil)
		m, _ = HandleTestMsg(m, done)
	}
	if h.lastProps["fs"] != "true" || h.lastProps["ef"] != "high" {
		t.Fatalf("freechat properties must reach Execute: %v", h.lastProps)
	}
}

func TestHeroNewExecutionSendsFreechatProperties(t *testing.T) {
	svc, h := newConversationTestService(t)
	commandDir := filepath.Join(svc.ProjectDir, ".cursor", "commands")
	if err := os.MkdirAll(commandDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commandDir, "hero-new.md"), []byte("prepare a new cycle"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m.chatHarnessID = "cursor"
	m.chatModelSlug = "composer-2.5"
	m.freechatProps = map[string]string{"fs": "true", "th": "max", "ef": "high"}
	next, cmd := BeginHeroRuntimeConversationForTest(m, "new")
	if cmd == nil || !IsConversationStreaming(next) {
		t.Fatal("/hero-new must start an Execute stream")
	}
	next = drainConversationStream(t, next, cmd)
	if IsConversationStreaming(next) {
		t.Fatal("/hero-new stream did not finish")
	}
	if h.lastProps["fs"] != "true" || h.lastProps["th"] != "max" || h.lastProps["ef"] != "high" {
		t.Fatalf("/hero-new must use freechat properties: %v", h.lastProps)
	}
}

func TestPropertyRejectionSurfacesRedErrorAndKeepsChoices(t *testing.T) {
	svc, h := newConversationTestService(t)
	dir := svc.ProjectDir
	m := NewTestModel(svc)
	m.freechatProps = map[string]string{"fs": "true", "ef": "high"}
	m.chatHarnessID = "cursor"
	m.chatModelSlug = "composer-2.5"

	h.err = harness.PropertyRejection("ef", "cursor", "composer-2.5", errors.New("unsupported"))
	m = SetConversationInput(m, "hello")
	_, cmd := SubmitConversationForTest(m)
	_ = RunCmdForTest(cmd)

	m, _ = HandleTestMsg(m, ExecuteDoneMsgForTest(h.err))
	if StatusKindForTest(m) != "err" {
		t.Fatalf("rejection must be a red execution error, got %s", StatusKindForTest(m))
	}
	if !strings.Contains(StatusTextForTest(m), `property "ef" rejected`) {
		t.Fatalf("rejection must identify the property: %q", StatusTextForTest(m))
	}
	// The persisted choice stays intact for the next correction.
	hero, err := install.LoadHeroJSON(dir)
	if err == nil {
		_ = hero // fresh project; nothing written by the rejection
	}
	if h.executeCount != 1 {
		t.Fatalf("executeCount=%d want 1 (no silent retry)", h.executeCount)
	}
	if h.lastProps["ef"] != "high" {
		t.Fatalf("adapter received the chosen property (no stripping): %v", h.lastProps)
	}
}

func TestWorkflowExecutionSendsYAMLProjection(t *testing.T) {
	svc, h := newConversationTestService(t)
	// Orchestrator block (see helper YAML): fast disabled, reasoning_effort
	// medium, thinking na. The YAML is authoritative for workflow commands.
	m := NewTestModel(svc)
	m = m.withRuntimeAgent(agentOrchestration)
	m.runtimeModelSlug = "composer-2.5"

	m = SetConversationInput(m, "start")
	_, cmd := SubmitConversationForTest(m)
	_ = RunCmdForTest(cmd)
	for i := 0; i < 50 && IsConversationStreaming(m); i++ {
		m, _ = HandleTestMsg(m, ExecuteDoneResultForTest(nil, nil))
	}
	if h.lastProps["ef"] != "medium" {
		t.Fatalf("workflow ef must come from YAML: %v", h.lastProps)
	}
	if h.lastProps["fs"] != "false" {
		t.Fatalf("workflow fs=false must be projected: %v", h.lastProps)
	}
	// thinking: na is an effective sentinel and must not reach the adapter.
	if _, ok := h.lastProps["th"]; ok {
		t.Fatalf("YAML thinking na must be omitted from transport: %v", h.lastProps)
	}
}

func TestPropertyPickerEscapeRestoresChatSlashOverlay(t *testing.T) {
	m, _ := newPickerTestModel(t)
	m = EnterConversationForTest(m)
	m = typeChat(t, m, "/model")
	m, _ = HandleTestKey(m, "tab")
	if CurrentScreen(m) != ScreenPalette {
		t.Fatalf("screen=%v want palette", CurrentScreen(m))
	}
	m, _ = DeliverRefreshDoneForTest(m, nil)
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")
	if !m.pickingProps {
		t.Fatal("property picker must be open")
	}
	m, _ = HandleTestKey(m, "esc")
	if CurrentScreen(m) != ScreenConversation {
		t.Fatalf("screen=%v want conversation", CurrentScreen(m))
	}
	m = typeChat(t, m, "/")
	if !ChatSlashOverlayActiveForTest(m) {
		t.Fatalf("chat slash overlay must work after esc from property picker, items=%d",
			len(FilteredChatSlashForTest(m)))
	}
	view := ViewForTest(m)
	if !strings.Contains(view, "/model") {
		t.Fatalf("overlay missing /model: %q", view)
	}
}
