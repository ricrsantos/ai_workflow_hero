package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

// Options holds upgrade configuration.
type Options struct {
	ProjectDir string
	Version    string
	AssetsFS   fs.FS
}

// Result reports the outcome of an upgrade operation.
type Result struct {
	Updated []string
	Skipped []string // skipped due to local customization
}

// Run performs the upgrade: re-copies assets, protecting customized files.
func Run(opts Options, stdout, stderr io.Writer) (Result, error) {
	var result Result

	// Load existing checksums.
	originalChecksums, err := loadChecksums(filepath.Join(opts.ProjectDir, cursoradapter.ChecksumsJSONPath))
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
			newHash := sha256hex(newData)

			// Check if file exists on disk.
			existingData, readErr := os.ReadFile(dstPath)
			if readErr == nil {
				existingHash := sha256hex(existingData)
				originalHash := originalChecksums[relKey]

				// File is customized if its current hash differs from the originally installed hash.
				if originalHash != "" && existingHash != originalHash {
					output.Warningf(stderr, "%s was customized locally and was not overwritten.", relKey)
					result.Skipped = append(result.Skipped, relKey)
					// Keep the new hash for when it is eventually merged.
					newChecksums[relKey] = newHash
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

func loadChecksums(path string) (install.Checksums, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(install.Checksums), nil
		}
		return nil, err
	}
	var c install.Checksums
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return c, nil
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
