package opencode

import (
	"context"
	"testing"
	"time"
)

func TestStopProcessHandleWaitsForChild(t *testing.T) {
	runner := ExecRunner{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := runner.Start(ctx, t.TempDir(), "sleep", "30")
	if err != nil {
		t.Skip("sleep not available")
	}
	pid := handle.PID()
	if err := stopProcessHandle(ctx, handle); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if processAlive(pid) {
		t.Fatalf("expected pid %d to be reaped", pid)
	}
}
