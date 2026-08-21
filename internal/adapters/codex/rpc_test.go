package codex

import (
	"bufio"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestNotifyHandlerDoesNotBlockStdoutDrain ensures a slow onNotify cannot stall
// the stdout reader (the pipe-deadlock class seen with Codex app-server).
func TestNotifyHandlerDoesNotBlockStdoutDrain(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	defer func() { _ = stdinR.Close() }()
	defer func() { _ = stdinW.Close() }()
	defer func() { _ = stdoutR.Close() }()
	defer func() { _ = stdoutW.Close() }()

	rpc := newRPCConn(stdinW, stdoutR)
	defer func() { _ = rpc.Close() }()

	block := make(chan struct{})
	entered := make(chan struct{})
	rpc.SetHandlers(func(method string, _ json.RawMessage) {
		if method == "slow" {
			close(entered)
			<-block
		}
	}, nil)

	// Answer the client Call by reading the JSON-RPC request and writing result.
	go func() {
		sc := bufio.NewScanner(stdinR)
		if !sc.Scan() {
			return
		}
		var req struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			return
		}
		payload, _ := json.Marshal(map[string]any{"id": req.ID, "result": map[string]any{"ok": true}})
		_, _ = stdoutW.Write(append(payload, '\n'))
	}()

	if _, err := stdoutW.Write([]byte(`{"method":"slow","params":{}}` + "\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("notify handler did not start")
	}

	done := make(chan error, 1)
	go func() {
		var result map[string]any
		done <- rpc.Call(t.Context(), "ping", map[string]any{}, &result)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Call failed while notify blocked: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Call timed out — readLoop blocked inside onNotify")
	}

	close(block)
}

func TestNotifyOrderPreserved(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	defer func() { _ = stdinR.Close() }()
	defer func() { _ = stdinW.Close() }()
	defer func() { _ = stdoutR.Close() }()
	defer func() { _ = stdoutW.Close() }()
	go func() {
		_, _ = io.Copy(io.Discard, stdinR)
	}()

	rpc := newRPCConn(stdinW, stdoutR)
	defer func() { _ = rpc.Close() }()

	var mu sync.Mutex
	var methods []string
	var n atomic.Int32
	done := make(chan struct{})
	rpc.SetHandlers(func(method string, _ json.RawMessage) {
		mu.Lock()
		methods = append(methods, method)
		mu.Unlock()
		if n.Add(1) == 3 {
			close(done)
		}
	}, nil)

	for _, line := range []string{
		`{"method":"a","params":{}}`,
		`{"method":"b","params":{}}`,
		`{"method":"c","params":{}}`,
	} {
		if _, err := stdoutW.Write([]byte(line + "\n")); err != nil {
			t.Fatal(err)
		}
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for notifies")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(methods) != 3 || methods[0] != "a" || methods[1] != "b" || methods[2] != "c" {
		t.Fatalf("order=%v want [a b c]", methods)
	}
}
