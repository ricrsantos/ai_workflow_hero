package harness_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

type fakeRemoteReader struct {
	supported bool
	events    []harness.NormalizedEvent
	err       error
}

func (f fakeRemoteReader) Name() string                      { return "fake-remote" }
func (f fakeRemoteReader) IsAvailable(context.Context) error { return nil }
func (f fakeRemoteReader) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return nil, errors.New("not implemented")
}
func (f fakeRemoteReader) ResumeSession(context.Context, string) error { return nil }
func (f fakeRemoteReader) Execute(context.Context, harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	return nil, errors.New("not implemented")
}
func (f fakeRemoteReader) Cancel(context.Context, string) error { return nil }
func (f fakeRemoteReader) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return nil, errors.New("not implemented")
}
func (f fakeRemoteReader) Dispatch(context.Context, harness.DispatchRequest) (harness.DispatchResult, error) {
	return harness.DispatchResult{}, errors.New("not implemented")
}
func (f fakeRemoteReader) SupportsRemoteHistory() bool { return f.supported }
func (f fakeRemoteReader) ReadRemoteHistory(context.Context, string) ([]harness.NormalizedEvent, error) {
	return f.events, f.err
}

func TestRemoteHistoryReaderOptionalContract(t *testing.T) {
	var _ harness.RemoteHistoryReader = fakeRemoteReader{supported: true}

	var base harness.HarnessAdapter = stubAdapter{name: "stub"}
	if _, ok := base.(harness.RemoteHistoryReader); ok {
		t.Fatal("stub HarnessAdapter must not implement RemoteHistoryReader")
	}

	var reader harness.HarnessAdapter = fakeRemoteReader{
		supported: true,
		events: []harness.NormalizedEvent{{
			EventType:       harness.NormalizedEventUser,
			Origin:          harness.NormalizedOriginLocal,
			PayloadJSON:     []byte(`{"text":"hi"}`),
			ProviderEventID: "evt-1",
			CreatedAt:       time.Unix(1, 0).UTC(),
		}},
	}
	r, ok := reader.(harness.RemoteHistoryReader)
	if !ok || !r.SupportsRemoteHistory() {
		t.Fatal("expected remote history support")
	}
	events, err := r.ReadRemoteHistory(context.Background(), "native-1")
	if err != nil || len(events) != 1 || events[0].EventType != harness.NormalizedEventUser {
		t.Fatalf("events=%v err=%v", events, err)
	}
}

type stubDeleter struct {
	stubAdapter
}

func (stubDeleter) SupportsNativeDelete() bool { return false }
func (stubDeleter) DeleteNativeSession(context.Context, string) error {
	return errors.New("unsupported")
}

func TestNativeSessionDeleterOptionalContract(t *testing.T) {
	var _ harness.NativeSessionDeleter = stubDeleter{stubAdapter{name: "del"}}
}

type fakeLiveAttacher struct {
	stubAdapter
	supported bool
}

func (f fakeLiveAttacher) SupportsLiveStreamAttach() bool { return f.supported }
func (f fakeLiveAttacher) AttachLiveStream(context.Context, harness.LiveStreamAttachRequest) (*harness.ExecutionResult, error) {
	return &harness.ExecutionResult{Output: "attached"}, nil
}

func TestLiveStreamAttacherOptionalContract(t *testing.T) {
	var _ harness.LiveStreamAttacher = fakeLiveAttacher{supported: true, stubAdapter: stubAdapter{name: "live"}}

	var base harness.HarnessAdapter = stubAdapter{name: "stub"}
	if _, ok := base.(harness.LiveStreamAttacher); ok {
		t.Fatal("stub HarnessAdapter must not implement LiveStreamAttacher")
	}
}

func TestExactResumeUnavailableError(t *testing.T) {
	cause := errors.New("session missing")
	err := harness.NewExactResumeUnavailable("abc", cause)
	if !errors.Is(err, harness.ErrExactResumeUnavailable) {
		t.Fatalf("errors.Is failed: %v", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("unwrap failed: %v", err)
	}
	msg := err.Error()
	if msg == "" {
		t.Fatal("message empty")
	}
	if msg != harness.ErrExactResumeUnavailable.Error() {
		t.Fatalf("message=%q want constant sentinel", msg)
	}
	if strings.Contains(msg, "abc") || strings.Contains(msg, "session missing") {
		t.Fatalf("message must not leak session id or cause: %q", msg)
	}
	var exact *harness.ExactResumeError
	if !errors.As(err, &exact) || exact.NativeSessionID != "abc" || exact.Cause != cause {
		t.Fatalf("typed fields lost: %+v", err)
	}
}
