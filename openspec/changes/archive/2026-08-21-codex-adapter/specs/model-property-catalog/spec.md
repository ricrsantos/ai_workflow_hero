# model-property-catalog Specification

## MODIFIED Requirements

### Requirement: Catalog fallback SHALL support Codex-native model keys
Embedded and installed `assets/models/*.yml` SHALL include Codex-native ids so C5 resolver can fall back when live discovery is pending or failed (PRD-C06-001 §4.8).

#### Scenario: Codex catalog properties
- **WHEN** a Codex model id exists in catalog with C5 `properties`
- **THEN** `/hero-model` property picker can render `fs`/`th`/`ef` rows from catalog fallback

#### Scenario: Per-harness persistence key
- **WHEN** the user saves properties for a Codex model
- **THEN** `hero.json` `model_properties` is keyed by harness `codex` and the native model id
