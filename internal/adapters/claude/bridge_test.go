package claude

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestPermissionBridgeForwardsOnlyOneValidatedRequest(t *testing.T) {
	request := `{"jsonrpc":"2.0","id":"permission-1","method":"tools/call","params":{"name":"approval_prompt","arguments":{"token":"bridge-token","tool_name":"Bash","input":{"command":"go test ./..."}}}}`
	var output bytes.Buffer
	var received harness.PermissionRequest
	bridge := PermissionBridge{
		Reader: strings.NewReader(request + "\n"),
		Writer: &output,
		Token:  NewOneTimeToken("bridge-token"),
		Request: func(_ context.Context, req harness.PermissionRequest) (harness.PermissionResponse, error) {
			received = req
			return harness.PermissionResponse{Approved: true}, nil
		},
		Logger: testLogger(),
	}
	if err := bridge.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if received.ID != "permission-1" || received.Title != "Claude permission: Bash" {
		t.Fatalf("received=%+v", received)
	}
	if err := ValidatePermissionToolDecision(output.Bytes()); err != nil {
		t.Fatalf("response=%s err=%v", output.String(), err)
	}
}

func TestPermissionBridgeForwardsMultipleRequestsOnAuthenticatedConnection(t *testing.T) {
	requests := strings.Join([]string{
		`{"jsonrpc":"2.0","id":"permission-1","method":"tools/call","params":{"name":"approval_prompt","arguments":{"token":"bridge-token","tool_name":"Bash","input":{"command":"go test ./..."}}}}`,
		`{"jsonrpc":"2.0","id":"permission-2","method":"tools/call","params":{"name":"approval_prompt","arguments":{"token":"bridge-token","tool_name":"Write","input":{"file_path":"result.txt"}}}}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	var received []string
	bridge := PermissionBridge{
		Reader: strings.NewReader(requests), Writer: &output, Token: NewOneTimeToken("bridge-token"),
		Request: func(_ context.Context, req harness.PermissionRequest) (harness.PermissionResponse, error) {
			received = append(received, req.ID)
			return harness.PermissionResponse{Approved: true}, nil
		},
		Logger: testLogger(),
	}

	if err := bridge.Run(context.Background()); err != nil {
		t.Fatalf("Run() error=%v", err)
	}
	if got := strings.Join(received, ","); got != "permission-1,permission-2" {
		t.Fatalf("received=%q", got)
	}
	if got := strings.Count(strings.TrimSpace(output.String()), "\n") + 1; got != 2 {
		t.Fatalf("decision count=%d, output=%q", got, output.String())
	}
}

func TestPermissionBridgeRejectsReplayAndMalformedRequest(t *testing.T) {
	token := NewOneTimeToken("bridge-token")
	valid := `{"jsonrpc":"2.0","id":"permission-1","method":"tools/call","params":{"name":"approval_prompt","arguments":{"token":"bridge-token","tool_name":"Bash","input":{}}}}`
	bridge := PermissionBridge{Reader: strings.NewReader(valid + "\n"), Writer: &bytes.Buffer{}, Token: token, Request: func(context.Context, harness.PermissionRequest) (harness.PermissionResponse, error) {
		return harness.PermissionResponse{}, nil
	}, Logger: testLogger()}
	if err := bridge.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	bridge.Reader = strings.NewReader(valid + "\n")
	if err := bridge.Run(context.Background()); !errors.Is(err, ErrPermissionTokenInvalid) {
		t.Fatalf("replay error=%v", err)
	}
	bridge.Reader = strings.NewReader(`{"jsonrpc":"2.0","method":"tools/call"}` + "\n")
	if err := bridge.Run(context.Background()); err == nil {
		t.Fatal("malformed request was accepted")
	}
}

func TestPermissionMCPHelperForwardsThroughLiveExecutionBridge(t *testing.T) {
	var received []string
	handle, err := (defaultPermissionBridgeStarter{}).StartPermissionBridge(context.Background(), PermissionBridgeConfig{
		Token: "bridge-token",
		Request: func(_ context.Context, request harness.PermissionRequest) (harness.PermissionResponse, error) {
			received = append(received, request.ID)
			return harness.PermissionResponse{Approved: true}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	live := handle.(permissionMCPConfigProvider).permissionMCPConfig()
	done := make(chan error, 1)
	go func() { done <- handle.Run(context.Background()) }()

	request := strings.Join([]string{
		`{"jsonrpc":"2.0","id":"permission-1","method":"tools/call","params":{"name":"approval_prompt","arguments":{"tool_name":"Bash","input":{"command":"go test ./..."}}}}`,
		`{"jsonrpc":"2.0","id":"permission-2","method":"tools/call","params":{"name":"approval_prompt","arguments":{"tool_name":"Write","input":{"file_path":"result.txt"}}}}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	if err := RunPermissionMCPHelper(context.Background(), strings.NewReader(request), &output, live.Address, live.Token); err != nil {
		t.Fatal(err)
	}
	for _, decision := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		if err := ValidatePermissionToolDecision([]byte(decision)); err != nil {
			t.Fatalf("response=%s err=%v", decision, err)
		}
	}
	if got := strings.Join(received, ","); got != "permission-1,permission-2" {
		t.Fatalf("received=%q", got)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("live bridge did not complete")
	}
}
