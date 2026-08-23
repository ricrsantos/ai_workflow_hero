package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/engine"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// freeChatConfigRoot returns the user home used as the synthetic project root for
// free-chat config (~/.workflow-hero/config/hero.json + hero.db).
func freeChatConfigRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if home == "" {
		return "", fmt.Errorf("home directory is empty")
	}
	return home, nil
}

// ensureFreeChatHome creates ~/.workflow-hero/config/hero.json with Cursor
// enabled when missing. Does not require git or a project install.
func ensureFreeChatHome(home string) error {
	configDir := filepath.Join(home, cursoradapter.HeroDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("create free-chat config dir: %w", err)
	}
	path := filepath.Join(home, cursoradapter.HeroJSONPath)
	if _, err := os.Stat(path); err == nil {
		hero, loadErr := install.LoadHeroJSON(home)
		if loadErr != nil {
			return loadErr
		}
		if install.MigrateHarnessState(&hero) || install.EnsureHarnessDefaults(&hero) {
			return writeFreeChatHeroJSON(home, hero)
		}
		if len(install.ListEnabledHarnesses(hero)) == 0 {
			hero.Harnesses = install.HarnessesFromSelection([]string{"cursor"})
			hero.FreechatDefault = install.DefaultFreechatDefault(hero.Harnesses)
			return writeFreeChatHeroJSON(home, hero)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	harnesses := install.HarnessesFromSelection([]string{"cursor"})
	hero := install.HeroJSON{
		CLI: install.CLIInfo{
			Version:     "freechat",
			InstalledAt: now,
			Tools:       []string{"cursor"},
		},
		Assets: install.AssetsInfo{
			Version:     "freechat",
			InstalledAt: now,
		},
		Harnesses:       harnesses,
		FreechatDefault: install.DefaultFreechatDefault(harnesses),
	}
	return writeFreeChatHeroJSON(home, hero)
}

func writeFreeChatHeroJSON(home string, hero install.HeroJSON) error {
	path := filepath.Join(home, cursoradapter.HeroJSONPath)
	encoded, err := json.MarshalIndent(hero, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

// OpenFreeChatService prepares a Service for `hero chat`: config under the
// user home (.workflow-hero), Execute workspace = cwd. No project install or git.
func OpenFreeChatService() (*cycle.Service, error) {
	home, err := freeChatConfigRoot()
	if err != nil {
		return nil, err
	}
	if err := ensureFreeChatHome(home); err != nil {
		return nil, err
	}
	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not determine current directory: %w", err)
	}
	st, err := store.OpenProject(home)
	if err != nil {
		return nil, fmt.Errorf("open free-chat store: %w", err)
	}
	return &cycle.Service{
		ProjectDir: home,
		WorkDir:    workDir,
		Store:      st,
		Engine:     engine.New(st),
		Registry:   harnessmgr.NewRegistry(home, st),
	}, nil
}
