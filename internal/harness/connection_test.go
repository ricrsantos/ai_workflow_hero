package harness_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestIsConnectionClosed(t *testing.T) {
	t.Parallel()
	if !harness.IsConnectionClosed(harness.ErrConnectionClosed) {
		t.Fatal("ErrConnectionClosed")
	}
	if !harness.IsConnectionClosed(fmt.Errorf("codex app-server: %w", harness.ErrConnectionClosed)) {
		t.Fatal("wrapped ErrConnectionClosed")
	}
	if !harness.IsConnectionClosed(errors.New("codex app-server connection closed")) {
		t.Fatal("legacy connection closed text")
	}
	if harness.IsConnectionClosed(errors.New("model not found")) {
		t.Fatal("non-transport error")
	}
	if harness.IsConnectionClosed(nil) {
		t.Fatal("nil")
	}
}

func TestConnectionLifecycleDeltas(t *testing.T) {
	t.Parallel()
	closed := harness.ConnectionClosedDelta("sess-1")
	if closed.Kind != harness.StreamKindWarning || closed.HarnessType != harness.ConnectionClosedHarnessType {
		t.Fatalf("closed=%+v", closed)
	}
	if !harness.IsConnectionLifecycleDelta(closed) {
		t.Fatal("closed not lifecycle")
	}
	re := harness.ConnectionReconnectedDelta("sess-1")
	if re.Kind != harness.StreamKindWarning || re.HarnessType != harness.ConnectionReconnectedHarnessType {
		t.Fatalf("reconnected=%+v", re)
	}
	if !harness.IsConnectionLifecycleDelta(re) {
		t.Fatal("reconnected not lifecycle")
	}
}
