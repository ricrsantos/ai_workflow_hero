package tui

import (
	"context"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// conversationStreamRelay lets a harness keep producing while Bubble Tea is
// busy rendering a long transcript. It preserves delivery order and coalesces
// adjacent text deltas instead of making the harness callback wait on the TUI.
type conversationStreamRelay struct {
	ctx       context.Context
	cancel    context.CancelFunc
	executeID string
	out       chan<- tea.Msg

	mu      sync.Mutex
	pending []harness.StreamDelta
	closed  bool
	wake    chan struct{}
	done    chan struct{}
}

const conversationRelayBestEffortLimit = 512

func newConversationStreamRelay(executeID string, out chan<- tea.Msg) *conversationStreamRelay {
	ctx, cancel := context.WithCancel(context.Background())
	r := &conversationStreamRelay{
		ctx:       ctx,
		cancel:    cancel,
		executeID: executeID,
		out:       out,
		wake:      make(chan struct{}, 1),
		done:      make(chan struct{}),
	}
	go r.run()
	return r
}

// Enqueue returns immediately for text deltas even when the TUI is behind.
// Best-effort activity/tool events may be discarded once the relay backlog is
// large; transcript-critical events are always retained.
func (r *conversationStreamRelay) Enqueue(delta harness.StreamDelta) {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	if r.coalesceTailLocked(delta) {
		r.mu.Unlock()
		return
	}
	if len(r.pending) >= conversationRelayBestEffortLimit && !streamDeltaMustDeliver(delta.Kind) {
		r.mu.Unlock()
		return
	}
	r.pending = append(r.pending, delta)
	r.mu.Unlock()
	r.signal()
}

func (r *conversationStreamRelay) coalesceTailLocked(delta harness.StreamDelta) bool {
	if len(r.pending) == 0 || (delta.Kind != harness.StreamKindText && delta.Kind != harness.StreamKindThinking) {
		return false
	}
	last := &r.pending[len(r.pending)-1]
	if last.Kind != delta.Kind ||
		last.Phase != delta.Phase ||
		last.AgentName != delta.AgentName ||
		last.Model != delta.Model ||
		last.CallID != delta.CallID ||
		last.HarnessType != delta.HarnessType ||
		last.SessionID != delta.SessionID {
		return false
	}
	last.Text += delta.Text
	return true
}

func (r *conversationStreamRelay) run() {
	defer close(r.done)
	for {
		delta, ok := r.next()
		if !ok {
			return
		}
		if !deliverStreamDeltaContext(r.ctx, r.out, r.executeID, delta) {
			return
		}
	}
}

func (r *conversationStreamRelay) next() (harness.StreamDelta, bool) {
	for {
		r.mu.Lock()
		if len(r.pending) > 0 {
			delta := r.pending[0]
			r.pending = r.pending[1:]
			r.mu.Unlock()
			return delta, true
		}
		closed := r.closed
		r.mu.Unlock()
		if closed {
			return harness.StreamDelta{}, false
		}
		select {
		case <-r.ctx.Done():
			return harness.StreamDelta{}, false
		case <-r.wake:
		}
	}
}

func (r *conversationStreamRelay) signal() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// CloseAndWait stops accepting new deltas and drains all accepted deltas in
// order before the Execute completion message is delivered.
func (r *conversationStreamRelay) CloseAndWait() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	r.signal()
	<-r.done
}

// Stop abandons pending UI output after the user cancels the harness and
// releases a relay that is blocked waiting for Bubble Tea to receive a delta.
func (r *conversationStreamRelay) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.closed = true
	r.pending = nil
	r.mu.Unlock()
	r.cancel()
	r.signal()
}

func (r *conversationStreamRelay) SendControl(msg tea.Msg) bool {
	if r == nil {
		return false
	}
	select {
	case <-r.ctx.Done():
		return false
	case r.out <- msg:
		return true
	}
}
