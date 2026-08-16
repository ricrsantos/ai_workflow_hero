# harness-fallback-chain Specification

## Purpose
Extend the three-level fallback chain to include harness+model pairs; never pick a third harness (ADR-033; amends ADR-008; UI-C04-001 §6).

## ADDED Requirements

### Requirement: Fallback SHALL try agent pair then fallback_model pair
When the agent's `(harness, model)` cannot run, Hero SHALL warn and try `fallback_model.harness` + `fallback_model.model` (ADR-033).

#### Scenario: Agent harness unavailable
- **WHEN** `planning_agent` is `cursor/composer-2.5` but Cursor is unavailable
- **THEN** the TUI prints a fallback warning and attempts the fallback_model pair

#### Scenario: Cross-harness fallback allowed by YAML
- **WHEN** `fallback_model.harness` is `opencode` and that pair is available
- **THEN** Execute uses OpenCode for the fallback attempt

### Requirement: Double failure SHALL stop and require hero-continue
If both the agent pair and fallback pair fail, Hero SHALL stop, explain the problem, and wait for `/hero-continue` — not invent a third harness (ADR-033).

#### Scenario: Hard stop copy
- **WHEN** both configured pairs fail
- **THEN** the message matches UI-C04-001 §6 structure naming harness and model for both attempts

### Requirement: Fallback warnings SHALL name harness and model
Every fallback SHALL emit a warning that includes both harness id and model id (UI-C04-001 §6).

#### Scenario: Warning text
- **WHEN** fallback occurs from cursor/composer-2.5 to opencode/anthropic/claude-sonnet-4
- **THEN** the warning lines name both pairs explicitly
