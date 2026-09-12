package conversation

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func openConversationTestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hero.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestHasAcceptedTurnAndEmptySurface(t *testing.T) {
	st := openConversationTestStore(t)
	svc := NewSessionService(st, store.DefaultClock())

	res, err := svc.EnsureFirstTurn(context.Background(), "", CreateSessionParams{Kind: store.SessionKindFreechat}, FirstTurnContent{})
	if err != nil {
		t.Fatal(err)
	}
	if res.SessionID != "" || res.Created {
		t.Fatalf("empty surface should not create: %+v", res)
	}
	count, err := countSessions(st)
	if err != nil || count != 0 {
		t.Fatalf("sessions=%d err=%v", count, err)
	}
}

func TestEnsureFirstTurnCreatesAtomically(t *testing.T) {
	st := openConversationTestStore(t)
	svc := NewSessionService(st, store.DefaultClock())

	res, err := svc.EnsureFirstTurn(context.Background(), "", CreateSessionParams{
		Kind:  store.SessionKindFreechat,
		Title: TitleFreeChat("Hello history", false),
	}, FirstTurnContent{Text: "Hello history", Origin: OriginLocal})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Created || res.SessionID == "" || res.FirstEvent.Seq != 1 {
		t.Fatalf("result=%+v", res)
	}
	events, err := st.ListSessionEventsNewest(res.SessionID, 0, 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%v err=%v", events, err)
	}
}

func TestEnsureFirstTurnHonorsPredeterminedSessionID(t *testing.T) {
	st := openConversationTestStore(t)
	svc := NewSessionService(st, store.DefaultClock())
	predetermined, err := store.NewSessionID()
	if err != nil {
		t.Fatal(err)
	}

	res, err := svc.EnsureFirstTurn(context.Background(), "", CreateSessionParams{
		ID:    predetermined,
		Kind:  store.SessionKindFreechat,
		Title: TitleFreeChat("media aligned", false),
	}, FirstTurnContent{Text: "media aligned", Origin: OriginLocal})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Created || res.SessionID != predetermined {
		t.Fatalf("result=%+v", res)
	}
}

func TestEnsureFirstTurnImageOnlyTitle(t *testing.T) {
	st := openConversationTestStore(t)
	svc := NewSessionService(st, store.DefaultClock())

	res, err := svc.EnsureFirstTurn(context.Background(), "", CreateSessionParams{
		Kind:  store.SessionKindFreechat,
		Title: TitleFreeChat("", true),
	}, FirstTurnContent{AttachmentCount: 1, Origin: OriginLocal})
	if err != nil {
		t.Fatal(err)
	}
	sess, err := st.GetSession(res.SessionID)
	if err != nil || sess.Title != titleImageConversation {
		t.Fatalf("title=%q err=%v", sess.Title, err)
	}
}

func TestEnsureFirstTurnStageAgentPrompt(t *testing.T) {
	st := openConversationTestStore(t)
	svc := NewSessionService(st, store.DefaultClock())
	cycleID, err := st.CreateCycle(store.Cycle{
		Number: 1, Title: "c", Objective: "o", Status: store.CycleStatusActive, StartedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}
	title := TitleStageAgent(1, "implementation", "backend_agent")
	res, err := svc.EnsureFirstTurn(context.Background(), "", CreateSessionParams{
		Kind: store.SessionKindStageAgent, Title: title, CycleID: &cycleID, StageName: "implementation", AgentName: "backend_agent",
	}, FirstTurnContent{StageAgentPrompt: "Run tasks", Origin: OriginLocal})
	if err != nil || !res.Created {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if res.Session.Title != title {
		t.Fatalf("title=%q", res.Session.Title)
	}
}

func TestSessionServiceListRenameArchiveRestore(t *testing.T) {
	st := openConversationTestStore(t)
	svc := NewSessionService(st, store.DefaultClock())
	ctx := context.Background()

	created, err := svc.EnsureFirstTurn(ctx, "", CreateSessionParams{
		Kind: store.SessionKindFreechat, Title: "Alpha",
	}, FirstTurnContent{Text: "x"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.EnsureFirstTurn(ctx, "", CreateSessionParams{
		Kind: store.SessionKindFreechat, Title: "Beta",
	}, FirstTurnContent{Text: "y"})
	if err != nil {
		t.Fatal(err)
	}

	active, err := svc.ListSessions(ctx, store.ListSessionsFilter{})
	if err != nil || len(active) != 2 {
		t.Fatalf("active=%d err=%v", len(active), err)
	}
	found, err := svc.ListSessions(ctx, store.ListSessionsFilter{Query: "alp"})
	if err != nil || len(found) != 1 || found[0].ID != created.SessionID {
		t.Fatalf("search=%+v err=%v", found, err)
	}

	renamed, err := svc.RenameSession(ctx, created.SessionID, "  Gamma  ")
	if err != nil || renamed.Title != "Gamma" {
		t.Fatalf("rename=%+v err=%v", renamed, err)
	}
	if _, err := svc.RenameSession(ctx, created.SessionID, "   "); !errors.Is(err, store.ErrEmptySessionTitle) {
		t.Fatalf("empty rename err=%v", err)
	}

	archived, err := svc.ArchiveSession(ctx, created.SessionID)
	if err != nil || archived.Lifecycle != store.SessionLifecycleArchived {
		t.Fatalf("archive=%+v err=%v", archived, err)
	}
	active, _ = svc.ListSessions(ctx, store.ListSessionsFilter{})
	if len(active) != 1 {
		t.Fatalf("active after archive=%d", len(active))
	}
	restored, err := svc.RestoreSession(ctx, created.SessionID)
	if err != nil || restored.Lifecycle != store.SessionLifecycleActive {
		t.Fatalf("restore=%+v err=%v", restored, err)
	}
}

func TestSessionServiceLeaseAcquireRelease(t *testing.T) {
	st := openConversationTestStore(t)
	clock := &fixedClock{t: time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)}
	svc := NewSessionService(st, clock)
	ctx := context.Background()

	res, err := svc.EnsureFirstTurn(ctx, "", CreateSessionParams{
		Kind: store.SessionKindFreechat, Title: "Lease test",
	}, FirstTurnContent{Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.AcquireLease(ctx, res.SessionID, "owner-a")
	if err != nil {
		t.Fatal(err)
	}
	if got.Lease.OwnerID != "owner-a" {
		t.Fatalf("lease=%+v", got.Lease)
	}
	if _, err := svc.AcquireLease(ctx, res.SessionID, "owner-b"); !errors.Is(err, store.ErrSessionBusy) {
		t.Fatalf("busy err=%v", err)
	}
	if err := svc.ReleaseLease(ctx, res.SessionID, "owner-a"); err != nil {
		t.Fatal(err)
	}
}

func TestSessionServiceForkLeavesSourceUnchanged(t *testing.T) {
	st := openConversationTestStore(t)
	svc := NewSessionService(st, store.DefaultClock())
	ctx := context.Background()

	first, err := svc.EnsureFirstTurn(ctx, "", CreateSessionParams{
		Kind: store.SessionKindFreechat, Title: "Source",
	}, FirstTurnContent{Text: "user one"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.AppendEvent(ctx, store.AppendSessionEventInput{
		BoundSessionID: first.SessionID, SessionID: first.SessionID,
		EventType: store.SessionEventAssistant, PayloadJSON: `{"text":"assistant reply"}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	forked, err := svc.ForkSession(ctx, ForkSessionInput{SourceSessionID: first.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	if forked.ID == first.SessionID {
		t.Fatal("fork must use new id")
	}
	source, err := st.GetSession(first.SessionID)
	if err != nil || source.Title != "Source" {
		t.Fatalf("source changed: %+v err=%v", source, err)
	}
	events, err := st.ListSessionEventsNewest(forked.ID, 0, 5)
	if err != nil || len(events) != 1 || events[0].EventType != store.SessionEventNote {
		t.Fatalf("fork events=%+v err=%v", events, err)
	}
	if !strings.Contains(events[0].PayloadJSON, "[Hero context fork from session Source]") {
		t.Fatalf("payload=%s", events[0].PayloadJSON)
	}
}

func TestSessionServiceDeleteLocalFirst(t *testing.T) {
	st := openConversationTestStore(t)
	svc := NewSessionService(st, store.DefaultClock())
	ctx := context.Background()

	res, err := svc.EnsureFirstTurn(ctx, "", CreateSessionParams{
		Kind: store.SessionKindFreechat, Title: "Delete me",
	}, FirstTurnContent{Text: "x"})
	if err != nil {
		t.Fatal(err)
	}
	op, paths, err := svc.DeleteLocalFirst(ctx, res.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if op.SessionID != res.SessionID {
		t.Fatalf("op=%+v", op)
	}
	if err := PurgeManagedAssetFiles(paths); err != nil {
		t.Fatal(err)
	}
	if err := svc.CompleteDeleteAfterPurge(ctx, op, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetSession(res.SessionID); !errors.Is(err, store.ErrSessionNotFound) {
		t.Fatalf("session still exists: %v", err)
	}
}

type fakeRemoteReader struct {
	events []harness.NormalizedEvent
}

func (f *fakeRemoteReader) SupportsRemoteHistory() bool { return true }
func (f *fakeRemoteReader) ReadRemoteHistory(context.Context, string) ([]harness.NormalizedEvent, error) {
	return f.events, nil
}

func TestSessionServiceImportRemoteHistory(t *testing.T) {
	st := openConversationTestStore(t)
	svc := NewSessionService(st, store.DefaultClock())
	svc.Remote = &fakeRemoteReader{events: []harness.NormalizedEvent{{
		EventType: harness.NormalizedEventUser, Origin: harness.NormalizedOriginLocal,
		PayloadJSON: []byte(`{"text":"remote"}`), ProviderEventID: "rid-1",
	}}}
	ctx := context.Background()

	sess, err := st.CreateSession(store.CreateSessionInput{
		Kind: store.SessionKindFreechat, Title: "import", NativeSessionID: "native-1", HarnessID: "opencode",
	})
	if err != nil {
		t.Fatal(err)
	}
	n, err := svc.ImportRemoteHistory(ctx, sess.ID, true)
	if err != nil || n != 1 {
		t.Fatalf("imported=%d err=%v", n, err)
	}
}

type fixedClock struct{ t time.Time }

func (f *fixedClock) Now() time.Time { return f.t }

func countSessions(st *store.Store) (int, error) {
	ids, err := st.ListRegisteredSessionIDs()
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

type captureLogHandler struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (h *captureLogHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureLogHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	var b strings.Builder
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		b.WriteByte(' ')
		b.WriteString(a.Key)
		b.WriteByte('=')
		b.WriteString(a.Value.String())
		return true
	})
	b.WriteByte('\n')
	_, err := h.buf.WriteString(b.String())
	return err
}

func (h *captureLogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *captureLogHandler) WithGroup(string) slog.Handler { return h }

func (h *captureLogHandler) String() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.buf.String()
}

func assertLogsExcludeIDs(t *testing.T, logs string, ids ...string) {
	t.Helper()
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if strings.Contains(logs, id) {
			t.Fatalf("log output must not contain identifier %q; got:\n%s", id, logs)
		}
	}
	if strings.Contains(logs, "session_id=") || strings.Contains(logs, "op_id=") || strings.Contains(logs, "source_session_id=") {
		t.Fatalf("log output must not contain session/op id attrs; got:\n%s", logs)
	}
}

func TestSessionServiceDiagnosticLogsRedactIdentifiers(t *testing.T) {
	st := openConversationTestStore(t)
	capture := &captureLogHandler{}
	svc := NewSessionService(st, store.DefaultClock())
	svc.Log = slog.New(capture)
	svc.Remote = &fakeRemoteReader{events: []harness.NormalizedEvent{{
		EventType: harness.NormalizedEventUser, Origin: harness.NormalizedOriginLocal,
		PayloadJSON: []byte(`{"text":"remote"}`), ProviderEventID: "rid-1",
	}}}
	ctx := context.Background()

	created, err := svc.EnsureFirstTurn(ctx, "", CreateSessionParams{
		Kind: store.SessionKindFreechat, Title: "Log redaction",
	}, FirstTurnContent{Text: "hello", Origin: OriginLocal})
	if err != nil {
		t.Fatal(err)
	}
	sessionID := created.SessionID
	nativeID := "native-log-redaction-1"

	if _, err := svc.RenameSession(ctx, sessionID, "Renamed"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BindNativeSession(ctx, sessionID, "opencode", nativeID, "model", `{}`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportRemoteHistory(ctx, sessionID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ForkSession(ctx, ForkSessionInput{SourceSessionID: sessionID}); err != nil {
		t.Fatal(err)
	}
	op, paths, err := svc.DeleteLocalFirst(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	_ = PurgeManagedAssetFiles(paths)
	if err := svc.CompleteDeleteAfterPurge(ctx, op, ""); err != nil {
		t.Fatal(err)
	}

	assertLogsExcludeIDs(t, capture.String(), sessionID, nativeID)
}

func TestSessionDeleteManagedPathsPersistedInManifest(t *testing.T) {
	st := openConversationTestStore(t)
	svc := NewSessionService(st, store.DefaultClock())
	ctx := context.Background()
	dir := t.TempDir()
	managedFile := filepath.Join(dir, "copy.png")
	if err := os.WriteFile(managedFile, []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := svc.EnsureFirstTurn(ctx, "", CreateSessionParams{
		Kind: store.SessionKindFreechat, Title: "assets",
	}, FirstTurnContent{Text: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertSessionAsset(store.SessionAsset{
		SessionID: res.SessionID, AssetID: "m1", Ownership: store.AssetOwnershipManagedCopy,
		Path: managedFile, Mime: "image/png",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertSessionAsset(store.SessionAsset{
		SessionID: res.SessionID, AssetID: "e1", Ownership: store.AssetOwnershipExternalSource,
		Path: filepath.Join(dir, "original.png"), Mime: "image/png",
	}); err != nil {
		t.Fatal(err)
	}

	op, paths, err := svc.DeleteLocalFirst(ctx, res.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != managedFile {
		t.Fatalf("paths=%v", paths)
	}
	manifest, err := st.ListManagedPathsForDeleteOp(op.ID)
	if err != nil || len(manifest) != 1 || manifest[0] != managedFile {
		t.Fatalf("manifest=%v err=%v", manifest, err)
	}
}

func TestSessionDeleteResumeAfterCrashPurgesManagedFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hero.db")
	managedFile := filepath.Join(dir, "managed.bin")
	if err := os.WriteFile(managedFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	st1, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	sess, err := st1.CreateSession(store.CreateSessionInput{Kind: store.SessionKindFreechat, Title: "crash"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st1.UpsertSessionAsset(store.SessionAsset{
		SessionID: sess.ID, AssetID: "a1", Ownership: store.AssetOwnershipManagedCopy,
		Path: managedFile, Mime: "application/octet-stream",
	}); err != nil {
		t.Fatal(err)
	}
	op, _, err := st1.DeleteSessionLocalFirst(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := st1.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(managedFile); err != nil {
		t.Fatalf("managed file should exist before resume purge: %v", err)
	}

	st2, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st2.Close() })
	svc := NewSessionService(st2, store.DefaultClock())
	ctx := context.Background()

	incomplete, err := st2.ListIncompleteSessionDeleteOps()
	if err != nil || len(incomplete) != 1 || incomplete[0].ID != op.ID {
		t.Fatalf("incomplete=%v err=%v", incomplete, err)
	}
	if err := svc.ResumeIncompleteDelete(ctx, incomplete[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(managedFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("managed file should be purged: %v", err)
	}
	incomplete, err = st2.ListIncompleteSessionDeleteOps()
	if err != nil || len(incomplete) != 0 {
		t.Fatalf("incomplete after resume=%v err=%v", incomplete, err)
	}
}

func TestSessionDeletePurgeFailureKeepsOpIncomplete(t *testing.T) {
	st := openConversationTestStore(t)
	svc := NewSessionService(st, store.DefaultClock())
	ctx := context.Background()
	dir := t.TempDir()
	managedDir := filepath.Join(dir, "manageddir")
	if err := os.Mkdir(managedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(managedDir, "nested"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := svc.EnsureFirstTurn(ctx, "", CreateSessionParams{
		Kind: store.SessionKindFreechat, Title: "purge fail",
	}, FirstTurnContent{Text: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertSessionAsset(store.SessionAsset{
		SessionID: res.SessionID, AssetID: "d1", Ownership: store.AssetOwnershipManagedCopy,
		Path: managedDir, Mime: "application/octet-stream",
	}); err != nil {
		t.Fatal(err)
	}
	op, _, err := svc.DeleteLocalFirst(ctx, res.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ResumeIncompleteDelete(ctx, op); err == nil {
		t.Fatal("expected purge failure removing directory")
	}
	incomplete, err := st.ListIncompleteSessionDeleteOps()
	if err != nil || len(incomplete) != 1 || incomplete[0].Status != store.DeleteOpIntent {
		t.Fatalf("incomplete=%v err=%v", incomplete, err)
	}
}
