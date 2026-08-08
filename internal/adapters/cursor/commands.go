package cursor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CommandSource identifies where a discovered Cursor command file came from.
type CommandSource string

const (
	// CommandSourceProject is <project>/.cursor/commands.
	CommandSourceProject CommandSource = "project"
	// CommandSourceUser is ~/.cursor/commands.
	CommandSourceUser CommandSource = "user"
)

// DiscoveredCommand is a non-Hero Cursor slash command markdown file.
type DiscoveredCommand struct {
	// Label is "/"+stem (filename without .md).
	Label string
	// Stem is the filename without .md.
	Stem string
	// Path is the absolute path to the .md file.
	Path string
	// Source is "project" or "user".
	Source CommandSource
}

// DiscoverCommands scans project and user Cursor command directories for non-Hero
// .md files (excludes hero-*.md). projectDir is the Hero project root; userHome
// may be empty to use os.UserHomeDir. Design D3 / ADR-021.
func DiscoverCommands(projectDir, userHome string) ([]DiscoveredCommand, error) {
	if userHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("user home: %w", err)
		}
		userHome = home
	}
	var out []DiscoveredCommand
	seen := map[string]bool{}

	projectCmds := filepath.Join(projectDir, CommandsDir)
	userCmds := filepath.Join(userHome, CommandsDir)

	for _, item := range []struct {
		dir    string
		source CommandSource
	}{
		{projectCmds, CommandSourceProject},
		{userCmds, CommandSourceUser},
	} {
		cmds, err := listNonHeroCommands(item.dir, item.source)
		if err != nil {
			return nil, err
		}
		for _, c := range cmds {
			key := c.Stem
			if seen[key] {
				continue // project wins over user for same stem
			}
			seen[key] = true
			out = append(out, c)
		}
	}
	return out, nil
}

// StripFrontmatter removes a leading YAML frontmatter block (--- … ---) from
// markdown content. If no frontmatter is present, the input is returned unchanged.
func StripFrontmatter(content string) string {
	s := strings.TrimLeft(content, "\ufeff")
	if !strings.HasPrefix(s, "---") {
		return content
	}
	rest := s[3:]
	// Allow optional newline after opening ---
	if len(rest) > 0 && (rest[0] == '\n' || (rest[0] == '\r' && len(rest) > 1 && rest[1] == '\n')) {
		if rest[0] == '\r' {
			rest = rest[2:]
		} else {
			rest = rest[1:]
		}
	} else {
		return content
	}
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return content
	}
	after := rest[idx+len("\n---"):]
	after = strings.TrimPrefix(after, "\r")
	after = strings.TrimPrefix(after, "\n")
	return after
}

// ReadCommandPrompt reads a command markdown file and returns the body with
// YAML frontmatter stripped (design D3).
func ReadCommandPrompt(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return StripFrontmatter(string(b)), nil
}

func listNonHeroCommands(dir string, source CommandSource) ([]DiscoveredCommand, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []DiscoveredCommand
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		if matched, _ := filepath.Match(HeroOwnedCommandPattern, name); matched {
			continue
		}
		stem := strings.TrimSuffix(name, ".md")
		if stem == "" {
			continue
		}
		out = append(out, DiscoveredCommand{
			Label:  "/" + stem,
			Stem:   stem,
			Path:   filepath.Join(dir, name),
			Source: source,
		})
	}
	return out, nil
}
