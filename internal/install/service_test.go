package install_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func makeGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	return dir
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func TestRun_BasicInstall(t *testing.T) {
	dir := makeGitRepo(t)

	opts := install.Options{
		ProjectDir: dir,
		Name:       "Test Project",
		Summary:    "A test project",
		Tools:      []string{"cursor"},
		Version:    "1.0.0",
		GitInit:    false,
		AssetsFS:   assets.FS,
	}

	var out strings.Builder
	if err := install.Run(opts, &out, &out); err != nil {
		t.Fatalf("install.Run failed: %v", err)
	}

	// Verify key directories were created.
	expectDirs := []string{
		cursoradapter.CommandsDir,
		cursoradapter.AgentsDir,
		cursoradapter.WorkflowHeroSkillDir,
		cursoradapter.GrillingSkillDir,
		cursoradapter.HeroConfigDir,
		cursoradapter.HeroTemplatesDir,
		cursoradapter.HeroModelsDir,
		cursoradapter.HeroDocsDir,
		cursoradapter.HeroCurrentCycleDir,
	}
	for _, d := range expectDirs {
		full := filepath.Join(dir, d)
		info, err := os.Stat(full)
		if err != nil {
			t.Errorf("expected dir %s to exist: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", d)
		}
	}

	// Verify hero.json.
	heroData, err := os.ReadFile(filepath.Join(dir, cursoradapter.HeroJSONPath))
	if err != nil {
		t.Fatalf("hero.json not found: %v", err)
	}
	var heroJSON install.HeroJSON
	if err := json.Unmarshal(heroData, &heroJSON); err != nil {
		t.Fatalf("invalid hero.json: %v", err)
	}
	if heroJSON.CLI.Version != "1.0.0" {
		t.Errorf("hero.json cli.version = %q, want %q", heroJSON.CLI.Version, "1.0.0")
	}
	if heroJSON.Assets.Version != "1.0.0" {
		t.Errorf("hero.json assets.version = %q, want %q", heroJSON.Assets.Version, "1.0.0")
	}
	if len(heroJSON.CLI.Tools) == 0 || heroJSON.CLI.Tools[0] != "cursor" {
		t.Errorf("hero.json cli.tools = %v, want [cursor]", heroJSON.CLI.Tools)
	}
	cfg, ok := heroJSON.Harnesses["cursor"]
	if !ok {
		t.Fatal("hero.json missing harnesses.cursor")
	}
	if cfg.Model != "" || cfg.EnableFastModel {
		t.Errorf("harnesses.cursor = %+v, want empty model and enable_fast_model=false", cfg)
	}

	// Verify project.json.
	projectData, err := os.ReadFile(filepath.Join(dir, cursoradapter.ProjectJSONPath))
	if err != nil {
		t.Fatalf("project.json not found: %v", err)
	}
	var projectJSON install.ProjectJSON
	if err := json.Unmarshal(projectData, &projectJSON); err != nil {
		t.Fatalf("invalid project.json: %v", err)
	}
	if projectJSON.Name != "Test Project" {
		t.Errorf("project.json name = %q, want %q", projectJSON.Name, "Test Project")
	}
	if projectJSON.Summary != "A test project" {
		t.Errorf("project.json summary = %q, want %q", projectJSON.Summary, "A test project")
	}

	// Verify checksums.json exists and is valid JSON with entries.
	checksumsData, err := os.ReadFile(filepath.Join(dir, cursoradapter.ChecksumsJSONPath))
	if err != nil {
		t.Fatalf("checksums.json not found: %v", err)
	}
	var checksums install.Checksums
	if err := json.Unmarshal(checksumsData, &checksums); err != nil {
		t.Fatalf("invalid checksums.json: %v", err)
	}
	if len(checksums) == 0 {
		t.Error("checksums.json is empty — expected at least one entry")
	}

	// Verify command files were installed.
	for _, cmdFile := range cursoradapter.RequiredCommandFiles() {
		if _, err := os.Stat(filepath.Join(dir, cmdFile)); err != nil {
			t.Errorf("expected command file %s to exist: %v", cmdFile, err)
		}
	}

	// Verify agent files were installed.
	for _, agentFile := range cursoradapter.RequiredAgentFiles() {
		if _, err := os.Stat(filepath.Join(dir, agentFile)); err != nil {
			t.Errorf("expected agent file %s to exist: %v", agentFile, err)
		}
	}

	// Verify metrics-summary.md.
	if _, err := os.Stat(filepath.Join(dir, cursoradapter.MetricsSummaryPath)); err != nil {
		t.Errorf("metrics-summary.md not found: %v", err)
	}

	// Verify end-user guide was installed.
	helpData, err := os.ReadFile(filepath.Join(dir, cursoradapter.WorkflowHelpPath))
	if err != nil {
		t.Fatalf("workflow-help.md not found: %v", err)
	}
	helpStr := string(helpData)
	for _, kw := range []string{"Philosophy", "hero install", "workflow-config.yml", "/hero-start", "Logging"} {
		if !strings.Contains(helpStr, kw) {
			t.Errorf("workflow-help.md missing %q", kw)
		}
	}

	// Soft secrets hygiene at project root.
	if _, err := os.Stat(filepath.Join(dir, ".env.example")); err != nil {
		t.Errorf(".env.example missing after install: %v", err)
	}
	gi, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore missing after install: %v", err)
	}
	if !strings.Contains(string(gi), ".env") {
		t.Error(".gitignore missing .env after install")
	}

	// Operational store (hero.db) created on install.
	if _, err := os.Stat(filepath.Join(dir, store.RelativeDBPath)); err != nil {
		t.Errorf("hero.db missing after install: %v", err)
	}
	s, err := store.OpenProject(dir)
	if err != nil {
		t.Fatalf("OpenProject after install: %v", err)
	}
	defer s.Close()
	cycles, err := s.ListCycles()
	if err != nil {
		t.Fatalf("ListCycles: %v", err)
	}
	if len(cycles) != 0 {
		t.Errorf("fresh install should have no cycles, got %d", len(cycles))
	}
}

func TestRun_NoGitRepo_WithGitInit(t *testing.T) {
	dir := t.TempDir()

	opts := install.Options{
		ProjectDir: dir,
		Name:       "No Git Project",
		Summary:    "Testing git init",
		Tools:      []string{"cursor"},
		Version:    "dev",
		GitInit:    true,
		AssetsFS:   assets.FS,
	}

	var out strings.Builder
	if err := install.Run(opts, &out, &out); err != nil {
		t.Fatalf("install.Run with GitInit=true failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Error(".git directory was not created by git init")
	}
}

func TestRun_NoGitRepo_WithoutGitInit_Fails(t *testing.T) {
	dir := t.TempDir()

	opts := install.Options{
		ProjectDir: dir,
		Name:       "No Git Project",
		Summary:    "Should fail",
		Tools:      []string{"cursor"},
		Version:    "dev",
		GitInit:    false,
		AssetsFS:   assets.FS,
	}

	var out strings.Builder
	err := install.Run(opts, &out, &out)
	if err == nil {
		t.Fatal("expected error when no git repo and GitInit=false")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_ChecksumIsAccurate(t *testing.T) {
	dir := makeGitRepo(t)

	opts := install.Options{
		ProjectDir: dir,
		Name:       "Checksum Project",
		Summary:    "Test checksums",
		Tools:      []string{"cursor"},
		Version:    "1.0.0",
		GitInit:    false,
		AssetsFS:   assets.FS,
	}

	var out strings.Builder
	if err := install.Run(opts, &out, &out); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	// Load checksums and verify one file's checksum matches.
	checksumsData, _ := os.ReadFile(filepath.Join(dir, cursoradapter.ChecksumsJSONPath))
	var checksums install.Checksums
	_ = json.Unmarshal(checksumsData, &checksums)

	// Pick backend_agent.md and verify its checksum.
	targetRel := filepath.Join(cursoradapter.AgentsDir, "backend_agent.md")
	storedHash, ok := checksums[targetRel]
	if !ok {
		t.Fatalf("checksum for %q not found; available keys: %v", targetRel, checksumKeys(checksums))
	}

	actualData, _ := os.ReadFile(filepath.Join(dir, targetRel))
	actualHash := sha256hex(actualData)
	if actualHash != storedHash {
		t.Errorf("checksum mismatch for %s: stored=%s, actual=%s", targetRel, storedHash, actualHash)
	}
}

func checksumKeys(m install.Checksums) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestRun_UnsupportedHarnessMarker_Warns(t *testing.T) {
	dir := makeGitRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".windsurf"), 0o755); err != nil {
		t.Fatal(err)
	}

	opts := install.Options{
		ProjectDir: dir,
		Name:       "Marker Project",
		Summary:    "test harness markers",
		Tools:      []string{"cursor"},
		Version:    "1.0.0",
		GitInit:    false,
		AssetsFS:   assets.FS,
	}

	var out, errOut strings.Builder
	if err := install.Run(opts, &out, &errOut); err != nil {
		t.Fatalf("install.Run failed: %v", err)
	}

	stderr := errOut.String()
	if !strings.Contains(stderr, "Detected .windsurf/ but cli.tools does not include it") {
		t.Errorf("stderr missing windsurf warning: %q", stderr)
	}
	if !strings.Contains(stderr, "Supported today: cursor") {
		t.Errorf("stderr missing supported-tools hint: %q", stderr)
	}

	// Cursor assets still installed; no windsurf pack materialized.
	if _, err := os.Stat(filepath.Join(dir, ".windsurf", "commands")); err == nil {
		t.Error("expected no windsurf commands directory from Hero install")
	}
	if _, err := os.Stat(filepath.Join(dir, cursoradapter.CommandsDir)); err != nil {
		t.Errorf("cursor commands should be installed: %v", err)
	}
}
