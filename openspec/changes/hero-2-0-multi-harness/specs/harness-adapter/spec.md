# harness-adapter Specification

## Purpose
Extend harness adapter layer for multi-harness registry with Cursor and OpenCode implementations (ADR-016 amended; PRD-C04-001 §4.2).

## MODIFIED Requirements

### Requirement: Hero SHALL support Cursor and OpenCode adapters
The harness registry SHALL resolve `cursor` to the existing Cursor adapter and `opencode` to `OpenCodeAdapter` (ADR-016; PRD-C04-001 §4.2).

#### Scenario: Registry lookup cursor
- **WHEN** harness id is `cursor`
- **THEN** the Cursor adapter is returned without behavioral regression

#### Scenario: Registry lookup opencode
- **WHEN** harness id is `opencode`
- **THEN** the OpenCode adapter is returned

### Requirement: SupportedToolIDs SHALL include cursor and opencode
`harness.SupportedToolIDs` (or successor API) SHALL list both supported harness ids for install and doctor messages (ADR-034).

#### Scenario: Supported list
- **WHEN** install or doctor queries supported harnesses
- **THEN** the list is `cursor` and `opencode`

### Requirement: Cursor adapter Execute behavior SHALL not regress
Changes to `internal/adapters/cursor` SHALL be limited to registry integration; existing Execute, stream-json, cancel, and session tests SHALL pass (PRD-C04-001 §4.1).

#### Scenario: Cursor adapter tests green
- **WHEN** `go test ./internal/adapters/cursor/...` runs after C4 changes
- **THEN** all existing tests pass without rewriting Cursor CLI invocation semantics
