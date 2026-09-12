package conversation

import (
	"context"
	"errors"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestShouldOfferRemoteImport(t *testing.T) {
	st := openConversationTestStore(t)
	svc := NewSessionService(st, store.DefaultClock())
	svc.Remote = &fakeRemoteReader{events: nil}

	sess := store.Session{
		ID:                    "s1",
		NativeSessionID:       "native-1",
		HarnessID:             "opencode",
		TranscriptState:       store.TranscriptUnavailableLegacy,
		RemoteImportConfirmed: false,
	}
	if !svc.ShouldOfferRemoteImport(sess, false) {
		t.Fatal("expected offer for legacy without local events")
	}
	sess.TranscriptState = store.TranscriptAvailable
	if svc.ShouldOfferRemoteImport(sess, true) {
		t.Fatal("session with local events should not re-offer")
	}
	sess.TranscriptState = store.TranscriptUnavailableLegacy
	sess.RemoteImportConfirmed = true
	if svc.ShouldOfferRemoteImport(sess, false) {
		t.Fatal("confirmed session should not offer")
	}
	svc.Remote = nil
	if svc.ShouldOfferRemoteImport(sess, false) {
		t.Fatal("unsupported remote should not offer")
	}
}

func TestImportRemoteHistoryFailedReadLeavesEvents(t *testing.T) {
	st := openConversationTestStore(t)
	svc := NewSessionService(st, store.DefaultClock())
	svc.Remote = &fakeRemoteReaderFail{}
	ctx := context.Background()

	sess, err := st.CreateSession(store.CreateSessionInput{
		Kind: store.SessionKindFreechat, Title: "import", NativeSessionID: "native-1", HarnessID: "opencode",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.AppendSessionEvent(store.AppendSessionEventInput{
		BoundSessionID: sess.ID,
		SessionID:      sess.ID,
		EventType:      store.SessionEventUser,
		Origin:         store.SessionOriginLocal,
		PayloadJSON:    `{"text":"local"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.ImportRemoteHistory(ctx, sess.ID, true)
	if err == nil {
		t.Fatal("expected import error")
	}
	events, err := st.ListSessionEventsNewest(sess.ID, 0, 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%d err=%v", len(events), err)
	}
}

type fakeRemoteReaderFail struct{}

func (f *fakeRemoteReaderFail) SupportsRemoteHistory() bool { return true }
func (f *fakeRemoteReaderFail) ReadRemoteHistory(context.Context, string) ([]harness.NormalizedEvent, error) {
	return nil, errors.New("read failed")
}

func TestImportRemoteHistoryAtomicFailureRollsBack(t *testing.T) {
	st := openConversationTestStore(t)
	svc := NewSessionService(st, store.DefaultClock())
	svc.Remote = &fakeRemoteReader{events: []harness.NormalizedEvent{
		{
			EventType: harness.NormalizedEventUser, Origin: harness.NormalizedOriginLocal,
			PayloadJSON: []byte(`{"text":"remote"}`), ProviderEventID: "rid-ok",
		},
		{
			EventType: "not-a-real-event", Origin: harness.NormalizedOriginLocal,
			PayloadJSON: []byte(`{}`), ProviderEventID: "rid-bad",
		},
	}}
	ctx := context.Background()

	sess, err := st.CreateSession(store.CreateSessionInput{
		Kind:            store.SessionKindFreechat,
		Title:           "import",
		NativeSessionID: "native-1",
		HarnessID:       "opencode",
		TranscriptState: store.TranscriptUnavailableLegacy,
	})
	if err != nil {
		t.Fatal(err)
	}
	n, err := svc.ImportRemoteHistory(ctx, sess.ID, true)
	if err == nil {
		t.Fatal("expected import transaction error")
	}
	if n != 0 {
		t.Fatalf("imported=%d want 0 on rollback", n)
	}
	updated, err := st.GetSession(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.RemoteImportConfirmed {
		t.Fatal("remote_import_confirmed must remain false when import fails")
	}
	if updated.TranscriptState != store.TranscriptUnavailableLegacy {
		t.Fatalf("transcript_state=%q want unavailable_legacy", updated.TranscriptState)
	}
	events, err := st.ListSessionEventsNewest(sess.ID, 0, 10)
	if err != nil || len(events) != 0 {
		t.Fatalf("events=%d err=%v want none after rollback", len(events), err)
	}
}

func TestImportRemoteHistorySetsTranscriptAvailable(t *testing.T) {
	st := openConversationTestStore(t)
	svc := NewSessionService(st, store.DefaultClock())
	svc.Remote = &fakeRemoteReader{events: []harness.NormalizedEvent{{
		EventType: harness.NormalizedEventUser, Origin: harness.NormalizedOriginLocal,
		PayloadJSON: []byte(`{"text":"remote"}`), ProviderEventID: "rid-1",
	}}}
	ctx := context.Background()

	sess, err := st.CreateSession(store.CreateSessionInput{
		Kind:            store.SessionKindFreechat,
		Title:           "import",
		NativeSessionID: "native-1",
		HarnessID:       "opencode",
		TranscriptState: store.TranscriptUnavailableLegacy,
	})
	if err != nil {
		t.Fatal(err)
	}
	n, err := svc.ImportRemoteHistory(ctx, sess.ID, true)
	if err != nil || n != 1 {
		t.Fatalf("imported=%d err=%v", n, err)
	}
	updated, err := st.GetSession(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.TranscriptState != store.TranscriptAvailable {
		t.Fatalf("transcript_state=%q want available", updated.TranscriptState)
	}
	if !updated.RemoteImportConfirmed {
		t.Fatal("expected remote_import_confirmed")
	}
}
