package modelprops

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
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
	// refreshErrors records the latest explicit live-refresh failure per
	// harness so a later local snapshot can explain stale cache data.
	refreshErrors map[string]error
}

// NewService builds the model-property service for a project. Registry may be
// nil (tests); the embedded catalog is always available.
func NewService(projectDir string, st *store.Store, reg harnessmgr.Registry, embedded fs.FS) *Service {
	return &Service{
		ProjectDir:    projectDir,
		Store:         st,
		Registry:      reg,
		Catalog:       LoadCatalog(embedded, projectDir),
		pending:       map[string]int64{},
		refreshErrors: map[string]error{},
	}
}

// Snapshot returns the best local view immediately: project cache first, then
// catalog, then unknown/na (ADR-039). No harness API is touched here, so
// `/hero-model` never blocks and OpenCode is never started at TUI boot.
func (s *Service) Snapshot(harnessID, modelID string) Snapshot {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	modelID = strings.TrimSpace(modelID)
	s.mu.Lock()
	refreshErr := s.refreshErrors[harnessID]
	s.mu.Unlock()
	var snap Snapshot
	if s.Store != nil {
		if row, ok, err := s.Store.Capabilities(harnessID, modelID); err == nil && ok {
			snap = Resolve(harnessID, modelID, nil, refreshErr, &row, true, s.Catalog)
			return applyCursorSlugLocks(snap)
		}
	}
	snap = Resolve(harnessID, modelID, nil, refreshErr, nil, false, s.Catalog)
	return applyCursorSlugLocks(snap)
}

// SnapshotCacheOnly returns capabilities from the project SQLite cache, enriched
// with installed/embedded catalog metadata when the cached harness response is
// incomplete (PRD-C05-001 §4.2.5).
func (s *Service) SnapshotCacheOnly(harnessID, modelID string) Snapshot {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	modelID = strings.TrimSpace(modelID)
	s.mu.Lock()
	refreshErr := s.refreshErrors[harnessID]
	s.mu.Unlock()
	if s.Store != nil {
		if row, ok, err := s.Store.Capabilities(harnessID, modelID); err == nil && ok {
			snap := Resolve(harnessID, modelID, nil, refreshErr, &row, true, s.Catalog)
			return applyCursorSlugLocks(snap)
		}
	}
	snap := Resolve(harnessID, modelID, nil, refreshErr, nil, false, s.Catalog)
	return applyCursorSlugLocks(snap)
}

func enrichCapabilitiesFromCatalog(cat Catalog, modelID string, caps harness.ModelCapabilities) harness.ModelCapabilities {
	if cat == nil {
		return caps
	}
	byKey := map[string]harness.PropertyCapability{}
	for _, p := range caps.Properties {
		byKey[p.Key] = p
	}
	for _, key := range harness.PropertyKeys() {
		catProp, ok := cat.CatalogValues(modelID, key)
		if !ok || !catProp.HasProperty {
			continue
		}
		cur, ok := byKey[key]
		if !ok {
			cur = harness.PropertyCapability{Key: key}
		}
		byKey[key] = mergePropertyCapability(cur, catProp)
	}
	caps.Properties = caps.Properties[:0]
	for _, key := range harness.PropertyKeys() {
		if p, ok := byKey[key]; ok && (p.Available || p.DefaultValue != "" || len(p.AcceptedValues) > 0) {
			caps.Properties = append(caps.Properties, p)
		}
	}
	return caps
}

func applyCursorSlugLocks(snap Snapshot) Snapshot {
	if snap.HarnessID != "cursor" {
		return snap
	}
	if len(cursor.SlugLockedProperties(snap.ModelID)) == 0 {
		return snap
	}
	caps := harness.ModelCapabilities{HarnessID: snap.HarnessID, ModelID: snap.ModelID}
	for _, key := range harness.PropertyKeys() {
		if p, ok := snap.Properties[key]; ok {
			caps.Properties = append(caps.Properties, p)
		}
	}
	caps = cursor.ApplySlugLocks(caps, snap.ModelID)
	snap.Properties = map[string]harness.PropertyCapability{}
	for _, p := range caps.Properties {
		snap.Properties[p.Key] = p
	}
	for _, key := range harness.PropertyKeys() {
		if _, ok := snap.Properties[key]; !ok {
			snap.Properties[key] = harness.PropertyCapability{Key: key, Available: false}
		}
	}
	return snap
}

// Models returns the best immediately available model rows for a harness.
// A successful live list is persisted in the project store and wins on the
// next picker opening; an absent cache falls back to the local catalog without
// starting a harness process.
func (s *Service) Models(harnessID string) []string {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if s == nil || harnessID == "" {
		return nil
	}
	if s.Store != nil {
		if models, _, err := s.Store.ModelList(harnessID); err == nil && len(models) > 0 {
			return uniqueModelIDs(models)
		}
	}
	if s.Catalog == nil {
		return nil
	}
	return uniqueModelIDs(s.Catalog.ModelsForHarness(harnessID))
}

// CachedModels returns only a persisted API model list.  It is separate from
// Models so a caller with an in-memory boot list can keep that list ahead of a
// catalog while still applying a completed background refresh on the next
// picker opening.
func (s *Service) CachedModels(harnessID string) []string {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if s == nil || s.Store == nil || harnessID == "" {
		return nil
	}
	models, _, err := s.Store.ModelList(harnessID)
	if err != nil {
		return nil
	}
	return uniqueModelIDs(models)
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
	ids := make([]string, 0, len(enabled))
	for _, id := range enabled {
		id = strings.TrimSpace(strings.ToLower(id))
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	summaries := make([]RefreshSummary, len(ids))
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(index int, harnessID string) {
			defer wg.Done()
			summaries[index] = s.refreshHarness(ctx, harnessID)
		}(i, id)
	}
	wg.Wait()
	return summaries
}

func (s *Service) refreshHarness(ctx context.Context, harnessID string) RefreshSummary {
	summary := RefreshSummary{HarnessID: harnessID}
	var generation int64
	if s.Store != nil {
		var beginErr error
		generation, beginErr = s.Store.BeginRefresh(harnessID)
		if beginErr != nil {
			summary.Err = beginErr
			slog.Error("modelprops refresh state begin failed", "harness", harnessID, "error", beginErr)
		}
	}
	s.mu.Lock()
	if s.pending == nil {
		s.pending = make(map[string]int64)
	}
	if s.refreshErrors == nil {
		s.refreshErrors = make(map[string]error)
	}
	s.pending[harnessID] = generation
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if current, ok := s.pending[harnessID]; ok && current == generation {
			delete(s.pending, harnessID)
		}
		s.refreshErrors[harnessID] = summary.Err
		s.mu.Unlock()
		if s.Store != nil && generation > 0 {
			_ = s.Store.CompleteRefresh(harnessID, generation)
		}
	}()

	if s.Registry == nil {
		summary.Err = fmt.Errorf("harness registry unavailable")
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
			slog.Error("modelprops refresh list models failed", "harness", harnessID, "error", listErr)
		} else {
			models = uniqueModelIDs(models)
			summary.Models = len(models)
			if s.Store != nil {
				if err := s.Store.UpsertModelList(harnessID, models, time.Now().UTC().Format(time.RFC3339)); err != nil {
					summary.Err = firstRefreshError(summary.Err, err)
					slog.Error("modelprops refresh persist model list failed", "harness", harnessID, "error", err)
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
						summary.Err = firstRefreshError(summary.Err, discErr)
						continue
					}
					caps = harness.NormalizeModelCapabilities(caps)
					if strings.TrimSpace(caps.HarnessID) == "" {
						caps.HarnessID = harnessID
					}
					if strings.TrimSpace(caps.ModelID) == "" {
						caps.ModelID = model
					}
					if err := s.persistCapabilities(harnessID, model, caps); err != nil {
						summary.Err = firstRefreshError(summary.Err, err)
						slog.Error("modelprops refresh persist capabilities failed", "harness", harnessID, "model", model, "error", err)
						continue
					}
					if len(caps.Properties) > 0 {
						summary.Capabilities++
					}
				}
			} else if harnessID == "cursor" {
				for _, model := range models {
					if ctx.Err() != nil {
						break
					}
					caps := cursor.InferCapabilitiesFromModelList(models, model)
					caps = harness.NormalizeModelCapabilities(caps)
					if err := s.persistCapabilities(harnessID, model, caps); err != nil {
						summary.Err = firstRefreshError(summary.Err, err)
						slog.Error("modelprops refresh persist cursor capabilities failed", "harness", harnessID, "model", model, "error", err)
						continue
					}
					if cursor.HasSelectableCapability(caps) {
						summary.Capabilities++
					}
				}
			}
		}
	}
	slog.Info("modelprops refresh complete", "harness", harnessID, "models", summary.Models, "capabilities", summary.Capabilities)
	return summary
}

func firstRefreshError(current, next error) error {
	if current != nil {
		return current
	}
	return next
}

func (s *Service) persistCapabilities(harnessID, model string, caps harness.ModelCapabilities) error {
	caps = enrichCapabilitiesFromCatalog(s.Catalog, model, caps)
	if s.Store == nil {
		return nil
	}
	if cached, ok, readErr := s.Store.Capabilities(harnessID, model); readErr == nil && ok {
		caps.Properties = mergeCapabilityProperties(decodeCacheProperties(cached.PropertiesJSON), caps.Properties)
	}
	ts := caps.RetrievedAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	return s.Store.UpsertCapabilities(store.CapabilityCacheRow{
		Harness:        harnessID,
		Model:          strings.TrimSpace(caps.ModelID),
		PropertiesJSON: EncodeCapabilities(caps),
		RetrievedAt:    ts.Format(time.RFC3339),
	})
}

func uniqueModelIDs(models []string) []string {
	if len(models) == 0 {
		return nil
	}
	out := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}
	return out
}

func mergeCapabilityProperties(cached map[string]harness.PropertyCapability, live []harness.PropertyCapability) []harness.PropertyCapability {
	merged := make(map[string]harness.PropertyCapability, len(cached)+len(live))
	for key, property := range cached {
		merged[key] = property
	}
	for _, property := range live {
		merged[property.Key] = property
	}
	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		rank := func(key string) int {
			for index, c5Key := range harness.PropertyKeys() {
				if key == c5Key {
					return index
				}
			}
			return len(harness.PropertyKeys())
		}
		ri, rj := rank(keys[i]), rank(keys[j])
		if ri != rj {
			return ri < rj
		}
		return keys[i] < keys[j]
	})
	out := make([]harness.PropertyCapability, 0, len(keys))
	for _, key := range keys {
		out = append(out, merged[key])
	}
	return out
}
