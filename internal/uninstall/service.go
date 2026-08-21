package uninstall

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

// Options holds uninstall configuration.
type Options struct {
	ProjectDir string
}

// Run removes only Hero-owned paths from the project.
// Preserved: AGENTS.md, context/, docs/, openspec/, and user-added files under
// .opencode/ or .codex/ that Hero does not manage (ADR-046).
func Run(opts Options, stdout, stderr io.Writer) error {
	_ = stdout
	_ = stderr

	// Hero-owned directories to remove entirely.
	dirsToRemove := []string{
		filepath.Join(opts.ProjectDir, cursoradapter.AgentsDir),
		filepath.Join(opts.ProjectDir, cursoradapter.WorkflowHeroSkillDir),
		filepath.Join(opts.ProjectDir, cursoradapter.GrillingSkillDir),
		filepath.Join(opts.ProjectDir, cursoradapter.HeroDir),
	}
	dirsToRemove = append(dirsToRemove, install.OpenCodeOwnedPaths(opts.ProjectDir)...)
	dirsToRemove = append(dirsToRemove, install.CodexOwnedPaths(opts.ProjectDir)...)

	for _, d := range dirsToRemove {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			continue
		}
		if err := os.RemoveAll(d); err != nil {
			return fmt.Errorf("remove %s: %w", d, err)
		}
		slog.Info("uninstall removed hero-owned path", "path", d)
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
