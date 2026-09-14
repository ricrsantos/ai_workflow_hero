package modelprops

import "strings"

// ModelListSource identifies the source of a harness model inventory.
type ModelListSource string

const (
	// ModelListSourceLive is a successful ListModels response in this process.
	ModelListSourceLive ModelListSource = "live"
	// ModelListSourceCache is a previously successful ListModels response read
	// from the project-scoped SQLite cache.
	ModelListSourceCache ModelListSource = "cache"
	// ModelListSourceCatalog is the embedded/installed static fallback catalog.
	ModelListSourceCatalog ModelListSource = "catalog"
)

// ModelListState describes the best model inventory currently available for a
// harness. Authoritative means that the model IDs came from a successful
// Harness listing, including an explicitly empty response.
type ModelListState struct {
	HarnessID     string
	Models        []string
	Source        ModelListSource
	Authoritative bool
	Pending       bool
	Err           error
	RefreshedAt   string
}

type liveModelList struct {
	models      []string
	refreshedAt string
}

// ModelListState returns the source-aware model inventory without starting a
// Harness. A successful live list wins, followed by its persisted cache, and
// finally the static catalog. The catalog is never merged into an
// authoritative Harness list.
func (s *Service) ModelListState(harnessID string) ModelListState {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	state := ModelListState{HarnessID: harnessID}
	if s == nil || harnessID == "" {
		return state
	}

	s.mu.Lock()
	live, liveOK := s.liveModelLists[harnessID]
	pending := false
	if _, ok := s.pending[harnessID]; ok {
		pending = true
	}
	listErr := s.modelListErrors[harnessID]
	s.mu.Unlock()

	if s.Store != nil {
		if _, storePending, err := s.Store.RefreshState(harnessID); err == nil {
			pending = storePending
		}
	}
	state.Pending = pending
	state.Err = listErr

	if liveOK {
		state.Models = uniqueModelIDs(live.models)
		state.Source = ModelListSourceLive
		state.Authoritative = true
		state.RefreshedAt = live.refreshedAt
		return state
	}

	if s.Store != nil {
		models, refreshedAt, found, err := s.Store.ModelListSnapshot(harnessID)
		if err != nil {
			if state.Err == nil {
				state.Err = err
			}
		} else if found {
			state.Models = uniqueModelIDs(models)
			state.Source = ModelListSourceCache
			state.Authoritative = true
			state.RefreshedAt = refreshedAt
			return state
		}
	}

	if s.Catalog != nil {
		state.Models = uniqueModelIDs(s.Catalog.ModelsForHarness(harnessID))
		state.Source = ModelListSourceCatalog
	}
	return state
}

func (s *Service) setLiveModelList(harnessID string, models []string, refreshedAt string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.liveModelLists == nil {
		s.liveModelLists = make(map[string]liveModelList)
	}
	s.liveModelLists[harnessID] = liveModelList{
		models:      append([]string(nil), models...),
		refreshedAt: refreshedAt,
	}
	if s.modelListErrors == nil {
		s.modelListErrors = make(map[string]error)
	}
	s.modelListErrors[harnessID] = nil
}

func (s *Service) setModelListError(harnessID string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.modelListErrors == nil {
		s.modelListErrors = make(map[string]error)
	}
	s.modelListErrors[harnessID] = err
}
