package tui

import (
	"net"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

func TestAlwaysSendDisabledExplainsLocalScreenshotAvailability(t *testing.T) {
	var replies []string
	m := NewTestModel(nil)
	m.telegram = &telegramState{
		connected: true, paired: true, address: "proj", alwaysSend: false,
		recordOutbound: func(text string) { replies = append(replies, text) },
	}
	next, cmd := m.handleTelegramInbound(telegramInboundMsg{text: "/hero-screenshot todos", isCommand: true, address: "proj"})
	if cmd != nil || len(next.executes) != 0 || next.screenshots.open {
		t.Fatalf("disabled screenshot request dispatched or opened a capture path: cmd=%t executes=%d state=%+v", cmd != nil, len(next.executes), next.screenshots)
	}
	if len(replies) != 1 || replies[0] != "Screenshots are available in the TUI. Enable Always send reply in Telegram settings to receive images here." {
		t.Fatalf("disabled reply=%q", replies)
	}
}

func TestAlwaysSendRequiresDaemonImageCapability(t *testing.T) {
	var replies []string
	m := NewTestModel(nil)
	m.telegram = &telegramState{
		connected: true, paired: true, address: "proj", alwaysSend: true,
		recordOutbound: func(text string) { replies = append(replies, text) },
	}
	next, cmd := m.handleTelegramInbound(telegramInboundMsg{text: "/hero-screenshot latest", isCommand: true, address: "proj"})
	if cmd != nil || next.screenshots.open || len(next.executes) != 0 {
		t.Fatalf("missing capability dispatched a harness or loaded images: cmd=%t state=%+v", cmd != nil, next.screenshots)
	}
	var transcript strings.Builder
	for _, entry := range next.transcript {
		transcript.WriteString(entry.content)
		transcript.WriteByte('\n')
	}
	if len(replies) == 0 || !strings.Contains(replies[0], "local TUI screenshot collection remains available") || !strings.Contains(transcript.String(), "update the Telegram daemon") {
		t.Fatalf("capability warning missing: replies=%q transcript=%q", replies, transcript.String())
	}
}

func TestImageDeliveryVersionMismatchWarnsAndKeepsCards(t *testing.T) {
	m := NewTestModel(nil)
	m.telegram = &telegramState{pluginVersion: "3.2.0", connected: true, paired: true, address: "proj"}
	m.screenshots = screenshotCollectionState{open: true, assets: []store.ScreenshotManifest{{ID: "shot-local"}}}
	updated, _ := m.handleTelegramMsg(telegramRegisteredMsg{address: "proj", paired: true, daemonVersion: "3.1.9"})
	next := updated.(model)
	if len(next.screenshots.assets) != 1 || next.screenshots.assets[0].ID != "shot-local" {
		t.Fatal("daemon version warning removed local screenshot cards")
	}
	var transcript strings.Builder
	for _, entry := range next.transcript {
		transcript.WriteString(entry.content)
		transcript.WriteByte('\n')
	}
	if !strings.Contains(transcript.String(), "versions differ") || !strings.Contains(transcript.String(), "update or restart the daemon") {
		t.Fatalf("version mismatch warning=%q", transcript.String())
	}
}

func TestImageDeliveryBatchesAreBoundedAndOrdered(t *testing.T) {
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
	client := &telegramClient{conn: ipc.NewConn(local)}
	manifests := make([]store.ScreenshotManifest, 9)
	for i := range manifests {
		manifests[i] = store.ScreenshotManifest{ID: screenshotTestID(i), CycleID: 7, StageName: "qa_end_to_end", Attempt: 1, Path: "screenshots/" + screenshotTestID(i) + ".png", CapturedAt: "2026-10-01T12:00:00Z", Result: store.ScreenshotResultPassed}
	}
	cmd := telegramScreenshotDeliveryCmd(client, manifests, 0)
	if cmd == nil {
		t.Fatal("delivery command is required")
	}
	finished := make(chan tea.Msg, 1)
	go func() { finished <- cmd() }()
	receiver := ipc.NewConn(peer)
	wantSizes := []int{4, 4, 1}
	index := 0
	for batchIndex, size := range wantSizes {
		message, err := receiver.Recv()
		if err != nil {
			t.Fatalf("receive batch %d: %v", batchIndex, err)
		}
		if message.Type != ipc.TypeOutboundImageBatch || len(message.Images) != size || message.ImageRetryAttempt != 0 || message.ImageBatchID == "" {
			t.Fatalf("batch %d=%+v, want %d ordered refs", batchIndex, message, size)
		}
		for _, ref := range message.Images {
			if ref.ScreenshotID != manifests[index].ID {
				t.Fatalf("image order[%d]=%q, want %q", index, ref.ScreenshotID, manifests[index].ID)
			}
			index++
		}
	}
	if got := <-finished; got != nil {
		t.Fatalf("delivery command returned %T %v", got, got)
	}
}

func TestPartialBatchRetriesOnlyFailedAndRetainsLocalCards(t *testing.T) {
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
	client := &telegramClient{conn: ipc.NewConn(local)}
	assets := []store.ScreenshotManifest{
		{ID: "shot-a", CycleID: 2, StageName: "browser_ui_validation", Attempt: 1, Path: "screenshots/shot-a.png"},
		{ID: "shot-b", CycleID: 2, StageName: "browser_ui_validation", Attempt: 1, Path: "screenshots/shot-b.png"},
		{ID: "shot-c", CycleID: 2, StageName: "browser_ui_validation", Attempt: 1, Path: "screenshots/shot-c.png"},
	}
	m := NewTestModel(nil)
	m.screenshots = screenshotCollectionState{open: true, assets: append([]store.ScreenshotManifest(nil), assets...)}
	m.telegram = &telegramState{connected: true, paired: true, address: "proj", alwaysSend: true, client: client, daemonCaps: []string{ipc.CapabilityImageDelivery}}
	next, cmd := m.handleTelegramImageDeliveryResult(telegramImageDeliveryResultMsg{
		batchID: "original", failed: []string{"shot-b"}, delivered: []string{"shot-a", "shot-c"}, errorCode: "bot_api_delivery_failed",
	})
	if len(next.screenshots.assets) != len(assets) || next.screenshots.assets[1].ID != "shot-b" {
		t.Fatalf("local screenshot cards were lost after partial delivery: %+v", next.screenshots.assets)
	}
	if cmd == nil {
		t.Fatal("transient Bot API failure should schedule exactly one retry")
	}
	finished := make(chan tea.Msg, 1)
	go func() { finished <- cmd() }()
	message, err := ipc.NewConn(peer).Recv()
	if err != nil {
		t.Fatal(err)
	}
	if message.Type != ipc.TypeOutboundImageBatch || len(message.Images) != 1 || message.Images[0].ScreenshotID != "shot-b" || message.ImageRetryAttempt != 1 {
		t.Fatalf("retry frame=%+v, want only shot-b at retry 1", message)
	}
	if got := <-finished; got != nil {
		t.Fatalf("retry command returned %T %v", got, got)
	}
	_, noSecondRetry := next.handleTelegramImageDeliveryResult(telegramImageDeliveryResultMsg{
		batchID: message.ImageBatchID, retryAttempt: 1, failed: []string{"shot-b"}, errorCode: "bot_api_delivery_failed",
	})
	if noSecondRetry != nil {
		t.Fatal("delivery retries must be bounded to one automatic retry")
	}
}

func screenshotTestID(index int) string {
	return []string{"shot-0", "shot-1", "shot-2", "shot-3", "shot-4", "shot-5", "shot-6", "shot-7", "shot-8"}[index]
}
