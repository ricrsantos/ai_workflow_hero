package daemon

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "telegram-daemon.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStoreUpdateDedup(t *testing.T) {
	s := openTestStore(t)
	now := time.Now()
	if err := s.MarkUpdateProcessed(42, now); err != nil {
		t.Fatal(err)
	}
	got, err := s.UpdateProcessed(42)
	if err != nil || !got {
		t.Fatalf("got=%v err=%v", got, err)
	}
	got, err = s.UpdateProcessed(43)
	if err != nil || got {
		t.Fatalf("update 43 should be unprocessed: got=%v", got)
	}
}

func TestStorePendingLifecycle(t *testing.T) {
	s := openTestStore(t)
	now := time.Now()
	if err := s.EnqueuePending(PendingMessage{Address: "proj", Text: "hi", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.PendingForAddress("proj")
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	id := rows[0].ID
	if err := s.Transition(id, StatusDelivered); err != nil {
		t.Fatal(err)
	}
	if err := s.Transition(id, StatusProcessed); err != nil {
		t.Fatal(err)
	}
	rows, _ = s.PendingForAddress("proj")
	if len(rows) != 0 {
		t.Fatalf("expected no pending rows, got %d", len(rows))
	}
}

func TestStoreCancelPendingForAddress(t *testing.T) {
	s := openTestStore(t)
	now := time.Now()
	_ = s.EnqueuePending(PendingMessage{Address: "proj", Text: "a", CreatedAt: now})
	_ = s.EnqueuePending(PendingMessage{Address: "proj", Text: "b", CreatedAt: now})
	_ = s.EnqueuePending(PendingMessage{Address: "other", Text: "c", CreatedAt: now})
	n, err := s.CancelPendingForAddress("proj")
	if err != nil || n != 2 {
		t.Fatalf("cancelled=%d err=%v", n, err)
	}
	rows, _ := s.PendingForAddress("other")
	if len(rows) != 1 {
		t.Fatalf("other address affected: %d rows", len(rows))
	}
}

func TestStoreExpirePending(t *testing.T) {
	s := openTestStore(t)
	now := time.Now()
	_ = s.EnqueuePending(PendingMessage{Address: "proj", Text: "old", CreatedAt: now.Add(-25 * time.Hour)})
	_ = s.EnqueuePending(PendingMessage{Address: "proj", Text: "new", CreatedAt: now})
	n, err := s.ExpirePending(now)
	if err != nil || n != 1 {
		t.Fatalf("expired=%d err=%v", n, err)
	}
	rows, _ := s.PendingForAddress("proj")
	if len(rows) != 1 || rows[0].Text != "new" {
		t.Fatalf("unexpected pending rows: %+v", rows)
	}
}
