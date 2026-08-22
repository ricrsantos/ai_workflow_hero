# codex-model-catalog Specification

## Purpose
Add Codex-native model metadata to embedded/installed catalogs for context bar, metrics, and C5 property fallback (PRD-C06-001 §4.8).

## ADDED Requirements

### Requirement: assets/models SHALL include Codex-native model ids
Hero SHALL ship `assets/models/*.yml` entries keyed by Codex-native model ids returned by the adapter/`ListModels`, including `context_window` and C5 `properties` when known (PRD-C06-001 §4.8).

#### Scenario: Catalog lookup for Codex id
- **WHEN** Chat uses a Codex native model id present in the catalog
- **THEN** context bar and metrics can resolve `context_window` and property defaults

#### Scenario: Cursor and OpenCode entries preserved
- **WHEN** Codex catalog entries are added
- **THEN** existing Cursor slugs and OpenCode provider/model keys continue to resolve unchanged

### Requirement: Unknown ChatGPT-subsidized pricing SHALL not be invented
When USD rates for a Codex model are unknown, catalog and metrics SHALL leave cost unset or zero and emit a warning — Hero SHALL NOT invent subsidized pricing (PRD-C06-001 §4.4, §4.8).

#### Scenario: Missing USD rate
- **WHEN** metrics runs for a Codex id without catalog pricing
- **THEN** cost is unset/zero with a warning and execution continues

### Requirement: update-models documentation SHALL mention Codex ids
User-facing docs for `hero update-models` SHALL list Codex-native ids alongside Cursor and OpenCode entries (PRD-C06-001 §4.8).

#### Scenario: Docs mention Codex
- **WHEN** a user reads workflow-help or README model catalog guidance
- **THEN** Codex-native ids are named as supported catalog keys
