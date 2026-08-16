package harnessmgr

import (
	"fmt"
	"slices"
	"strings"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	opencodeadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/opencode"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// Registry resolves HarnessAdapter implementations by id (design D5).
type Registry interface {
	Adapter(id string) (harness.HarnessAdapter, error)
	SupportedIDs() []string
	EnabledIDs(hero install.HeroJSON) []string
}

// DefaultRegistry is the production harness registry for Hero 2.0.
type DefaultRegistry struct {
	ProjectDir string
	Store      *store.Store
}

// NewRegistry returns a registry for projectDir with optional operational store.
func NewRegistry(projectDir string, st *store.Store) *DefaultRegistry {
	return &DefaultRegistry{ProjectDir: projectDir, Store: st}
}

// Adapter implements Registry.
func (r *DefaultRegistry) Adapter(id string) (harness.HarnessAdapter, error) {
	id = strings.TrimSpace(strings.ToLower(id))
	switch id {
	case "cursor", "":
		return cursoradapter.NewAdapter(r.ProjectDir), nil
	case "opencode":
		return opencodeadapter.NewAdapter(r.ProjectDir, r.Store), nil
	default:
		return nil, fmt.Errorf("unsupported harness %q", id)
	}
}

// SupportedIDs implements Registry.
func (r *DefaultRegistry) SupportedIDs() []string {
	return slices.Clone(install.SupportedHarnessIDs)
}

// EnabledIDs implements Registry.
func (r *DefaultRegistry) EnabledIDs(hero install.HeroJSON) []string {
	return install.ListEnabledHarnesses(hero)
}

// ResolvePair picks adapter + native model slug for a harness/model pair.
func (r *DefaultRegistry) ResolvePair(harnessID, model string) (harness.HarnessAdapter, string, error) {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	model = strings.TrimSpace(model)
	if harnessID == "" {
		return nil, "", fmt.Errorf("harness id is required")
	}
	adapter, err := r.Adapter(harnessID)
	if err != nil {
		return nil, "", err
	}
	if model == "" {
		return adapter, "", fmt.Errorf("model id is required for harness %q", harnessID)
	}
	slug := model
	if harnessID == "cursor" {
		slug = install.ResolveHarnessModelSlug(install.HarnessConfig{Model: model})
	}
	return adapter, slug, nil
}
