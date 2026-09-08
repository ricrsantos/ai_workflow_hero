## ADDED Requirements

### Requirement: TUI workflow routing SHALL honor explicit Claude pairs without changing Cursor Runtime

The existing two-step resolution SHALL accept `harness: claude` and a native model, try that pair first, warn before the configured `fallback_model` pair, and stop after the configured pair fails. Cursor IDE Runtime slash dispatch SHALL remain Cursor-only and SHALL never launch Claude.

#### Scenario: Explicit Claude stage agent
- **WHEN** a TUI stage agent declares `harness: claude`
- **THEN** the stage executes through the Claude adapter with the configured native model and existing stage/session bookkeeping

#### Scenario: Claude fallback
- **WHEN** the Claude pair is unavailable but the configured fallback pair is available
- **THEN** Hero emits the existing explicit fallback warning and executes only the configured fallback pair

#### Scenario: No third fallback
- **WHEN** both the Claude agent pair and configured fallback pair fail while another harness is enabled
- **THEN** Hero hard-stops with `/hero-continue` guidance and does not invent a third harness

#### Scenario: Cursor Runtime isolation
- **WHEN** a Cursor IDE Runtime slash command or Cursor-only flow runs
- **THEN** it behaves as before and never dispatches to Claude based on workflow YAML

### Requirement: Claude preparation SHALL be non-blocking to the TUI and scoped to marked assets

When workflow configuration uses Claude agents, start preparation SHALL update only managed Claude agent fields and perform any required compatibility probe through asynchronous TUI commands. It SHALL not create a persistent Claude server or block Bubble Tea `Update` with process or filesystem I/O.

#### Scenario: Claude prepare failure
- **WHEN** the configured Claude CLI or marked agent preparation is invalid
- **THEN** `/hero-start` reports an actionable failure and does not partially rewrite user-owned instructions

