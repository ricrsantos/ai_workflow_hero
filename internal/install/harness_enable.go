package install

import (
	"fmt"
	"io/fs"
	"strings"
)

// EnableHarnessWithProjection enables a harness in hero.json and provisions
// projection assets for harnesses that have a native project layout.
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

	if assetsFS != nil && (harnessID == "opencode" || harnessID == "codex" || harnessID == "claude") {
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
		case "claude":
			if err := ProvisionClaude(projectDir, assetsFS, checksums); err != nil {
				return fmt.Errorf("provision Claude: %w", err)
			}
		}
		if err := WriteChecksums(projectDir, checksums); err != nil {
			return fmt.Errorf("write checksums: %w", err)
		}
	}
	return saveHeroJSON(projectDir, hero)
}

// EnableClaudeHarness enables the Claude harness, provisions .claude/, and
// applies the managed root CLAUDE.md context decision. Projection and enable
// failures are returned as errors; a context decision that cannot be applied
// (for example a missing AGENTS.md) is returned for the caller to surface
// without treating the harness enable itself as failed.
func EnableClaudeHarness(projectDir string, assetsFS fs.FS, decision ClaudeContextDecision) (ClaudeContextResult, error) {
	if err := EnableHarnessWithProjection(projectDir, "claude", assetsFS); err != nil {
		return ClaudeContextResult{}, err
	}
	return ApplyClaudeContext(projectDir, decision)
}

func containsHarness(ids []string, id string) bool {
	for _, h := range ids {
		if h == id {
			return true
		}
	}
	return false
}
