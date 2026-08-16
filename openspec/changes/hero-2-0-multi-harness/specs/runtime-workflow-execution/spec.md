# runtime-workflow-execution Specification

## Purpose
Runtime workflow config carries `harness` per agent; Cursor IDE ignores harness and remains Cursor-only (ADR-031, ADR-032).

## MODIFIED Requirements

### Requirement: workflow-config agents SHALL declare harness for TUI
Each agent block in cycle `workflow-config.yml` SHALL include `harness` and native `model` for TUI Execute routing (ADR-032).

#### Scenario: YAML consumed by TUI
- **WHEN** the orchestrator dispatches a stage agent in the TUI
- **THEN** model resolution reads both `harness` and `model` from YAML

### Requirement: Cursor IDE Runtime SHALL ignore harness field
Cursor IDE chat and slash commands SHALL continue using `model` as a Cursor slug and SHALL NOT start OpenCode (ADR-031).

#### Scenario: IDE planning agent
- **WHEN** planning runs in Cursor IDE chat
- **THEN** `agents.planning_agent.harness` is ignored and `model` is passed to Cursor Task as today

### Requirement: fallback_model SHALL include harness
`fallback_model` in workflow-config SHALL include `harness` and `model` for pair-aware fallback (ADR-033).

#### Scenario: Fallback YAML
- **WHEN** fallback is triggered
- **THEN** both `fallback_model.harness` and `fallback_model.model` are read
