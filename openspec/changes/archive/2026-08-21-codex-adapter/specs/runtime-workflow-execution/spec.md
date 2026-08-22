# runtime-workflow-execution Specification

## MODIFIED Requirements

### Requirement: /hero-start SHALL Prepare Codex when YAML agents use harness codex
When any `agents.*` block uses `harness: codex` and Codex is enabled, `/hero-start` SHALL sync `.codex/agents` from workflow-config, reset the managed Codex app-server, and probe one configured agent before streaming orchestration prompts — matching the OpenCode Prepare-on-start pattern (PRD-C06-001 §4.3; design D9).

#### Scenario: Prepare runs for Codex agents
- **WHEN** `/hero-start` runs and `planning_agent.harness` is `codex` with Codex enabled
- **THEN** Hero syncs agent definitions, resets app-server, and probes before continuing

#### Scenario: Probe failure stops hero-start
- **WHEN** the Codex probe Execute fails
- **THEN** `/hero-start` stops with instructions to exit the TUI and retry, without silently continuing

### Requirement: Cursor IDE Runtime SHALL ignore harness codex
Cursor IDE slash execution SHALL continue to ignore `agents.*.harness` and SHALL NOT start Codex app-server (ADR-043).

#### Scenario: IDE ignores codex YAML
- **WHEN** workflow-config contains `harness: codex` for stage agents
- **THEN** Cursor IDE Runtime behavior is unchanged and does not read `.codex/` for Hero execution
