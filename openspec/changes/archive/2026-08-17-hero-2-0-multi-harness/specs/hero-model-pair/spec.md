# hero-model-pair Specification

## Purpose
`/hero-model` selects a default (harness, model) pair for freechat and `/hero-new`; does not edit stage agent YAML (ADR-037; amends ADR-030; UI-C04-001 §4).

## ADDED Requirements

### Requirement: hero-model SHALL pick harness then models
`/hero-model` SHALL show a harness submenu first, then the models of the selected harness — never a mixed model-only list (UI-C04-001 §4; design D10).

#### Scenario: Harness submenu then OpenCode models
- **WHEN** the user opens `/hero-model` with both harnesses enabled
- **THEN** the first screen lists Cursor and OpenCode, and choosing OpenCode lists only OpenCode native model ids

### Requirement: hero-model SHALL NOT invent a default model
The TUI SHALL leave the freechat default empty until the user selects a pair. Install and migration SHALL NOT write `composer-2.5` (or any other slug) as `freechat_default.model`.

#### Scenario: Fresh TUI session
- **WHEN** `freechat_default.model` is empty
- **THEN** Chat and `/hero-new` require `/hero-model` before Execute

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
