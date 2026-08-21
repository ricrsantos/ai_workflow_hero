package install

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

const codexDirName = ".codex"

// CodexPaths holds project-relative Codex projection paths.
type CodexPaths struct {
	Root     string
	Agents   string
	Commands string
	Skills   string
}

// CodexPathsFor returns projection directory paths under projectDir.
func CodexPathsFor(projectDir string) CodexPaths {
	root := filepath.Join(projectDir, codexDirName)
	return CodexPaths{
		Root:     root,
		Agents:   filepath.Join(root, "agents"),
		Commands: filepath.Join(root, "commands"),
		Skills:   filepath.Join(root, "skills"),
	}
}

// CodexOwnedPaths returns Hero-managed directories under .codex/ that uninstall removes.
// User-added files (e.g. config.toml) outside these trees are left alone (ADR-046).
func CodexOwnedPaths(projectDir string) []string {
	p := CodexPathsFor(projectDir)
	return []string{
		p.Agents,
		p.Commands,
		filepath.Join(p.Skills, "workflow-hero"),
		filepath.Join(p.Skills, "grilling"),
	}
}

// CodexAssetGroups returns embed.FS src → destination pairs for Codex projection.
func CodexAssetGroups(projectDir string) []struct{ Src, Dst string } {
	paths := CodexPathsFor(projectDir)
	return []struct{ Src, Dst string }{
		{"codex/agents", paths.Agents},
		{"codex/commands", paths.Commands},
		{"codex/skills/workflow-hero", filepath.Join(paths.Skills, "workflow-hero")},
		{"codex/skills/grilling", filepath.Join(paths.Skills, "grilling")},
	}
}

// ProvisionCodex writes .codex/ assets from embedded FS when codex is enabled
// (ADR-046; design D5/D6). Layout mirrors OpenCode (agents/commands/skills).
// No minimal Codex config file is written — the adapter does not require one.
// Root AGENTS.md is never copied into .codex/.
func ProvisionCodex(projectDir string, assetsFS fs.FS, checksums Checksums) error {
	paths := CodexPathsFor(projectDir)
	dirs := []string{paths.Root, paths.Agents, paths.Commands, paths.Skills}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", d, err)
		}
	}
	for _, g := range CodexAssetGroups(projectDir) {
		if err := copyAssetDir(assetsFS, g.Src, g.Dst, projectDir, checksums); err != nil {
			return fmt.Errorf("copy codex %s: %w", g.Src, err)
		}
	}
	slog.Info("codex projection provisioned", "path", paths.Root)
	return nil
}
