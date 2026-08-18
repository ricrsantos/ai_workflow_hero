package modelprops

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// PropertySource identifies where a snapshot came from (ADR-039).
type PropertySource string

const (
	// SourceAPI means a successful live harness capability response.
	SourceAPI PropertySource = "api"
	// SourceCache means the project SQLite capability cache.
	SourceCache PropertySource = "cache"
	// SourceCatalog means embedded/installed assets/models/*.yml.
	SourceCatalog PropertySource = "catalog"
	// SourceUnknown means no metadata source existed.
	SourceUnknown PropertySource = "unknown"
)

// Warning texts required by UI-C05-001 §5.
const (
	WarningMissingCatalog = "No catalog is available for the selected model. Model properties will use their default values."
	WarningStaleCache     = "Using stale model properties because the harness API is unavailable."
	WarningInvalidated    = "The selected value is no longer supported by this model and was reset to na."
)

// Snapshot is the best available normalized capability view for one pair.
type Snapshot struct {
	HarnessID   string
	ModelID     string
	Properties  map[string]harness.PropertyCapability
	Source      PropertySource
	RetrievedAt time.Time
	Stale       bool
	Warning     string
}

// Property returns the capability for a normalized key.
func (s Snapshot) Property(key string) harness.PropertyCapability {
	return s.Properties[key]
}

// SelectableKeys returns the C5 keys that are editable for this snapshot.
func (s Snapshot) SelectableKeys() []string {
	var out []string
	for _, key := range harness.PropertyKeys() {
		if cap, ok := s.Properties[key]; ok && cap.Available && len(cap.AcceptedValues) > 0 {
			out = append(out, key)
		}
	}
	return out
}

// HasSelectableProperty reports whether the picker should open the property screen.
func (s Snapshot) HasSelectableProperty() bool {
	return len(s.SelectableKeys()) > 0
}

// Resolve applies the strict source precedence: live API → project cache →
// catalog → unknown/na (ADR-039). Per-property merging keeps API-authoritative
// data for the properties it covers while allowing partial responses to fall
// back per key.
func Resolve(harnessID, modelID string, api *harness.ModelCapabilities, apiErr error, cacheRow *store.CapabilityCacheRow, cacheFound bool, cat Catalog) Snapshot {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	modelID = strings.TrimSpace(modelID)
	snap := Snapshot{HarnessID: harnessID, ModelID: modelID, Properties: map[string]harness.PropertyCapability{}}

	apiProps := map[string]harness.PropertyCapability{}
	if api != nil {
		for _, p := range api.Properties {
			apiProps[p.Key] = p
		}
		snap.RetrievedAt = api.RetrievedAt
	}

	cacheProps := map[string]harness.PropertyCapability{}
	if cacheFound && cacheRow != nil {
		cacheProps = decodeCacheProperties(cacheRow.PropertiesJSON)
		if snap.RetrievedAt.IsZero() {
			if ts, err := time.Parse(time.RFC3339, cacheRow.RetrievedAt); err == nil {
				snap.RetrievedAt = ts
			}
		}
	}

	usedAny := false
	usedCache := false
	for _, key := range harness.PropertyKeys() {
		if cap, ok := apiProps[key]; ok {
			snap.Properties[key] = cap
			usedAny = true
			continue
		}
		if cap, ok := cacheProps[key]; ok {
			snap.Properties[key] = cap
			usedAny = true
			usedCache = true
			continue
		}
		if cat != nil {
			if p, ok := cat.CatalogValues(modelID, key); ok && p.HasProperty {
				snap.Properties[key] = harness.PropertyCapability{
					Key:            key,
					AcceptedValues: append([]string(nil), p.Values...),
					DefaultValue:   p.Default,
					Available:      p.Available,
				}
				usedAny = true
				continue
			}
		}
		snap.Properties[key] = harness.PropertyCapability{Key: key, Available: false}
	}

	if len(apiProps) > 0 {
		snap.Source = SourceAPI
	} else if usedCache {
		snap.Source = SourceCache
	} else if usedAny {
		snap.Source = SourceCatalog
	} else {
		snap.Source = SourceUnknown
		snap.Warning = WarningMissingCatalog
	}

	// A failed live refresh falling back to cache is reported as stale (the
	// persisted timestamp is retained; no age threshold is invented).
	if apiErr != nil && usedCache {
		snap.Stale = true
		snap.Warning = WarningStaleCache
	}
	return snap
}

func decodeCacheProperties(raw string) map[string]harness.PropertyCapability {
	out := map[string]harness.PropertyCapability{}
	var data map[string]struct {
		Available bool     `json:"available"`
		Values    []string `json:"values"`
		Default   string   `json:"default"`
	}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return out
	}
	for key, opt := range data {
		out[key] = harness.PropertyCapability{
			Key:            key,
			AcceptedValues: append([]string(nil), opt.Values...),
			DefaultValue:   strings.TrimSpace(opt.Default),
			Available:      opt.Available,
		}
	}
	return out
}

// EncodeCapabilities serializes capabilities into the normalized cache JSON shape.
func EncodeCapabilities(caps harness.ModelCapabilities) string {
	data := map[string]any{}
	for _, p := range caps.Properties {
		data[p.Key] = map[string]any{
			"available": p.Available,
			"values":    p.AcceptedValues,
			"default":   p.DefaultValue,
		}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// EffectiveValues reconciles saved user choices against the snapshot (ADR-040):
// a valid saved value wins over defaults; a removed/invalid value becomes "na"
// and is reported in invalidated; unset values fall back to API default → catalog
// default → "na".
func EffectiveValues(snap Snapshot, saved map[string]string) (values map[string]string, invalidated map[string]string) {
	values = make(map[string]string, len(harness.PropertyKeys()))
	for _, key := range harness.PropertyKeys() {
		cap, ok := snap.Properties[key]
		if !ok || !cap.Available {
			values[key] = "na"
			continue
		}
		chosen := strings.TrimSpace(saved[key])
		if chosen != "" && chosen != "na" {
			if containsValue(cap.AcceptedValues, chosen) {
				values[key] = chosen
				continue
			}
			// The saved value is no longer accepted: invalidate with a warning.
			if invalidated == nil {
				invalidated = map[string]string{}
			}
			invalidated[key] = chosen
			values[key] = "na"
			continue
		}
		if def := strings.TrimSpace(cap.DefaultValue); def != "" {
			values[key] = def
		} else {
			values[key] = "na"
		}
	}
	return values, invalidated
}

func containsValue(values []string, want string) bool {
	for _, v := range values {
		if strings.TrimSpace(v) == want {
			return true
		}
	}
	return false
}
