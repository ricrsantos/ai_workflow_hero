package upgrade

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/assetconflict"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/envhygiene"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// Options holds upgrade configuration.
type Options struct {
	ProjectDir string
	Version    string
	AssetsFS   fs.FS
}

// Result reports the outcome of an upgrade operation.
type Result struct {
	Updated        []string
	Replaced       []string // replaced after local customization conflict (backup saved)
	Migrated       []string // workflow-config.yml files migrated from generic_model
	LegacyImported bool
	LegacyCycleNum int
}

// Run performs the upgrade: re-copies assets, protecting customized files.
func Run(opts Options, stdout, stderr io.Writer) (Result, error) {
	var result Result

	// Load existing checksums.
	originalChecksums, err := install.LoadChecksums(opts.ProjectDir)
	if err != nil {
		return result, fmt.Errorf("load checksums: %w", err)
	}

	newChecksums := make(install.Checksums)

	// Copy assets, checking for local customizations.
	assetGroups := []struct {
		src string
		dst string
	}{
		{"cursor/commands", filepath.Join(opts.ProjectDir, cursoradapter.CommandsDir)},
		{"cursor/agents", filepath.Join(opts.ProjectDir, cursoradapter.AgentsDir)},
		{"cursor/skills/workflow-hero", filepath.Join(opts.ProjectDir, cursoradapter.WorkflowHeroSkillDir)},
		{"cursor/skills/grilling", filepath.Join(opts.ProjectDir, cursoradapter.GrillingSkillDir)},
		{"templates", filepath.Join(opts.ProjectDir, cursoradapter.HeroTemplatesDir)},
		{"models", filepath.Join(opts.ProjectDir, cursoradapter.HeroModelsDir)},
		{"docs", filepath.Join(opts.ProjectDir, cursoradapter.HeroDocsDir)},
	}

	for _, group := range assetGroups {
		if err := fs.WalkDir(opts.AssetsFS, group.src, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			relPath := strings.TrimPrefix(path, group.src+"/")
			dstPath := filepath.Join(group.dst, relPath)

			newData, err := fs.ReadFile(opts.AssetsFS, path)
			if err != nil {
				return fmt.Errorf("read asset %s: %w", path, err)
			}

			relKey, _ := filepath.Rel(opts.ProjectDir, dstPath)
			newHash := assetconflict.SHA256Hex(newData)

			// Check if file exists on disk.
			existingData, readErr := os.ReadFile(dstPath)
			if readErr == nil {
				existingHash := assetconflict.SHA256Hex(existingData)
				originalHash := originalChecksums[relKey]

				// File is customized if its current hash differs from the originally installed hash.
				if assetconflict.IsCustomized(existingData, originalHash) {
					if existingHash == newHash {
						// Disk already matches the embedded asset; checksums were stale (e.g. git-updated files).
						newChecksums[relKey] = newHash
						return nil
					}
					if _, err := assetconflict.Replace(dstPath, existingData, newData, relKey, stderr, time.Now()); err != nil {
						return err
					}
					newChecksums[relKey] = newHash
					result.Replaced = append(result.Replaced, relKey)
					result.Updated = append(result.Updated, relKey)
					return nil
				}
			}

			// Write the new file.
			if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
				return fmt.Errorf("create dir: %w", err)
			}
			if err := os.WriteFile(dstPath, newData, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", dstPath, err)
			}

			newChecksums[relKey] = newHash
			result.Updated = append(result.Updated, relKey)
			return nil
		}); err != nil {
			return result, fmt.Errorf("upgrade asset group %s: %w", group.src, err)
		}
	}

	migrated, err := migrateLegacyWorkflowConfigs(opts.ProjectDir, stderr)
	if err != nil {
		return result, fmt.Errorf("migrate workflow-config: %w", err)
	}
	result.Migrated = migrated

	if err := ensureStoreAndImportLegacy(opts, &result, stderr); err != nil {
		return result, err
	}

	// Soft secrets hygiene for projects upgraded from older Hero versions.
	if err := envhygiene.EnsureProjectRoot(opts.ProjectDir, opts.AssetsFS); err != nil {
		return result, fmt.Errorf("env hygiene: %w", err)
	}

	// Update hero.json versions.
	heroPath := filepath.Join(opts.ProjectDir, cursoradapter.HeroJSONPath)
	heroData, err := os.ReadFile(heroPath)
	if err != nil {
		return result, fmt.Errorf("read hero.json: %w", err)
	}
	var heroJSON install.HeroJSON
	if err := json.Unmarshal(heroData, &heroJSON); err != nil {
		return result, fmt.Errorf("parse hero.json: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	heroJSON.CLI.Version = opts.Version
	heroJSON.Assets.Version = opts.Version
	heroJSON.Assets.InstalledAt = now
	_ = install.MigrateHarnessState(&heroJSON)
	_ = install.EnsureHarnessDefaults(&heroJSON)
	updatedHero, _ := json.MarshalIndent(heroJSON, "", "  ")
	if err := os.WriteFile(heroPath, append(updatedHero, '\n'), 0o644); err != nil {
		return result, fmt.Errorf("write hero.json: %w", err)
	}

	// Write updated checksums.
	updatedChecks, _ := json.MarshalIndent(newChecksums, "", "  ")
	if err := os.WriteFile(filepath.Join(opts.ProjectDir, cursoradapter.ChecksumsJSONPath), append(updatedChecks, '\n'), 0o644); err != nil {
		return result, fmt.Errorf("write checksums.json: %w", err)
	}

	return result, nil
}

// ensureStoreAndImportLegacy opens hero.db and, when the store has no cycles yet,
// imports legacy workflow.md / metrics.md from the current cycle directory.
func ensureStoreAndImportLegacy(opts Options, result *Result, stderr io.Writer) error {
	s, ensureRes, err := cycle.EnsureOperationalStore(opts.ProjectDir)
	if err != nil {
		return err
	}
	defer s.Close()

	slog.Info("operational store ready", "path", filepath.Join(opts.ProjectDir, store.RelativeDBPath))

	if ensureRes.LegacyImported {
		output.Warningf(stderr,
			"Imported legacy cycle — operational state now lives in %s; markdown is no longer canonical.",
			store.RelativeDBPath,
		)
		result.LegacyImported = true
		result.LegacyCycleNum = ensureRes.LegacyCycleNum
		slog.Info("legacy cycle imported", "cycle", ensureRes.LegacyCycleNum)
	}

	return nil
}
