package modelprops

import (
	"fmt"
	"strings"
)

// EstimateCatalogCostUSD estimates USD cost from the embedded/installed catalog.
// Unknown model ids and Codex ChatGPT-subsidized rows (provider:codex, no invented
// rates) return cost 0 with a warning. Never panics (PRD-C06-001 §4.8).
//
// Non-Codex providers are out of scope here — callers that need Cursor/OpenCode
// API rates should read pricing YAML directly (orchestration Metrics Procedure).
func EstimateCatalogCostUSD(cat Catalog, modelID string, inputTokens, outputTokens int64) (cost float64, warning string) {
	defer func() {
		if r := recover(); r != nil {
			cost = 0
			warning = fmt.Sprintf("catalog cost panic recovered for %q: %v", modelID, r)
		}
	}()
	_ = inputTokens
	_ = outputTokens
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return 0, "model id empty; cost left unset/zero"
	}
	if cat == nil || !cat.HasModel(modelID) {
		return 0, fmt.Sprintf("unknown model %q not in catalog; cost left unset/zero", modelID)
	}
	if strings.EqualFold(strings.TrimSpace(cat[modelID].Provider), "codex") {
		// Codex catalog intentionally ships 0.00 USD rates (no ChatGPT invention).
		return 0, fmt.Sprintf("model %q has no USD rates in catalog; cost left unset/zero", modelID)
	}
	return 0, ""
}
