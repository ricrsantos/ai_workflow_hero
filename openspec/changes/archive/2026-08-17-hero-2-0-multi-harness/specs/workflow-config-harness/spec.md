# workflow-config-harness Specification

## Purpose
Require `harness` and native `model` on every agent and `fallback_model` in `workflow-config.yml` (ADR-032; PRD-C04-001 §4.3).

## ADDED Requirements

### Requirement: Every agent SHALL declare harness and model
Each entry under `agents` and `fallback_model` SHALL include `harness` and `model` as harness-native identifiers (design D2).

#### Scenario: Valid workflow config
- **WHEN** `planning_agent` has `harness: cursor` and `model: composer-2.5`
- **THEN** workflowconfig validation succeeds

#### Scenario: Missing harness
- **WHEN** an agent block omits `harness`
- **THEN** validation fails with an error naming the agent

### Requirement: Execute SHALL error when harness is missing at runtime
The TUI/engine SHALL NOT infer a default harness when `harness` is absent at Execute time (ADR-032).

#### Scenario: Missing harness at hero-start
- **WHEN** `/hero-start` runs against YAML without `harness` on an agent
- **THEN** Execute stops with an error explaining the missing field

### Requirement: hero-new SHALL inject harness from enabled set
`/hero-new` SHALL inject missing `harness` from enabled project harnesses: one enabled → that id; both Cursor and OpenCode enabled → `cursor`; never a disabled harness (ADR-032).

#### Scenario: Single enabled harness injection
- **WHEN** only OpenCode is enabled and a new cycle is created
- **THEN** injected agents receive `harness: opencode`

#### Scenario: Preserve explicit import
- **WHEN** a prior cycle YAML already has `harness: opencode` on an agent
- **THEN** `/hero-new` keeps that value
