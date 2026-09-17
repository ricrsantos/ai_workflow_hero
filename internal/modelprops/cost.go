package modelprops

import (
	"fmt"
	"strings"
)

// perMillionTokens is the only pricing unit Hero catalogs ship. A row using a
// different unit is treated as unpriced rather than converted by guesswork.
const perMillionTokens = "per_1m_tokens"

// EstimateCatalogCostUSD estimates USD cost from the embedded/installed catalog.
// Unknown model ids and Codex ChatGPT-subsidized rows (provider:codex, no invented
// rates) return cost 0 with a warning. Never panics (PRD-C06-001 §4.8).
//
// This is the single cost authority for the TUI runtime: stage metrics are
// priced here, from the same `models/*.yml` rates the Config screen displays.
func EstimateCatalogCostUSD(cat Catalog, modelID string, inputTokens, outputTokens int64) (cost float64, warning string) {
	defer func() {
		if r := recover(); r != nil {
			cost = 0
			warning = fmt.Sprintf("catalog cost panic recovered for %q: %v", modelID, r)
		}
	}()
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return 0, "model id empty; cost left unset/zero"
	}
	if cat == nil || !cat.HasModel(modelID) {
		return 0, fmt.Sprintf("unknown model %q not in catalog; cost left unset/zero", modelID)
	}
	row := cat[modelID]
	if strings.EqualFold(strings.TrimSpace(row.Provider), "codex") {
		// The Codex catalog carries list rates for reference, but ChatGPT
		// subscription usage is not billed per token. Pricing it would invent
		// a charge the user never sees.
		return 0, fmt.Sprintf("model %q is ChatGPT-subsidized; cost left unset/zero", modelID)
	}
	if !row.HasPricing {
		return 0, fmt.Sprintf("model %q has no USD rates in catalog; cost left unset/zero", modelID)
	}
	if unit := strings.TrimSpace(row.Unit); unit != "" && !strings.EqualFold(unit, perMillionTokens) {
		return 0, fmt.Sprintf("model %q prices in %q, not %s; cost left unset/zero", modelID, unit, perMillionTokens)
	}
	if currency := strings.TrimSpace(row.Currency); currency != "" && !strings.EqualFold(currency, "usd") {
		return 0, fmt.Sprintf("model %q prices in %q, not USD; cost left unset/zero", modelID, currency)
	}
	if inputTokens < 0 {
		inputTokens = 0
	}
	if outputTokens < 0 {
		outputTokens = 0
	}
	cost = (float64(inputTokens)/1e6)*row.Input + (float64(outputTokens)/1e6)*row.Output
	if cost < 0 {
		return 0, fmt.Sprintf("model %q produced a negative cost; cost left unset/zero", modelID)
	}
	return cost, ""
}
