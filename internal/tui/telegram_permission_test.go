package tui

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestTelegramHarnessPermissionRoundTripByID(t *testing.T) {
	m := NewTestModel(nil)
	var outbound []string
	m.telegram = &telegramState{
		connected: true,
		address:   "hero_1",
		paired:    true,
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}
	respCh := make(chan harness.PermissionResponse, 1)
	updated, _ := m.handleConversationMsg(harnessPermissionRequestMsg{
		executeID: "exec-1",
		req: harness.PermissionRequest{
			ID:          "perm-1",
			Title:       "Run command",
			Description: "The agent wants to run a project command.",
		},
		respCh: respCh,
	})
	m = updated.(model)
	if !m.harnessPermissionPending || !m.hasHarnessPermission("perm-1") {
		t.Fatalf("permission not pending: %+v", m)
	}
	if len(outbound) != 1 || !strings.Contains(outbound[0], "/hero-permission perm-1 allow") {
		t.Fatalf("outbound permission prompt=%q", outbound)
	}

	m, _ = m.handleTelegramInbound(telegramInboundMsg{
		text:    "/hero-permission perm-1 allow",
		address: "hero_1",
	})
	select {
	case response := <-respCh:
		if !response.Approved || response.Reason != "" {
			t.Fatalf("response=%+v", response)
		}
	default:
		t.Fatal("Telegram permission response was not delivered")
	}
	if m.harnessPermissionPending || m.hasHarnessPermission("perm-1") {
		t.Fatalf("permission remained pending: %+v", m)
	}
	if !strings.Contains(strings.Join(outbound, "\n"), "Harness permission perm-1 allowed.") {
		t.Fatalf("missing confirmation: %q", outbound)
	}
}

func TestTelegramHarnessPermissionRejectsUnknownID(t *testing.T) {
	m := NewTestModel(nil)
	var outbound []string
	m.telegram = &telegramState{
		connected: true,
		address:   "hero_1",
		paired:    true,
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}
	respCh := make(chan harness.PermissionResponse, 1)
	updated, _ := m.handleConversationMsg(harnessPermissionRequestMsg{
		req:    harness.PermissionRequest{ID: "perm-current", Title: "Read file"},
		respCh: respCh,
	})
	m = updated.(model)
	m, _ = m.handleTelegramInbound(telegramInboundMsg{
		text:    "/hero-permission perm-other deny",
		address: "hero_1",
	})
	select {
	case response := <-respCh:
		t.Fatalf("unknown id answered current permission: %+v", response)
	default:
	}
	if !m.hasHarnessPermission("perm-current") {
		t.Fatal("current permission was cleared by unknown response")
	}
	if !strings.Contains(strings.Join(outbound, "\n"), "No pending harness permission") {
		t.Fatalf("missing unknown-id error: %q", outbound)
	}
}

func TestTelegramInterruptCancelsPendingHarnessPermission(t *testing.T) {
	m := NewTestModel(nil)
	m.streaming = true
	respCh := make(chan harness.PermissionResponse, 1)
	updated, _ := m.handleConversationMsg(harnessPermissionRequestMsg{
		req:    harness.PermissionRequest{ID: "perm-interrupt", Title: "Write file"},
		respCh: respCh,
	})
	m = updated.(model)
	if _, cmd := m.handleTelegramInterrupt(); cmd == nil {
		t.Fatal("interrupt must schedule cancellation")
	}
	updatedModel, _ := m.handleConversationMsg(streamCancelDoneMsg{})
	m = updatedModel.(model)
	select {
	case response := <-respCh:
		if response.Approved || response.Reason != "cancelled" {
			t.Fatalf("interrupt response=%+v", response)
		}
	default:
		t.Fatal("interrupt did not reject pending permission")
	}
	if m.harnessPermissionPending {
		t.Fatal("permission prompt remained visible after interrupt")
	}
}

func TestLifecycleEventQueuesUntilTelegramPairing(t *testing.T) {
	m := NewTestModel(nil)
	var outbound []string
	m.telegram = &telegramState{
		connected: false,
		paired:    false,
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}
	event := conversation.Event{EventID: 101, Kind: conversation.EventApprovalRequired, CycleID: 4, StageName: "qa"}
	m, cmd := m.handleLifecycleEvent(event)
	if cmd != nil || len(m.pendingLifecycleEvents) != 1 {
		t.Fatalf("event queue state pending=%d cmd=%v", len(m.pendingLifecycleEvents), cmd)
	}
	m.telegram.connected = true
	m.telegram.paired = true
	m, cmd = m.flushPendingLifecycleEvents()
	if len(m.pendingLifecycleEvents) != 0 || len(outbound) != 1 {
		t.Fatalf("flush state pending=%d outbound=%d cmd=%v", len(m.pendingLifecycleEvents), len(outbound), cmd)
	}
	if outbound[0] != "Approval required: qa" {
		t.Fatalf("outbound=%q", outbound[0])
	}
	if _, cmd = m.handleLifecycleEvent(event); cmd != nil || len(outbound) != 1 {
		t.Fatal("duplicate lifecycle event was forwarded")
	}
}
