package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestHeroScreenshotCommandParsingReservedSelectors(t *testing.T) {
	tests := []struct {
		input    string
		selector string
		id       string
		ok       bool
	}{
		{input: "/hero-screenshot", selector: screenshotSelectorLatest, ok: true},
		{input: "/hero-screenshot latest", selector: screenshotSelectorLatest, ok: true},
		{input: "/hero-screenshot list", selector: screenshotSelectorList, ok: true},
		{input: "/hero-screenshot todos", selector: screenshotSelectorTodos, ok: true},
		{input: "/hero-screenshot shot-17", selector: "id", id: "shot-17", ok: true},
		{input: "/hero-screenshot todos extra", selector: "invalid", ok: true},
		{input: "/hero-status", ok: false},
	}
	for _, test := range tests {
		selector, id, ok := parseScreenshotSlash(test.input)
		if selector != test.selector || id != test.id || ok != test.ok {
			t.Errorf("parseScreenshotSlash(%q)=(%q,%q,%t), want (%q,%q,%t)", test.input, selector, id, ok, test.selector, test.id, test.ok)
		}
	}
}

func TestHeroScreenshotControlsStayLocalDuringStreaming(t *testing.T) {
	svc := newTestService(t)
	m := NewTestModel(svc)
	m.streaming = true
	m = SetConversationInput(m, "/hero-screenshot todos")
	beforeRows, err := svc.Store.ListReadyScreenshotManifests(t.Context(), mustActiveCycleID(t, svc))
	if err != nil {
		t.Fatal(err)
	}
	beforeExecuteCount := len(m.executes)
	sink := m.convSink.(*recordingSink)

	next, cmd := HandleTestKey(m, "enter")
	if !next.screenshots.open || !next.screenshots.loading || !next.streaming {
		t.Fatalf("typed control did not open asynchronously while streaming: open=%t loading=%t streaming=%t", next.screenshots.open, next.screenshots.loading, next.streaming)
	}
	if next.input != "" {
		t.Fatalf("local control input was not consumed: %q", next.input)
	}
	if cmd == nil {
		t.Fatal("typed command should schedule only the local ready-set read")
	}
	msg, ok := cmd().(screenshotSnapshotMsg)
	if !ok || msg.err || msg.noCycle {
		t.Fatalf("snapshot command returned %T %+v", msg, msg)
	}
	updated, _ := next.Update(msg)
	next = updated.(model)
	if next.streaming != true || len(next.executes) != beforeExecuteCount || len(sink.ch) != 0 {
		t.Fatal("screenshot control changed or dispatched the active harness stream")
	}
	afterRows, err := svc.Store.ListReadyScreenshotManifests(t.Context(), mustActiveCycleID(t, svc))
	if err != nil {
		t.Fatal(err)
	}
	if len(afterRows) != len(beforeRows) {
		t.Fatalf("screenshot command created a capture: before=%d after=%d", len(beforeRows), len(afterRows))
	}
	if !strings.Contains(next.screenshots.message, "No screenshot has been captured yet.") {
		t.Fatalf("empty ready set message=%q", next.screenshots.message)
	}
}

func TestHeroScreenshotShortcutLoadsValidatedCardsAndMetadata(t *testing.T) {
	svc := newTestService(t)
	cycleID := mustActiveCycleID(t, svc)
	addTUIScreenshot(t, svc, cycleID, "shot-001", "screen-dashboard", "2026-09-30T12:00:00Z", 1)
	addTUIScreenshot(t, svc, cycleID, "shot-002", "screen-admin", "2026-09-30T12:01:00Z", 2)
	m := NewTestModel(svc)
	m.streaming = true

	next, cmd := HandleTestKey(m, "alt+b")
	if !next.screenshots.open || !next.screenshots.loading || !next.streaming {
		t.Fatalf("Alt+B did not expose screenshot collection during streaming: %+v", next.screenshots)
	}
	if cmd == nil {
		t.Fatal("Alt+B should asynchronously load the ready set")
	}
	snapshot, ok := cmd().(screenshotSnapshotMsg)
	if !ok || snapshot.err || snapshot.cycleID != cycleID || len(snapshot.assets) != 2 {
		t.Fatalf("snapshot=%T %+v", snapshot, snapshot)
	}
	updated, _ := next.Update(snapshot)
	next = updated.(model)
	view := stripANSI(ViewForTest(next))
	for _, want := range []string{
		"shot-002", "screen-admin", "stage: browser_ui_validation", "attempt: 2",
		"user: operator", "profile: Admin", "2026-09-30", "passed", "A open all",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("screenshot collection lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, filepath.Join(svc.ProjectDir, screenshotCurrentDir)) {
		t.Fatal("screenshot collection exposed an internal image path")
	}

	next, _ = HandleTestKey(next, "down")
	if next.screenshots.selected != 1 {
		t.Fatalf("down selected card %d, want 1", next.screenshots.selected)
	}
	next, _ = HandleTestKey(next, "home")
	if next.screenshots.selected != 0 {
		t.Fatalf("home selected card %d, want 0", next.screenshots.selected)
	}

	helpHasShortcut := false
	for _, group := range next.helpGroups() {
		for _, entry := range group.entries {
			if entry.keys == "Alt+B" {
				helpHasShortcut = true
			}
		}
	}
	if !helpHasShortcut || !strings.Contains(fixedFooterHints, "alt+b screenshots") {
		t.Fatal("Alt+B must be discoverable in both /help and the footer")
	}
}

func TestHeroScreenshotNoCaptureAndUnknownIDAreDistinct(t *testing.T) {
	emptyModel := NewTestModel(newTestService(t))
	emptyModel, emptyCmd := emptyModel.openScreenshotCollection("id", "missing")
	if emptyCmd == nil {
		t.Fatal("unknown ID request should load the ready-set snapshot")
	}
	emptyMsg, ok := emptyCmd().(screenshotSnapshotMsg)
	if !ok {
		t.Fatalf("empty snapshot message=%T", emptyMsg)
	}
	emptyModel = emptyModel.handleScreenshotSnapshot(emptyMsg)
	noCapture := emptyModel.screenshots.message
	if !strings.Contains(noCapture, "No screenshot has been captured yet.") {
		t.Fatalf("no-capture error=%q", noCapture)
	}

	svc := newTestService(t)
	cycleID := mustActiveCycleID(t, svc)
	addTUIScreenshot(t, svc, cycleID, "shot-001", "screen-dashboard", "2026-09-30T12:00:00Z", 1)
	unknownModel, unknownCmd := NewTestModel(svc).openScreenshotCollection("id", "missing")
	if unknownCmd == nil {
		t.Fatal("unknown ID request should load the ready-set snapshot")
	}
	unknownMsg := unknownCmd().(screenshotSnapshotMsg)
	unknownModel = unknownModel.handleScreenshotSnapshot(unknownMsg)
	unknown := unknownModel.screenshots.message
	if !strings.Contains(unknown, "Screenshot ID was not found.") || unknown == noCapture {
		t.Fatalf("unknown-ID error=%q; no-capture error=%q", unknown, noCapture)
	}
}

func TestTodosBatchReportsProgressAndPartialFailureIDs(t *testing.T) {
	m := NewTestModel(nil)
	m.screenshots = screenshotCollectionState{
		open:       true,
		selector:   screenshotSelectorTodos,
		cycleID:    9,
		assets:     []store.ScreenshotManifest{{ID: "shot-001"}, {ID: "shot-002"}, {ID: "shot-003"}},
		generation: 2,
	}
	m, cmd := m.beginOpenAllScreenshots()
	if cmd == nil || !m.screenshots.batchRunning || m.screenshots.batchIndex != 0 {
		t.Fatal("Open all should schedule one bounded asynchronous launch")
	}
	generation := m.screenshots.batchGeneration
	updated, tick := m.handleScreenshotViewerResult(screenshotViewerResultMsg{generation: generation, screenshotID: "shot-001", err: true, batch: true})
	m = updated.(model)
	if tick == nil || m.screenshots.batchIndex != 1 || !strings.Contains(m.screenshots.message, "Opening 1/3") {
		t.Fatalf("first batch result did not schedule bounded progress: state=%+v tick=%v", m.screenshots, tick != nil)
	}
	updated, nextLaunch := m.Update(screenshotBatchNextMsg{generation: generation})
	m = updated.(model)
	if nextLaunch == nil {
		t.Fatal("next screenshot should not launch until the scheduled batch tick")
	}
	updated, tick = m.handleScreenshotViewerResult(screenshotViewerResultMsg{generation: generation, screenshotID: "shot-002", batch: true})
	m = updated.(model)
	if tick == nil || m.screenshots.batchIndex != 2 {
		t.Fatal("second result should schedule the third bounded launch")
	}
	updated, _ = m.handleScreenshotViewerResult(screenshotViewerResultMsg{generation: generation, screenshotID: "shot-003", batch: true})
	m = updated.(model)
	if m.screenshots.batchRunning || !strings.Contains(m.screenshots.message, "failed IDs: shot-001") {
		t.Fatalf("batch should complete with exact partial-failure IDs: running=%t message=%q", m.screenshots.batchRunning, m.screenshots.message)
	}
}

func TestScreenshotSaveUsesPrivateCopyAndRefusesSymlink(t *testing.T) {
	projectDir := t.TempDir()
	source := writeAcceptancePNG(t, projectDir, "source.png")
	destination := filepath.Join(projectDir, "saved.png")
	needed, err := saveScreenshotCopy(source, destination, false, projectDir)
	if err != nil || needed {
		t.Fatalf("save: overwrite=%t err=%v", needed, err)
	}
	info, err := os.Stat(destination)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("saved screenshot mode=%v err=%v, want 0600", info, err)
	}
	if needed, err := saveScreenshotCopy(source, destination, false, projectDir); err != nil || !needed {
		t.Fatalf("existing destination: overwrite=%t err=%v, want overwrite confirmation", needed, err)
	}
	if needed, err := saveScreenshotCopy(source, destination, true, projectDir); err != nil || needed {
		t.Fatalf("confirmed overwrite: overwrite=%t err=%v", needed, err)
	}
	if err := os.Remove(destination); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(destination), "link.png")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	if _, err := saveScreenshotCopy(source, link, true, projectDir); err == nil {
		t.Fatal("save must refuse a symlink destination")
	}
	if _, err := saveScreenshotCopy(source, source, true, projectDir); err == nil {
		t.Fatal("save must refuse to overwrite its managed screenshot source")
	}
	credential := filepath.Join(projectDir, ".env.hero")
	const sentinel = "DO_NOT_OVERWRITE_CREDENTIALS"
	if err := os.WriteFile(credential, []byte(sentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := saveScreenshotCopy(source, credential, true, projectDir); err == nil {
		t.Fatal("save must refuse the managed .env.hero credential destination")
	}
	contents, err := os.ReadFile(credential)
	if err != nil || string(contents) != sentinel {
		t.Fatalf("credential destination changed: value=%q err=%v", contents, err)
	}
	otherCredential := filepath.Join(projectDir, "credentials.json")
	if _, err := saveScreenshotCopy(source, otherCredential, false, projectDir); err == nil {
		t.Fatal("save must refuse another credential-like destination")
	}
}

func addTUIScreenshot(t *testing.T, svc *cycle.Service, cycleID int64, id, coverage, timestamp string, attempt int) {
	t.Helper()
	stages, err := svc.Store.ListStages(cycleID)
	if err != nil {
		t.Fatal(err)
	}
	hasBrowserStage := false
	for _, stage := range stages {
		if stage.Name == "browser_ui_validation" {
			hasBrowserStage = true
			break
		}
	}
	if !hasBrowserStage {
		if err := svc.Store.CreateStages([]store.Stage{{
			CycleID:       cycleID,
			Name:          "browser_ui_validation",
			Status:        "Pending",
			MaxIterations: 1,
			SortOrder:     len(stages),
		}}); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(svc.ProjectDir, screenshotCurrentDir, "screenshots")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := writeAcceptancePNG(t, root, id+".png")
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	manifest := store.ScreenshotManifest{
		ID:            id,
		CycleID:       cycleID,
		StageName:     "browser_ui_validation",
		Attempt:       attempt,
		CoverageID:    coverage,
		UserID:        "operator",
		ProfileID:     "Admin",
		CapturedAt:    timestamp,
		Path:          "screenshots/" + id + ".png",
		Result:        store.ScreenshotResultPassed,
		CaptureStatus: store.ScreenshotCaptureReady,
	}
	if err := svc.Store.InsertScreenshotManifest(t.Context(), manifest); err != nil {
		t.Fatal(err)
	}
}

func mustActiveCycleID(t *testing.T, svc *cycle.Service) int64 {
	t.Helper()
	active, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	return active.ID
}
