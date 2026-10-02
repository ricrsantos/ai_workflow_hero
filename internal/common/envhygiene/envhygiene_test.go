package envhygiene_test

import (
	"io/fs"
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
	heroExample, err := os.ReadFile(filepath.Join(dir, envhygiene.EnvHeroExamplePath))
	if err != nil {
		t.Fatalf(".env.hero.example missing: %v", err)
	}
	for _, placeholder := range []string{
		"HERO_TEST_USERS=operator,administrator",
		"HERO_TEST_USER_OPERATOR_LOGIN=\"\"",
		"HERO_TEST_USER_OPERATOR_PASSWORD=\"\"",
		"HERO_TEST_USER_ADMINISTRATOR_LOGIN=\"\"",
		"HERO_TEST_USER_ADMINISTRATOR_PASSWORD=\"\"",
	} {
		if !strings.Contains(string(heroExample), placeholder) {
			t.Errorf(".env.hero.example missing placeholder %q", placeholder)
		}
	}
	if _, err := os.Lstat(filepath.Join(dir, ".env.hero")); !os.IsNotExist(err) {
		t.Fatalf("EnsureProjectRoot must not create .env.hero, stat err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, ".env")); !os.IsNotExist(err) {
		t.Fatalf("EnsureProjectRoot must not create application .env, stat err=%v", err)
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
	if !strings.Contains(content, "!.env.hero.example") {
		t.Error(".gitignore missing !.env.hero.example exception")
	}
	if !strings.Contains(content, envhygiene.TUILogGitignorePath) {
		t.Error(".gitignore missing TUI log path")
	}
}

func TestEnsureProjectRoot_PreservesCustomEnvHeroExampleAndApplicationEnv(t *testing.T) {
	dir := t.TempDir()
	example := []byte("# custom test account documentation\nHERO_TEST_USERS=local-users\n")
	applicationEnv := []byte("DATABASE_URL=application-local-value\n")
	if err := os.WriteFile(filepath.Join(dir, envhygiene.EnvHeroExamplePath), example, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), applicationEnv, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := envhygiene.EnsureProjectRoot(dir, assets.FS); err != nil {
		t.Fatalf("EnsureProjectRoot: %v", err)
	}
	gotExample, err := os.ReadFile(filepath.Join(dir, envhygiene.EnvHeroExamplePath))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotExample) != string(example) {
		t.Fatalf("custom .env.hero.example changed: %q", gotExample)
	}
	gotEnv, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotEnv) != string(applicationEnv) {
		t.Fatalf("application .env changed: %q", gotEnv)
	}
}

func TestEnvHeroExampleTemplateContainsOnlyEmptyCredentials(t *testing.T) {
	body, err := fs.ReadFile(assets.FS, "templates/env.hero.example")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"HERO_TEST_USERS":                       "operator,administrator",
		"HERO_TEST_USER_OPERATOR_LOGIN":         `""`,
		"HERO_TEST_USER_OPERATOR_PASSWORD":      `""`,
		"HERO_TEST_USER_OPERATOR_PROFILE":       "operator",
		"HERO_TEST_USER_ADMINISTRATOR_LOGIN":    `""`,
		"HERO_TEST_USER_ADMINISTRATOR_PASSWORD": `""`,
		"HERO_TEST_USER_ADMINISTRATOR_PROFILE":  "admin",
	}
	seen := make(map[string]string)
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("malformed placeholder assignment %q", line)
		}
		if _, duplicate := seen[key]; duplicate {
			t.Fatalf("duplicate placeholder assignment %q", key)
		}
		seen[key] = value
	}
	if len(seen) != len(want) {
		t.Fatalf("template has %d assignments, want %d", len(seen), len(want))
	}
	for key, value := range want {
		got, ok := seen[key]
		if !ok || got != value {
			t.Errorf("template %s = %q, want %q", key, got, value)
		}
	}
}

func TestEnsureProjectRoot_RefusesSymlinkedEnvHeroExample(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "custom-example.txt")
	contents := []byte("leave this target unchanged\n")
	if err := os.WriteFile(target, contents, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, envhygiene.EnvHeroExamplePath)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if err := envhygiene.EnsureProjectRoot(dir, assets.FS); err == nil || !strings.Contains(err.Error(), "unsafe .env.hero.example") {
		t.Fatalf("err=%v, want unsafe symlink refusal", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(contents) {
		t.Fatalf("symlink target changed: %q", got)
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
	content := string(got)
	if strings.Contains(content, envhygiene.MarkerBegin) {
		t.Error("should not append Hero secrets block when .env already ignored")
	}
	if !strings.Contains(content, envhygiene.TUILogGitignorePath) {
		t.Error("expected TUI log ignore when .env already ignored")
	}
	if !strings.HasPrefix(content, existing) {
		t.Errorf("gitignore changed unexpectedly at start: %q", got)
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

func TestEnsureProjectRoot_PatchesExistingHeroBlockWithTUILog(t *testing.T) {
	dir := t.TempDir()
	existing := envhygiene.MarkerBegin + "\n.env\n" + envhygiene.MarkerEnd + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := envhygiene.EnsureProjectRoot(dir, assets.FS); err != nil {
		t.Fatalf("EnsureProjectRoot: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	content := string(got)
	if !strings.Contains(content, envhygiene.TUILogGitignorePath) {
		t.Error("expected TUI log inserted into existing Hero block")
	}
	tuiIdx := strings.Index(content, envhygiene.TUILogGitignorePath)
	endIdx := strings.Index(content, envhygiene.MarkerEnd)
	if tuiIdx == -1 || endIdx == -1 || tuiIdx > endIdx {
		t.Errorf("TUI log should appear before Hero block end: %q", content)
	}
}

func TestGitignoreIgnoresTUILog(t *testing.T) {
	cases := map[string]bool{
		".workflow-hero/tui.log\n":            true,
		"# comment\n.workflow-hero/tui.log\n": true,
		".workflow-hero/*.log\n":              true,
		"node_modules/\n":                     false,
	}
	for content, want := range cases {
		if got := envhygiene.GitignoreIgnoresTUILog(content); got != want {
			t.Errorf("GitignoreIgnoresTUILog(%q)=%v, want %v", content, got, want)
		}
	}
}

func TestGitignoreIgnoresLogs(t *testing.T) {
	cases := map[string]bool{
		".workflow-hero/logs/\n":          true,
		".workflow-hero/logs\n":           true,
		"**/.workflow-hero/logs/\n":       true,
		"# comment\n.workflow-hero/logs/": true,
		"node_modules/\n":                 false,
		".workflow-hero/tui.log\n":        false,
	}
	for content, want := range cases {
		if got := envhygiene.GitignoreIgnoresLogs(content); got != want {
			t.Errorf("GitignoreIgnoresLogs(%q)=%v, want %v", content, got, want)
		}
	}
}

func TestEnsureProjectRoot_AddsLogsDirToFreshGitignore(t *testing.T) {
	dir := t.TempDir()
	if err := envhygiene.EnsureProjectRoot(dir, assets.FS); err != nil {
		t.Fatalf("EnsureProjectRoot: %v", err)
	}
	gi, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore missing: %v", err)
	}
	if !strings.Contains(string(gi), envhygiene.LogsDirGitignorePath) {
		t.Errorf(".gitignore missing rotating logs dir path")
	}
}

func TestEnsureProjectRoot_InsertsLogsDirIntoExistingHeroBlock(t *testing.T) {
	dir := t.TempDir()
	existing := envhygiene.MarkerBegin + "\n.env\n" + envhygiene.TUILogGitignorePath + "\n" + envhygiene.MarkerEnd + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := envhygiene.EnsureProjectRoot(dir, assets.FS); err != nil {
		t.Fatalf("EnsureProjectRoot: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	content := string(got)
	if !strings.Contains(content, envhygiene.LogsDirGitignorePath) {
		t.Error("expected logs dir inserted into existing Hero block")
	}
	logsIdx := strings.Index(content, envhygiene.LogsDirGitignorePath)
	endIdx := strings.Index(content, envhygiene.MarkerEnd)
	if logsIdx == -1 || endIdx == -1 || logsIdx > endIdx {
		t.Errorf("logs dir should appear before Hero block end: %q", content)
	}
	// User-owned content outside the managed block is preserved verbatim.
	if !strings.HasPrefix(content, envhygiene.MarkerBegin) {
		t.Errorf("block start changed: %q", content)
	}
}

func TestIsSensitivePath(t *testing.T) {
	cases := map[string]bool{
		".env":             true,
		".env.local":       true,
		".env.production":  true,
		".env.example":     false,
		"config/.env":      true,
		"credentials.json": true,
		"secrets.json":     true,
		"cert.pem":         true,
		"README.md":        false,
		"src/main.go":      false,
	}
	for path, want := range cases {
		if got := envhygiene.IsSensitivePath(path); got != want {
			t.Errorf("IsSensitivePath(%q)=%v, want %v", path, got, want)
		}
	}
}

func TestHasParentTraversal(t *testing.T) {
	allowed := []string{
		"internal/tui/stage_handoff.go",
		"go test ./internal/tui",
		"go test ./...",
		"go test ./src/api/...",
		"./...",
		`go test "./src/api/..."`,
	}
	for _, p := range allowed {
		if envhygiene.HasParentTraversal(p) {
			t.Errorf("HasParentTraversal(%q)=true, want false", p)
		}
	}
	rejected := []string{
		"../secret.txt",
		"foo/../bar.go",
		`foo\..\bar.go`,
		"go test ../pkg",
		"..",
	}
	for _, p := range rejected {
		if !envhygiene.HasParentTraversal(p) {
			t.Errorf("HasParentTraversal(%q)=false, want true", p)
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
	if err := os.WriteFile(filepath.Join(dir, envhygiene.EnvHeroExamplePath), []byte("HERO_TEST_USER_OPERATOR_PASSWORD=\"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	add := exec.Command("git", "-C", dir, "add", "-f", ".env", ".env.example", envhygiene.EnvHeroExamplePath)
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

func TestIsSensitivePath_AllowsOnlyRootHeroPlaceholder(t *testing.T) {
	if envhygiene.IsSensitivePath(envhygiene.EnvHeroExamplePath) {
		t.Fatal("root .env.hero.example placeholder should be an allowed tracked exception")
	}
	if !envhygiene.IsSensitivePath("nested/" + envhygiene.EnvHeroExamplePath) {
		t.Fatal("nested .env.hero.example should remain sensitive")
	}
}

func TestEnsureProjectRoot_MissingTemplateErrors(t *testing.T) {
	dir := t.TempDir()
	emptyFS := fstest.MapFS{}
	if err := envhygiene.EnsureProjectRoot(dir, emptyFS); err == nil {
		t.Error("expected error when templates missing from FS")
	}
}
