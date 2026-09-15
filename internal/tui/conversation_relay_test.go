package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestConversationStreamRelayPreservesTextUnderBackpressure(t *testing.T) {
	sink := newRecordingSink()
	relay := newConversationStreamRelay("ex-1", sink)
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
		case msg := <-sink.ch:
			batch, ok := msg.(conversationBatchMsg)
			if !ok {
				t.Fatalf("message type=%T want conversationBatchMsg", msg)
			}
			for _, m := range batch.messages {
				delta, ok := m.(streamDeltaMsg)
				if !ok {
					t.Fatalf("message type=%T want streamDeltaMsg", m)
				}
				got.WriteString(delta.delta.Text)
			}
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
	blocked := &recordingSink{ch: make(chan tea.Msg)}
	relay := newConversationStreamRelay("ex-1", blocked)
	relay.Enqueue(harness.StreamDelta{Kind: harness.StreamKindText, Text: "blocked"})
	relay.Stop()

	select {
	case <-relay.done:
	case <-time.After(time.Second):
		t.Fatal("stopped relay remained blocked")
	}
}

// The relay is the only writer into the event loop for an execute, so callers
// cannot reorder the transcript by racing it. Before this, every handler that
// re-armed a stream reader cmd added another concurrent reader of the shared
// channel, and two readers delivered adjacent deltas to Update in whichever
// order the scheduler picked — words swapped places mid-sentence.
func TestConversationStreamRelayPreservesOrderUnderConcurrentProducers(t *testing.T) {
	sink := newRecordingSink()
	relay := newConversationStreamRelay("ex-1", sink)

	const words = 500
	var want strings.Builder
	for i := 0; i < words; i++ {
		text := fmt.Sprintf("w%03d ", i)
		want.WriteString(text)
		// Alternate HarnessType so coalescing cannot hide a reordering by
		// merging neighbours into one delta.
		relay.Enqueue(harness.StreamDelta{
			Kind:        harness.StreamKindText,
			Text:        text,
			HarnessType: fmt.Sprintf("chunk-%d", i%2),
		})
	}
	relay.CloseAndWait()

	var got strings.Builder
	for {
		select {
		case msg := <-sink.ch:
			batch, ok := msg.(conversationBatchMsg)
			if !ok {
				t.Fatalf("message type=%T want conversationBatchMsg", msg)
			}
			for _, item := range batch.messages {
				delta, ok := item.(streamDeltaMsg)
				if !ok {
					t.Fatalf("message type=%T want streamDeltaMsg", item)
				}
				got.WriteString(delta.delta.Text)
			}
		default:
			if got.String() != want.String() {
				gw, ww := strings.Fields(got.String()), strings.Fields(want.String())
				for i := range ww {
					if i >= len(gw) || gw[i] != ww[i] {
						t.Fatalf("transcript diverges at word %d\n want: %v\n got : %v",
							i, ww[max(0, i-2):min(len(ww), i+4)], gw[max(0, i-2):min(len(gw), i+4)])
					}
				}
				t.Fatalf("transcript length got=%d want=%d", got.Len(), want.Len())
			}
			return
		}
	}
}

// Permission and question prompts used to be written straight to the shared
// channel, jumping ahead of deltas still queued in the relay: the prompt then
// rendered above the text that had actually preceded it.
func TestConversationStreamRelayControlStaysBehindQueuedText(t *testing.T) {
	sink := newRecordingSink()
	relay := newConversationStreamRelay("ex-1", sink)

	relay.Enqueue(harness.StreamDelta{Kind: harness.StreamKindText, Text: "before-prompt"})
	relay.SendControl(harnessPermissionRequestMsg{executeID: "ex-1"})
	relay.CloseAndWait()
	relay.SendControl(executeDoneMsg{executeID: "ex-1"})

	var order []string
	for len(order) < 3 {
		msg, ok := awaitConversationMsg(sink.ch, 2*time.Second)
		if !ok {
			t.Fatalf("relay stalled after %v", order)
		}
		items := []tea.Msg{msg}
		if batch, ok := msg.(conversationBatchMsg); ok {
			items = batch.messages
		}
		for _, item := range items {
			switch v := item.(type) {
			case streamDeltaMsg:
				order = append(order, "text:"+v.delta.Text)
			case harnessPermissionRequestMsg:
				order = append(order, "permission")
			case executeDoneMsg:
				order = append(order, "done")
			default:
				t.Fatalf("unexpected message %T", item)
			}
		}
	}
	want := []string{"text:before-prompt", "permission", "done"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("order=%v want %v", order, want)
	}
}

// Transcript exactness across a mid-stream session bind, which makes the batch
// handler drain a session persist. This pins coalescing and batching, not the
// reordering bug: that needed two concurrent readers of the shared channel, a
// condition the synchronous test driver cannot create and which the current
// design makes unrepresentable, since the relay is the only writer and there is
// no reader to duplicate.
func TestConversationTranscriptExactAcrossMidStreamPersist(t *testing.T) {
	m, h, _ := newConversationTestModel(t)

	const words = 300
	var want strings.Builder
	events := make([]harness.StreamDelta, 0, words+1)
	for i := 0; i < words; i++ {
		text := fmt.Sprintf("w%03d ", i)
		want.WriteString(text)
		events = append(events, harness.StreamDelta{
			Kind:        harness.StreamKindText,
			Text:        text,
			HarnessType: fmt.Sprintf("chunk-%d", i%2),
		})
		if i == words/2 {
			events = append(events, harness.StreamDelta{
				Kind:      harness.StreamKindSession,
				SessionID: "native-mid-stream",
			})
		}
	}
	h.events = events

	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "stream please")
	m, cmd := SubmitConversationForTest(m)
	if cmd == nil {
		t.Fatal("submit did not start execute")
	}
	m = drainConversationStream(t, m, cmd)

	transcript := ConversationTranscriptForTest(m)
	if !strings.Contains(transcript, want.String()) {
		gw := strings.Fields(transcript)
		ww := strings.Fields(want.String())
		for i := range ww {
			idx := -1
			for j, w := range gw {
				if w == ww[0] {
					idx = j
					break
				}
			}
			if idx < 0 {
				t.Fatalf("transcript does not contain the stream at all")
			}
			if idx+i >= len(gw) || gw[idx+i] != ww[i] {
				t.Fatalf("transcript diverges at word %d\n want: %v\n got : %v",
					i, ww[max(0, i-2):min(len(ww), i+4)],
					gw[max(0, idx+i-2):min(len(gw), idx+i+4)])
			}
		}
		t.Fatal("transcript does not contain the streamed text")
	}
}
