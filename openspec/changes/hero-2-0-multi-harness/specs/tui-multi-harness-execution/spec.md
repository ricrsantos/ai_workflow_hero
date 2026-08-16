# tui-multi-harness-execution Specification

## Purpose
Route TUI Execute to the correct harness adapter and display agent, model, and harness in Chat chrome (ADR-031; UI-C04-001 §5, §7).

## ADDED Requirements

### Requirement: Execute SHALL route by agent harness id
The TUI SHALL resolve the adapter from YAML `harness` for stage agents and from `freechat_default` for freechat (design D5).

#### Scenario: Mixed stage harnesses
- **WHEN** `planning_agent.harness` is `cursor` and `qa_agent.harness` is `opencode`
- **THEN** each stage Execute uses the corresponding adapter

### Requirement: Chat speaker label SHALL include harness
The green pane speaker line SHALL use `[LABEL - model · harness]` with lowercase harness id (UI-C04-001 §5; design D11).

#### Scenario: Orchestrator label
- **WHEN** the orchestrator runs on cursor with model cursor-grok-4.6
- **THEN** the label is `[ORCH - cursor-grok-4.6 · cursor]`

#### Scenario: Freechat HARN label
- **WHEN** freechat uses composer-2.5 on cursor
- **THEN** the label is `[HARN - composer-2.5 · cursor]`

### Requirement: TUI boot SHALL allow enabled but unavailable harnesses
The TUI SHALL start when at least one harness is enabled even if none are available, with a warning when no enabled harness is available (UI-C04-001 §7).

#### Scenario: Enabled OpenCode without CLI
- **WHEN** OpenCode is enabled but not on PATH
- **THEN** the TUI still launches and warns that no enabled harness is available

### Requirement: OpenCode serve SHALL NOT start at boot
Serve startup SHALL occur lazily on first OpenCode Execute only (UI-C04-001 §7).

#### Scenario: Boot without serve
- **WHEN** the TUI starts with OpenCode enabled
- **THEN** no `opencode serve` process is started until the first OpenCode Execute
