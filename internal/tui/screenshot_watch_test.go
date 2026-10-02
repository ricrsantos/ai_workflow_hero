package tui

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

func TestImageDeliveryAutoForwardsNewReadyCaptureOnceDuringStage(t *testing.T) {
	svc, cycleID := newScreenshotWatchFixture(t, true)
	addTUIScreenshot(t, svc, cycleID, "shot-auto-1", "screen-dashboard", "2026-10-01T12:00:00Z", 1)
	snapshot := screenshotWatchSnapshotCmd(svc.ProjectDir, svc.Store, stageBrowserUI, 1, 9)().(screenshotWatchSnapshotMsg)
	if snapshot.err || !snapshot.active || !snapshot.enabled || len(snapshot.assets) != 1 {
		t.Fatalf("watch snapshot did not load the active ready set: %+v", snapshot)
	}

	local, peer := net.Pipe()
	t.Cleanup(func() {
		if err := local.Close(); err != nil {
			t.Errorf("close local pipe: %v", err)
		}
	})
	t.Cleanup(func() {
		if err := peer.Close(); err != nil {
			t.Errorf("close peer pipe: %v", err)
		}
	})
	m := NewTestModel(svc)
	m.streaming = true
	m.screenshots = screenshotCollectionState{
		autoWatchActive: true, autoWatchGen: 9, autoWatchStage: stageBrowserUI,
		autoWatchAttempt: 1,
	}
	m.telegram = &telegramState{
		connected: true, paired: true, address: "proj", alwaysSend: true,
		client: &telegramClient{conn: ipc.NewConn(local)}, daemonCaps: []string{ipc.CapabilityImageDelivery},
	}
	next, command := m.handleScreenshotWatchSnapshot(snapshot)
	if command == nil || !containsScreenshotID(next.screenshots.autoWatchSeen, "shot-auto-1") {
		t.Fatalf("new image was not scheduled and recorded for delivery: command=%t state=%+v", command != nil, next.screenshots)
	}
	if !next.streaming || len(next.executes) != 0 || !next.screenshots.autoWatchActive {
		t.Fatal("automatic forwarding changed the active harness execution")
	}
	if got := strings.Count(transcriptText(next.transcript), "Sending 1 newly captured screenshot"); got != 1 {
		t.Fatalf("automatic delivery notice count=%d", got)
	}

	duplicate, duplicateCmd := next.handleScreenshotWatchSnapshot(snapshot)
	if duplicateCmd == nil || strings.Count(transcriptText(duplicate.transcript), "Sending 1 newly captured screenshot") != 1 {
		t.Fatal("the same ready manifest was scheduled more than once")
	}

	delivery := telegramScreenshotDeliveryCmd(m.telegram.client, next.screenshots.assets, 0)
	finished := make(chan tea.Msg, 1)
	go func() { finished <- delivery() }()
	frame, err := ipc.NewConn(peer).Recv()
	if err != nil {
		t.Fatal(err)
	}
	if frame.Type != ipc.TypeOutboundImageBatch || frame.ImageBatchID == "" || len(frame.Images) != 1 || frame.Images[0].ScreenshotID != "shot-auto-1" {
		t.Fatalf("automatic image delivery frame=%+v", frame)
	}
	if got := <-finished; got != nil {
		t.Fatalf("delivery command returned %T", got)
	}
}

func TestAlwaysSendOffKeepsCaptureLocalAndDoesNotQueueDelivery(t *testing.T) {
	svc, cycleID := newScreenshotWatchFixture(t, true)
	addTUIScreenshot(t, svc, cycleID, "shot-local-1", "screen-dashboard", "2026-10-01T12:00:00Z", 1)
	snapshot := screenshotWatchSnapshotCmd(svc.ProjectDir, svc.Store, stageBrowserUI, 1, 4)().(screenshotWatchSnapshotMsg)
	m := NewTestModel(svc)
	m.screenshots = screenshotCollectionState{
		autoWatchActive: true, autoWatchGen: 4, autoWatchStage: stageBrowserUI, autoWatchAttempt: 1,
	}
	m.telegram = &telegramState{connected: true, paired: true, address: "proj", alwaysSend: false}
	next, command := m.handleScreenshotWatchSnapshot(snapshot)
	if command == nil || len(next.screenshots.autoWatchSeen) != 0 || !hasScreenshotManifest(next.screenshots.assets, "shot-local-1") {
		t.Fatalf("Always-send Off did not retain a local, unsent capture: state=%+v command=%t", next.screenshots, command != nil)
	}
	if strings.Contains(transcriptText(next.transcript), "Sending 1 newly captured screenshot") {
		t.Fatal("Always-send Off queued screenshot delivery")
	}
}

func TestScreenshotWatchStopsWhenCaptureDisabledOrAttemptChanges(t *testing.T) {
	for _, test := range []struct {
		name     string
		enabled  bool
		attempt  int
		wantLive bool
	}{
		{name: "disabled config", enabled: false, attempt: 1},
		{name: "different attempt", enabled: true, attempt: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc, _ := newScreenshotWatchFixture(t, test.enabled)
			msg := screenshotWatchSnapshotCmd(svc.ProjectDir, svc.Store, stageBrowserUI, test.attempt, 3)().(screenshotWatchSnapshotMsg)
			if msg.err || msg.enabled != test.wantLive || msg.active != (test.attempt == 1) {
				t.Fatalf("snapshot=%+v", msg)
			}
		})
	}
}

func newScreenshotWatchFixture(t *testing.T, enabled bool) (*cycle.Service, int64) {
	t.Helper()
	projectDir := t.TempDir()
	current := filepath.Join(projectDir, screenshotCurrentDir)
	if err := os.MkdirAll(current, 0o700); err != nil {
		t.Fatal(err)
	}
	flag := "false"
	if enabled {
		flag = "true"
	}
	config := "title: screenshot watch\nobjective: synthetic\nscope:\n  frontend: true\nstages:\n  browser_ui_validation:\n    enabled: true\n    timeout_minutes: 10\n    screenshots:\n      enabled: " + flag + "\n"
	if err := os.WriteFile(filepath.Join(current, "workflow-config.yml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ".workflow-hero"), 0o700); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(projectDir, ".workflow-hero", "hero.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = st.Close()
		for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
			_ = os.Remove(filepath.Join(projectDir, ".workflow-hero", "hero.db") + suffix)
		}
	})
	cycleID, err := st.CreateCycle(store.Cycle{Number: 1, Title: "screenshot watch", Status: store.CycleStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateStages([]store.Stage{{
		CycleID: cycleID, Name: stageBrowserUI, Status: store.StageRunning,
		Iteration: 1, MaxIterations: 2, TimeoutMinutes: 10,
	}}); err != nil {
		t.Fatal(err)
	}
	return &cycle.Service{ProjectDir: projectDir, Store: st}, cycleID
}

func transcriptText(messages []convMessage) string {
	var out strings.Builder
	for _, message := range messages {
		out.WriteString(message.content)
		out.WriteByte('\n')
	}
	return out.String()
}
