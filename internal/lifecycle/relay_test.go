package lifecycle

import (
	"os"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
)

func TestRelayReceivesEnvironmentNotifierEvent(t *testing.T) {
	relay, err := NewRelay(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()

	if mode, err := os.Stat(relay.Path()); err != nil {
		t.Fatal(err)
	} else if mode.Mode().Perm() != 0o600 {
		t.Fatalf("socket mode=%o want 600", mode.Mode().Perm())
	}

	t.Setenv(EventSocketEnv, relay.Path())
	notifier := NewEnvNotifier()
	if notifier == nil {
		t.Fatal("expected environment notifier")
	}
	expected := conversation.Event{
		EventID:    42,
		Kind:       conversation.EventApprovalRequired,
		CycleID:    7,
		CycleTitle: "Relay test",
		StageName:  "qa",
		Message:    "waiting",
		Timestamp:  time.Now().UTC().Truncate(time.Microsecond),
	}
	notifier.Notify(expected)

	select {
	case got := <-relay.Events():
		if got.EventID != expected.EventID || got.Kind != expected.Kind || got.CycleID != expected.CycleID || got.StageName != expected.StageName {
			t.Fatalf("event=%+v want=%+v", got, expected)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for relayed event")
	}
}

func TestNewEnvNotifierWithoutEndpointIsNil(t *testing.T) {
	t.Setenv(EventSocketEnv, "")
	if notifier := NewEnvNotifier(); notifier != nil {
		t.Fatal("expected nil notifier without endpoint")
	}
}
