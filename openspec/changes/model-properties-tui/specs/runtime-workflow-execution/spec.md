# runtime-workflow-execution Specification

## Purpose

Keep workflow agent configuration authoritative while projecting its effective properties into TUI execution/status, without extending the Cursor IDE Runtime with freechat state (PRD-C05-001 §§4.5–5; ADR-040/042).

## MODIFIED Requirements

### Requirement: Workflow execution SHALL use agent YAML properties, not freechat selections

During a workflow/runtime execution, Hero SHALL derive normalized `fs` from `enable_fast_model`, `th` from `thinking`, and `ef` from `reasoning_effort` in the active agent/fallback resolution. It SHALL send those values to the selected adapter and project them in Chat as unvalidated/gray when capability validation is unavailable. `/hero-model` selections SHALL remain limited to ordinary Chat and `/hero-new`, and stage YAML SHALL not be modified (PRD-C05-001 §4.1.4/5; §4.5.6; §4.6.7; §5.1; ADR-040/042).

#### Scenario: Workflow agent overrides freechat properties
- **WHEN** ordinary Chat has `ef=high` saved but the active `qa_agent` YAML has `reasoning_effort: low`
- **THEN** the QA execution request and Chat property line use `ef=low`, shown as unvalidated/gray if not capability-validated, and do not change `hero.json`

#### Scenario: `/hero-new` uses freechat properties
- **WHEN** the user runs `/hero-new` from an empty Chat after saving a freechat pair with `fs=true`
- **THEN** the request uses the saved freechat `fs=true` value rather than an arbitrary stage-agent property

#### Scenario: Cursor IDE Runtime remains isolated
- **WHEN** the same project runs a Cursor IDE Runtime command outside the Hero TUI
- **THEN** the Runtime continues to resolve model properties from its workflow configuration and does not read or write `hero.json.model_properties`

### Requirement: C4 harness routing and lazy behavior SHALL remain unchanged

C5 property projection SHALL preserve C4 harness/model pair selection, two-level harness fallback, harness-scoped sessions, and lazy OpenCode serve lifecycle. A property rejection SHALL be reported as an execution error rather than triggering a silent C4 fallback (PRD-C05-001 §2/§5; ADR-041; C4 compatibility boundary).

#### Scenario: Workflow pair still follows C4 resolution
- **WHEN** the configured workflow pair is unavailable but the configured `fallback_model` pair is available
- **THEN** Hero uses the existing explicit fallback warning and pair routing while carrying only the resolved workflow property map

#### Scenario: OpenCode remains lazy at boot
- **WHEN** Hero TUI starts with OpenCode enabled and model properties persisted
- **THEN** it does not start OpenCode solely to preload metadata; the managed serve process is eligible only after explicit `/hero-model` refresh or execution
