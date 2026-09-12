package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

type remoteHarnessStub struct {
	name string
}

func (h *remoteHarnessStub) Name() string                      { return h.name }
func (h *remoteHarnessStub) IsAvailable(context.Context) error { return nil }
func (h *remoteHarnessStub) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return nil, errors.New("not implemented")
}
func (h *remoteHarnessStub) ResumeSession(context.Context, string) error { return nil }
func (h *remoteHarnessStub) Execute(context.Context, harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	return nil, errors.New("not implemented")
}
func (h *remoteHarnessStub) Cancel(context.Context, string) error { return nil }
func (h *remoteHarnessStub) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return nil, errors.New("not implemented")
}
func (h *remoteHarnessStub) Dispatch(context.Context, harness.DispatchRequest) (harness.DispatchResult, error) {
	return harness.DispatchResult{}, errors.New("not implemented")
}
func (h *remoteHarnessStub) SupportsRemoteHistory() bool { return true }
func (h *remoteHarnessStub) ReadRemoteHistory(context.Context, string) ([]harness.NormalizedEvent, error) {
	return nil, nil
}

func TestBindRemoteHistoryReaderRebindsPerHarness(t *testing.T) {
	cyc := newTestService(t)
	svc := conversation.NewSessionService(cyc.Store, store.DefaultClock())
	cursorReader := &remoteHarnessStub{name: "cursor"}
	openReader := &remoteHarnessStub{name: "opencode"}
	reg := routingRegistry{adapters: map[string]harness.HarnessAdapter{
		"cursor":   cursorReader,
		"opencode": openReader,
	}}

	bindRemoteHistoryReader(svc, reg, "cursor")
	if svc.Remote != cursorReader {
		t.Fatal("expected cursor remote reader")
	}
	bindRemoteHistoryReader(svc, reg, "opencode")
	if svc.Remote != openReader {
		t.Fatal("expected opencode remote reader after rebind")
	}
	bindRemoteHistoryReader(svc, reg, "unknown")
	if svc.Remote != nil {
		t.Fatal("expected remote cleared for unsupported harness")
	}
}
