package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/modelprops"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestSelectDuringRefreshWaitsAndOpensPicker(t *testing.T) {
	m, _ := newPickerTestModel(t)
	st := m.svc.Store
	caps := harness.ModelCapabilities{
		HarnessID: "cursor",
		ModelID:   "full/model",
		Properties: []harness.PropertyCapability{
			{Key: harness.PropertyFast, AcceptedValues: []string{"true", "false"}, DefaultValue: "false", Available: true},
			{Key: harness.PropertyThink, AcceptedValues: []string{"off", "max"}, DefaultValue: "off", Available: true},
			{Key: harness.PropertyEffort, AcceptedValues: []string{"low", "medium", "high"}, DefaultValue: "medium", Available: true},
		},
	}
	if err := st.UpsertCapabilities(store.CapabilityCacheRow{
		Harness:        "cursor",
		Model:          "full/model",
		PropertiesJSON: modelprops.EncodeCapabilities(caps),
		RetrievedAt:    time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}

	m = OpenHeroModelForTest(m)
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter") // cursor harness
	m = SetPropsRefreshBusyForTest(m, true)

	m, cmd := SelectChatModelPairForTest(m, "full/model", "cursor")
	if cmd == nil {
		t.Fatal("expected wait tick cmd while refresh is in flight")
	}
	if !PropsAwaitingRefreshForTest(m) {
		t.Fatal("must await refresh before applying selection")
	}
	if h, slug, ok := PropsPendingSelectForTest(m); !ok || h != "cursor" || slug != "full/model" {
		t.Fatalf("pending select = %q %q ok=%v", h, slug, ok)
	}
	view := ViewForTest(m)
	if !strings.Contains(view, "Waiting for harness refresh") {
		t.Fatalf("wait view missing:\n%s", view)
	}

	m, _ = DeliverRefreshDoneForTest(m, []modelprops.RefreshSummary{{HarnessID: "cursor"}})
	if PropsAwaitingRefreshForTest(m) {
		t.Fatal("await flag must clear after refresh completes")
	}
	if !m.pickingProps {
		t.Fatal("property picker must open after refresh wait")
	}
}

func TestRefreshWaitUsesCacheOnlyBeforeCatalog(t *testing.T) {
	m, dir := newPickerTestModel(t)
	st, err := store.Open(filepath.Join(dir, store.DBFileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	m.propsSvc.Store = st
	m.svc.Store = st

	caps := harness.ModelCapabilities{
		HarnessID: "cursor",
		ModelID:   "cache-only/model",
		Properties: []harness.PropertyCapability{
			{Key: harness.PropertyEffort, AcceptedValues: []string{"low", "high"}, DefaultValue: "low", Available: true},
		},
	}
	if err := st.UpsertCapabilities(store.CapabilityCacheRow{
		Harness:        "cursor",
		Model:          "cache-only/model",
		PropertiesJSON: modelprops.EncodeCapabilities(caps),
		RetrievedAt:    time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	// Catalog has no entry for cache-only/model; Snapshot() would still work via
	// unknown defaults, but SnapshotCacheOnly must surface the cached row.
	m = SetPropsRefreshBusyForTest(m, true)
	m, _ = SelectChatModelPairForTest(m, "cache-only/model", "cursor")
	m, _ = DeliverRefreshDoneForTest(m, []modelprops.RefreshSummary{{HarnessID: "cursor"}})

	if !m.pickingProps {
		t.Fatal("cached capabilities must open the property picker")
	}
	if m.propsDraft[harness.PropertyEffort] != "low" {
		t.Fatalf("draft effort=%q want low", m.propsDraft[harness.PropertyEffort])
	}
}

func TestSlugLockedPropertiesShownAndCommitted(t *testing.T) {
	m, dir := newPickerTestModel(t)
	m.propsSvc.Catalog = propsCatalog(map[string]map[string]modelprops.CatalogProperty{
		"cursor-grok-4.6-low": {
			"fs": {Available: true, Values: []string{"true", "false"}, Default: "false"},
			"th": {Available: true, Values: []string{"off", "max"}, Default: "off"},
			"ef": {Available: true, Values: []string{"low", "medium", "high"}, Default: "medium"},
		},
	})
	m = SetAvailableModelsForTest(m, []string{"cursor-grok-4.6-low"})
	m = OpenHeroModelForTest(m)
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")

	if !m.pickingProps {
		t.Fatal("variant slug with catalog metadata must open property picker")
	}
	view := ViewForTest(m)
	if !strings.Contains(view, "Reasoning effort: low") {
		t.Fatalf("locked effort must display slug value:\n%s", view)
	}
	if !strings.Contains(view, "Fast Mode: false") {
		t.Fatalf("variant slug must lock fast off:\n%s", view)
	}

	// Locked rows must ignore space toggles.
	m, _ = HandleTestKey(m, " ") // fast row
	if m.propsDraft[harness.PropertyFast] != "false" {
		t.Fatalf("locked fast must stay false, got %q", m.propsDraft[harness.PropertyFast])
	}
	m, _ = HandleTestKey(m, "down")
	m, _ = HandleTestKey(m, "down")
	m, _ = HandleTestKey(m, " ") // effort row
	if m.propsDraft[harness.PropertyEffort] != "low" {
		t.Fatalf("locked effort must stay low, got %q", m.propsDraft[harness.PropertyEffort])
	}

	m, _ = HandleTestKey(m, "up") // thinking row (still selectable)
	m, _ = HandleTestKey(m, "enter") // save complete draft
	if m.pickingProps {
		t.Fatal("property draft must commit after thinking confirm")
	}
	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	props := install.EffectivePairProperties(hero, "cursor", "cursor-grok-4.6-low")
	if props["ef"] != "low" || props["fs"] != "false" {
		t.Fatalf("committed props=%v", props)
	}
}

func TestLunaCatalogCyclesEffortWithSpace(t *testing.T) {
	m, _ := newPickerTestModel(t)
	m.propsSvc.Catalog = propsCatalog(map[string]map[string]modelprops.CatalogProperty{
		"opencode-go/gpt-5.6-luna": {
			"fs": {Available: false},
			"th": {Available: false},
			"ef": {Available: true, Values: []string{"none", "low", "medium", "high", "xhigh", "max"}, Default: "medium"},
		},
	})
	m = SetModelOptionsForTest(m, []harnessmgr.ModelOption{
		{Model: "opencode-go/gpt-5.6-luna", Harness: "opencode"},
	})
	m = OpenHeroModelForTest(m)
	m = SetPaletteIndexForTest(m, 1) // opencode harness
	m, _ = HandleTestKey(m, "enter")
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")

	if !m.pickingProps {
		t.Fatal("luna must open property picker")
	}
	view := ViewForTest(m)
	if !strings.Contains(view, "Fast Mode: na") {
		t.Fatalf("fast mode must be unavailable:\n%s", view)
	}
	if !strings.Contains(view, "Reasoning effort: medium") {
		t.Fatalf("effort draft missing:\n%s", view)
	}
	if strings.Contains(view, "esc close · enter select") {
		t.Fatalf("property picker must not duplicate footer hints:\n%s", view)
	}
	m, _ = HandleTestKey(m, "down") // thinking row (disabled)
	m, _ = HandleTestKey(m, "down") // effort row
	m, _ = HandleTestKey(m, " ")    // space cycles medium → high
	if m.propsDraft[harness.PropertyEffort] != "high" {
		t.Fatalf("effort must cycle to high, got %q", m.propsDraft[harness.PropertyEffort])
	}
	m, _ = HandleTestKey(m, "enter")
	if m.pickingProps {
		t.Fatal("enter must save and close")
	}
}

func TestIncompleteCacheStillCyclesEffortWithSpace(t *testing.T) {
	m, _ := newPickerTestModel(t)
	m.propsSvc.Catalog = modelprops.LoadCatalog(assets.FS, m.svc.ProjectDir)
	if err := m.svc.Store.UpsertCapabilities(store.CapabilityCacheRow{
		Harness:        "opencode",
		Model:          "opencode-go/gpt-5.6-luna",
		PropertiesJSON: `{"fs":{"available":true,"values":["true","false"],"default":"false"},"ef":{"available":true,"values":["medium"],"default":"medium"}}`,
		RetrievedAt:    time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	m = SetModelOptionsForTest(m, []harnessmgr.ModelOption{
		{Model: "opencode-go/gpt-5.6-luna", Harness: "opencode"},
	})
	m = OpenHeroModelForTest(m)
	m = SetPaletteIndexForTest(m, 1)
	m, _ = HandleTestKey(m, "enter")
	m = SetPaletteIndexForTest(m, 0)
	m, _ = HandleTestKey(m, "enter")

	if !m.pickingProps {
		t.Fatal("property picker must open")
	}
	m, _ = HandleTestKey(m, "down")
	m, _ = HandleTestKey(m, "down")
	m, _ = HandleTestKey(m, " ")
	if m.propsDraft[harness.PropertyEffort] != "high" {
		t.Fatalf("space must cycle enriched effort values, got %q", m.propsDraft[harness.PropertyEffort])
	}
}
