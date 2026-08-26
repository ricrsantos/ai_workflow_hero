package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestConversationStreamRelayPreservesTextUnderBackpressure(t *testing.T) {
	out := make(chan tea.Msg)
	relay := newConversationStreamRelay("ex-1", out)
	const deltas = 600
	for i := 0; i < deltas; i++ {
		relay.Enqueue(harness.StreamDelta{
			Kind:        harness.StreamKindText,
			Text:        "x",
			HarnessType: "item/agentMessage/delta",
			SessionID:   "thread-1",
		})
	}
	closed := make(chan struct{})
	go func() {
		relay.CloseAndWait()
		close(closed)
	}()

	var got strings.Builder
	for {
		select {
		case msg := <-out:
			delta, ok := msg.(streamDeltaMsg)
			if !ok {
				t.Fatalf("message type=%T want streamDeltaMsg", msg)
			}
			got.WriteString(delta.delta.Text)
		case <-closed:
			if want := strings.Repeat("x", deltas); got.String() != want {
				t.Fatalf("text length=%d want %d", got.Len(), len(want))
			}
			return
		case <-time.After(time.Second):
			t.Fatal("relay did not drain")
		}
	}
}

func TestConversationStreamRelayStopReleasesBlockedDelivery(t *testing.T) {
	out := make(chan tea.Msg)
	relay := newConversationStreamRelay("ex-1", out)
	relay.Enqueue(harness.StreamDelta{Kind: harness.StreamKindText, Text: "blocked"})
	relay.Stop()

	select {
	case <-relay.done:
	case <-time.After(time.Second):
		t.Fatal("stopped relay remained blocked")
	}
}
