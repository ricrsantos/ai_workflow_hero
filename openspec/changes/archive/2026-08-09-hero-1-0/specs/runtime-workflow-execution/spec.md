## MODIFIED Requirements

### Requirement: Runtime SHALL apply uniform approval/control-loop semantics
Every stage closure SHALL summarize output, request approval when required, persist stage transition and metrics to SQLite via the Hero CLI API, show a metrics summary in chat (tokens, duration, cost) with a pointer to `hero metrics`, and advance according to control commands (`/hero:approve`, `/hero:reject`, `/hero:cancel`, `/hero:finish`) that invoke the corresponding CLI mutations (PRD-C01-001 §5.4; ADR-012; ADR-014). Runtime SHALL NOT instruct agents to write `workflow.md` or `metrics.md` as operational state (Deferred D9).

#### Scenario: Stage requiring human approval
- **WHEN** a stage has `require_human_approval: true`
- **THEN** Runtime waits for an approval command and does not auto-advance until command resolution via CLI API

#### Scenario: Stage without required human approval
- **WHEN** a stage has `require_human_approval: false`
- **THEN** Runtime auto-completes the stage, persists state via CLI API, posts summary, and proceeds to the next configured stage

#### Scenario: Stage close without cycle markdown writes
- **WHEN** a stage closes successfully
- **THEN** operational state is updated through `hero` CLI commands and agents are not required to edit `workflow.md` or `metrics.md`

### Requirement: Runtime SHALL enforce iteration and timeout escalation behavior
Each stage SHALL honor `max_iterations` and `timeout_minutes`, escalate with `Human Approval = Escalated` when exhausted, and support `/hero:continue` extra-iteration grants tracked in SQLite via CLI (PRD-C01-001 §3; baseline PRD §5.4).

#### Scenario: Iteration budget exhausted
- **WHEN** a stage reaches its iteration or timeout limit
- **THEN** Runtime escalates and waits for `/hero:continue` (backed by `hero continue`) before additional iterations

### Requirement: Runtime SHALL maintain cycle and project metrics via store and summary file
Runtime SHALL persist per-cycle stage metrics in SQLite through the CLI API and SHALL continue to update project-level `metrics-summary.md` with aggregate values as designed, including token and cost estimates based on model pricing references (PRD-C01-001 §5.2, §5.4; baseline PRD §5.10). Per-cycle `metrics.md` SHALL NOT be required as operational state.

#### Scenario: Closing a stage with metrics update
- **WHEN** a stage closes successfully or with explicit terminal state
- **THEN** Runtime persists cycle metrics via CLI API and updates aggregated project summary before proceeding

#### Scenario: Chat metrics pointer
- **WHEN** Runtime shows the stage metrics summary in chat
- **THEN** it points the user to `hero metrics` for full cycle details (clickable path links remain for project files where applicable)

## ADDED Requirements

### Requirement: Runtime slash commands SHALL use CLI as API for operational state
`/hero:status`, `/hero:approve`, `/hero:reject`, `/hero:cancel`, `/hero:finish`, `/hero:continue`, `/hero:new`, `/hero:archive`, `/hero:resume`, and related commands SHALL read and write Hero operational state through documented `hero` subcommands rather than editing cycle markdown (PRD-C01-001 §5.1; ADR-014; UI-C01-001 §5).

#### Scenario: Status command uses hero status
- **WHEN** the user runs `/hero:status`
- **THEN** the command obtains stage state from `hero status` (SQLite) instead of reading `workflow.md`

#### Scenario: New cycle uses CLI lifecycle
- **WHEN** the user runs `/hero:new`
- **THEN** cycle initialization is performed via `hero cycle new` (or documented equivalent) against SQLite

### Requirement: Runtime and TUI SHALL offer dual entry with parity
Users SHALL be able to complete a full cycle via Cursor chat or via Hero TUI with the same stage/approval/metrics semantics over the shared Go engine (PRD-C01-001 §5.1; ADR-015). Chat-only monitoring from TUI is out of scope as the exclusive model (Deferred D13 rejected).

#### Scenario: Chat path remains first-class
- **WHEN** the user never launches the TUI
- **THEN** they can still complete the cycle entirely through Cursor chat and CLI API
