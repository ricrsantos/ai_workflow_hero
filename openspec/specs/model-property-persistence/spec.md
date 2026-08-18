# model-property-persistence Specification

## Purpose
Define backward-compatible per-pair selection persistence and atomic commit/cancel behavior for `/hero-model` (PRD-C05-001 §4.1; ADR-040).

## Requirements

### Requirement: `hero.json` SHALL persist string-valued properties per harness/native model pair

`hero.json` SHALL support an extensible `model_properties` map keyed by harness ID and native model ID, with string values for `fs`, `th`, `ef`, and future keys. Existing `harnesses.<harness>.model` and `freechat_default` fields SHALL continue to be updated, and a file without `model_properties` SHALL load without migration failure. Legacy `enable_fast_model` MAY seed an effective `fs` value when no new pair entry exists (PRD-C05-001 §4.1.5–8; §5; ADR-040).

#### Scenario: Existing project loads without the new map
- **WHEN** an existing C4 `hero.json` has no `model_properties` field
- **THEN** it parses successfully, preserves the selected pair, and exposes an empty property map until the user configures properties

#### Scenario: Two model pairs have independent values
- **WHEN** the user saves `fs=true, ef=high` for one harness/model and `fs=false, ef=low` for another
- **THEN** reading either pair returns only that pair's values and never carries a value across harnesses or native model IDs

### Requirement: Model/property selection SHALL commit atomically after final confirmation

The property picker SHALL keep model and property edits in memory and SHALL write the complete pair plus property selection only on main-picker Enter. The write SHALL use one atomic persistence operation; toggling a checkbox or choosing a secondary value SHALL not persist a partial state. Escape SHALL leave both the prior pair and prior saved properties unchanged (PRD-C05-001 §4.1.9; §4.4.6–7; UI-C05-001 §3; ADR-040/042).

#### Scenario: Enter saves the complete draft
- **WHEN** the user chooses a model, toggles `fs`, selects `th=max`, selects `ef=high`, and presses Enter on the main picker
- **THEN** `freechat_default`, the harness model field, and all three property values are present after one persisted selection commit

#### Scenario: Escape cancels the complete selection
- **WHEN** the user changes the model and one or more property rows and presses Escape before final confirmation
- **THEN** disk and the active in-memory pair/properties remain exactly as they were before opening `/hero-model`

#### Scenario: Model without selectable properties skips the submenu
- **WHEN** a selected model has no available editable C5 property
- **THEN** the model pair is saved immediately through the same complete-save path and the property picker is not shown

### Requirement: Freechat property choices SHALL remain separate from workflow configuration

Persisted `model_properties` SHALL apply only to ordinary freechat Chat and `/hero-new`. `/hero-model` SHALL not edit `agents.*` or `fallback_model` in `workflow-config.yml`; switching models SHALL restore valid choices for the selected pair and use `na` plus a warning for invalid choices (PRD-C05-001 §4.1.4/8; §4.5.6; §5; ADR-040).

#### Scenario: Switching models restores each draft
- **WHEN** the user switches from model A to model B and later returns to model A
- **THEN** model A's still-valid saved property choices are restored without changing model B's choices

#### Scenario: Workflow YAML is untouched
- **WHEN** the user saves freechat properties through `/hero-model`
- **THEN** the active cycle's agent and fallback YAML values are byte-for-byte unchanged
