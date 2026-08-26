package opencode

import (
	"context"
	"syscall"
	"testing"
	"time"
)

func TestExecRunnerStartIgnoresCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	h, err := ExecRunner{}.Start(ctx, t.TempDir(), "sleep", "10")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = h.Kill() })

	deadline := time.Now().Add(time.Second)
	for {
		err := syscall.Kill(h.PID(), 0)
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("serve child should outlive a canceled Execute context; pid=%d err=%v", h.PID(), err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
