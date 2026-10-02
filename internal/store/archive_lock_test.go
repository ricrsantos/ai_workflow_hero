package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestArchiveCleanupCycleOwnership(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "hero.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	id, err := s.CreateCycle(Cycle{Number: 1, Status: CycleStatusCompleted, ConfigSnapshotJSON: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AcquireArchiveLock(id, "archive-a", "now"); err != nil {
		t.Fatal(err)
	}
	if err := s.AcquireArchiveLock(id, "archive-b", "now"); !errors.Is(err, ErrBusy) {
		t.Fatal("competing archive acquired cycle")
	}
	if err := s.ReleaseArchiveLock(id, "archive-b"); err != nil {
		t.Fatal(err)
	}
	c, err := s.GetCycle(id)
	if err != nil || c.LockHolder != "archive-a" {
		t.Fatal("foreign release removed ownership")
	}
	if err := s.ReleaseArchiveLock(id, "archive-a"); err != nil {
		t.Fatal(err)
	}
	if err := s.AcquireArchiveLock(id, "archive-b", "now"); err != nil {
		t.Fatal(err)
	}
}
