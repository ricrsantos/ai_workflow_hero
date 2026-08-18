package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
)

// CommitModelSelection persists freechat_default, harnesses.<harness>.model, and
// the complete property draft in one atomic write (temp file + rename). Rows may
// be toggled freely in memory before this call; nothing is written until the
// final picker Enter (ADR-040/042). A nil/empty props map commits the pair only,
// which is the no-selectable-property skip path.
func CommitModelSelection(projectDir, harnessID, modelID string, props map[string]string) error {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	modelID = strings.TrimSpace(modelID)
	if harnessID == "" || modelID == "" {
		return fmt.Errorf("harness and model are required")
	}
	if !slicesContainsSupported(harnessID) {
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
	cfg.Model = modelID
	// New C5 commits use the model-property map; the legacy fast flag is cleared
	// so it can never re-seed fs on a later read (ADR-040).
	cfg.EnableFastModel = false
	hero.Harnesses[harnessID] = cfg
	hero.FreechatDefault = FreechatDefault{Harness: harnessID, Model: modelID}
	SetPairProperties(&hero, harnessID, modelID, props)
	return saveHeroJSONAtomic(projectDir, hero)
}

// saveHeroJSONAtomic writes hero.json via a temporary file plus rename so the
// pair and property selection commit together or not at all.
func saveHeroJSONAtomic(projectDir string, hero HeroJSON) error {
	path := filepath.Join(projectDir, cursoradapter.HeroJSONPath)
	encoded, err := json.MarshalIndent(hero, "", "  ")
	if err != nil {
		return err
	}
	data := append(encoded, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".hero.json.tmp-*")
	if err != nil {
		return fmt.Errorf("create hero.json temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write hero.json temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync hero.json temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close hero.json temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("commit hero.json: %w", err)
	}
	return nil
}

func slicesContainsSupported(id string) bool {
	for _, s := range SupportedHarnessIDs {
		if s == id {
			return true
		}
	}
	return false
}
