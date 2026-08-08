## ADDED Requirements

### Requirement: Go engine SHALL own deterministic cycle and stage transitions
The system SHALL implement an AI Loop / Workflow Engine in Go that advances cycle stages, enforces approval gates, tracks iterations and timeouts, and applies concurrency locks without performing LLM reasoning (PRD-C01-001 §3, §5.5; ADR-012; ADR-003 amendment).

#### Scenario: Stage advance after successful close
- **WHEN** the current stage is eligible to complete and approval rules are satisfied
- **THEN** the engine persists the transition and selects the next enabled stage from the cycle config snapshot

#### Scenario: Approval required before advance
- **WHEN** the current stage has `require_human_approval: true` and work has finished pending approval
- **THEN** the engine remains in a pending-approval state and does not advance until an approve/reject/cancel/finish command is applied

### Requirement: Engine SHALL enforce iteration and timeout escalation
The engine SHALL honor per-stage `max_iterations` and `timeout_minutes` from the imported workflow config, mark the stage Escalated when exhausted, and apply extra iterations granted via continue (PRD-C01-001 §3; baseline PRD §5.4 semantics).

#### Scenario: Iteration budget exhausted
- **WHEN** a stage reaches its iteration or timeout limit without a terminal resolution
- **THEN** the engine sets Escalated status and refuses further work until `continue` grants extra iterations or the user cancels/finishes

### Requirement: Engine SHALL serialize conflicting writers
The engine SHALL prevent concurrent conflicting mutations of the same active cycle and SHALL fail with a clear CLI error when another session holds the lock (UI-C01-001 §6).

#### Scenario: Second writer while cycle locked
- **WHEN** a mutating CLI command runs while another process holds the cycle lock
- **THEN** the command exits non-zero with an actionable busy/lock error and does not corrupt store state

### Requirement: Engine behavior SHALL be unit-testable without LLMs
Engine transition rules SHALL be covered by colocated Go tests using a real temporary SQLite store (ADR-009) with no harness or LLM dependency.

#### Scenario: Table-driven transition test
- **WHEN** `go test` runs for the engine package
- **THEN** approve/reject/cancel/finish/continue and advance paths are asserted against store state without invoking Cursor
