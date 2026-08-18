package modelprops

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// fakeRegistry maps harness ids to fake adapters (no live harness processes).
type fakeRegistry struct {
	adapters map[string]harness.HarnessAdapter
	calls    map[string]int
}

func (r *fakeRegistry) Adapter(id string) (harness.HarnessAdapter, error) {
	r.calls[id]++
	a, ok := r.adapters[id]
	if !ok {
		return nil, errors.New("unsupported harness " + id)
	}
	return a, nil
}

func (r *fakeRegistry) SupportedIDs() []string { return []string{"cursor", "opencode"} }
func (r *fakeRegistry) EnabledIDs(_ install.HeroJSON) []string {
	return nil
}

// fakeHarnessAdapter implements HarnessAdapter + optional lister/discoverer.
type fakeHarnessAdapter struct {
	name       string
	models     []string
	listErr    error
	discoverFn func(ctx context.Context, modelID string) (harness.ModelCapabilities, error)
	listCalls  int
}

func (f *fakeHarnessAdapter) Name() string { return f.name }
func (f *fakeHarnessAdapter) IsAvailable(context.Context) error {
	return nil
}
func (f *fakeHarnessAdapter) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return nil, nil
}
func (f *fakeHarnessAdapter) ResumeSession(context.Context, string) error { return nil }
func (f *fakeHarnessAdapter) Execute(context.Context, harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	return nil, nil
}
func (f *fakeHarnessAdapter) Cancel(context.Context, string) error { return nil }
func (f *fakeHarnessAdapter) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return nil, nil
}
func (f *fakeHarnessAdapter) Dispatch(context.Context, harness.DispatchRequest) (harness.DispatchResult, error) {
	return harness.DispatchResult{}, nil
}
func (f *fakeHarnessAdapter) ListModels(context.Context) ([]string, error) {
	f.listCalls++
	return f.models, f.listErr
}
func (f *fakeHarnessAdapter) DiscoverModelProperties(ctx context.Context, modelID string) (harness.ModelCapabilities, error) {
	if f.discoverFn != nil {
		return f.discoverFn(ctx, modelID)
	}
	return harness.ModelCapabilities{}, errors.New("no capability api")
}

func newTestService(t *testing.T, dir string) (*Service, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(dir, store.DBFileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewService(dir, st, nil, fstest.MapFS{}), st
}

func TestSnapshotUsesCatalogWhenStoreEmpty(t *testing.T) {
	dir := t.TempDir()
	svc, _ := newTestService(t, dir)
	svc.Catalog = Catalog{
		"acme/model": CatalogModel{Properties: map[string]CatalogProperty{
			"th": {Available: true, Values: []string{"off", "max"}, Default: "off", HasProperty: true},
		}},
	}
	snap := svc.Snapshot("acme", "acme/model")
	if snap.Source != SourceCatalog {
		t.Fatalf("source=%s", snap.Source)
	}
	if !snap.Property("th").Available {
		t.Fatal("catalog th must be selectable")
	}
	if snap.HasSelectableProperty() == false {
		t.Fatal("catalog snapshot must be selectable")
	}
}

func TestSnapshotUsesPersistedCache(t *testing.T) {
	dir := t.TempDir()
	svc, st := newTestService(t, dir)
	raw := `{"ef":{"available":true,"values":["low","high"],"default":"low"}}`
	if err := st.UpsertCapabilities(store.CapabilityCacheRow{
		Harness: "opencode", Model: "opencode-go/deepseek-v4-pro",
		PropertiesJSON: raw, RetrievedAt: "2026-08-15T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	snap := svc.Snapshot("opencode", "opencode-go/deepseek-v4-pro")
	if snap.Source != SourceCache || snap.Stale {
		t.Fatalf("source=%s stale=%v", snap.Source, snap.Stale)
	}
	if snap.Property("ef").Available != true {
		t.Fatal("cached ef must be selectable")
	}
}

func TestSnapshotUnknownWarns(t *testing.T) {
	dir := t.TempDir()
	svc, _ := newTestService(t, dir)
	snap := svc.Snapshot("cursor", "mystery")
	if snap.Source != SourceUnknown || snap.Warning != WarningMissingCatalog {
		t.Fatalf("snapshot=%+v", snap)
	}
}

func TestModelsUsePersistedListBeforeCatalog(t *testing.T) {
	dir := t.TempDir()
	svc, st := newTestService(t, dir)
	svc.Catalog = Catalog{
		"catalog/model": CatalogModel{Provider: "cursor"},
	}
	if got := svc.Models("cursor"); len(got) != 1 || got[0] != "catalog/model" {
		t.Fatalf("catalog models=%v", got)
	}
	if err := st.UpsertModelList("cursor", []string{"api/model", "api/model", ""}, "2026-08-17T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	got := svc.Models("cursor")
	if len(got) != 1 || got[0] != "api/model" {
		t.Fatalf("cached models=%v", got)
	}
}

func TestSnapshotUsesStaleCacheAfterRefreshFailure(t *testing.T) {
	dir := t.TempDir()
	svc, st := newTestService(t, dir)
	if err := st.UpsertCapabilities(store.CapabilityCacheRow{
		Harness: "opencode", Model: "m1",
		PropertiesJSON: `{"ef":{"available":true,"values":["high"],"default":"high"}}`,
		RetrievedAt:    "2026-08-15T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	adapter := &fakeHarnessAdapter{name: "opencode", listErr: errors.New("serve unavailable")}
	svc.Registry = &fakeRegistry{
		adapters: map[string]harness.HarnessAdapter{"opencode": adapter},
		calls:    map[string]int{},
	}
	if summaries := svc.Refresh(context.Background(), []string{"opencode"}); len(summaries) != 1 || summaries[0].Err == nil {
		t.Fatalf("refresh summaries=%+v", summaries)
	}
	snap := svc.Snapshot("opencode", "m1")
	if !snap.Stale || snap.Warning != WarningStaleCache || !snap.Property("ef").Available {
		t.Fatalf("stale snapshot=%+v", snap)
	}
}

func TestRefreshFansOutAndPersistsWithGeneration(t *testing.T) {
	dir := t.TempDir()
	svc, st := newTestService(t, dir)
	oc := &fakeHarnessAdapter{
		name:   "opencode",
		models: []string{"opencode-go/deepseek-v4-pro"},
		discoverFn: func(_ context.Context, modelID string) (harness.ModelCapabilities, error) {
			return harness.ModelCapabilities{
				HarnessID: "opencode",
				ModelID:   modelID,
				Properties: []harness.PropertyCapability{
					{Key: "fs", AcceptedValues: []string{"true", "false"}, DefaultValue: "false", Available: true},
				},
			}, nil
		},
	}
	cursor := &fakeHarnessAdapter{
		name:   "cursor",
		models: []string{"composer-2.5"},
		// No discovery support: capability calls fail → normal fallback.
	}
	reg := &fakeRegistry{adapters: map[string]harness.HarnessAdapter{
		"opencode": oc,
		"cursor":   cursor,
	}, calls: map[string]int{}}
	svc.Registry = reg

	gen1, _ := st.BeginRefresh("opencode")
	if gen1 != 1 {
		t.Fatalf("gen=%d", gen1)
	}
	summaries := svc.Refresh(context.Background(), []string{"opencode", "cursor"})
	if len(summaries) != 2 {
		t.Fatalf("summaries=%d", len(summaries))
	}
	if summaries[0].Models != 1 || summaries[0].Capabilities != 1 {
		t.Fatalf("opencode summary: %+v", summaries[0])
	}
	if summaries[1].Models != 1 || summaries[1].Capabilities != 0 {
		t.Fatalf("cursor summary: %+v", summaries[1])
	}

	row, ok, err := st.Capabilities("opencode", "opencode-go/deepseek-v4-pro")
	if err != nil || !ok {
		t.Fatalf("capabilities must persist: %v %v", err, ok)
	}
	if row.PropertiesJSON == "" {
		t.Fatal("properties json empty")
	}
	models, _, err := st.ModelList("cursor")
	if err != nil || len(models) != 1 {
		t.Fatalf("cursor model list must persist: %v %v", models, err)
	}
	// The generation marker completes after the refresh.
	_, pending, err := st.RefreshState("opencode")
	if err != nil || pending {
		t.Fatalf("refresh must be marked complete: pending=%v err=%v", pending, err)
	}
}

func TestRefreshIsInvocableOnlyOnDemand(t *testing.T) {
	dir := t.TempDir()
	svc, _ := newTestService(t, dir)
	oc := &fakeHarnessAdapter{name: "opencode", models: []string{"m1"}}
	reg := &fakeRegistry{adapters: map[string]harness.HarnessAdapter{"opencode": oc}, calls: map[string]int{}}
	svc.Registry = reg

	// Building the service and taking a snapshot must never call the harness API
	// (this is what keeps TUI boot lazy — no ListModels / serve start).
	if oc.listCalls != 0 {
		t.Fatalf("snapshot must not list models, calls=%d", oc.listCalls)
	}
	_ = svc.Snapshot("opencode", "m1")
	if oc.listCalls != 0 {
		t.Fatalf("snapshot must not list models, calls=%d", oc.listCalls)
	}
	// Refresh is explicit.
	svc.Refresh(context.Background(), []string{"opencode"})
	if oc.listCalls != 1 {
		t.Fatalf("refresh must list models exactly once, calls=%d", oc.listCalls)
	}
}

func TestRefreshPreservesOpenSnapshots(t *testing.T) {
	dir := t.TempDir()
	svc, st := newTestService(t, dir)
	// Start with a catalog snapshot (no cache).
	svc.Catalog = Catalog{
		"m1": CatalogModel{Properties: map[string]CatalogProperty{
			"th": {Available: true, Values: []string{"off", "max"}, Default: "off", HasProperty: true},
		}},
	}
	before := svc.Snapshot("opencode", "m1")
	if !before.Property("th").Available {
		t.Fatal("precondition: catalog th available")
	}

	// Refresh completes with different API data.
	oc := &fakeHarnessAdapter{
		name:   "opencode",
		models: []string{"m1"},
		discoverFn: func(_ context.Context, modelID string) (harness.ModelCapabilities, error) {
			return harness.ModelCapabilities{
				HarnessID: "opencode", ModelID: modelID,
				Properties: []harness.PropertyCapability{
					{Key: "ef", AcceptedValues: []string{"high"}, DefaultValue: "high", Available: true},
				},
			}, nil
		},
	}
	reg := &fakeRegistry{adapters: map[string]harness.HarnessAdapter{"opencode": oc}, calls: map[string]int{}}
	svc.Registry = reg
	svc.Refresh(context.Background(), []string{"opencode"})

	// The previously taken snapshot object is unchanged (no reorder under an
	// open selector), while a fresh snapshot sees the persisted API result.
	if !before.Property("th").Available || before.Property("ef").Available {
		t.Fatalf("refresh mutated the open snapshot: %+v", before.Properties)
	}
	after := svc.Snapshot("opencode", "m1")
	if !after.Property("ef").Available {
		t.Fatalf("next opening must see refreshed data: %+v", after.Properties)
	}
	if _, ok, _ := st.Capabilities("opencode", "m1"); !ok {
		t.Fatal("refreshed capabilities must be persisted")
	}
}

func TestRefreshPartialAPIReplacesCoveredKeyAndRetainsOtherCacheKeys(t *testing.T) {
	dir := t.TempDir()
	svc, st := newTestService(t, dir)
	if err := st.UpsertCapabilities(store.CapabilityCacheRow{
		Harness: "opencode", Model: "m1",
		PropertiesJSON: `{"fs":{"available":true,"values":["true","false"],"default":"false"},"ef":{"available":true,"values":["low","high"],"default":"low"}}`,
		RetrievedAt:    "2026-08-15T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	adapter := &fakeHarnessAdapter{
		name:   "opencode",
		models: []string{"m1"},
		discoverFn: func(context.Context, string) (harness.ModelCapabilities, error) {
			return harness.ModelCapabilities{Properties: []harness.PropertyCapability{
				{Key: "ef", Available: true, AcceptedValues: []string{"medium"}, DefaultValue: "medium"},
			}}, nil
		},
	}
	svc.Registry = &fakeRegistry{
		adapters: map[string]harness.HarnessAdapter{"opencode": adapter},
		calls:    map[string]int{},
	}
	svc.Refresh(context.Background(), []string{"opencode"})
	snap := svc.Snapshot("opencode", "m1")
	if !snap.Property("fs").Available || snap.Property("ef").DefaultValue != "medium" {
		t.Fatalf("partial API merge=%+v", snap.Properties)
	}
}

func TestRefreshListFailureKeepsOldCache(t *testing.T) {
	dir := t.TempDir()
	svc, st := newTestService(t, dir)
	if err := st.UpsertCapabilities(store.CapabilityCacheRow{
		Harness: "opencode", Model: "m1",
		PropertiesJSON: `{"th":{"available":true,"values":["off"],"default":"off"}}`,
		RetrievedAt:    "2026-08-15T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	oc := &fakeHarnessAdapter{name: "opencode", listErr: errors.New("serve down")}
	reg := &fakeRegistry{adapters: map[string]harness.HarnessAdapter{"opencode": oc}, calls: map[string]int{}}
	svc.Registry = reg
	summaries := svc.Refresh(context.Background(), []string{"opencode"})
	if summaries[0].Err == nil {
		t.Fatal("list failure must be reported")
	}
	row, ok, _ := st.Capabilities("opencode", "m1")
	if !ok || row.RetrievedAt != "2026-08-15T00:00:00Z" {
		t.Fatal("failed refresh must not delete or age the cached row")
	}
}
