package testaccess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func accessProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := exec.Command("git", "-C", root, "init", "--quiet").Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".workflow-hero"), 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func openAccessStore(t *testing.T, root string, output *bytes.Buffer) *SafeStore {
	t.Helper()
	if output == nil {
		output = &bytes.Buffer{}
	}
	s, err := OpenSafeStore(root, slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func accessDocument(t *testing.T, password string) Document {
	t.Helper()
	// Serialize the fixture through the parser so quote/escape behavior remains
	// exercised by the safe-store round trip.
	data := "# unrelated comment\nAPP_SETTING='leave # me alone'\nHERO_TEST_USERS=operator\nHERO_TEST_USER_OPERATOR_LOGIN='synthetic-login'\nHERO_TEST_USER_OPERATOR_PASSWORD='" + password + "'\nHERO_TEST_USER_OPERATOR_PROFILE=operator\n"
	doc, err := ParseDotenv([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestSafeStoreAtomicIgnoreAndSnapshot(t *testing.T) {
	root := accessProject(t)
	originalIgnore := "# custom ignore\nbuild/\n!/.env.hero"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(originalIgnore), 0o644); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	s := openAccessStore(t, root, &logs)
	draft, err := s.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	draft.Document = accessDocument(t, "SENTINEL_SAFE_STORE_1")
	saved, err := s.Save(context.Background(), draft)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, ".env.hero"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions: %v", err)
	}
	ignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil || string(ignore) != originalIgnore+"\n/.env.hero.tmp-*\n/.env.hero\n" {
		t.Fatal("ignore content was not preserved")
	}
	snapshot, err := s.Snapshot(context.Background(), "operator")
	if err != nil {
		t.Fatal(err)
	}
	saved.Document = accessDocument(t, "SENTINEL_SAFE_STORE_2")
	if _, err := s.Save(context.Background(), saved); err != nil {
		t.Fatal(err)
	}
	ignoreAgain, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil || !bytes.Equal(ignore, ignoreAgain) {
		t.Fatal("ignore insertion was not idempotent")
	}
	if snapshot.Account().Password() != "SENTINEL_SAFE_STORE_1" {
		t.Fatal("running snapshot changed")
	}
	next, err := s.Snapshot(context.Background(), "operator")
	if err != nil || next.Account().Password() != "SENTINEL_SAFE_STORE_2" {
		t.Fatal("explicit retry did not reread")
	}
	data, err := os.ReadFile(filepath.Join(root, ".env.hero"))
	if err != nil || !strings.Contains(string(data), "# unrelated comment") || !strings.Contains(string(data), "APP_SETTING") {
		t.Fatal("unrelated entries lost")
	}
	for _, value := range []any{draft, snapshot, next} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		for _, diagnostic := range []string{fmt.Sprintf("%+v %#v %s", value, value, value), string(encoded), logs.String()} {
			if strings.Contains(diagnostic, "SENTINEL_SAFE_STORE") || strings.Contains(diagnostic, "synthetic-login") {
				t.Fatal("secret or login leaked to diagnostics")
			}
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") || strings.Contains(entry.Name(), "backup") {
			t.Fatalf("residual file: %s", entry.Name())
		}
	}
}

func TestStaleDraftRequiresReload(t *testing.T) {
	root := accessProject(t)
	s := openAccessStore(t, root, nil)
	draft, err := s.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	draft.Document = accessDocument(t, "first")
	saved, err := s.Save(context.Background(), draft)
	if err != nil {
		t.Fatal(err)
	}
	external, err := SerializeDotenv(accessDocument(t, "external"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env.hero"), external, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(context.Background(), saved); !errors.Is(err, ErrStaleDraft) || !strings.Contains(err.Error(), "Reload") {
		t.Fatalf("stale save: %v", err)
	}
	current, err := os.ReadFile(filepath.Join(root, ".env.hero"))
	if err != nil || !bytes.Equal(current, external) {
		t.Fatal("external edit overwritten")
	}
	reloaded, err := s.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(context.Background(), reloaded); err != nil {
		t.Fatal(err)
	}
}

func TestTrackedCredentialsBlockWithoutMutation(t *testing.T) {
	root := accessProject(t)
	data, err := SerializeDotenv(accessDocument(t, "synthetic"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env.hero"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", root, "add", "--", ".env.hero").Run(); err != nil {
		t.Fatal(err)
	}
	s := openAccessStore(t, root, nil)
	if _, err := s.Load(context.Background()); !errors.Is(err, ErrTrackedCredentials) {
		t.Fatalf("tracked load: %v", err)
	}
	if _, err := s.Save(context.Background(), Draft{}); !errors.Is(err, ErrTrackedCredentials) {
		t.Fatalf("tracked save: %v", err)
	}
	if err := exec.Command("git", "-C", root, "ls-files", "--error-unmatch", ".env.hero").Run(); err != nil {
		t.Fatal("Hero untracked credentials")
	}
	current, err := os.ReadFile(filepath.Join(root, ".env.hero"))
	if err != nil || !bytes.Equal(current, data) {
		t.Fatal("tracked credentials changed")
	}
}

func TestSafeStoreManualValidation(t *testing.T) {
	for _, test := range []struct {
		name, data, ignore string
		mode               os.FileMode
	}{
		{"not-ignored", "HERO_TEST_USERS=operator\n", "", 0o600},
		{"bad-mode", "HERO_TEST_USERS=operator\n", "/.env.hero\n", 0o644},
		{"bad-parse", "HERO_TEST_USER_OPERATOR_PASSWORD=\"SENTINEL_BAD_PARSE\\q\"\n", "/.env.hero\n", 0o600},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := accessProject(t)
			if err := os.WriteFile(filepath.Join(root, ".env.hero"), []byte(test.data), test.mode); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(test.ignore), 0o644); err != nil {
				t.Fatal(err)
			}
			s := openAccessStore(t, root, nil)
			_, err := s.Load(context.Background())
			if err == nil {
				t.Fatal("unsafe manual file accepted")
			}
			if strings.Contains(err.Error(), "SENTINEL_BAD_PARSE") {
				t.Fatal("parser exposed a value")
			}
		})
	}
}

func TestUnsafeCredentialPaths(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			root := accessProject(t)
			target := filepath.Join(t.TempDir(), "private")
			if err := os.WriteFile(target, []byte("SENTINEL_UNTOUCHED"), 0o600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, ".env.hero")
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(target, path)
			case "hardlink":
				err = os.Link(target, path)
			case "directory":
				err = os.Mkdir(path, 0o700)
			}
			if err != nil {
				t.Fatal(err)
			}
			s := openAccessStore(t, root, nil)
			if _, err := s.Load(context.Background()); err == nil {
				t.Fatal("unsafe file accepted")
			}
			if _, err := s.Save(context.Background(), Draft{}); err == nil {
				t.Fatal("unsafe file replaced")
			}
			data, err := os.ReadFile(target)
			if err != nil || string(data) != "SENTINEL_UNTOUCHED" {
				t.Fatal("unsafe target changed")
			}
		})
	}
}

func TestSafeStoreCompetingOwnership(t *testing.T) {
	root := accessProject(t)
	first := openAccessStore(t, root, nil)
	if _, err := OpenSafeStore(root, nil); !errors.Is(err, ErrStoreBusy) {
		t.Fatalf("competing owner: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second := openAccessStore(t, root, nil)
	if _, err := second.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestUnsafeStoreRootAndIgnore(t *testing.T) {
	root := accessProject(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenSafeStore(alias, nil); err == nil {
		t.Fatal("symlink root accepted")
	}
	if _, err := OpenSafeStore(".", nil); err == nil {
		t.Fatal("relative root accepted")
	}
	s := openAccessStore(t, root, nil)
	draft, err := s.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	draft.Document = accessDocument(t, "synthetic")
	target := filepath.Join(t.TempDir(), "ignore")
	if err := os.WriteFile(target, []byte("unchanged"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(context.Background(), draft); err == nil {
		t.Fatal("symlink ignore accepted")
	}
	if _, err := os.Stat(filepath.Join(root, ".env.hero")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("credentials written before ignore safety")
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "unchanged" {
		t.Fatal("external ignore changed")
	}
}

func TestSafeStoreCancelledSaveLeavesNoPlaintext(t *testing.T) {
	root := accessProject(t)
	s := openAccessStore(t, root, nil)
	draft, err := s.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	draft.Document = accessDocument(t, "synthetic")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Save(ctx, draft); err == nil {
		t.Fatal("cancelled save accepted")
	}
	if _, err := os.Stat(filepath.Join(root, ".env.hero")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancelled save created credentials")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatal("cancel left temporary plaintext")
		}
	}
}

func TestSafeStoreForeignDraftAndClosedOwner(t *testing.T) {
	first := openAccessStore(t, accessProject(t), nil)
	second := openAccessStore(t, accessProject(t), nil)
	draft, err := first.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	draft.Document = accessDocument(t, "synthetic")
	if _, err := second.Save(context.Background(), draft); err == nil {
		t.Fatal("foreign draft accepted")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Save(context.Background(), draft); err == nil {
		t.Fatal("closed store accepted save")
	}
}

func TestSafeStoreEmptyIgnoreIsIdempotent(t *testing.T) {
	root := accessProject(t)
	s := openAccessStore(t, root, nil)
	draft, err := s.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	draft.Document = accessDocument(t, "synthetic")
	draft, err = s.Save(context.Background(), draft)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(context.Background(), draft); err != nil {
		t.Fatal(err)
	}
	ignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(ignore) != "/.env.hero.tmp-*\n/.env.hero\n" {
		t.Fatal("empty ignore insertion duplicated rules")
	}
}

func TestSafeStoreInterruptedTemporaryCleanup(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(fmt.Sprint(linked), func(t *testing.T) {
			root := accessProject(t)
			temporary := filepath.Join(root, ".env.hero.tmp-0123456789abcdef0123456789abcdef")
			if err := os.WriteFile(temporary, []byte("synthetic"), 0o600); err != nil {
				t.Fatal(err)
			}
			if linked {
				if err := os.Link(temporary, filepath.Join(root, ".env.hero")); err != nil {
					t.Fatal(err)
				}
			}
			_ = openAccessStore(t, root, nil)
			if _, err := os.Lstat(temporary); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("interrupted plaintext temp retained")
			}
			if linked {
				data, err := os.ReadFile(filepath.Join(root, ".env.hero"))
				if err != nil || string(data) != "synthetic" {
					t.Fatal("committed credentials removed")
				}
			}
		})
	}
}

func TestSafeStoreAtomicFailureAndCreateRace(t *testing.T) {
	root := accessProject(t)
	s := openAccessStore(t, root, nil)
	if err := os.Mkdir(filepath.Join(root, ".env.hero"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := s.replace(".env.hero", []byte("synthetic"), 0o600, nil, false); err == nil {
		t.Fatal("rename over directory succeeded")
	}
	if err := os.Remove(filepath.Join(root, ".env.hero")); err != nil {
		t.Fatal(err)
	}
	err := s.replace(".env.hero", []byte("synthetic"), 0o600, func() error {
		return os.WriteFile(filepath.Join(root, ".env.hero"), []byte("external"), 0o600)
	}, true)
	if !errors.Is(err, ErrStaleDraft) {
		t.Fatalf("create race: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".env.hero"))
	if err != nil || string(data) != "external" {
		t.Fatal("concurrent manual creation overwritten")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatal("failure left plaintext temp")
		}
	}
}

func TestUnsafeOwnershipAndSpecialMode(t *testing.T) {
	root := accessProject(t)
	path := filepath.Join(root, ".env.hero")
	if err := os.WriteFile(path, []byte("HERO_TEST_USERS=\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600|os.ModeSetuid); err != nil {
		t.Fatal(err)
	}
	s := openAccessStore(t, root, nil)
	if _, err := s.Load(context.Background()); err == nil {
		t.Fatal("special mode accepted")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		if err := os.Chown(path, 12345, -1); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Load(context.Background()); err == nil {
			t.Fatal("foreign owner accepted")
		}
	}
}
