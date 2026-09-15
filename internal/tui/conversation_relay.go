package tui

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// msgSink receives messages destined for the Bubble Tea event loop.
// *tea.Program satisfies it directly; tests substitute a recorder.
//
// This is the fan-out seam for the live stream. A future renderer that wants
// live deltas attaches here, as an extra sink on the relay, rather than by
// reading a shared channel: sinks are typed and scoped to a single execute.
// Transports that only need lifecycle events or the final turn output keep
// using conversation.Notifier and executeDoneMsg instead.
type msgSink interface {
	Send(tea.Msg)
}

// programSink defers binding to the Bubble Tea program, which is created after
// the model it renders. launch.go attaches the program once per run, so the
// restart watchdog can rebind a fresh program without rebuilding the model.
type programSink struct {
	mu      sync.RWMutex
	program *tea.Program
}

func newProgramSink() *programSink { return &programSink{} }

func (s *programSink) attach(p *tea.Program) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.program = p
	s.mu.Unlock()
}

// Send drops the message when no program is attached, which is the case while
// the TUI is still booting and after it has exited.
func (s *programSink) Send(msg tea.Msg) {
	if s == nil {
		return
	}
	s.mu.RLock()
	p := s.program
	s.mu.RUnlock()
	if p == nil {
		return
	}
	p.Send(msg)
}

// conversationStreamRelay lets a harness keep producing while Bubble Tea is
// busy rendering a long transcript. It owns a single ordered queue per execute
// and is the only writer into the event loop for that execute, so transcript
// order is a property of the structure rather than of caller discipline.
//
// Producers must never block on the TUI: a harness callback that stalls also
// stalls the adapter's stdout read loop, which deadlocks the subprocess pipe.
// Enqueue therefore returns immediately and this relay absorbs the backpressure.
type conversationStreamRelay struct {
	ctx       context.Context
	cancel    context.CancelFunc
	executeID string
	sink      msgSink

	mu      sync.Mutex
	pending []tea.Msg
	closed  bool
	wake    chan struct{}
	done    chan struct{}
}

const conversationRelayBestEffortLimit = 512

func newConversationStreamRelay(executeID string, sink msgSink) *conversationStreamRelay {
	ctx, cancel := context.WithCancel(context.Background())
	r := &conversationStreamRelay{
		ctx:       ctx,
		cancel:    cancel,
		executeID: executeID,
		sink:      sink,
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
	r.pending = append(r.pending, streamDeltaMsg{executeID: r.executeID, delta: delta})
	r.mu.Unlock()
	r.signal()
}

// SendControl delivers a non-delta message in stream order. While the queue is
// open the message joins it, so the TUI never sees a permission prompt or a
// session bind jump ahead of the text that preceded it. After CloseAndWait has
// drained the queue it sends directly, which keeps executeDone strictly after
// every delta of the turn (find-qa-28).
func (r *conversationStreamRelay) SendControl(msg tea.Msg) bool {
	if r == nil {
		return false
	}
	select {
	case <-r.ctx.Done():
		return false
	default:
	}
	r.mu.Lock()
	if !r.closed {
		r.pending = append(r.pending, msg)
		r.mu.Unlock()
		r.signal()
		return true
	}
	r.mu.Unlock()
	r.sink.Send(msg)
	return true
}

func (r *conversationStreamRelay) coalesceTailLocked(delta harness.StreamDelta) bool {
	if len(r.pending) == 0 || (delta.Kind != harness.StreamKindText && delta.Kind != harness.StreamKindThinking) {
		return false
	}
	tail, ok := r.pending[len(r.pending)-1].(streamDeltaMsg)
	if !ok {
		return false
	}
	last := tail.delta
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
	tail.delta = last
	r.pending[len(r.pending)-1] = tail
	return true
}

func (r *conversationStreamRelay) run() {
	defer close(r.done)
	for {
		batch, ok := r.nextBatch()
		if !ok {
			return
		}
		if len(batch) == 0 {
			continue
		}
		if !r.send(conversationBatchMsg{messages: batch}) {
			return
		}
	}
}

// send delivers one batch and abandons it if the user cancels the turn first.
// msgSink.Send is opaque and blocking — *tea.Program cannot be interrupted
// mid-send — so the delivery races the relay context on its own goroutine.
// Only the final send of a cancelled relay can ever be abandoned, because run
// is sequential and Stop clears the queue, so this cannot reorder anything.
func (r *conversationStreamRelay) send(msg tea.Msg) bool {
	sent := make(chan struct{})
	go func() {
		r.sink.Send(msg)
		close(sent)
	}()
	select {
	case <-sent:
		return true
	case <-r.ctx.Done():
		return false
	}
}

// nextBatch blocks for one message and then coalesces a short burst behind it.
// Harnesses commonly emit many small deltas while the agent is thinking; the
// batch window keeps the terminal responsive without making output feel delayed.
func (r *conversationStreamRelay) nextBatch() ([]tea.Msg, bool) {
	first, ok := r.next()
	if !ok {
		return nil, false
	}
	batch := []tea.Msg{first}
	if isImmediateConversationMessage(first) {
		return batch, true
	}
	timer := time.NewTimer(conversationBatchWindow)
	defer timer.Stop()
	for len(batch) < conversationBatchMax {
		msg, ok, closed := r.tryNext()
		if ok {
			batch = append(batch, msg)
			if isImmediateConversationMessage(msg) {
				return batch, true
			}
			continue
		}
		if closed {
			return batch, true
		}
		select {
		case <-timer.C:
			return batch, true
		case <-r.ctx.Done():
			return batch, true
		case <-r.wake:
		}
	}
	return batch, true
}

// next blocks until a message is available, the relay is closed and drained, or
// the relay is cancelled.
func (r *conversationStreamRelay) next() (tea.Msg, bool) {
	for {
		msg, ok, closed := r.tryNext()
		if ok {
			return msg, true
		}
		if closed {
			return nil, false
		}
		select {
		case <-r.ctx.Done():
			return nil, false
		case <-r.wake:
		}
	}
}

// tryNext pops without blocking. The closed return reports that no further
// message can ever arrive, so callers can finish a batch without waiting out
// the batch window.
func (r *conversationStreamRelay) tryNext() (msg tea.Msg, ok bool, closed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.pending) > 0 {
		msg = r.pending[0]
		r.pending = r.pending[1:]
		return msg, true, false
	}
	return nil, false, r.closed
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
