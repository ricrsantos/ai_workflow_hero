# codex-adapter Specification

## Purpose
Implement `CodexAdapter` as a full `HarnessAdapter` using Hero-managed `codex app-server` over stdio/JSON-RPC, with OpenCodeAdapter behavioral parity (ADR-044, ADR-045; PRD-C06-001 §4.2–4.4).

## ADDED Requirements

### Requirement: Codex adapter SHALL implement HarnessAdapter
`internal/adapters/codex` SHALL implement `IsAvailable`, sessions, `Execute`, `Cancel`, `Status`, `Dispatch`, and `ListModels` as the third harness implementation (ADR-016 amended; ADR-043).

#### Scenario: Adapter identity
- **WHEN** the registry resolves harness id `codex`
- **THEN** the Codex adapter's `Name()` returns `codex`

#### Scenario: IsAvailable without CLI
- **WHEN** `codex` is not on PATH
- **THEN** `IsAvailable` returns an error indicating the harness is unavailable

### Requirement: Execute SHALL use Hero-managed codex app-server
The adapter SHALL start `codex app-server` lazily on first Execute, communicate via stdio/JSON-RPC (JSONL), and use thread/turn APIs — not ephemeral `codex exec` per prompt (ADR-044; design D3).

#### Scenario: Lazy app-server start
- **WHEN** the first Codex Execute runs in a TUI session
- **THEN** the adapter starts a child `codex app-server` and records it in the project registry

#### Scenario: Stream execution
- **WHEN** `Execute` is called with `Stream: true`
- **THEN** the adapter streams mapped `StreamDelta` events via JSON-RPC and invokes `OnStreamDelta`

### Requirement: Adapter SHALL NOT attach to foreign app-server
The adapter SHALL only connect to the `codex app-server` process Hero started for this project (ADR-044).

#### Scenario: Foreign process ignored
- **WHEN** a user-started `codex app-server` is already running outside Hero
- **THEN** the adapter does not attach to it and starts its own Hero-managed child instead

### Requirement: Only codex package MAY exec codex CLI
Workflow, TUI, and engine code SHALL call `HarnessAdapter` methods — not shell out to `codex` directly (ADR-003 layer separation).

#### Scenario: TUI calls interface
- **WHEN** the TUI runs a Codex agent interação
- **THEN** it calls `HarnessAdapter.Execute` on the registry-resolved adapter

### Requirement: Unauthenticated Codex SHALL fail with login guidance
When Codex is not authenticated or auth expired, the adapter SHALL return an explicit error instructing the user to run `codex login` in a regular terminal. Hero SHALL NOT prompt for an API key or embed interactive login in the TUI (ADR-047; PRD-C06-001 §4.4).

#### Scenario: Missing ChatGPT login
- **WHEN** Execute runs and the app-server reports unauthenticated state
- **THEN** Chat shows the unauthenticated error with `codex login` suggestion and no API key field

#### Scenario: No in-TUI OAuth
- **WHEN** Codex requires authentication
- **THEN** Hero does not open a browser or Bubble Tea login flow inside the TUI

### Requirement: Codex adapter SHALL accept PATH binary without version pin
The adapter SHALL use whatever `codex` binary is on PATH. If `app-server` is missing or the JSON-RPC handshake fails, it SHALL return an explicit incompatible/not-installed error (ADR-047).

#### Scenario: Incompatible CLI
- **WHEN** the installed `codex` lacks a usable `app-server` subcommand or handshake fails
- **THEN** Execute fails with an explicit message naming incompatible or not installed, not a silent hang

### Requirement: Unknown Codex app-server events SHALL warn
Unrecognized JSON-RPC/event payloads SHALL emit `StreamKindWarning` with harness name and truncated payload; they SHALL NOT be silently dropped (ADR-045; PRD-C06-001 §4.3).

#### Scenario: Unknown event type
- **WHEN** the app-server emits an event type not in the handler list
- **THEN** the adapter emits `StreamKindWarning` and continues streaming

### Requirement: Codex adapter SHALL map C5 properties natively
The adapter SHALL map normalized `fs`, `th`, and `ef` from `ExecuteRequest.Properties` to native app-server fields where supported. Unsupported properties SHALL behave as `na`. Rejection SHALL be explicit without silent retry (PRD-C06-001 §4.3; ADR-045).

#### Scenario: Native property payload
- **WHEN** Execute receives normalized `fs=true`, `th=max`, and `ef=high`
- **THEN** only the Codex adapter converts those values into the native JSON-RPC payload

#### Scenario: Property rejection is explicit
- **WHEN** the app-server rejects a selected property for the model
- **THEN** Execute fails with a property-aware error and does not retry with the property stripped
