package harness

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Normalized C5 model-property keys (ADR-038). The TUI renders and executes only
// these three keys in C5; adapters may accept future keys through the extensible
// cache/JSON shapes but the C5 projection ignores them.
const (
	PropertyFast   = "fs"
	PropertyThink  = "th"
	PropertyEffort = "ef"
)

// PropertyKeys returns the visible C5 property keys in stable render order.
func PropertyKeys() []string {
	return []string{PropertyFast, PropertyThink, PropertyEffort}
}

// PropertyCapability describes one normalized model property (ADR-038).
type PropertyCapability struct {
	// Key is a Hero-owned normalized key (fs, th, ef, or a future key).
	Key string
	// AcceptedValues are dynamic string values supplied by the harness or catalog,
	// preserved in source order. No fixed enum is defined.
	AcceptedValues []string
	// DefaultValue is the preferred default when the user has not chosen a value.
	// Empty means "no default supplied".
	DefaultValue string
	// Available reports whether the property is editable for this model.
	Available bool
}

// ModelCapabilities is the normalized capability result for one harness/model pair.
type ModelCapabilities struct {
	HarnessID   string
	ModelID     string
	Properties  []PropertyCapability
	RetrievedAt time.Time
}

// CloneModelCapabilities returns a deep copy of normalized capability data.
// AcceptedValues is copied because adapters commonly build it from a decoded
// response and the cache/TUI must not share mutable backing arrays.
func CloneModelCapabilities(in ModelCapabilities) ModelCapabilities {
	out := in
	if len(in.Properties) == 0 {
		return out
	}
	out.Properties = make([]PropertyCapability, len(in.Properties))
	for i, property := range in.Properties {
		out.Properties[i] = property
		out.Properties[i].AcceptedValues = append([]string(nil), property.AcceptedValues...)
	}
	return out
}

// NormalizeModelCapabilities trims identifiers and dynamic string values while
// preserving property/value order and unknown future keys.  C5 projection
// filtering happens later at the request/UI boundary, not while caching.
func NormalizeModelCapabilities(in ModelCapabilities) ModelCapabilities {
	out := CloneModelCapabilities(in)
	out.HarnessID = strings.TrimSpace(strings.ToLower(out.HarnessID))
	out.ModelID = strings.TrimSpace(out.ModelID)
	for i := range out.Properties {
		property := &out.Properties[i]
		property.Key = strings.TrimSpace(property.Key)
		property.DefaultValue = strings.TrimSpace(property.DefaultValue)
		values := property.AcceptedValues[:0]
		for _, value := range property.AcceptedValues {
			if value = strings.TrimSpace(value); value != "" {
				values = append(values, value)
			}
		}
		property.AcceptedValues = values
	}
	return out
}

// Property returns the capability for a normalized key, or nil.
func (c ModelCapabilities) Property(key string) *PropertyCapability {
	for i := range c.Properties {
		if c.Properties[i].Key == key {
			return &c.Properties[i]
		}
	}
	return nil
}

// ModelPropertyDiscoverer is the optional capability-discovery contract (ADR-038).
// Adapters that cannot describe model options simply do not implement it; that is
// a normal fallback condition, not an adapter failure.
type ModelPropertyDiscoverer interface {
	DiscoverModelProperties(ctx context.Context, modelID string) (ModelCapabilities, error)
}

// NormalizeProperties returns a defensive copy of props containing only the
// normalized C5 keys with trimmed non-empty values. Unknown, empty, and "na"
// entries are dropped: "na" is an effective-display sentinel and must never
// reach adapter transport (design §1; ADR-038/041).
func NormalizeProperties(props map[string]string) map[string]string {
	if len(props) == 0 {
		return nil
	}
	out := make(map[string]string, len(props))
	for _, key := range PropertyKeys() {
		v := strings.TrimSpace(props[key])
		if v == "" || v == "na" {
			continue
		}
		out[key] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// CloneProperties returns a deep copy of a normalized property map.
func CloneProperties(props map[string]string) map[string]string {
	if len(props) == 0 {
		return nil
	}
	out := make(map[string]string, len(props))
	for k, v := range props {
		out[k] = v
	}
	return out
}

// PropertyRejectionError identifies a property the harness refused for a model.
// The TUI/execution layer must not strip the property, retry silently, or convert
// the rejection into an unrelated pair fallback (ADR-041).
type PropertyRejectionError struct {
	Property string
	Harness  string
	Model    string
	Err      error
}

func (e *PropertyRejectionError) Error() string {
	prop := strings.TrimSpace(e.Property)
	harness := strings.TrimSpace(e.Harness)
	model := strings.TrimSpace(e.Model)
	msg := fmt.Sprintf("property %q rejected", prop)
	if harness != "" {
		msg += " by " + harness
	}
	if model != "" {
		msg += fmt.Sprintf(" for model %q", model)
	}
	if e.Err != nil {
		detail := strings.TrimSpace(e.Err.Error())
		if detail != "" {
			msg += ": " + detail
		}
	}
	return msg
}

// Unwrap implements errors.Unwrap for PropertyRejectionError.
func (e *PropertyRejectionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// IsPropertyRejection reports whether err is (or wraps) a PropertyRejectionError.
func IsPropertyRejection(err error) bool {
	var pre *PropertyRejectionError
	return errors.As(err, &pre)
}

// PropertyRejection builds a property-aware rejection error for a normalized key.
func PropertyRejection(property, harnessID, model string, cause error) error {
	return &PropertyRejectionError{Property: property, Harness: harnessID, Model: model, Err: cause}
}
