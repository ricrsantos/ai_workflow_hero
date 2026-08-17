# opencode-adapter Specification

## Purpose
Implement `OpenCodeAdapter` as a full `HarnessAdapter` using Hero-managed `opencode serve` and its HTTP API (ADR-035; PRD-C04-001 §4.2, §4.8).

## ADDED Requirements

### Requirement: OpenCode adapter SHALL implement HarnessAdapter
`internal/adapters/opencode` SHALL implement `IsAvailable`, sessions, `Execute`, `Cancel`, `Status`, and `ListModels` (ADR-016 amended).

#### Scenario: Adapter identity
- **WHEN** the registry resolves harness id `opencode`
- **THEN** the OpenCode adapter's `Name()` returns `opencode`

#### Scenario: IsAvailable without CLI
- **WHEN** `opencode` is not on PATH
- **THEN** `IsAvailable` returns an error indicating the harness is unavailable

### Requirement: Execute SHALL use opencode serve HTTP API
The adapter SHALL start `opencode serve` lazily on first Execute, use localhost with an ephemeral port, and communicate via the serve HTTP API — not `opencode run` per prompt (ADR-035; design D6).

#### Scenario: Lazy serve start
- **WHEN** the first OpenCode Execute runs in a TUI session
- **THEN** the adapter starts a child `opencode serve` and records it in the project registry

#### Scenario: Stream execution
- **WHEN** `Execute` is called with `Stream: true`
- **THEN** the adapter streams deltas via the HTTP API and invokes `OnStreamDelta`

### Requirement: Adapter SHALL NOT attach to foreign serve
The adapter SHALL only connect to the `opencode serve` process Hero started for this project (ADR-035).

#### Scenario: Foreign port ignored
- **WHEN** an unrelated `opencode serve` is already listening on a known port
- **THEN** the adapter does not attach to it and starts its own child instead

### Requirement: Only opencode package MAY exec opencode CLI
Workflow, TUI, and engine code SHALL call `HarnessAdapter` methods — not shell out to `opencode` directly (ADR-003 layer separation).

#### Scenario: TUI calls interface
- **WHEN** the TUI runs an OpenCode agent interação
- **THEN** it calls `HarnessAdapter.Execute` on the registry-resolved adapter
