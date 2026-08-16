package install

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
)

const (
	opencodeDirName     = ".opencode"
	opencodeJSONRelPath = ".opencode/opencode.json"
)

// OpenCodePaths holds project-relative OpenCode projection paths.
type OpenCodePaths struct {
	Root     string
	Agents   string
	Commands string
	Skills   string
}

// OpenCodePathsFor returns projection directory paths under projectDir.
func OpenCodePathsFor(projectDir string) OpenCodePaths {
	root := filepath.Join(projectDir, opencodeDirName)
	return OpenCodePaths{
		Root:     root,
		Agents:   filepath.Join(root, "agents"),
		Commands: filepath.Join(root, "commands"),
		Skills:   filepath.Join(root, "skills"),
	}
}

// ProvisionOpenCode writes .opencode/ assets from embedded FS when opencode is enabled (design D7).
func ProvisionOpenCode(projectDir string, assetsFS fs.FS, checksums Checksums) error {
	paths := OpenCodePathsFor(projectDir)
	dirs := []string{paths.Root, paths.Agents, paths.Commands, paths.Skills}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", d, err)
		}
	}
	groups := []struct {
		src string
		dst string
	}{
		{"opencode/agents", paths.Agents},
		{"opencode/commands", paths.Commands},
		{"opencode/skills/workflow-hero", filepath.Join(paths.Skills, "workflow-hero")},
		{"opencode/skills/grilling", filepath.Join(paths.Skills, "grilling")},
	}
	for _, g := range groups {
		if err := copyAssetDir(assetsFS, g.src, g.dst, projectDir, checksums); err != nil {
			return fmt.Errorf("copy opencode %s: %w", g.src, err)
		}
	}
	if err := writeMinimalOpenCodeJSON(paths.Root, checksums, projectDir); err != nil {
		return err
	}
	slog.Info("opencode projection provisioned", "path", paths.Root)
	return nil
}

func writeMinimalOpenCodeJSON(root string, checksums Checksums, projectDir string) error {
	path := filepath.Join(root, "opencode.json")
	body := []byte("{\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return err
	}
	relKey, _ := filepath.Rel(projectDir, path)
	checksums[relKey] = sha256Bytes(body)
	return nil
}

// CopyCursorAssets copies Cursor runtime assets when cursor harness is enabled.
func CopyCursorAssets(opts Options, checksums Checksums) error {
	groups := []struct {
		src string
		dst string
	}{
		{"cursor/commands", filepath.Join(opts.ProjectDir, cursoradapter.CommandsDir)},
		{"cursor/agents", filepath.Join(opts.ProjectDir, cursoradapter.AgentsDir)},
		{"cursor/skills/workflow-hero", filepath.Join(opts.ProjectDir, cursoradapter.WorkflowHeroSkillDir)},
		{"cursor/skills/grilling", filepath.Join(opts.ProjectDir, cursoradapter.GrillingSkillDir)},
	}
	for _, g := range groups {
		if err := copyAssetDir(opts.AssetsFS, g.src, g.dst, opts.ProjectDir, checksums); err != nil {
			return fmt.Errorf("copy %s: %w", g.src, err)
		}
	}
	return nil
}

// CopyCoreAssets copies harness-agnostic Hero assets (templates, models, docs, config dirs).
func CopyCoreAssets(opts Options, checksums Checksums) error {
	dirs := []string{
		filepath.Join(opts.ProjectDir, cursoradapter.HeroConfigDir),
		filepath.Join(opts.ProjectDir, cursoradapter.HeroTemplatesDir),
		filepath.Join(opts.ProjectDir, cursoradapter.HeroModelsDir),
		filepath.Join(opts.ProjectDir, cursoradapter.HeroDocsDir),
		filepath.Join(opts.ProjectDir, cursoradapter.HeroCurrentCycleDir),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
	}
	groups := []struct {
		src string
		dst string
	}{
		{"templates", filepath.Join(opts.ProjectDir, cursoradapter.HeroTemplatesDir)},
		{"models", filepath.Join(opts.ProjectDir, cursoradapter.HeroModelsDir)},
		{"docs", filepath.Join(opts.ProjectDir, cursoradapter.HeroDocsDir)},
	}
	for _, g := range groups {
		if err := copyAssetDir(opts.AssetsFS, g.src, g.dst, opts.ProjectDir, checksums); err != nil {
			return fmt.Errorf("copy %s: %w", g.src, err)
		}
	}
	return nil
}

// EnabledHarnessSummary formats harness names for install success line.
func EnabledHarnessSummary(selected []string) string {
	var names []string
	for _, id := range selected {
		switch strings.ToLower(id) {
		case "cursor":
			names = append(names, "Cursor")
		case "opencode":
			names = append(names, "OpenCode")
		default:
			names = append(names, id)
		}
	}
	return strings.Join(names, ", ")
}
