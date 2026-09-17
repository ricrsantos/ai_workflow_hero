package modelprops

import (
	"math"
	"testing"
)

func costCatalog() Catalog {
	return Catalog{
		"claude-sonnet-5": {
			Provider: "anthropic", Input: 2.00, Output: 10.00,
			Currency: "usd", Unit: "per_1m_tokens", HasPricing: true,
		},
		"gpt-5.4": {
			Provider: "codex", Input: 2.50, Output: 15.00,
			Currency: "usd", Unit: "per_1m_tokens", HasPricing: true,
		},
		"free-local": {
			Provider: "opencode", Currency: "usd", Unit: "per_1m_tokens",
		},
		"eur-model": {
			Provider: "opencode", Input: 1, Output: 2,
			Currency: "eur", Unit: "per_1m_tokens", HasPricing: true,
		},
	}
}

func TestEstimateCatalogCostUSD_PricesFromCatalogRates(t *testing.T) {
	cost, warn := EstimateCatalogCostUSD(costCatalog(), "claude-sonnet-5", 1_000_000, 500_000)
	if warn != "" {
		t.Fatalf("unexpected warning: %s", warn)
	}
	// 1M input @ $2.00 + 0.5M output @ $10.00 = 2.00 + 5.00
	if math.Abs(cost-7.00) > 1e-9 {
		t.Fatalf("cost=%v want 7.00", cost)
	}
}

func TestEstimateCatalogCostUSD_SmallCountsStayProportional(t *testing.T) {
	cost, warn := EstimateCatalogCostUSD(costCatalog(), "claude-sonnet-5", 1000, 250)
	if warn != "" {
		t.Fatalf("unexpected warning: %s", warn)
	}
	want := (1000.0/1e6)*2.00 + (250.0/1e6)*10.00
	if math.Abs(cost-want) > 1e-12 {
		t.Fatalf("cost=%v want %v", cost, want)
	}
}

func TestEstimateCatalogCostUSD_UnpricedCasesReturnZeroWithWarning(t *testing.T) {
	cat := costCatalog()
	for _, tc := range []struct{ name, model string }{
		{"chatgpt subsidized", "gpt-5.4"},
		{"no rates", "free-local"},
		{"non-usd", "eur-model"},
		{"unknown", "not-in-catalog"},
		{"empty", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cost, warn := EstimateCatalogCostUSD(cat, tc.model, 1_000_000, 1_000_000)
			if cost != 0 {
				t.Fatalf("cost=%v want 0", cost)
			}
			if warn == "" {
				t.Fatal("expected a warning explaining why cost is unset")
			}
		})
	}
}

func TestEstimateCatalogCostUSD_NilCatalogAndNegativeTokens(t *testing.T) {
	if cost, warn := EstimateCatalogCostUSD(nil, "claude-sonnet-5", 10, 10); cost != 0 || warn == "" {
		t.Fatalf("nil catalog: cost=%v warn=%q", cost, warn)
	}
	cost, warn := EstimateCatalogCostUSD(costCatalog(), "claude-sonnet-5", -100, -100)
	if cost != 0 || warn != "" {
		t.Fatalf("negative tokens: cost=%v warn=%q", cost, warn)
	}
}
