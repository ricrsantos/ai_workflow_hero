package testaccess

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveCleanupExactPathAndIdempotence(t *testing.T) {
	root := accessProject(t)
	s := openAccessStore(t, root, nil)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/.env.hero\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".env.hero", ".env", ".env.hero.example"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("synthetic-fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RemoveForArchive(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveForArchive(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".env.hero")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("credential file was not removed")
	}
	for _, name := range []string{".env", ".env.hero.example", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatal("unrelated root file removed")
		}
	}
}
