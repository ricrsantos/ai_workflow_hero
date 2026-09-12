package cursor_test

import (
	"context"
	"testing"
	"time"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestCheckHealthCompletedSessionStaysAlive(t *testing.T) {
	dir := withCursorAssets(t)
	fixture := `{"type":"result","subtype":"success","is_error":false,"duration_ms":10,"result":"done","session_id":"sess-done"}`
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.Runner = &fakeRunner{t: t, handlers: []fakeCall{{
		matchArgs: func(args []string) bool { return containsArg(args, "--print") },
		result:    cursoradapter.RunResult{Stdout: []byte(fixture)},
	}}}

	if _, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
		ProjectDir: dir,
		Prompt:     "go",
		StageName:  "planning",
	}); err != nil {
		t.Fatal(err)
	}
	if adapter.HasInFlight() {
		t.Fatal("expected no in-flight execute after completion")
	}

	health, err := adapter.CheckHealth(context.Background(), "sess-done")
	if err != nil {
		t.Fatal(err)
	}
	if !health.ProcessAlive || !health.SessionAlive {
		t.Fatalf("completed session must stay alive for the TUI watchdog: %+v", health)
	}
	var watchdog harness.Watchdog
	watchdog.Reset(time.Now())
	if got := watchdog.Evaluate(time.Now(), health, harness.CursorStallTimeout); got != harness.HealthHealthy {
		t.Fatalf("Evaluate=%q want healthy for completed session %+v", got, health)
	}
}

func TestCheckHealthUnknownIdleWithoutProcessStaysAlive(t *testing.T) {
	adapter := cursoradapter.NewAdapter(t.TempDir())
	health, err := adapter.CheckHealth(context.Background(), "sess-unknown")
	if err != nil {
		t.Fatal(err)
	}
	if !health.ProcessAlive || !health.SessionAlive {
		t.Fatalf("idle known-id session must not HealthFailed: %+v", health)
	}
}

func TestCheckHealthEmptySessionWithoutProcessIsDead(t *testing.T) {
	adapter := cursoradapter.NewAdapter(t.TempDir())
	health, err := adapter.CheckHealth(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if health.ProcessAlive {
		t.Fatalf("empty session with no process should be dead: %+v", health)
	}
}
