package codex

import (
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// nativePropertyOptions maps normalized C5 values to Codex turn/start fields.
// Unsupported properties are omitted (effective na). The TUI never builds
// Codex JSON-RPC payloads (ADR-045).
//
// Mapping:
//   - ef → effort (reasoning effort)
//   - th → summary (reasoning summary verbosity)
//   - fs → no stable native field; omitted (na)
func nativePropertyOptions(props map[string]string) map[string]any {
	props = harness.NormalizeProperties(props)
	if len(props) == 0 {
		return nil
	}
	out := make(map[string]any, len(props))
	if ef := strings.TrimSpace(props[harness.PropertyEffort]); ef != "" {
		out["effort"] = ef
	}
	if th := strings.TrimSpace(props[harness.PropertyThink]); th != "" {
		out["summary"] = mapThinkingSummary(th)
	}
	// fs (fast): Codex has no stable turn-level fast flag in the documented API.
	// Leaving it out is the na behavior; app-server rejection of effort/summary
	// still surfaces as PropertyRejectionError.
	_ = props[harness.PropertyFast]
	if len(out) == 0 {
		return nil
	}
	return out
}

func mapThinkingSummary(th string) string {
	switch strings.ToLower(strings.TrimSpace(th)) {
	case "off", "none", "false", "0":
		return "none"
	case "max", "detailed", "high", "true", "on":
		return "detailed"
	case "concise", "low", "medium", "auto":
		return strings.ToLower(strings.TrimSpace(th))
	default:
		return th
	}
}

func mapPropertyRejection(err error, model string, props map[string]string) error {
	if err == nil {
		return nil
	}
	props = harness.NormalizeProperties(props)
	if len(props) == 0 {
		return mapRPCError(err)
	}
	lower := strings.ToLower(err.Error())
	blame := func(names ...string) bool {
		for _, n := range names {
			if strings.Contains(lower, n) {
				return true
			}
		}
		return false
	}
	switch {
	case blame("effort", "reasoning_effort", "reasoning effort", `"ef"`):
		if props[harness.PropertyEffort] != "" {
			return &harness.PropertyRejectionError{Property: harness.PropertyEffort, Harness: adapterName, Model: model, Err: err}
		}
	case blame("summary", "thinking", `"th"`):
		if props[harness.PropertyThink] != "" {
			return &harness.PropertyRejectionError{Property: harness.PropertyThink, Harness: adapterName, Model: model, Err: err}
		}
	case blame("fast", `"fs"`):
		if props[harness.PropertyFast] != "" {
			return &harness.PropertyRejectionError{Property: harness.PropertyFast, Harness: adapterName, Model: model, Err: err}
		}
	case blame("option", "property", "not supported", "invalid"):
		for _, key := range harness.PropertyKeys() {
			if props[key] != "" {
				return &harness.PropertyRejectionError{Property: key, Harness: adapterName, Model: model, Err: err}
			}
		}
	}
	return mapRPCError(err)
}

func mapRPCError(err error) error {
	if err == nil {
		return nil
	}
	if isAuthMessage(err.Error()) {
		return &AuthError{Err: err}
	}
	return err
}

// UnsupportedPropertyError documents an explicit na rejection when Hero must
// refuse a property the catalog marked unsupported for Codex.
type UnsupportedPropertyError struct {
	Property string
	Model    string
}

func (e *UnsupportedPropertyError) Error() string {
	return fmt.Sprintf("property %q unsupported (na) by codex for model %q", e.Property, e.Model)
}
