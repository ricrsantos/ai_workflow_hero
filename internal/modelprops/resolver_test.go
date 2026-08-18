package modelprops

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func apiCaps(props ...harness.PropertyCapability) *harness.ModelCapabilities {
	return &harness.ModelCapabilities{
		HarnessID:   "opencode",
		ModelID:     "opencode-go/deepseek-v4-pro",
		Properties:  props,
		RetrievedAt: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC),
	}
}

func cacheRow(raw, ts string) *store.CapabilityCacheRow {
	return &store.CapabilityCacheRow{Harness: "opencode", Model: "opencode-go/deepseek-v4-pro", PropertiesJSON: raw, RetrievedAt: ts}
}

func TestResolveAPIFirstComplete(t *testing.T) {
	api := apiCaps(
		harness.PropertyCapability{Key: "fs", AcceptedValues: []string{"true", "false"}, DefaultValue: "false", Available: true},
		harness.PropertyCapability{Key: "th", AcceptedValues: []string{"off", "max"}, DefaultValue: "off", Available: true},
		harness.PropertyCapability{Key: "ef", AcceptedValues: []string{"low", "high"}, DefaultValue: "low", Available: true},
	)
	cached := `{"ef":{"available":true,"values":["medium"],"default":"medium"}}`
	snap := Resolve("opencode", "opencode-go/deepseek-v4-pro", api, nil, cacheRow(cached, "2026-08-15T09:00:00Z"), true, nil)
	if snap.Source != SourceAPI || snap.Stale {
		t.Fatalf("source=%s stale=%v", snap.Source, snap.Stale)
	}
	ef := snap.Property("ef")
	if len(ef.AcceptedValues) != 2 || ef.AcceptedValues[1] != "high" {
		t.Fatalf("API must replace cached values: %+v", ef)
	}
	if !snap.HasSelectableProperty() {
		t.Fatal("complete API snapshot must be selectable")
	}
}

func TestResolvePartialAPIMergesPerProperty(t *testing.T) {
	api := apiCaps(
		harness.PropertyCapability{Key: "th", AcceptedValues: []string{"off", "max"}, DefaultValue: "off", Available: true},
	)
	cached := `{"fs":{"available":true,"values":["true","false"],"default":"false"},"ef":{"available":true,"values":["high"],"default":"high"}}`
	snap := Resolve("opencode", "opencode-go/deepseek-v4-pro", api, nil, cacheRow(cached, "2026-08-15T09:00:00Z"), true, nil)
	if snap.Source != SourceAPI {
		t.Fatalf("source=%s want api", snap.Source)
	}
	if !snap.Property("fs").Available || !snap.Property("ef").Available || !snap.Property("th").Available {
		t.Fatalf("partial API must merge with cache per property: %+v", snap.Properties)
	}
}

func TestResolveStaleCacheAfterAPIFailure(t *testing.T) {
	cached := `{"th":{"available":true,"values":["off","max"],"default":"off"}}`
	snap := Resolve("opencode", "opencode-go/deepseek-v4-pro", nil, errors.New("serve down"),
		cacheRow(cached, "2026-08-15T09:00:00Z"), true, nil)
	if snap.Source != SourceCache {
		t.Fatalf("source=%s want cache", snap.Source)
	}
	if !snap.Stale {
		t.Fatal("failed refresh must mark cache stale")
	}
	if snap.Warning != WarningStaleCache {
		t.Fatalf("warning=%q", snap.Warning)
	}
	if snap.Property("th").Available != true {
		t.Fatal("stale cache must remain selectable")
	}
	if snap.RetrievedAt.Format(time.RFC3339) != "2026-08-15T09:00:00Z" {
		t.Fatalf("persisted timestamp must be retained: %v", snap.RetrievedAt)
	}
}

func TestResolveCacheWithoutAPIAttempt(t *testing.T) {
	// Cursor-style: no capability API at all → normal fallback, not stale.
	cached := `{"fs":{"available":true,"values":["true","false"],"default":"false"}}`
	snap := Resolve("cursor", "composer-2.5", nil, nil, cacheRow(cached, "2026-08-15T09:00:00Z"), true, nil)
	if snap.Source != SourceCache || snap.Stale {
		t.Fatalf("source=%s stale=%v (missing API is a normal fallback)", snap.Source, snap.Stale)
	}
	if snap.Warning != "" {
		t.Fatalf("no warning expected without a failed refresh: %q", snap.Warning)
	}
}

func TestResolveCatalogFallback(t *testing.T) {
	cat := Catalog{
		"cursor-grok-4.6": CatalogModel{Properties: map[string]CatalogProperty{
			"th": {Available: true, Values: []string{"off", "max"}, Default: "off", HasProperty: true},
		}},
	}
	snap := Resolve("cursor", "cursor-grok-4.6", nil, nil, nil, false, cat)
	if snap.Source != SourceCatalog {
		t.Fatalf("source=%s want catalog", snap.Source)
	}
	if !snap.Property("th").Available || len(snap.Property("th").AcceptedValues) != 2 {
		t.Fatalf("catalog th lost: %+v", snap.Property("th"))
	}
	if snap.Property("fs").Available || snap.Property("ef").Available {
		t.Fatal("absent catalog properties must be unavailable")
	}
	if snap.Warning != "" {
		t.Fatalf("catalog fallback is not a warning condition: %q", snap.Warning)
	}
}

func TestResolveUnknownSnapshotWarns(t *testing.T) {
	snap := Resolve("cursor", "mystery-model", nil, nil, nil, false, nil)
	if snap.Source != SourceUnknown {
		t.Fatalf("source=%s want unknown", snap.Source)
	}
	if snap.Warning != WarningMissingCatalog {
		t.Fatalf("warning=%q", snap.Warning)
	}
	if snap.HasSelectableProperty() {
		t.Fatal("unknown snapshot must have no selectable property")
	}
	for _, key := range harness.PropertyKeys() {
		if snap.Property(key).Available {
			t.Fatalf("%s must be unavailable", key)
		}
	}
}

func TestEffectiveValuesPreservesValidAndInvalidatesRemoved(t *testing.T) {
	snap := Snapshot{Properties: map[string]harness.PropertyCapability{
		"fs": {Key: "fs", AcceptedValues: []string{"true", "false"}, DefaultValue: "false", Available: true},
		"th": {Key: "th", AcceptedValues: []string{"off", "max"}, DefaultValue: "off", Available: true},
		"ef": {Key: "ef", AcceptedValues: []string{"low", "high"}, DefaultValue: "low", Available: true},
	}}
	saved := map[string]string{"fs": "true", "th": "legacy", "ef": "high"}
	values, invalidated := EffectiveValues(snap, saved)
	if values["fs"] != "true" || values["ef"] != "high" {
		t.Fatalf("valid saved values must be preserved: %v", values)
	}
	if values["th"] != "na" {
		t.Fatalf("removed value must become na: %v", values)
	}
	if invalidated["th"] != "legacy" {
		t.Fatalf("invalidated=%v", invalidated)
	}
}

func TestEffectiveValuesDefaultPrecedence(t *testing.T) {
	snap := Snapshot{Properties: map[string]harness.PropertyCapability{
		"fs": {Key: "fs", AcceptedValues: []string{"true", "false"}, DefaultValue: "false", Available: true},
		"th": {Key: "th", AcceptedValues: []string{"off", "max"}, Available: true}, // no default anywhere
		"ef": {Key: "ef", AcceptedValues: []string{"high"}, DefaultValue: "high", Available: false},
	}}
	values, invalidated := EffectiveValues(snap, nil)
	if values["fs"] != "false" {
		t.Fatalf("default must apply: %v", values)
	}
	if values["th"] != "na" {
		t.Fatalf("missing default must resolve to na: %v", values)
	}
	if values["ef"] != "na" {
		t.Fatalf("unavailable property must be na: %v", values)
	}
	if len(invalidated) != 0 {
		t.Fatalf("nothing was saved; no invalidation: %v", invalidated)
	}
}

func TestEffectiveValuesIgnoresSavedForUnavailable(t *testing.T) {
	snap := Snapshot{Properties: map[string]harness.PropertyCapability{
		"ef": {Key: "ef", Available: false},
	}}
	values, _ := EffectiveValues(snap, map[string]string{"ef": "high"})
	if values["ef"] != "na" {
		t.Fatalf("unavailable saved value must not surface: %v", values)
	}
}

func TestEncodeDecodeCapabilitiesRoundTrip(t *testing.T) {
	caps := *apiCaps(
		harness.PropertyCapability{Key: "th", AcceptedValues: []string{"off", "max"}, DefaultValue: "off", Available: true},
	)
	raw := EncodeCapabilities(caps)
	if !strings.Contains(raw, `"off"`) || !strings.Contains(raw, `"max"`) {
		t.Fatalf("encoded: %s", raw)
	}
	decoded := decodeCacheProperties(raw)
	if len(decoded["th"].AcceptedValues) != 2 || decoded["th"].DefaultValue != "off" {
		t.Fatalf("decoded: %+v", decoded)
	}
	if decodeCacheProperties("{not json") != nil && len(decodeCacheProperties("{not json")) != 0 {
		t.Fatal("malformed cache json must decode empty")
	}
}
