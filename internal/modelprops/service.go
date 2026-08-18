package modelprops

import (
	"context"
	"io/fs"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// Service owns C5 model-property snapshots and refresh orchestration (ADR-039).
type Service struct {
	ProjectDir string
	Store      *store.Store
	Registry   harnessmgr.Registry
	Catalog    Catalog

	mu      sync.Mutex
	pending map[string]int64 // harness → in-flight refresh generation
}

// NewService builds the model-property service for a project. Registry may be
// nil (tests); the embedded catalog is always available.
func NewService(projectDir string, st *store.Store, reg harnessmgr.Registry, embedded fs.FS) *Service {
	return &Service{
		ProjectDir: projectDir,
		Store:      st,
		Registry:   reg,
		Catalog:    LoadCatalog(embedded, projectDir),
		pending:    map[string]int64{},
	}
}

// Snapshot returns the best local view immediately: project cache first, then
// catalog, then unknown/na (ADR-039). No harness API is touched here, so
// `/hero-model` never blocks and OpenCode is never started at TUI boot.
func (s *Service) Snapshot(harnessID, modelID string) Snapshot {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	modelID = strings.TrimSpace(modelID)
	if s.Store != nil {
		if row, ok, err := s.Store.Capabilities(harnessID, modelID); err == nil && ok {
			// Reading from the project cache is not a failed-refresh fallback,
			// so the snapshot is never marked stale here.
			return Resolve(harnessID, modelID, nil, nil, &row, true, s.Catalog)
		}
	}
	return Resolve(harnessID, modelID, nil, nil, nil, false, s.Catalog)
}

// PendingRefresh reports whether a background refresh is in flight for a harness.
func (s *Service) PendingRefresh(harnessID string) bool {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if s.Store != nil {
		_, pending, err := s.Store.RefreshState(harnessID)
		if err == nil {
			return pending
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.pending[harnessID]
	return ok
}

// RefreshSummary reports one harness refresh outcome.
type RefreshSummary struct {
	HarnessID    string
	Models       int
	Capabilities int
	Err          error
}

// Refresh fans out across every enabled harness: model lists where the adapter
// supports them and capabilities where it implements ModelPropertyDiscoverer.
// Results are persisted with a generation/pending marker; completed refreshes
// never reorder an open selector — the TUI applies them on the next opening.
// Refresh is only invoked when /hero-model opens (never at TUI boot).
func (s *Service) Refresh(ctx context.Context, enabled []string) []RefreshSummary {
	summaries := make([]RefreshSummary, 0, len(enabled))
	for _, id := range enabled {
		id = strings.TrimSpace(strings.ToLower(id))
		if id == "" {
			continue
		}
		summaries = append(summaries, s.refreshHarness(ctx, id))
	}
	return summaries
}

func (s *Service) refreshHarness(ctx context.Context, harnessID string) RefreshSummary {
	summary := RefreshSummary{HarnessID: harnessID}
	var generation int64
	if s.Store != nil {
		generation, _ = s.Store.BeginRefresh(harnessID)
	}
	s.mu.Lock()
	s.pending[harnessID] = generation
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, harnessID)
		s.mu.Unlock()
		if s.Store != nil && generation > 0 {
			_ = s.Store.CompleteRefresh(harnessID, generation)
		}
	}()

	if s.Registry == nil {
		return summary
	}
	adapter, err := s.Registry.Adapter(harnessID)
	if err != nil {
		summary.Err = err
		slog.Debug("modelprops refresh adapter unavailable", "harness", harnessID, "error", err)
		return summary
	}

	lister, hasLister := adapter.(harness.ModelLister)
	discoverer, hasDiscoverer := adapter.(harness.ModelPropertyDiscoverer)

	if hasLister {
		models, listErr := lister.ListModels(ctx)
		if listErr != nil {
			summary.Err = listErr
			slog.Warn("modelprops refresh list models failed", "harness", harnessID, "error", listErr)
		} else {
			summary.Models = len(models)
			if s.Store != nil {
				if err := s.Store.UpsertModelList(harnessID, models, time.Now().UTC().Format(time.RFC3339)); err != nil {
					slog.Warn("modelprops refresh persist model list failed", "harness", harnessID, "error", err)
				}
			}
			if hasDiscoverer {
				for _, model := range models {
					if ctx.Err() != nil {
						break
					}
					caps, discErr := discoverer.DiscoverModelProperties(ctx, model)
					if discErr != nil {
						// Missing capability support is a normal fallback condition.
						slog.Debug("modelprops discovery unavailable", "harness", harnessID, "model", model, "error", discErr)
						continue
					}
					if len(caps.Properties) == 0 {
						continue
					}
					summary.Capabilities++
					if s.Store != nil {
						ts := caps.RetrievedAt
						if ts.IsZero() {
							ts = time.Now().UTC()
						}
						if err := s.Store.UpsertCapabilities(store.CapabilityCacheRow{
							Harness:        harnessID,
							Model:          strings.TrimSpace(caps.ModelID),
							PropertiesJSON: EncodeCapabilities(caps),
							RetrievedAt:    ts.Format(time.RFC3339),
						}); err != nil {
							slog.Warn("modelprops refresh persist capabilities failed", "harness", harnessID, "error", err)
						}
					}
				}
			}
		}
	}
	slog.Info("modelprops refresh complete", "harness", harnessID, "models", summary.Models, "capabilities", summary.Capabilities)
	return summary
}
