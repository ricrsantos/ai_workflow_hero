package store

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hero.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSessionsCRUDAndNativeUniqueness(t *testing.T) {
	s := openTestStore(t)

	sess, err := s.CreateSession(CreateSessionInput{
		Kind:  SessionKindFreechat,
		Title: "First chat",
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID == "" || sess.Lifecycle != SessionLifecycleActive || sess.TranscriptState != TranscriptAvailable {
		t.Fatalf("unexpected session: %+v", sess)
	}

	got, err := s.GetSession(sess.ID)
	if err != nil || got.Title != "First chat" {
		t.Fatalf("GetSession: %+v %v", got, err)
	}

	renamed, err := s.UpdateSessionTitle(sess.ID, "  Renamed  ")
	if err != nil || renamed.Title != "Renamed" {
		t.Fatalf("UpdateSessionTitle: %+v %v", renamed, err)
	}
	if _, err := s.UpdateSessionTitle(sess.ID, "   "); !errors.Is(err, ErrEmptySessionTitle) {
		t.Fatalf("empty title err=%v", err)
	}

	// Duplicate titles allowed.
	other, err := s.CreateSession(CreateSessionInput{Kind: SessionKindFreechat, Title: "Renamed"})
	if err != nil {
		t.Fatalf("duplicate title create: %v", err)
	}

	bound, err := s.BindNativeSession(sess.ID, "cursor", "native-1", "composer-2.5", `{"ef":"na"}`)
	if err != nil || bound.NativeSessionID != "native-1" {
		t.Fatalf("BindNativeSession: %+v %v", bound, err)
	}
	if _, err := s.BindNativeSession(other.ID, "cursor", "native-1", "x", "{}"); !errors.Is(err, ErrDuplicateNativeSession) {
		t.Fatalf("want duplicate native err, got %v", err)
	}

	archived, err := s.UpdateSessionLifecycle(sess.ID, SessionLifecycleArchived, nil)
	if err != nil || archived.Lifecycle != SessionLifecycleArchived {
		t.Fatalf("archive: %+v %v", archived, err)
	}
}

func TestSessionsCycleOnDeleteSetNull(t *testing.T) {
	s := openTestStore(t)
	cycleID, err := s.CreateCycle(Cycle{
		Number: 1, Title: "c", Objective: "o", Status: CycleStatusActive, StartedAt: nowRFC3339(),
	})
	if err != nil {
		t.Fatalf("CreateCycle: %v", err)
	}
	sess, err := s.CreateSession(CreateSessionInput{
		Kind: SessionKindOrchestration, Title: "C1 · Orchestration · ORCH", CycleID: &cycleID,
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if !sess.CycleID.Valid || sess.CycleID.Int64 != cycleID {
		t.Fatalf("cycle_id=%v", sess.CycleID)
	}
	if _, err := s.db.Exec(`DELETE FROM cycles WHERE id = ?`, cycleID); err != nil {
		t.Fatalf("delete cycle: %v", err)
	}
	got, err := s.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession after cycle delete: %v", err)
	}
	if got.CycleID.Valid {
		t.Fatalf("expected NULL cycle_id, got %v", got.CycleID.Int64)
	}
}

func TestListSessionsActiveArchivedAndSearch(t *testing.T) {
	s := openTestStore(t)
	older := "2026-09-10T10:00:00Z"
	newer := "2026-09-11T12:00:00Z"
	a, err := s.CreateSession(CreateSessionInput{Kind: SessionKindFreechat, Title: "Deployment review", LastActivityAt: older, CreatedAt: older})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.CreateSession(CreateSessionInput{Kind: SessionKindFreechat, Title: "Image exploration", LastActivityAt: newer, CreatedAt: newer})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateSessionLifecycle(a.ID, SessionLifecycleArchived, nil); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		filter  ListSessionsFilter
		wantIDs []string
	}{
		{name: "active ordered", filter: ListSessionsFilter{}, wantIDs: []string{b.ID}},
		{name: "archived", filter: ListSessionsFilter{Archived: true}, wantIDs: []string{a.ID}},
		{name: "search active case-insensitive", filter: ListSessionsFilter{Query: "image"}, wantIDs: []string{b.ID}},
		{name: "search archived", filter: ListSessionsFilter{Archived: true, Query: "DEPLOY"}, wantIDs: []string{a.ID}},
		{name: "search miss", filter: ListSessionsFilter{Query: "nope"}, wantIDs: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.ListSessions(tt.filter)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("len=%d want %d (%v)", len(got), len(tt.wantIDs), got)
			}
			for i, id := range tt.wantIDs {
				if got[i].ID != id {
					t.Fatalf("got[%d]=%s want %s", i, got[i].ID, id)
				}
			}
		})
	}
}

func TestAppendSessionEventMonotonicAndActivity(t *testing.T) {
	s := openTestStore(t)
	sess, err := s.CreateSession(CreateSessionInput{Kind: SessionKindFreechat, Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	e1, err := s.AppendSessionEvent(AppendSessionEventInput{
		SessionID: sess.ID, BoundSessionID: sess.ID,
		EventType: SessionEventUser, Origin: SessionOriginTelegram, OriginAddress: "aiwk",
		PayloadJSON: `{"text":"hi"}`,
	})
	if err != nil || e1.Seq != 1 {
		t.Fatalf("e1=%+v err=%v", e1, err)
	}
	e2, err := s.AppendSessionEvent(AppendSessionEventInput{
		SessionID: sess.ID, BoundSessionID: sess.ID,
		EventType: SessionEventAssistant, PayloadJSON: `{"text":"yo"}`,
	})
	if err != nil || e2.Seq != 2 {
		t.Fatalf("e2=%+v err=%v", e2, err)
	}
	got, _ := s.GetSession(sess.ID)
	if got.LastActivityAt != e2.CreatedAt || got.LastOrigin != SessionOriginLocal {
		t.Fatalf("activity=%+v", got)
	}
	if _, err := s.AppendSessionEvent(AppendSessionEventInput{
		SessionID: sess.ID, BoundSessionID: "other", EventType: SessionEventUser, PayloadJSON: `{}`,
	}); !errors.Is(err, ErrCrossSessionRouting) {
		t.Fatalf("cross-session err=%v", err)
	}
}

func TestAppendSessionEventProviderIdempotencyAndPaging(t *testing.T) {
	s := openTestStore(t)
	sess, first, err := s.CreateSessionWithFirstEvent(
		CreateSessionInput{Kind: SessionKindFreechat, Title: "stream"},
		AppendSessionEventInput{EventType: SessionEventUser, PayloadJSON: `{"text":"1"}`, ProviderEventID: "p1"},
	)
	if err != nil || first.Seq != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	again, err := s.AppendSessionEvent(AppendSessionEventInput{
		SessionID: sess.ID, BoundSessionID: sess.ID,
		EventType: SessionEventUser, PayloadJSON: `{"text":"1"}`, ProviderEventID: "p1",
	})
	if err != nil || again.Seq != 1 {
		t.Fatalf("idempotent=%+v err=%v", again, err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.AppendSessionEvent(AppendSessionEventInput{
				SessionID: sess.ID, BoundSessionID: sess.ID,
				EventType: SessionEventAssistant, PayloadJSON: `{}`,
				ProviderEventID: "concurrent-" + string(rune('A'+(i%26))) + "-" + time.Now().Format("15:04:05.000000000") + "-" + string(rune('0'+i%10)),
			})
			// Unique provider ids via index; on collision retry without id.
			if err != nil {
				_, err = s.AppendSessionEvent(AppendSessionEventInput{
					SessionID: sess.ID, BoundSessionID: sess.ID,
					EventType: SessionEventAssistant, PayloadJSON: `{}`,
				})
			}
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent append: %v", err)
		}
	}
	page, err := s.ListSessionEventsNewest(sess.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 10 {
		t.Fatalf("page len=%d", len(page))
	}
	for i := 1; i < len(page); i++ {
		if page[i].Seq <= page[i-1].Seq {
			t.Fatalf("non-monotonic page: %+v", page)
		}
	}
	all, err := s.ListSessionEventsNewest(sess.ID, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(all); i++ {
		if all[i].Seq != all[i-1].Seq+1 {
			t.Fatalf("gap at %d: %d -> %d", i, all[i-1].Seq, all[i].Seq)
		}
	}
}

func TestSessionLeaseCAS(t *testing.T) {
	s := openTestStore(t)
	sess, err := s.CreateSession(CreateSessionInput{Kind: SessionKindFreechat, Title: "lease"})
	if err != nil {
		t.Fatal(err)
	}
	clock := &manualClock{now: time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)}
	a, err := s.AcquireSessionLease(sess.ID, "owner-a", clock)
	if err != nil || a.TookOver {
		t.Fatalf("acquire a: %+v %v", a, err)
	}
	if _, err := s.AcquireSessionLease(sess.ID, "owner-b", clock); !errors.Is(err, ErrSessionBusy) {
		t.Fatalf("busy err=%v", err)
	}
	clock.now = clock.now.Add(LeaseTTL + time.Second)
	b, err := s.AcquireSessionLease(sess.ID, "owner-b", clock)
	if err != nil || !b.TookOver {
		t.Fatalf("stale takeover: %+v %v", b, err)
	}
	if _, err := s.HeartbeatSessionLease(sess.ID, "owner-a", clock); !errors.Is(err, ErrSessionBusy) {
		t.Fatalf("foreign heartbeat err=%v", err)
	}
	if _, err := s.HeartbeatSessionLease(sess.ID, "owner-b", clock); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if err := s.ReleaseSessionLease(sess.ID, "owner-b"); err != nil {
		t.Fatal(err)
	}
	_, ok, err := s.GetSessionLease(sess.ID)
	if err != nil || ok {
		t.Fatalf("lease after release ok=%v err=%v", ok, err)
	}
}

type manualClock struct{ now time.Time }

func (c *manualClock) Now() time.Time { return c.now }

func TestSessionAssetsAndDeleteOps(t *testing.T) {
	s := openTestStore(t)
	sess, err := s.CreateSession(CreateSessionInput{Kind: SessionKindFreechat, Title: "media"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSessionAsset(SessionAsset{
		SessionID: sess.ID, AssetID: "a1", Ownership: AssetOwnershipManagedCopy,
		Path: "/tmp/managed.png", Mime: "image/png", OriginalName: "x.png", CardMetaJSON: `{"w":1}`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSessionAsset(SessionAsset{
		SessionID: sess.ID, AssetID: "a2", Ownership: AssetOwnershipExternalSource,
		Path: "/home/user/photo.jpg", Mime: "image/jpeg",
	}); err != nil {
		t.Fatal(err)
	}
	assets, err := s.ListSessionAssets(sess.ID)
	if err != nil || len(assets) != 2 {
		t.Fatalf("assets=%v err=%v", assets, err)
	}
	managed, err := s.ListManagedAssetPaths(sess.ID)
	if err != nil || len(managed) != 1 || managed[0] != "/tmp/managed.png" {
		t.Fatalf("managed=%v err=%v", managed, err)
	}

	op, paths, err := s.DeleteSessionLocalFirst(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if op.Status != DeleteOpIntent || len(paths) != 1 || paths[0] != "/tmp/managed.png" {
		t.Fatalf("op=%+v paths=%v", op, paths)
	}
	manifest, err := s.ListManagedPathsForDeleteOp(op.ID)
	if err != nil || len(manifest) != 1 || manifest[0] != "/tmp/managed.png" {
		t.Fatalf("manifest=%v err=%v", manifest, err)
	}
	if _, err := s.GetSession(sess.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("session should be gone: %v", err)
	}
	if err := s.UpdateSessionDeleteOpStatus(op.ID, DeleteOpLocalPurged, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateSessionDeleteOpStatus(op.ID, DeleteOpRemoteAttempted, "unsupported"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateSessionDeleteOpStatus(op.ID, DeleteOpCompleted, "unsupported"); err != nil {
		t.Fatal(err)
	}
	incomplete, err := s.ListIncompleteSessionDeleteOps()
	if err != nil || len(incomplete) != 0 {
		t.Fatalf("incomplete=%v err=%v", incomplete, err)
	}
}
