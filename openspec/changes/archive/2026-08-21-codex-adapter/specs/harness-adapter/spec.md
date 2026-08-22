# harness-adapter Specification

## MODIFIED Requirements

### Requirement: Harness registry SHALL resolve a third Codex adapter
The harness manager SHALL register Cursor, OpenCode, and Codex adapters. TUI Execute SHALL resolve adapters by YAML `harness` id including `codex` (ADR-043; PRD-C06-001 §4.2).

#### Scenario: Registry returns Codex
- **WHEN** `SupportedIDs()` is queried
- **THEN** the list includes `cursor`, `opencode`, and `codex`

#### Scenario: Codex Execute uses CodexAdapter
- **WHEN** an agent block specifies `harness: codex`
- **THEN** TUI routes Execute to `internal/adapters/codex` without importing JSON-RPC types outside that package

### Requirement: Codex adapter SHALL own native property mapping
Codex SHALL serialize normalized C5 values using native app-server request fields. Shared TUI and workflow code SHALL NOT construct Codex JSON-RPC payloads (PRD-C06-001 §4.3; ADR-045).

#### Scenario: Codex builds a native payload
- **WHEN** Codex executes a native model id with normalized C5 values
- **THEN** only the Codex adapter converts those values into the native JSON-RPC payload

### Requirement: Session binding SHALL prevent cross-harness resume
A Codex thread/session id SHALL never be resumed as Cursor or OpenCode, and vice versa (PRD-C06-001 §4.3; ADR-044).

#### Scenario: Codex thread not reused as OpenCode
- **WHEN** a stored session id belongs to harness `codex`
- **THEN** OpenCode Execute does not resume it with `--resume` or HTTP session APIs

### Requirement: Codex stream events SHALL follow OpenCode warning discipline
During Codex streaming, unrecognized app-server events SHALL emit `StreamKindWarning` consistent with OpenCode unknown SSE handling (ADR-045).

#### Scenario: Unknown Codex app-server event
- **WHEN** a Codex JSON-RPC notification is not handled
- **THEN** the adapter emits `StreamKindWarning` and continues streaming
