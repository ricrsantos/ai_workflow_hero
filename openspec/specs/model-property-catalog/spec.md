# model-property-catalog Specification

## Purpose
Define the local model catalog shape and value/default reconciliation used when live metadata is unavailable or when a refreshed model changes its capabilities (PRD-C05-001 §§4.2–4.3; ADR-039).

## Requirements

### Requirement: Local catalogs SHALL support dynamic per-model properties

Embedded and installed `assets/models/*.yml` catalogs SHALL accept an optional per-model `properties` map. Each entry SHALL support `available`, dynamic string `values`, and an optional string `default`; model map keys SHALL be usable as native model IDs. The parser SHALL remain compatible with existing pricing-only entries and SHALL not hardcode a fixed enum of thinking or effort values (PRD-C05-001 §4.2.8/12; §4.3.1–2; UI-C05-001 §6).

#### Scenario: Pricing-only catalog remains valid
- **WHEN** a model YAML entry has pricing and `context_window` but no `properties` block
- **THEN** the catalog loads successfully and its property metadata is treated as unavailable/`na`

#### Scenario: Dynamic values are preserved
- **WHEN** a catalog defines `th` values `off`, `max`, and `vendor-custom` with a default of `max`
- **THEN** all three strings and the default are returned in source order without enum filtering

#### Scenario: Catalog supplies a model row
- **WHEN** live model listing and cache are unavailable but a catalog contains a native model key
- **THEN** that key is included in the selectable model list for its harness

### Requirement: Property reconciliation SHALL preserve valid choices and invalidate removed choices

When API or catalog metadata is refreshed, an existing user value SHALL be preserved if it remains accepted. A removed or otherwise invalid value SHALL become `na`, and the user SHALL receive a yellow warning. Effective defaults SHALL use live API default, then local catalog default, then `na`; unsupported properties remain visible but unavailable. Unknown future keys may remain in stored metadata but SHALL not enter the C5 projection (PRD-C05-001 §4.3.3–7; ADR-040/042).

#### Scenario: Valid saved value survives refresh
- **WHEN** a saved `ef=high` value remains in the refreshed accepted-value array
- **THEN** the draft continues to use `high` instead of replacing it with the refreshed default

#### Scenario: Removed saved value is invalidated
- **WHEN** a saved `th=legacy` value is absent from refreshed metadata
- **THEN** the effective value is `na`, a yellow warning identifies the unsupported value, and `legacy` is not sent as a freechat property

#### Scenario: Missing default resolves to `na`
- **WHEN** a property is available with accepted values but neither live API nor catalog supplies a default
- **THEN** its initial effective value is `na` and the user may choose an accepted value in the picker
