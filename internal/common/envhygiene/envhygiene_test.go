package envhygiene_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/envhygiene"
)

func TestEnsureProjectRoot_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	if err := envhygiene.EnsureProjectRoot(dir, assets.FS); err != nil {
		t.Fatalf("EnsureProjectRoot: %v", err)
	}

	envEx, err := os.ReadFile(filepath.Join(dir, ".env.example"))
	if err != nil {
		t.Fatalf(".env.example missing: %v", err)
	}
	if !strings.Contains(string(envEx), "never commit") {
		t.Errorf(".env.example missing guidance text")
	}

	gi, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore missing: %v", err)
	}
	content := string(gi)
	if !strings.Contains(content, envhygiene.MarkerBegin) {
		t.Error(".gitignore missing Hero marker")
	}
	if !strings.Contains(content, ".env") {
		t.Error(".gitignore missing .env")
	}
	if !strings.Contains(content, "!.env.example") {
		t.Error(".gitignore missing !.env.example exception")
	}
}

func TestEnsureProjectRoot_PreservesExistingEnvExample(t *testing.T) {
	dir := t.TempDir()
	custom := []byte("# custom example\nFOO=\n")
	if err := os.WriteFile(filepath.Join(dir, ".env.example"), custom, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := envhygiene.EnsureProjectRoot(dir, assets.FS); err != nil {
		t.Fatalf("EnsureProjectRoot: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, ".env.example"))
	if string(got) != string(custom) {
		t.Errorf(".env.example overwritten: got %q", got)
	}
}

func TestEnsureProjectRoot_SkipsAppendWhenEnvAlreadyIgnored(t *testing.T) {
	dir := t.TempDir()
	existing := "# project\n.env\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := envhygiene.EnsureProjectRoot(dir, assets.FS); err != nil {
		t.Fatalf("EnsureProjectRoot: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if strings.Contains(string(got), envhygiene.MarkerBegin) {
		t.Error("should not append Hero block when .env already ignored")
	}
	if string(got) != existing {
		t.Errorf("gitignore changed unexpectedly: %q", got)
	}
}

func TestEnsureProjectRoot_AppendsWhenMissingEnvIgnore(t *testing.T) {
	dir := t.TempDir()
	existing := "node_modules/\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := envhygiene.EnsureProjectRoot(dir, assets.FS); err != nil {
		t.Fatalf("EnsureProjectRoot: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(got), "node_modules/") {
		t.Error("lost existing gitignore content")
	}
	if !strings.Contains(string(got), envhygiene.MarkerBegin) {
		t.Error("expected Hero secrets block append")
	}
}

func TestIsSensitivePath(t *testing.T) {
	cases := map[string]bool{
		".env":              true,
		".env.local":        true,
		".env.production":   true,
		".env.example":      false,
		"config/.env":       true,
		"credentials.json":  true,
		"secrets.json":      true,
		"cert.pem":          true,
		"README.md":         false,
		"src/main.go":       false,
	}
	for path, want := range cases {
		if got := envhygiene.IsSensitivePath(path); got != want {
			t.Errorf("IsSensitivePath(%q)=%v, want %v", path, got, want)
		}
	}
}

func TestTrackedSensitiveFiles(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env.example"), []byte("SECRET=\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	add := exec.Command("git", "-C", dir, "add", "-f", ".env", ".env.example")
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	found, err := envhygiene.TrackedSensitiveFiles(dir)
	if err != nil {
		t.Fatalf("TrackedSensitiveFiles: %v", err)
	}
	if len(found) != 1 || found[0] != ".env" {
		t.Errorf("TrackedSensitiveFiles = %v, want [.env]", found)
	}
}

func TestEnsureProjectRoot_MissingTemplateErrors(t *testing.T) {
	dir := t.TempDir()
	emptyFS := fstest.MapFS{}
	if err := envhygiene.EnsureProjectRoot(dir, emptyFS); err == nil {
		t.Error("expected error when templates missing from FS")
	}
}
