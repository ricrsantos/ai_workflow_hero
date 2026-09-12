package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupRetainsRegisteredSessionsDespiteAge(t *testing.T) {
	dataHome := t.TempDir()
	root := filepath.Join(dataHome, "hero", "sessions")
	registeredID := "durable-hero-session-id"
	oldRegistered := filepath.Join(root, registeredID)
	oldOrphan := filepath.Join(root, "orphan-temp-run")
	if err := os.MkdirAll(filepath.Join(oldRegistered, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(oldOrphan, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	oldTime := now.Add(-8 * 24 * time.Hour)
	for _, dir := range []string{oldRegistered, oldOrphan} {
		if err := os.Chtimes(dir, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
	}

	result, err := CleanupExpiredSessions(context.Background(), CleanupOptions{
		DataHome:   dataHome,
		Retention:  7 * 24 * time.Hour,
		Now:        func() time.Time { return now },
		Registered: RegisteredSessionIDSet([]string{registeredID}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RemovedSessions != 1 || result.RetainedSessions != 1 {
		t.Fatalf("cleanup counts=%+v want 1 removed 1 retained", result)
	}
	if _, err := os.Stat(oldRegistered); err != nil {
		t.Fatalf("registered session dir removed: %v", err)
	}
	if _, err := os.Stat(oldOrphan); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphan session dir still exists: %v", err)
	}
}

func TestCleanupListRegisteredCallback(t *testing.T) {
	dataHome := t.TempDir()
	root := filepath.Join(dataHome, "hero", "sessions")
	dir := filepath.Join(root, "callback-protected")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(dir, now.Add(-10*24*time.Hour), now.Add(-10*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	_, err := CleanupExpiredSessions(context.Background(), CleanupOptions{
		DataHome:  dataHome,
		Retention: 7 * 24 * time.Hour,
		Now:       func() time.Time { return now },
		ListRegistered: func() (RegisteredSessionIDs, error) {
			return RegisteredSessionIDSet([]string{"callback-protected"}), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("callback-registered dir removed: %v", err)
	}
}
