package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/telegram"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

func TestRequestAutoUpdateRestartTimesOutWhenDaemonDoesNotReply(t *testing.T) {
	home, err := os.MkdirTemp("/tmp", "h-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("HOME", home)
	socketPath, err := telegram.SocketPath(telegram.PluginName)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := ipc.Listen(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	release := make(chan struct{})
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		raw, err := listener.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		server := ipc.NewConn(raw)
		if _, err := server.Recv(); err != nil {
			return
		}
		<-release
	}()

	started := time.Now()
	err = requestAutoUpdateRestartWithTimeout(context.Background(), 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout when daemon never sends an acknowledgement")
	}
	if !strings.Contains(err.Error(), "read TUI restart response") {
		t.Fatalf("error=%v want bounded response-read error", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("restart request took %s; stale daemon must not block the updater", elapsed)
	}

	close(release)
	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("test daemon did not stop")
	}
}
