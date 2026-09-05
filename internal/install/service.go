package install

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/envhygiene"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/logrotate"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// Options holds the resolved options for an install operation.
type Options struct {
	// ProjectDir is the target directory for installation.
	ProjectDir string
	// Name is the project name.
	Name string
	// Summary is the project summary.
	Summary string
	// Tools is the list of IDE tools (only "cursor" is supported in V1).
	Tools []string
	// Version is the Hero CLI version (injected at build time).
	Version string
	// GitInit specifies whether to initialize a git repo if one is missing.
	GitInit bool
	// AssetsFS is the embedded filesystem to copy assets from.
	AssetsFS fs.FS
}

// HeroJSON is the hero.json schema.
type HeroJSON struct {
	CLI             CLIInfo                  `json:"cli"`
	Assets          AssetsInfo               `json:"assets"`
	Harnesses       map[string]HarnessConfig `json:"harnesses,omitempty"`
	FreechatDefault FreechatDefault          `json:"freechat_default,omitempty"`
	// ChatVerbosity controls how much live harness activity the TUI transcript
	// renders. An omitted value intentionally resolves to Debug for backwards
	// compatibility with the pre-settings behaviour.
	ChatVerbosity ChatVerbosity `json:"chat_verbosity,omitempty"`
	// Telegram holds non-secret TUI Telegram settings (PRD-C09-001 §3.2;
	// ADR-062). Credentials stay in the OS vault, never here.
	Telegram TelegramConfig `json:"telegram,omitempty"`
	// ModelProperties persists string-valued model properties per harness/native
	// model pair (C5; ADR-040). Missing maps unmarshal as empty and never cause
	// a migration failure.
	ModelProperties map[string]map[string]map[string]string `json:"model_properties,omitempty"`
}

// TelegramConfig persists the editable project abbreviation used for Telegram
// addressing. An omitted ProjectAbbrev falls back to the directory basename.
type TelegramConfig struct {
	ProjectAbbrev string `json:"project_abbrev,omitempty"`
}

// CLIInfo holds CLI installation metadata.
type CLIInfo struct {
	Version     string   `json:"version"`
	InstalledAt string   `json:"installedAt"`
	Tools       []string `json:"tools"`
}

// AssetsInfo holds assets installation metadata.
type AssetsInfo struct {
	Version     string `json:"version"`
	InstalledAt string `json:"installedAt"`
}

// ProjectJSON is the project.json schema.
type ProjectJSON struct {
	Name       string         `json:"name"`
	Summary    string         `json:"summary"`
	CreatedAt  string         `json:"createdAt"`
	Workflow   WorkflowInfo   `json:"workflow"`
	Technology TechnologyInfo `json:"technology"`
}

// WorkflowInfo holds workflow state metadata.
type WorkflowInfo struct {
	Name  string `json:"name"`
	Phase string `json:"phase"`
	Cycle int    `json:"cycle"`
}

// TechnologyInfo holds technology stack metadata.
type TechnologyInfo struct {
	Stack     string   `json:"stack"`
	Backend   string   `json:"backend"`
	Languages []string `json:"languages"`
}

// DocumentsJSON is the documents.json schema.
type DocumentsJSON struct {
	Documents []DocumentEntry `json:"documents"`
}

// DocumentEntry is a single document registry entry.
type DocumentEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Purpose string `json:"purpose"`
}

// Checksums maps relative path to SHA256 hex digest.
type Checksums map[string]string

// Run performs the full Hero installation into opts.ProjectDir.
func Run(opts Options, stdout, stderr io.Writer) error {
	now := time.Now().UTC().Format(time.RFC3339)

	// 1. Ensure git repo.
	if !isGitRepo(opts.ProjectDir) {
		if opts.GitInit {
			if err := runGitInit(opts.ProjectDir, stdout); err != nil {
				return fmt.Errorf("git init failed: %w", err)
			}
		} else {
			return fmt.Errorf("not a git repository")
		}
	}

	// 2. Create Hero directory structure.
	dirs := []string{
		filepath.Join(opts.ProjectDir, cursoradapter.CommandsDir),
		filepath.Join(opts.ProjectDir, cursoradapter.AgentsDir),
		filepath.Join(opts.ProjectDir, cursoradapter.WorkflowHeroSkillDir),
		filepath.Join(opts.ProjectDir, cursoradapter.GrillingSkillDir),
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

	checksums := make(Checksums)

	if err := CopyCoreAssets(opts, checksums); err != nil {
		return fmt.Errorf("copy core assets: %w", err)
	}

	enabled := normalizeSelectedTools(opts.Tools)
	if len(enabled) == 0 {
		return fmt.Errorf("at least one harness must be enabled")
	}
	if containsTool(enabled, "cursor") {
		if err := CopyCursorAssets(opts, checksums); err != nil {
			return fmt.Errorf("copy cursor assets: %w", err)
		}
	}
	if containsTool(enabled, "opencode") {
		if err := ProvisionOpenCode(opts.ProjectDir, opts.AssetsFS, checksums); err != nil {
			return fmt.Errorf("provision opencode: %w", err)
		}
	}
	if containsTool(enabled, "codex") {
		if err := ProvisionCodex(opts.ProjectDir, opts.AssetsFS, checksums); err != nil {
			return fmt.Errorf("provision codex: %w", err)
		}
	}

	harnesses := HarnessesFromSelection(enabled)
	freechat := DefaultFreechatDefault(harnesses)

	// 10. Write hero.json
	heroData := HeroJSON{
		CLI: CLIInfo{
			Version:     opts.Version,
			InstalledAt: now,
			Tools:       enabled,
		},
		Assets: AssetsInfo{
			Version:     opts.Version,
			InstalledAt: now,
		},
		Harnesses:       harnesses,
		FreechatDefault: freechat,
	}
	if err := writeJSON(filepath.Join(opts.ProjectDir, cursoradapter.HeroJSONPath), heroData); err != nil {
		return fmt.Errorf("write hero.json: %w", err)
	}

	// 11. Write project.json
	projectData := ProjectJSON{
		Name:      opts.Name,
		Summary:   opts.Summary,
		CreatedAt: now,
		Workflow: WorkflowInfo{
			Name:  "Feature Development",
			Phase: "Research",
			Cycle: 0,
		},
		Technology: TechnologyInfo{
			Stack:     "",
			Backend:   "",
			Languages: []string{},
		},
	}
	if err := writeJSON(filepath.Join(opts.ProjectDir, cursoradapter.ProjectJSONPath), projectData); err != nil {
		return fmt.Errorf("write project.json: %w", err)
	}

	// 12. Write documents.json
	docsData := DocumentsJSON{Documents: []DocumentEntry{}}
	if err := writeJSON(filepath.Join(opts.ProjectDir, cursoradapter.DocumentsJSONPath), docsData); err != nil {
		return fmt.Errorf("write documents.json: %w", err)
	}

	// 13. Write metrics-summary.md
	metricsSummaryPath := filepath.Join(opts.ProjectDir, cursoradapter.MetricsSummaryPath)
	if err := os.WriteFile(metricsSummaryPath, []byte("# Metrics Summary\n\nNo cycles completed yet.\n"), 0o644); err != nil {
		return fmt.Errorf("write metrics-summary.md: %w", err)
	}

	// 14. Write checksums.json
	if err := writeJSON(filepath.Join(opts.ProjectDir, cursoradapter.ChecksumsJSONPath), checksums); err != nil {
		return fmt.Errorf("write checksums.json: %w", err)
	}

	// 15. Soft secrets hygiene: `.env.example` + `.gitignore` patterns (never overwrite existing).
	if err := envhygiene.EnsureProjectRoot(opts.ProjectDir, opts.AssetsFS); err != nil {
		return fmt.Errorf("env hygiene: %w", err)
	}

	// 15b. One-time migration of the legacy TUI log into the rotating logs dir.
	if err := MigrateTuiLog(opts.ProjectDir); err != nil {
		return fmt.Errorf("migrate tui log: %w", err)
	}

	// 16. Create/migrate operational SQLite store (hero.db).
	s, err := store.OpenProject(opts.ProjectDir)
	if err != nil {
		return fmt.Errorf("open operational store: %w", err)
	}
	if err := s.Close(); err != nil {
		return fmt.Errorf("close operational store: %w", err)
	}
	slog.Info("operational store ready", "path", filepath.Join(opts.ProjectDir, store.RelativeDBPath))

	emitHarnessMarkerWarnings(opts.ProjectDir, enabled, stderr)

	return nil
}

// MigrateTuiLog migrates the legacy single-file TUI log into the rotating logs
// directory once (ADR-064). It is a no-op when there is nothing to migrate.
func MigrateTuiLog(projectDir string) error {
	dir := filepath.Join(projectDir, ".workflow-hero")
	legacy := filepath.Join(dir, "tui.log")
	newPath := filepath.Join(dir, "logs", "tui.log")
	return logrotate.MigrateLegacy(legacy, newPath)
}

func normalizeSelectedTools(tools []string) []string {
	var out []string
	for _, t := range tools {
		t = strings.TrimSpace(strings.ToLower(t))
		if t != "" && !containsTool(out, t) {
			out = append(out, t)
		}
	}
	return out
}

func containsTool(tools []string, want string) bool {
	for _, t := range tools {
		if t == want {
			return true
		}
	}
	return false
}

func emitHarnessMarkerWarnings(projectDir string, configuredTools []string, stderr io.Writer) {
	res, err := harness.DetectMarkers(projectDir, configuredTools)
	if err != nil {
		slog.Error("harness marker detection failed", "error", err)
		return
	}
	if len(res.UnsupportedPresent) == 0 {
		return
	}

	configured := map[string]bool{}
	for _, t := range configuredTools {
		configured[t] = true
	}

	fmt.Fprintln(stderr)
	for _, m := range res.UnsupportedPresent {
		output.Warning(stderr, harness.UnsupportedMarkerWarningText(m, configured[m.ToolID]))
		output.Progress(stderr, harness.UnsupportedMarkerSuggestionText())
	}
}

// runGitInit runs `git init` in the given directory.
func runGitInit(dir string, stdout io.Writer) error {
	cmd := exec.Command("git", "init", dir)
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	return cmd.Run()
}

// copyAssetDir copies all files from srcDir (in assetsFS) to dstDir on disk.
// projectDir is used to compute relative checksum keys.
// Checksums are updated with the SHA256 of each file's contents.
func copyAssetDir(assetsFS fs.FS, srcDir, dstDir, projectDir string, checksums Checksums) error {
	return fs.WalkDir(assetsFS, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath := strings.TrimPrefix(path, srcDir+"/")
		dstPath := filepath.Join(dstDir, relPath)

		data, err := fs.ReadFile(assetsFS, path)
		if err != nil {
			return fmt.Errorf("read asset %s: %w", path, err)
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", dstPath, err)
		}

		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dstPath, err)
		}

		// Record checksum keyed by path relative to project dir.
		rel, err := filepath.Rel(projectDir, dstPath)
		if err != nil {
			rel = dstPath
		}
		checksums[rel] = sha256Bytes(data)

		return nil
	})
}

// sha256Bytes computes the SHA256 hex digest of data.
func sha256Bytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// writeJSON marshals v to JSON and writes it to path (pretty-printed).
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
