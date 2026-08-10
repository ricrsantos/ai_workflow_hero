## ADDED Requirements

### Requirement: System SHALL define a stable HarnessAdapter interface
The Go codebase SHALL define a `HarnessAdapter` interface used by the engine/TUI for harness interaction, enabling future harnesses without rewriting the state machine (PRD-C01-001 §3; ADR-016). Concrete adapters beyond Cursor are out of scope (Deferred D1).

#### Scenario: Interface compiles with Cursor implementation
- **WHEN** the Cursor adapter is registered
- **THEN** it satisfies `HarnessAdapter` and is selectable as the 1.0 harness

### Requirement: Cursor adapter SHALL support chat-driven execution
The Cursor implementation SHALL preserve chat-driven stage execution via existing slash-command UX as the reliable baseline path (PRD-C01-001 §5.1; ADR-015; ADR-016).

#### Scenario: Chat entry runs full cycle
- **WHEN** the user drives the cycle from Cursor chat using `/hero:*` commands backed by CLI API
- **THEN** stage work executes through Cursor agents without requiring TUI

### Requirement: Cursor adapter SHALL offer best-effort dispatch for TUI/CLI run
`hero run` / TUI dispatch SHALL call the Cursor adapter’s dispatch method when available; if the harness cannot push execution, the adapter SHALL return a clear fallback instructing the user to continue via chat (ADR-016).

#### Scenario: Dispatch unavailable
- **WHEN** TUI requests dispatch and Cursor push APIs are unavailable
- **THEN** the command reports an actionable fallback and does not corrupt cycle state

#### Scenario: Dispatch attempted when available
- **WHEN** TUI/`hero run` requests dispatch and the Cursor adapter can initiate work
- **THEN** the engine records a `harness_invoked` (or equivalent) event and leaves stage state consistent with Running/pending semantics
