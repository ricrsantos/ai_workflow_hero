package install

import (
	"fmt"
	"io/fs"
	"strings"
)

// EnableHarnessWithProjection enables a harness in hero.json and provisions
// projection assets when harnessID is opencode or codex (UI-C04-001 §3;
// UI-C06-001 §3; ADR-046).
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

	if assetsFS != nil && (harnessID == "opencode" || harnessID == "codex") {
		checksums, err := LoadChecksums(projectDir)
		if err != nil {
			return fmt.Errorf("load checksums: %w", err)
		}
		switch harnessID {
		case "opencode":
			if err := ProvisionOpenCode(projectDir, assetsFS, checksums); err != nil {
				return fmt.Errorf("provision opencode: %w", err)
			}
		case "codex":
			if err := ProvisionCodex(projectDir, assetsFS, checksums); err != nil {
				return fmt.Errorf("provision codex: %w", err)
			}
		}
		if err := WriteChecksums(projectDir, checksums); err != nil {
			return fmt.Errorf("write checksums: %w", err)
		}
	}
	return saveHeroJSON(projectDir, hero)
}

func containsHarness(ids []string, id string) bool {
	for _, h := range ids {
		if h == id {
			return true
		}
	}
	return false
}
