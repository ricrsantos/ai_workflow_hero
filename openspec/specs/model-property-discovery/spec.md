# model-property-discovery Specification

## Purpose
Define optional harness capability discovery, normalized model-property metadata, source precedence, and non-blocking refresh for C5 (PRD-C05-001 §§4.2–4.3; ADR-038–039).

## Requirements

### Requirement: Capability discovery SHALL be optional and normalized

Harness adapters SHALL retain the existing model-ID `ListModels` contract and MAY implement a separate capability-discovery contract. A discovery result SHALL normalize each property into a Hero-owned key, dynamic accepted string values, an optional default, and availability. Native response shapes and transport mapping data SHALL remain adapter-owned, and the TUI SHALL consume only the normalized representation (PRD-C05-001 §4.2.1–3; ADR-038).

#### Scenario: Adapter has model listing but no capability API
- **WHEN** an enabled adapter implements `ListModels` but not capability discovery
- **THEN** model selection continues and the property resolver uses cache/catalog/`na` fallback without treating the adapter as failed

#### Scenario: Native capability response is normalized
- **WHEN** an adapter returns native metadata for a model with dynamic thinking and effort values
- **THEN** the resolver exposes `th` and `ef` as normalized keys with the returned string arrays, defaults, and availability, without exposing the native response shape to the TUI

#### Scenario: Future key is discovered
- **WHEN** a live response contains a property key other than `fs`, `th`, or `ef`
- **THEN** the metadata may be retained in cache but the C5 TUI and execution projection ignore that key

### Requirement: Capability resolution SHALL be API-first with deterministic fallbacks

For a harness/model pair, Hero SHALL resolve live API metadata first, then the project SQLite cache, then embedded/installed local catalogs, and finally an unknown snapshot whose visible values are `na`. Successful API data SHALL replace cached accepted-value arrays and defaults. A cache SHALL remain usable regardless of age after API failure, with its refresh timestamp retained and staleness reported when relevant. Missing all sources SHALL produce a yellow warning without blocking the selected model (PRD-C05-001 §4.2.4–12; §4.3.3–5; UI-C05-001 §5–6; ADR-039).

#### Scenario: Live API supersedes cache
- **WHEN** live discovery succeeds with a different accepted-value array for `ef`
- **THEN** the live array and default become authoritative, replace the cached values, and are used for the next property picker opening

#### Scenario: Stale cache is used after API failure
- **WHEN** live discovery fails and an older project cache row exists
- **THEN** the cached capabilities remain selectable, the cached timestamp is retained, and the TUI shows a yellow stale-data warning

#### Scenario: Catalog fallback supplies a model and properties
- **WHEN** live listing/capability discovery and cache are unavailable but an installed catalog contains the native model ID and `th` metadata
- **THEN** the model remains selectable, the catalog values are used, and unavailable `fs`/`ef` values are represented as `na`

#### Scenario: No metadata source exists
- **WHEN** the selected model has no live response, cache row, or local catalog entry
- **THEN** Chat continues with visible `na` values and a warning equivalent to `No catalog is available for the selected model. Model properties will use their default values.`

### Requirement: Model-property refresh SHALL be background and stable for an open selector

Opening `/hero-model` SHALL immediately use the best in-memory, project-cache, or catalog snapshot and SHALL start refresh work for every enabled harness. Refresh SHALL not run solely at TUI boot, and OpenCode SHALL remain lazy until the explicit picker action. A result that completes while a selector is open SHALL be stored for the next opening and SHALL not move current rows or replace the active property draft (PRD-C05-001 §4.2.4–7; §4.4.8; UI-C05-001 §§2,5; ADR-039).

#### Scenario: Picker is usable while refresh is pending
- **WHEN** `/hero-model` opens and live discovery has not completed
- **THEN** the picker renders local/cache values immediately and remains navigable without waiting for the refresh

#### Scenario: All enabled harnesses refresh from explicit picker use
- **WHEN** the user opens `/hero-model` with Cursor and OpenCode enabled
- **THEN** refresh jobs are started for both enabled harnesses, while no OpenCode serve process is started during ordinary TUI boot

#### Scenario: Refresh does not reorder the active selector
- **WHEN** a background refresh completes while the model or property list is open
- **THEN** the visible rows and cursor remain unchanged, and the refreshed snapshot is applied only when `/hero-model` is opened again
