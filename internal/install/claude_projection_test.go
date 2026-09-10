package install_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

func TestProvisionClaude_ProjectsMirroredFamiliesAndChecksums(t *testing.T) {
	dir := t.TempDir()
	checksums := make(install.Checksums)

	if err := install.ProvisionClaude(dir, assets.FS, checksums); err != nil {
		t.Fatalf("ProvisionClaude: %v", err)
	}

	for _, name := range []string{
		"backend_agent.md", "browser_ui_agent.md", "context_agent.md",
		"discover_agent.md", "end2end_qa_agent.md", "frontend_agent.md",
		"generic_agent.md", "judge_agent.md", "orchestration_agent.md",
		"planning_agent.md", "qa_agent.md",
	} {
		path := filepath.Join(dir, ".claude", "agents", name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing Claude agent %s: %v", name, err)
		}
		rel := filepath.ToSlash(filepath.Join(".claude", "agents", name))
		if checksums[rel] == "" {
			t.Fatalf("checksum missing for %s", rel)
		}
	}
	for _, rel := range []string{
		filepath.Join(".claude", "commands", "hero-start.md"),
		filepath.Join(".claude", "skills", "workflow-hero", "SKILL.md"),
		filepath.Join(".claude", "skills", "grilling", "SKILL.md"),
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("missing Claude asset %s: %v", rel, err)
		}
		if checksums[filepath.ToSlash(rel)] == "" {
			t.Fatalf("checksum missing for %s", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("Claude projection must not copy root AGENTS.md")
	}
}

func TestClaudeContext_InsertUpdateLeaveAndRemovePreserveUserText(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Project instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "CLAUDE.md")
	userPrefix := "# User instructions\nKeep this byte-for-byte.\n"
	userSuffix := "\n## Claude-specific notes\nDo not change this either.\n"
	if err := os.WriteFile(path, []byte(userPrefix), 0o600); err != nil {
		t.Fatal(err)
	}

	inserted, err := install.ApplyClaudeContext(dir, install.ClaudeContextInsertOrUpdate)
	if err != nil {
		t.Fatalf("insert context: %v", err)
	}
	if !inserted.Changed || !strings.Contains(inserted.Content, install.ClaudeContextMarkerBegin) || !strings.Contains(inserted.Content, "@AGENTS.md") {
		t.Fatalf("unexpected insert result: %+v", inserted)
	}
	if !strings.Contains(inserted.Content, "`.claude/skills/workflow-hero/`") {
		t.Fatalf("managed block missing Hero skill reference: %q", inserted.Content)
	}
	if !strings.Contains(inserted.Content, "context/current-state.md") || !strings.Contains(inserted.Content, "context/context-log.md") {
		t.Fatalf("managed block missing context-file references: %q", inserted.Content)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("CLAUDE.md mode=%#o, want 0600", got)
	}
	if !strings.HasPrefix(inserted.Content, userPrefix) {
		t.Fatalf("insert changed unmarked user text: %q", inserted.Content)
	}
	withSuffix := inserted.Content + userSuffix
	if err := os.WriteFile(path, []byte(withSuffix), 0o600); err != nil {
		t.Fatal(err)
	}

	unchanged, err := install.ApplyClaudeContext(dir, install.ClaudeContextLeaveUnchanged)
	if err != nil {
		t.Fatalf("leave unchanged: %v", err)
	}
	if unchanged.Changed || unchanged.Content != withSuffix {
		t.Fatal("leave-unchanged changed CLAUDE.md")
	}

	updated, err := install.UpdateClaudeContext(dir)
	if err != nil {
		t.Fatalf("update context: %v", err)
	}
	if updated.Changed || !strings.HasPrefix(updated.Content, userPrefix) || !strings.HasSuffix(updated.Content, userSuffix) {
		t.Fatalf("update changed unmarked user text: %q prefix=%v suffix=%v changed=%v (want prefix %q suffix %q)", updated.Content, strings.HasPrefix(updated.Content, userPrefix), strings.HasSuffix(updated.Content, userSuffix), updated.Changed, userPrefix, userSuffix)
	}

	removed, err := install.RemoveClaudeManagedContext(dir)
	if err != nil {
		t.Fatalf("remove context: %v", err)
	}
	if !removed.Changed {
		t.Fatal("expected managed context removal")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != userPrefix+userSuffix {
		t.Fatalf("unmarked text changed on removal: %q", got)
	}
}

func TestClaudeContext_RequiresAgentsAndRejectsMalformedOrSymlinkedFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := install.ApplyClaudeContext(dir, install.ClaudeContextInsertOrUpdate); err == nil || !strings.Contains(err.Error(), "AGENTS.md") {
		t.Fatalf("missing AGENTS.md error=%v", err)
	}
	path := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	malformed := install.ClaudeContextMarkerBegin + "\nuser text\n" + install.ClaudeContextMarkerBegin + "\n"
	if err := os.WriteFile(path, []byte(malformed), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := install.ApplyClaudeContext(dir, install.ClaudeContextInsertOrUpdate); err == nil {
		t.Fatal("duplicate markers must fail closed")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != malformed {
		t.Fatal("malformed CLAUDE.md was modified")
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "AGENTS.md"), path); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := install.ApplyClaudeContext(dir, install.ClaudeContextInsertOrUpdate); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink error=%v", err)
	}
}

func TestRemoveClaudeProjection_PreservesUserFiles(t *testing.T) {
	dir := t.TempDir()
	checksums := make(install.Checksums)
	if err := install.ProvisionClaude(dir, assets.FS, checksums); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte("{\"user\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude", "commands", "custom.md"), []byte("user command\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := install.WriteChecksums(dir, checksums); err != nil {
		t.Fatal(err)
	}
	if err := install.RemoveClaudeProjection(dir); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(dir, ".claude", "settings.json"),
		filepath.Join(dir, ".claude", "commands", "custom.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("user Claude file removed: %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "commands", "hero-start.md")); !os.IsNotExist(err) {
		t.Fatal("Hero command survived Claude projection removal")
	}
	var found bool
	_ = fs.WalkDir(os.DirFS(filepath.Join(dir, ".claude")), ".", func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, "AGENTS.md") {
			found = true
		}
		return err
	})
	if found {
		t.Fatal("projection must not create a nested AGENTS.md")
	}
}
