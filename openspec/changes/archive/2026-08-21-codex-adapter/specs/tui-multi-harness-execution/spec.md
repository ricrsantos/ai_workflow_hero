# tui-multi-harness-execution Specification

## MODIFIED Requirements

### Requirement: TUI Execute SHALL route codex harness to CodexAdapter
When YAML or freechat resolution yields `harness: codex`, Chat Execute and Dispatch SHALL call the registry-resolved Codex adapter (PRD-C06-001 §4.2; ADR-043).

#### Scenario: Stage agent on Codex
- **WHEN** `qa_agent.harness` is `codex` and Codex is available
- **THEN** TUI Execute uses CodexAdapter for that interação

#### Scenario: Enabled but unavailable fails explicitly
- **WHEN** Codex is enabled but `IsAvailable` fails
- **THEN** Execute surfaces CLI/auth errors without silently falling back to another harness unless the two-step fallback chain applies

### Requirement: Session ids SHALL remain harness-scoped for Codex
Hero SHALL bind Codex thread/session ids to harness `codex` in SQLite and memory so resume never crosses Cursor or OpenCode sessions (PRD-C06-001 §4.3).

#### Scenario: No Cursor resume for Codex thread
- **WHEN** a Codex thread id is stored for a stage
- **THEN** a subsequent Cursor Execute does not pass that id as `--resume`
