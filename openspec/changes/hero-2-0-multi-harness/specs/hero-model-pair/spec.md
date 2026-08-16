# hero-model-pair Specification

## Purpose
`/hero-model` selects a default (harness, model) pair for freechat and `/hero-new`; does not edit stage agent YAML (ADR-037; amends ADR-030; UI-C04-001 §4).

## ADDED Requirements

### Requirement: hero-model picker SHALL show Model and Harness columns
The picker SHALL list models with an associated harness column — never model-only rows (UI-C04-001 §4; design D10).

#### Scenario: Mixed harness list
- **WHEN** both Cursor and OpenCode are enabled
- **THEN** the picker shows Cursor slugs and OpenCode provider/model ids with harness labels

### Requirement: Selection SHALL persist pair to hero.json
Choosing a row SHALL write `freechat_default` and `harnesses.<harness>.model` (ADR-037).

#### Scenario: Select OpenCode model
- **WHEN** the user selects `anthropic/claude-sonnet-4` for OpenCode
- **THEN** `freechat_default` stores harness `opencode` and that model id

### Requirement: hero-model SHALL NOT modify cycle agent YAML
Stage agents SHALL continue to use `workflow-config.yml` harness and model; `/hero-model` only affects freechat and `/hero-new` (ADR-037).

#### Scenario: Stage agent unchanged
- **WHEN** the user changes the default model via `/hero-model`
- **THEN** `agents.qa_agent` in the active cycle YAML is unchanged

### Requirement: Input status SHALL show model and harness
After selection, the Chat input status SHALL display `{model} · {harness}` (UI-C04-001 §4).

#### Scenario: Status line
- **WHEN** freechat default is `composer-2.5` on cursor
- **THEN** input status includes `composer-2.5 · cursor`
