package uninstall

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
)

// Options holds uninstall configuration.
type Options struct {
	ProjectDir string
}

// Run removes only Hero-owned paths from the project.
// Preserved: AGENTS.md, context/, docs/, openspec/
func Run(opts Options, stdout, stderr io.Writer) error {
	// Hero-owned directories to remove entirely.
	dirsToRemove := []string{
		filepath.Join(opts.ProjectDir, cursoradapter.AgentsDir),
		filepath.Join(opts.ProjectDir, cursoradapter.WorkflowHeroSkillDir),
		filepath.Join(opts.ProjectDir, cursoradapter.GrillingSkillDir),
		filepath.Join(opts.ProjectDir, cursoradapter.HeroDir),
	}

	for _, d := range dirsToRemove {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			continue
		}
		if err := os.RemoveAll(d); err != nil {
			return fmt.Errorf("remove %s: %w", d, err)
		}
	}

	// Remove hero command files: .cursor/commands/hero-*.md
	commandsDir := filepath.Join(opts.ProjectDir, cursoradapter.CommandsDir)
	entries, err := os.ReadDir(commandsDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read commands dir: %w", err)
	}
	for _, e := range entries {
		name := e.Name()
		if len(name) > 5 && name[:5] == "hero-" && len(name) > 3 && name[len(name)-3:] == ".md" {
			path := filepath.Join(commandsDir, name)
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove %s: %w", path, err)
			}
		}
	}

	return nil
}
