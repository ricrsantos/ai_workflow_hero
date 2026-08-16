package install

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
)

// EnableHarnessWithProjection enables a harness in hero.json and provisions OpenCode
// projection assets when harnessID is opencode (UI-C04-001 §3; design D7).
func EnableHarnessWithProjection(projectDir, harnessID string, assetsFS fs.FS) error {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if harnessID == "" {
		return fmt.Errorf("harness id is required")
	}
	if !containsHarness(SupportedHarnessIDs, harnessID) {
		return fmt.Errorf("unsupported harness %q", harnessID)
	}
	hero, err := LoadHeroJSON(projectDir)
	if err != nil {
		return err
	}
	if hero.Harnesses == nil {
		hero.Harnesses = HarnessesFromSelection([]string{harnessID})
	}
	cfg := hero.Harnesses[harnessID]
	cfg.Enabled = true
	hero.Harnesses[harnessID] = cfg

	if harnessID == "opencode" && assetsFS != nil {
		checksums, err := loadProjectChecksums(projectDir)
		if err != nil {
			return fmt.Errorf("load checksums: %w", err)
		}
		if err := ProvisionOpenCode(projectDir, assetsFS, checksums); err != nil {
			return fmt.Errorf("provision opencode: %w", err)
		}
		if err := writeProjectChecksums(projectDir, checksums); err != nil {
			return fmt.Errorf("write checksums: %w", err)
		}
	}
	return saveHeroJSON(projectDir, hero)
}

func loadProjectChecksums(projectDir string) (Checksums, error) {
	path := filepath.Join(projectDir, cursoradapter.ChecksumsJSONPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(Checksums), nil
		}
		return nil, err
	}
	var checksums Checksums
	if err := json.Unmarshal(data, &checksums); err != nil {
		return nil, err
	}
	if checksums == nil {
		checksums = make(Checksums)
	}
	return checksums, nil
}

func writeProjectChecksums(projectDir string, checksums Checksums) error {
	path := filepath.Join(projectDir, cursoradapter.ChecksumsJSONPath)
	encoded, err := json.MarshalIndent(checksums, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

func containsHarness(ids []string, id string) bool {
	for _, h := range ids {
		if h == id {
			return true
		}
	}
	return false
}
