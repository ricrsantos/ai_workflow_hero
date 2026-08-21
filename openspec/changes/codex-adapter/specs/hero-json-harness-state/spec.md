# hero-json-harness-state Specification

## MODIFIED Requirements

### Requirement: hero.json SHALL track Codex harness enablement and model
Project `hero.json` SHALL support `harnesses.codex.enabled`, `harnesses.codex.model`, and C5 `model_properties` entries keyed by harness `codex` and native model id (PRD-C06-001 §4.10; ADR-048).

#### Scenario: Default disabled on fresh 2.5 install
- **WHEN** a new project installs with only Cursor selected
- **THEN** `harnesses.codex.enabled` is false and no freechat default points to Codex unless chosen

#### Scenario: Upgrade from 2.4.x adds codex disabled
- **WHEN** a 2.4.x project upgrades to 2.5.0
- **THEN** `harnesses.codex.enabled` is false and existing Cursor/OpenCode settings are preserved

### Requirement: freechat_default MAY select Codex pair
When the user picks Codex in `/hero-model`, Hero SHALL persist `freechat_default {harness: codex, model: <native id>}` without modifying cycle agent YAML (PRD-C06-001 §4.8).

#### Scenario: Codex freechat pair saved
- **WHEN** the user completes `/hero-model` with harness Codex and a native model
- **THEN** `freechat_default.harness` is `codex` and `freechat_default.model` is the native id
