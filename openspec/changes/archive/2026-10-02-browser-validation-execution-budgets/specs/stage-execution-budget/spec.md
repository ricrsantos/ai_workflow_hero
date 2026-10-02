# stage-execution-budget Specification (new)

## Purpose
Cumulative active wall-time budgets enforced independently of traffic, model cooperation, and adapter health. PRD-C17-001 FR-09–FR-11; ADR-105; amends baseline PRD §5.4.

## ADDED Requirements

### Requirement: timeout_minutes SHALL be cumulative active wall time per stage
Budgets SHALL accumulate across all attempts, partial waves, reconnects, and resumptions of a stage. Parallel agents SHALL count once by elapsed time. Execution, bounded preparation, and scheduler-owned work SHALL consume budget; human-credential/question/permission/approval waits with no active work SHALL pause it (a runnable sibling keeps it running); idle blocked/offline time SHALL be excluded; free chat SHALL have no budget. Consumed/remaining time, state (active/waiting/blocked/expired/interrupted), and pause reasons SHALL persist and display. Restart SHALL charge the last persisted checkpoint, mark interrupted, require `/hero-continue`, and never grant fresh budget. Zero balance SHALL require an explicit validated user increase.

#### Scenario: Parallel waves share one clock
- **WHEN** two Implementation agents run concurrently for 3 minutes
- **THEN** consumed budget increases by 3 minutes, not 6

#### Scenario: Restart after disconnect
- **WHEN** the TUI restarts mid-stage with 8min20s consumed
- **THEN** the stage shows Interrupted with consumed/remaining and waits for `/hero-continue` without resuming automatically

### Requirement: Expiry SHALL revoke acceptance before scoped cancellation
The TUI scheduler SHALL enforce the deadline during active Execute for all seven configured stages. On expiry it SHALL revoke the execution generation's acceptance authority, start scoped controlled cancellation, wait a bounded fixed termination interval, retain partial evidence/metrics, and persist reason `timeout`. Completion and expiry SHALL share one serialized acceptance boundary requiring positive remaining time; expired/cancelled/interrupted results SHALL NOT close stages, accept tasks/findings, or start successors. No unrelated harness processes SHALL be stopped. Watchdog, CheckHealth, and `handleHarnessHealthResult` SHALL remain observational.

#### Scenario: Late completion after expiry
- **WHEN** a harness result arrives after its generation was revoked
- **THEN** the result is recorded as evidence only and mutates no stage, finding, or checkbox

### Requirement: Progress SHALL be visible and warnings SHALL stay passive
The TUI SHALL show preparation phase, current coverage ID, profile ID, completed/required counts, remaining time, and sanitized tool lifecycle. After 60 seconds without coverage progress or 30 repetitive events without progress it SHALL emit one coalesced diagnostic for the current phase; warnings SHALL pause during human waits and SHALL never cancel, restart, or otherwise act correctively. OpenCode patch diagnostics SHALL use safe identifiers/counts, never raw content.

#### Scenario: Repetitive transport activity without progress
- **WHEN** 30 consecutive stream events arrive with no coverage progress
- **THEN** exactly one diagnostic appears and execution continues unchanged
