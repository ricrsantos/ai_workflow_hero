# opencode-model-catalog Specification

## Purpose
Update `assets/models/*.yml` with OpenCode-native model ids and pricing so metrics and the Chat context bar resolve for OpenCode harness runs (PRD-C04-001 §4.10; design D12).

## ADDED Requirements

### Requirement: Model catalog SHALL include OpenCode-native ids
`assets/models/*.yml` (installed as `.workflow-hero/models/*.yml`) SHALL include entries keyed by OpenCode `provider/model` identifiers with `input`, `output`, cache fields, and `context_window` where known.

#### Scenario: Anthropic OpenCode id lookup
- **WHEN** metrics resolve model `anthropic/claude-sonnet-4`
- **THEN** pricing and `context_window` are found in the catalog

#### Scenario: Cursor slug still resolves
- **WHEN** metrics resolve model `composer-2.5`
- **THEN** pricing is found via existing `cursor.yml` slug entries

### Requirement: Pricing SHALL reuse known provider rates
New OpenCode id entries SHALL use rates from existing provider YAML for the same underlying model — prices SHALL NOT be invented (PRD-C04-001 §4.10).

#### Scenario: Rate consistency
- **WHEN** an OpenCode id maps to a model already priced under `anthropic.yml`
- **THEN** the OpenCode key uses the same per-token rates as the sibling entry

### Requirement: Unknown model id SHALL not crash metrics
When a model id is absent from the catalog, Hero SHALL warn and leave cost unset or zero without panicking (PRD-C04-001 §4.10).

#### Scenario: Unknown OpenCode id
- **WHEN** Execute completes with an unlisted OpenCode model id
- **THEN** metrics recording succeeds with a warning and no context bar crash

### Requirement: update-models SHALL distribute new catalog entries
The installed model files under `.workflow-hero/models/` SHALL receive the same OpenCode entries after install or upgrade.

#### Scenario: Post-install catalog
- **WHEN** install materializes `.workflow-hero/models/`
- **THEN** OpenCode-native ids are present alongside Cursor slugs
