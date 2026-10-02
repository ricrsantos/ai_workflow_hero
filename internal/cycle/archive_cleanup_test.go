package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/testaccess"
)

func TestArchivePendingCrashRecovery(t *testing.T) {
	for _, cleaned := range []bool{false, true} {
		t.Run(fmt.Sprintf("cleanup_%t", cleaned), func(t *testing.T) {
			dir, svc := credentialArchiveProject(t)
			finishCycleForArchive(t, svc)
			c, err := svc.Store.GetCurrentCycle()
			if err != nil {
				t.Fatal(err)
			}
			current := filepath.Join(dir, ".workflow-hero/cycles/current")
			writeArchiveTestFile(t, filepath.Join(current, "screenshots", "retained.png"), "evidence")
			info, err := os.Stat(current)
			if err != nil {
				t.Fatal(err)
			}
			stat := info.Sys().(*syscall.Stat_t)
			name := fmt.Sprintf("C%d-%s-safe-archive", c.Number, c.CompletedAt[:10])
			payload, err := json.Marshal(struct {
				Destination string `json:"destination"`
				Device      uint64 `json:"device"`
				Inode       uint64 `json:"inode"`
			}{name, uint64(stat.Dev), uint64(stat.Ino)})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Store.AppendEvent(store.Event{CycleID: c.ID, Type: "archive_move_intent", PayloadJSON: string(payload)}); err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(dir, ".workflow-hero/cycles/archive", name)
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(current, dest); err != nil {
				t.Fatal(err)
			}
			if err := svc.Store.AcquireArchiveLock(c.ID, "archive-999999-123456", "crash"); err != nil {
				t.Fatal(err)
			}
			if cleaned {
				if err := os.Remove(filepath.Join(dir, ".env.hero")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(current, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			result, err := svc.Archive()
			if err != nil {
				t.Fatal(err)
			}
			requireArchiveTestFileContent(t, filepath.Join(result.ArchiveDir, "screenshots/retained.png"), "evidence")
			if _, err := os.Stat(filepath.Join(dir, ".env.hero")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("credentials retained after recovery")
			}
		})
	}
}

func TestArchivePendingDatabaseFailureRetainsCredentials(t *testing.T) {
	dir, svc := credentialArchiveProject(t)
	finishCycleForArchive(t, svc)
	if _, err := svc.Store.DB().Exec(`CREATE TRIGGER reject_archive BEFORE UPDATE OF status ON cycles WHEN NEW.status = 'archived' BEGIN SELECT RAISE(ABORT, 'synthetic archive failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Archive(); err == nil {
		t.Fatal("archive accepted rejected database prerequisite")
	}
	assertCredentialsRetained(t, dir)
	if _, err := svc.Store.DB().Exec(`DROP TRIGGER reject_archive`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Archive(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveCleanupForeignProjectStoreRefused(t *testing.T) {
	original, svc := credentialArchiveProject(t)
	finishCycleForArchive(t, svc)
	foreign, _ := credentialArchiveProject(t)
	svc.ProjectDir = foreign
	if _, err := svc.Archive(); err == nil {
		t.Fatal("foreign project credential cleanup accepted")
	}
	assertCredentialsRetained(t, original)
	assertCredentialsRetained(t, foreign)
}

func credentialArchiveProject(t *testing.T) (string, *cycle.Service) {
	t.Helper()
	dir := setupProject(t)
	if err := exec.Command("git", "-C", dir, "init", "--quiet").Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("/.env.hero\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env.hero"), []byte("SYNTHETIC_ARCHIVE_SENTINEL"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if _, err := svc.NewCycle("Safe archive", ""); err != nil {
		t.Fatal(err)
	}
	return dir, svc
}

func assertCredentialsRetained(t *testing.T, dir string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".env.hero"))
	if err != nil || string(data) != "SYNTHETIC_ARCHIVE_SENTINEL" {
		t.Fatal("project-root credentials were not retained")
	}
}

func TestArchiveCleanupRetainsScreenshotsAndIgnore(t *testing.T) {
	dir, svc := credentialArchiveProject(t)
	finishCycleForArchive(t, svc)
	writeArchiveTestFile(t, filepath.Join(dir, ".workflow-hero", "cycles", "current", "screenshots", "shot-1.png"), "synthetic-evidence")
	result, err := svc.Archive()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, ".env.hero")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("root credentials remain after archive")
	}
	requireArchiveTestFileContent(t, filepath.Join(result.ArchiveDir, "screenshots", "shot-1.png"), "synthetic-evidence")
	requireArchiveTestFileContent(t, filepath.Join(dir, ".gitignore"), "/.env.hero\n")
	if err := filepath.WalkDir(result.ArchiveDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Name() == ".env.hero" {
			t.Error("archive contains credentials")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Resume(1); err != nil {
		t.Fatal(err)
	}
	access, err := testaccess.OpenSafeStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = access.Close() }()
	if _, err := access.Snapshot(context.Background(), "operator"); err == nil {
		t.Fatal("archived credentials reused on resume")
	}
}

func TestArchiveCleanupFailedOpenspecRetainsCredentials(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "force"}[force], func(t *testing.T) {
			dir, svc := credentialArchiveProject(t)
			finishCycleForArchive(t, svc)
			mkdirOpenspecChange(t, dir, "c17-fixture")
			svc.OpenspecRunner = func(context.Context, string) error { return errors.New("synthetic failure") }
			_, err := svc.ArchiveWithOptions(cycle.ArchiveOptions{OpenspecChange: "c17-fixture", Force: force})
			if !errors.Is(err, cycle.ErrOpenspecArchiveFailed) {
				t.Fatalf("expected OpenSpec blocker: %v", err)
			}
			assertCredentialsRetained(t, dir)
		})
	}
}

func TestArchivePendingUnsafeCleanupRestoresEvidenceAndRetries(t *testing.T) {
	dir, svc := credentialArchiveProject(t)
	finishCycleForArchive(t, svc)
	current := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	writeArchiveTestFile(t, filepath.Join(current, "screenshots", "shot-1.png"), "evidence")
	if err := os.Chmod(filepath.Join(dir, ".env.hero"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Archive()
	if err == nil || !strings.Contains(err.Error(), "/hero-archive") {
		t.Fatalf("missing actionable pending cleanup: %v", err)
	}
	assertCredentialsRetained(t, dir)
	requireArchiveTestFileContent(t, filepath.Join(current, "screenshots", "shot-1.png"), "evidence")
	c, err := svc.Store.GetCurrentCycle()
	if err != nil || c.Status == store.CycleStatusArchived || c.LockHolder != "" {
		t.Fatal("failed cleanup archived or retained operation lock")
	}
	events, err := svc.Store.ListEvents(c.ID, "archive_pending", 0)
	if err != nil || len(events) != 1 {
		t.Fatal("archive pending was not durable")
	}
	if err := os.Chmod(filepath.Join(dir, ".env.hero"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Archive(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveCleanupSymlinkAndCompetingOwners(t *testing.T) {
	for _, mode := range []string{"symlink", "tui", "cycle", "destination"} {
		t.Run(mode, func(t *testing.T) {
			dir, svc := credentialArchiveProject(t)
			finishCycleForArchive(t, svc)
			switch mode {
			case "symlink":
				if err := os.Rename(filepath.Join(dir, ".env.hero"), filepath.Join(dir, "retained.txt")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("retained.txt", filepath.Join(dir, ".env.hero")); err != nil {
					t.Fatal(err)
				}
			case "tui":
				access, err := testaccess.OpenSafeStore(dir, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = access.Close() }()
			case "cycle":
				c, err := svc.Store.GetCurrentCycle()
				if err != nil {
					t.Fatal(err)
				}
				if err := svc.Store.SetCycleLock(c.ID, "other-session", "now"); err != nil {
					t.Fatal(err)
				}
			case "destination":
				c, err := svc.Store.GetCurrentCycle()
				if err != nil {
					t.Fatal(err)
				}
				completed, err := time.Parse(time.RFC3339, c.CompletedAt)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(dir, ".workflow-hero", "cycles", "archive", "C1-"+completed.Format("2006-01-02")+"-safe-archive")
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := svc.Archive(); err == nil {
				t.Fatal("unsafe or competing archive accepted")
			}
			assertCredentialsRetained(t, dir)
		})
	}
}

func TestArchiveCleanupMissingIsSuccess(t *testing.T) {
	dir, svc := credentialArchiveProject(t)
	finishCycleForArchive(t, svc)
	if err := os.Remove(filepath.Join(dir, ".env.hero")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Archive(); err != nil {
		t.Fatal(err)
	}
}

func TestLifecyclePreserveFinishCancel(t *testing.T) {
	for _, action := range []string{"finish", "cancel"} {
		t.Run(action, func(t *testing.T) {
			dir, svc := credentialArchiveProject(t)
			if action == "finish" {
				finishCycleForArchive(t, svc)
			} else if err := svc.Cancel("synthetic user cancellation"); err != nil {
				t.Fatal(err)
			}
			assertCredentialsRetained(t, dir)
		})
	}
}
