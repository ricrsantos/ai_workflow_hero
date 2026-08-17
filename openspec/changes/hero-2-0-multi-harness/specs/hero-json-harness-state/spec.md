# hero-json-harness-state Specification

## Purpose
Persist per-harness enabled state and freechat default pair in `hero.json`, migrating from legacy `cli.tools` (ADR-034, ADR-037; PRD-C04-001 §4.11).

## ADDED Requirements

### Requirement: hero.json SHALL store enabled state per harness
Each supported harness (`cursor`, `opencode`) SHALL have a `harnesses.<id>` entry with `enabled`, `model`, and existing flags such as `enable_fast_model` (design D3).

#### Scenario: Fresh install with Cursor only
- **WHEN** the user selects only Cursor during install
- **THEN** `harnesses.cursor.enabled` is true and `harnesses.opencode.enabled` is false

#### Scenario: Fresh install with both harnesses
- **WHEN** the user selects Cursor and OpenCode during install
- **THEN** both `harnesses.cursor.enabled` and `harnesses.opencode.enabled` are true

### Requirement: Upgrade from 1.x SHALL enable Cursor only
`hero upgrade` from a 1.x project SHALL set `harnesses.cursor.enabled` true from legacy `cli.tools` and SHALL NOT auto-enable OpenCode (ADR-034).

#### Scenario: Upgrade cursor-only 1.x project
- **WHEN** upgrade runs on a project with `cli.tools: ["cursor"]` and no `harnesses` block
- **THEN** `harnesses.cursor.enabled` is true and OpenCode remains disabled with no `.opencode/` projection

### Requirement: Freechat default SHALL be a harness+model pair
`hero.json` SHALL persist `freechat_default.harness` and `freechat_default.model` used by TUI freechat and `/hero-new` (ADR-037; amends ADR-030).

#### Scenario: Default pair after install
- **WHEN** install completes with Cursor enabled
- **THEN** `freechat_default.harness` is an enabled harness and `freechat_default.model` is empty until `/hero-model`
