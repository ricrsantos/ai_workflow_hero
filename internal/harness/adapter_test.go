package harness_test

import (
	"context"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// stubAdapter satisfies HarnessAdapter for compile/contract tests.
type stubAdapter struct {
	name string
}

func (s stubAdapter) Name() string { return s.name }

func (s stubAdapter) IsAvailable(context.Context) error { return nil }

func (s stubAdapter) CreateSession(_ context.Context, req harness.SessionRequest) (*harness.Session, error) {
	return &harness.Session{
		ID:         "sess-1",
		ProjectDir: req.ProjectDir,
		StageName:  req.StageName,
		AgentName:  req.AgentName,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

func (s stubAdapter) ResumeSession(context.Context, string) error { return nil }

func (s stubAdapter) Execute(_ context.Context, req harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	return &harness.ExecutionResult{
		SessionID:  "sess-1",
		Output:     "ok: " + req.Prompt,
		Summary:    "ok",
		StreamDone: true,
		Duration:   time.Millisecond,
	}, nil
}

func (s stubAdapter) Cancel(context.Context, string) error { return nil }

func (s stubAdapter) Status(_ context.Context, sessionID string) (*harness.ExecutionStatus, error) {
	return &harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusIdle}, nil
}

func (s stubAdapter) Dispatch(_ context.Context, req harness.DispatchRequest) (harness.DispatchResult, error) {
	return harness.DispatchResult{Dispatched: true, Message: req.StageName}, nil
}

func TestHarnessAdapterContractCompile(t *testing.T) {
	var _ harness.HarnessAdapter = stubAdapter{name: "stub"}
}

func TestExecuteRequestFields(t *testing.T) {
	req := harness.ExecuteRequest{
		ProjectDir: "/tmp/proj",
		Prompt:     "hello",
		SessionID:  "abc",
		Stream:     true,
		StageName:  "research",
		AgentName:  "discover_agent",
	}
	if req.ProjectDir == "" || req.Prompt == "" || !req.Stream {
		t.Fatalf("unexpected request: %+v", req)
	}
}

func TestNormalizePermissionProfileDefaultsToAsk(t *testing.T) {
	if got := harness.NormalizePermissionProfile(""); got != harness.PermissionProfileAsk {
		t.Fatalf("blank profile = %q, want ask", got)
	}
	if got := harness.NormalizePermissionProfile("unknown"); got != harness.PermissionProfileAsk {
		t.Fatalf("unknown profile = %q, want ask", got)
	}
	if got := harness.NormalizePermissionProfile(harness.PermissionProfileAutoProject); got != harness.PermissionProfileAutoProject {
		t.Fatalf("auto profile = %q", got)
	}
	if got := harness.NormalizePermissionProfile(harness.PermissionProfileAutoAll); got != harness.PermissionProfileAutoAll {
		t.Fatalf("yolo profile = %q", got)
	}
}

func TestExecutionResultUsage(t *testing.T) {
	res := harness.ExecutionResult{
		SessionID: "s1",
		Output:    "done",
		Usage:     harness.Usage{InputTokens: 10, OutputTokens: 5},
		Duration:  2 * time.Second,
	}
	if res.Usage.InputTokens != 10 || res.Usage.OutputTokens != 5 {
		t.Fatalf("usage = %+v", res.Usage)
	}
}

func TestStubDispatchWrapper(t *testing.T) {
	a := stubAdapter{name: "stub"}
	res, err := a.Dispatch(context.Background(), harness.DispatchRequest{StageName: "qa"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Dispatched || res.Message != "qa" {
		t.Fatalf("result = %+v", res)
	}
}
