package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// ChatVerbosity is the persisted TUI transcript detail profile.
type ChatVerbosity string

const (
	ChatVerbosityCompact  ChatVerbosity = "compact"
	ChatVerbosityStandard ChatVerbosity = "standard"
	ChatVerbosityDetailed ChatVerbosity = "detailed"
	ChatVerbosityDebug    ChatVerbosity = "debug"
)

// NormalizeChatVerbosity returns Debug for absent or unknown values so older
// projects retain Hero's historical "show everything" behaviour.
func NormalizeChatVerbosity(value ChatVerbosity) ChatVerbosity {
	switch ChatVerbosity(strings.ToLower(strings.TrimSpace(string(value)))) {
	case ChatVerbosityCompact:
		return ChatVerbosityCompact
	case ChatVerbosityStandard:
		return ChatVerbosityStandard
	case ChatVerbosityDetailed:
		return ChatVerbosityDetailed
	default:
		return ChatVerbosityDebug
	}
}

// SetChatVerbosity persists the selected TUI transcript profile.
func SetChatVerbosity(projectDir string, value ChatVerbosity) error {
	hero, err := LoadHeroJSON(projectDir)
	if err != nil {
		return err
	}
	hero.ChatVerbosity = NormalizeChatVerbosity(value)
	return saveHeroJSON(projectDir, hero)
}

// SupportedHarnessIDs lists harness identifiers Hero supports in the TUI
// (ADR-034; Cursor + OpenCode from C4; Codex added in C6 / Hero 2.5.0 per ADR-043/048).
var SupportedHarnessIDs = []string{"cursor", "opencode", "codex"}

// FreechatDefault is the persisted freechat /hero-new default pair (ADR-037).
type FreechatDefault struct {
	Harness string `json:"harness"`
	Model   string `json:"model"`
}

// HarnessesFromSelection builds the harnesses map for a fresh install.
func HarnessesFromSelection(selected []string) map[string]HarnessConfig {
	enabled := make(map[string]bool, len(selected))
	for _, id := range selected {
		id = strings.TrimSpace(strings.ToLower(id))
		if id != "" {
			enabled[id] = true
		}
	}
	out := make(map[string]HarnessConfig, len(SupportedHarnessIDs))
	for _, id := range SupportedHarnessIDs {
		out[id] = HarnessConfig{
			Enabled:           enabled[id],
			Model:             "",
			EnableFastModel:   false,
			PermissionProfile: harness.PermissionProfileAsk,
		}
	}
	return out
}

// DefaultFreechatDefault picks the initial freechat harness after install.
// Model stays empty until the user chooses via /hero-model.
func DefaultFreechatDefault(harnesses map[string]HarnessConfig) FreechatDefault {
	enabled := ListEnabledHarnesses(HeroJSON{Harnesses: harnesses})
	if len(enabled) == 0 {
		return FreechatDefault{}
	}
	h := enabled[0]
	if len(enabled) > 1 && slices.Contains(enabled, "cursor") {
		h = "cursor"
	}
	return FreechatDefault{Harness: h, Model: ""}
}

// MigrateHarnessState upgrades legacy hero.json (cli.tools only) to harnesses.*.enabled (ADR-034)
// and fills missing supported harness keys (C6 / ADR-048: adds harnesses.codex.enabled=false on
// 2.4.x → 2.5.0 without enabling Codex or provisioning .codex/).
// Returns true when hero was modified.
func MigrateHarnessState(hero *HeroJSON) bool {
	if hero == nil {
		return false
	}
	modified := false
	if hero.Harnesses == nil {
		hero.Harnesses = make(map[string]HarnessConfig)
		modified = true
	}
	// Legacy 1.x: derive enabled from cli.tools when harnesses lack enabled flags.
	hasEnabled := false
	for _, id := range SupportedHarnessIDs {
		if cfg, ok := hero.Harnesses[id]; ok && cfg.Enabled {
			hasEnabled = true
			break
		}
	}
	if !hasEnabled {
		tools := nonEmptyStrings(hero.CLI.Tools)
		if len(tools) == 0 {
			for _, id := range SupportedHarnessIDs {
				if _, ok := hero.Harnesses[id]; !ok {
					hero.Harnesses[id] = HarnessConfig{Enabled: false}
					modified = true
				}
			}
		} else {
			for _, id := range SupportedHarnessIDs {
				cfg := hero.Harnesses[id]
				cfg.Enabled = slices.Contains(tools, id)
				if !cfg.Enabled && id == "cursor" && slices.Contains(tools, "cursor") {
					cfg.Enabled = true
				}
				hero.Harnesses[id] = cfg
				modified = true
			}
		}
	}
	for _, id := range SupportedHarnessIDs {
		if _, ok := hero.Harnesses[id]; !ok {
			hero.Harnesses[id] = HarnessConfig{Enabled: id == "cursor" && slices.Contains(nonEmptyStrings(hero.CLI.Tools), "cursor")}
			modified = true
			continue
		}
	}
	return modified
}

// SetHarnessPermissionProfile persists the simple approval preset for one
// enabled harness. Blank and invalid values intentionally become Ask.
func SetHarnessPermissionProfile(projectDir, harnessID string, profile harness.PermissionProfile) error {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if !slices.Contains(SupportedHarnessIDs, harnessID) {
		return fmt.Errorf("unsupported harness %q", harnessID)
	}
	hero, err := LoadHeroJSON(projectDir)
	if err != nil {
		return err
	}
	if hero.Harnesses == nil {
		hero.Harnesses = HarnessesFromSelection(nil)
	}
	cfg := hero.Harnesses[harnessID]
	cfg.PermissionProfile = harness.NormalizePermissionProfile(profile)
	hero.Harnesses[harnessID] = cfg
	return saveHeroJSON(projectDir, hero)
}

// ListEnabledHarnesses returns enabled harness ids in stable order.
func ListEnabledHarnesses(hero HeroJSON) []string {
	var out []string
	for _, id := range SupportedHarnessIDs {
		if IsHarnessEnabled(hero, id) {
			out = append(out, id)
		}
	}
	return out
}

// IsHarnessEnabled reports whether harness id is enabled in hero.json.
func IsHarnessEnabled(hero HeroJSON, harnessID string) bool {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if harnessID == "" {
		return false
	}
	if hero.Harnesses != nil {
		if cfg, ok := hero.Harnesses[harnessID]; ok {
			return cfg.Enabled
		}
	}
	// Legacy fallback: cli.tools membership.
	return slices.Contains(nonEmptyStrings(hero.CLI.Tools), harnessID)
}

// GetFreechatDefault returns the persisted freechat pair.
// Model is empty until the user selects one with /hero-model (never invented).
func GetFreechatDefault(hero HeroJSON) (harness, model string) {
	h := strings.TrimSpace(strings.ToLower(hero.FreechatDefault.Harness))
	m := strings.TrimSpace(hero.FreechatDefault.Model)
	if h == "" {
		def := DefaultFreechatDefault(hero.Harnesses)
		h = def.Harness
	}
	return h, m
}

// SetHarnessEnabled toggles harnesses.<id>.enabled in hero.json.
func SetHarnessEnabled(projectDir, harnessID string, enabled bool) error {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if harnessID == "" {
		return fmt.Errorf("harness id is required")
	}
	if !slices.Contains(SupportedHarnessIDs, harnessID) {
		return fmt.Errorf("unsupported harness %q", harnessID)
	}
	hero, err := LoadHeroJSON(projectDir)
	if err != nil {
		return err
	}
	if hero.Harnesses == nil {
		hero.Harnesses = HarnessesFromSelection(nil)
	}
	cfg := hero.Harnesses[harnessID]
	cfg.Enabled = enabled
	hero.Harnesses[harnessID] = cfg
	if !enabled {
		enabledCount := len(ListEnabledHarnesses(hero))
		if enabledCount == 0 {
			return fmt.Errorf("cannot disable the last enabled harness (%s)", harnessID)
		}
	}
	return saveHeroJSON(projectDir, hero)
}

// SetFreechatDefault persists freechat_default and harnesses.<harness>.model.
func SetFreechatDefault(projectDir, harnessID, model string) error {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	model = strings.TrimSpace(model)
	if harnessID == "" || model == "" {
		return fmt.Errorf("harness and model are required")
	}
	if !slices.Contains(SupportedHarnessIDs, harnessID) {
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
	cfg.Model = model
	cfg.EnableFastModel = false
	hero.Harnesses[harnessID] = cfg
	hero.FreechatDefault = FreechatDefault{Harness: harnessID, Model: model}
	return saveHeroJSON(projectDir, hero)
}

func saveHeroJSON(projectDir string, hero HeroJSON) error {
	path := filepath.Join(projectDir, cursoradapter.HeroJSONPath)
	encoded, err := json.MarshalIndent(hero, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

func nonEmptyStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, s := range items {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}
