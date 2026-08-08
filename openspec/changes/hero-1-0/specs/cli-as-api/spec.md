## ADDED Requirements

### Requirement: CLI SHALL expose read APIs for status, metrics, and events
The CLI SHALL provide `hero status`, `hero metrics`, and `hero events` that read Hero operational state from SQLite, defaulting to human-readable tables and supporting `--json` for machine output (PRD-C01-001 §5.2; UI-C01-001 §4; UI.md §3.1; ADR-014).

#### Scenario: Status replaces workflow.md reads
- **WHEN** a user or agent needs current cycle stage state
- **THEN** `hero status` (and `hero status --json`) returns stage machine state from SQLite without requiring `workflow.md`

#### Scenario: Metrics replaces metrics.md reads
- **WHEN** a user or agent needs token/cost estimates for the cycle
- **THEN** `hero metrics` returns per-stage and totals from SQLite

#### Scenario: Events query
- **WHEN** a user runs `hero events` with optional `--limit`
- **THEN** recent append-only events from SQLite are displayed (table or JSON)

### Requirement: CLI SHALL expose mutation APIs for cycle control
The CLI SHALL provide deterministic mutation commands `hero approve`, `hero reject`, `hero cancel`, `hero finish`, and `hero continue` that apply engine transitions and persist to SQLite without LLM reasoning (PRD-C01-001 §5.1, §5.4; ADR-012; ADR-014).

#### Scenario: Approve advances when rules allow
- **WHEN** the user runs `hero approve` with a pending-approval stage and valid metrics payload as designed
- **THEN** the engine records approval, persists metrics, emits events, and advances per workflow rules

#### Scenario: Cancel terminates the cycle
- **WHEN** the user runs `hero cancel`
- **THEN** cycle status becomes cancelled in SQLite and further stage work is blocked

### Requirement: CLI SHALL expose cycle lifecycle commands
The CLI SHALL provide `hero cycle new`, `hero cycle archive`, and `hero cycle resume` to create, archive, and resume cycles in the store (mapping prior slash-command lifecycle semantics) (PRD-C01-001 §5.1; ADR-014).

#### Scenario: Creating a new cycle
- **WHEN** the user or Runtime runs `hero cycle new` with imported workflow-config
- **THEN** a cycle and stage rows are created in SQLite with Waiting statuses and no canonical `workflow.md` initialization is required

#### Scenario: Archive uses completed date from store
- **WHEN** the user runs `hero cycle archive` for a finished cycle
- **THEN** archive folder naming uses the cycle `completed_at` date from SQLite (equivalent to prior workflow.md Completed field)

### Requirement: Runtime and TUI SHALL invoke CLI or shared services only for ops state
Chat Runtime SHALL shell out to these CLI commands for Hero operational reads/writes; TUI MAY call the same Go services in-process but MUST NOT maintain a separate ops source of truth (ADR-014; ADR-015).

#### Scenario: Slash command approve uses CLI
- **WHEN** `/hero:approve` runs in Cursor chat
- **THEN** it invokes `hero approve` (or documented equivalent) rather than editing cycle markdown

### Requirement: CLI API errors SHALL follow UI conventions
Mutation and query failures SHALL emit `✗ Error: <message>.` style errors and non-zero exit codes (UI.md §5; UI-C01-001 §4).

#### Scenario: Missing active cycle
- **WHEN** a mutation command runs with no active cycle in the store
- **THEN** the CLI prints a structured error and exits non-zero
