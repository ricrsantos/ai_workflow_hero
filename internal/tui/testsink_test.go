package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// conversationSinkChan returns the recorded stream of a model wired by
// newConversationTestModel, or nil when the model uses the production sink.
func conversationSinkChan(m model) chan tea.Msg {
	if s, ok := m.convSink.(*recordingSink); ok {
		return s.ch
	}
	return nil
}

// awaitSinkMsg pulls the next message the relay delivered, failing the test
// rather than hanging when the stream stalls.
func awaitSinkMsg(t *testing.T, sink chan tea.Msg, timeout time.Duration) tea.Msg {
	t.Helper()
	if sink == nil {
		t.Fatal("model has no recording sink")
	}
	select {
	case msg := <-sink:
		return msg
	case <-time.After(timeout):
		t.Fatal("timeout waiting for relay message")
		return nil
	}
}
