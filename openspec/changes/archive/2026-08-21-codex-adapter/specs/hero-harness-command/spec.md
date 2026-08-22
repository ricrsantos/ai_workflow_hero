# hero-harness-command Specification

## MODIFIED Requirements

### Requirement: /hero-harness picker SHALL include Codex row
The TUI harness command SHALL list Codex with enabled state and `(available)` or `(unavailable)` suffix derived from `IsAvailable` (UI-C06-001 §3).

#### Scenario: Unavailable but enabled allowed
- **WHEN** Codex is enabled but CLI is missing
- **THEN** the picker shows Codex enabled and unavailable, and Execute fails later with explicit error

#### Scenario: Last harness guard applies to Codex
- **WHEN** Codex is the only enabled harness and the user attempts to disable it
- **THEN** the command rejects with the existing last-harness error
