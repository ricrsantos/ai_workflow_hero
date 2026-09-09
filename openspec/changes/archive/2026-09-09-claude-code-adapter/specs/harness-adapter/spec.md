## ADDED Requirements

### Requirement: Claude SHALL implement the normalized Hero harness boundary

The registry SHALL resolve `claude` to an adapter implementing the existing session, execution, cancellation, status, dispatch, model-listing, permission, health, and normalized stream contracts. Claude-specific protocol values SHALL remain inside the adapter; shared TUI and engine code SHALL receive only Hero types (PRD-C13-001 §2; ADR-070–071).

#### Scenario: Registry resolves Claude
- **WHEN** the TUI resolves the explicit harness id `claude`
- **THEN** the returned adapter identifies itself as `claude` and satisfies the shared execution contract

#### Scenario: TUI uses the adapter boundary
- **WHEN** a workflow agent declares `harness: claude`
- **THEN** routing calls the registry adapter rather than shelling out to `claude` from TUI, engine, or workflow code

### Requirement: Claude events and decisions SHALL use existing normalized types

Claude text, thinking, tool, activity, warning, permission, session, usage, and health outcomes SHALL be represented through the existing normalized Hero contract, including Telegram-forwarded permission decisions. The Claude adapter SHALL not import Telegram implementation types or expose raw NDJSON to the TUI outside the Debug warning path.

#### Scenario: Claude permission reaches existing UI
- **WHEN** Claude emits a permission request
- **THEN** Hero routes it through the existing permission gate and any configured Telegram forwarding path

#### Scenario: Unknown Claude event is visible
- **WHEN** an unknown Claude event is received
- **THEN** the shared stream contains a warning and the TUI does not silently lose the event

