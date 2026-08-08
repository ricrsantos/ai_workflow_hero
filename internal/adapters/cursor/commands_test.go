package cursor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverCommandsExcludesHero(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	projCmds := filepath.Join(project, CommandsDir)
	userCmds := filepath.Join(home, CommandsDir)
	_ = os.MkdirAll(projCmds, 0o755)
	_ = os.MkdirAll(userCmds, 0o755)

	_ = os.WriteFile(filepath.Join(projCmds, "hero-approve.md"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(projCmds, "opsx-propose.md"), []byte("---\ndescription: x\n---\nbody"), 0o644)
	_ = os.WriteFile(filepath.Join(userCmds, "my-tool.md"), []byte("user body"), 0o644)

	cmds, err := DiscoverCommands(project, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 2 {
		t.Fatalf("cmds = %+v", cmds)
	}
	byStem := map[string]DiscoveredCommand{}
	for _, c := range cmds {
		byStem[c.Stem] = c
	}
	if byStem["opsx-propose"].Label != "/opsx-propose" || byStem["opsx-propose"].Source != CommandSourceProject {
		t.Fatalf("opsx: %+v", byStem["opsx-propose"])
	}
	if byStem["my-tool"].Source != CommandSourceUser {
		t.Fatalf("my-tool: %+v", byStem["my-tool"])
	}
	if _, ok := byStem["hero-approve"]; ok {
		t.Fatal("hero-approve should be excluded")
	}
}

func TestStripFrontmatter(t *testing.T) {
	in := "---\ndescription: hi\n---\n# Title\n\nDo the thing.\n"
	got := StripFrontmatter(in)
	want := "# Title\n\nDo the thing.\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if StripFrontmatter("no frontmatter") != "no frontmatter" {
		t.Fatal("passthrough failed")
	}
}

func TestReadCommandPrompt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.md")
	if err := os.WriteFile(path, []byte("---\na: 1\n---\nprompt body"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := ReadCommandPrompt(path)
	if err != nil || body != "prompt body" {
		t.Fatalf("body=%q err=%v", body, err)
	}
}

func TestDiscoverCommandsProjectWinsOverUser(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	projCmds := filepath.Join(project, CommandsDir)
	userCmds := filepath.Join(home, CommandsDir)
	_ = os.MkdirAll(projCmds, 0o755)
	_ = os.MkdirAll(userCmds, 0o755)

	_ = os.WriteFile(filepath.Join(projCmds, "shared-cmd.md"), []byte("project body"), 0o644)
	_ = os.WriteFile(filepath.Join(userCmds, "shared-cmd.md"), []byte("user body"), 0o644)

	cmds, err := DiscoverCommands(project, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 1 {
		t.Fatalf("cmds = %+v", cmds)
	}
	if cmds[0].Source != CommandSourceProject {
		t.Fatalf("expected project source, got %+v", cmds[0])
	}
}

func TestDiscoverCommandsMissingDirs(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()

	cmds, err := DiscoverCommands(project, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 0 {
		t.Fatalf("expected empty list, got %+v", cmds)
	}
}

func TestDiscoverCommandsIgnoresNonMdAndDirs(t *testing.T) {
	project := t.TempDir()
	projCmds := filepath.Join(project, CommandsDir)
	_ = os.MkdirAll(projCmds, 0o755)
	_ = os.MkdirAll(filepath.Join(projCmds, "nested"), 0o755)
	_ = os.WriteFile(filepath.Join(projCmds, "readme.txt"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(projCmds, "valid.md"), []byte("ok"), 0o644)

	cmds, err := DiscoverCommands(project, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 1 || cmds[0].Stem != "valid" {
		t.Fatalf("cmds = %+v", cmds)
	}
}

func TestDiscoverCommandsHeroPattern(t *testing.T) {
	project := t.TempDir()
	projCmds := filepath.Join(project, CommandsDir)
	_ = os.MkdirAll(projCmds, 0o755)
	_ = os.WriteFile(filepath.Join(projCmds, "hero-approve.md"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(projCmds, "herox.md"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(projCmds, "custom.md"), []byte("x"), 0o644)

	cmds, err := DiscoverCommands(project, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stems := map[string]bool{}
	for _, c := range cmds {
		stems[c.Stem] = true
	}
	if stems["hero-approve"] {
		t.Fatal("hero-approve should be excluded")
	}
	if !stems["herox"] || !stems["custom"] {
		t.Fatalf("expected herox and custom, got %v", stems)
	}
}

func TestStripFrontmatterEdgeCases(t *testing.T) {
	bom := "\ufeff---\ndescription: hi\n---\nbody after bom\n"
	if got := StripFrontmatter(bom); got != "body after bom\n" {
		t.Fatalf("BOM: got %q", got)
	}
	crlf := "---\na: 1\r\n---\r\nCRLF body\r\n"
	if got := StripFrontmatter(crlf); got != "CRLF body\r\n" {
		t.Fatalf("CRLF: got %q", got)
	}
	if StripFrontmatter("---not frontmatter") != "---not frontmatter" {
		t.Fatal("unclosed frontmatter should passthrough")
	}
}
