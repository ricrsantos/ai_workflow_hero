package opencode

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

func TestTerminateManagedProcessLiveOpencode(t *testing.T) {
	if os.Getenv("HERO_LIVE_OPENCODE_TEST") == "" {
		t.Skip("set HERO_LIVE_OPENCODE_TEST=1 to run")
	}
	pidStr := os.Getenv("HERO_LIVE_OPENCODE_PID")
	if pidStr == "" {
		t.Skip("set HERO_LIVE_OPENCODE_PID")
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		t.Fatal(err)
	}
	if !IsManagedOpenCodeServe(pid) {
		cmdline, _ := processCommandLine(pid)
		t.Fatalf("not managed: cmdline=%q", cmdline)
	}
	if err := TerminateManagedProcess(context.Background(), pid); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if processAlive(pid) {
		t.Fatal("process still alive after terminate")
	}
}

func TestIsManagedOpenCodeServeDetectsSpawnedServe(t *testing.T) {
	if _, err := exec.LookPath("opencode"); err != nil {
		t.Skip("opencode not on PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	dir := t.TempDir()
	a := NewAdapter(dir, nil)
	a.LookPath = exec.LookPath
	handle, err := a.Runner.Start(ctx, dir, "opencode", "serve", "--port", "0", "--hostname", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Kill() })

	url, _, err := defaultServeURLResolver(handle)
	if err != nil {
		t.Fatal(err)
	}
	pid := handle.PID()
	cmdline, err := processCommandLine(pid)
	if err != nil {
		t.Fatalf("processCommandLine: %v", err)
	}
	if !IsManagedOpenCodeServe(pid) {
		t.Fatalf("expected managed serve pid=%d cmdline=%q url=%s", pid, cmdline, url)
	}
}
