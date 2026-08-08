## ADDED Requirements

### Requirement: SQLite SHALL be the sole Hero operational store
Hero-exclusive operational data (cycle/stage status, events, metrics/costs, operational conversation history, artifact metadata) SHALL be persisted only in SQLite under `.workflow-hero/` (path `.workflow-hero/hero.db`) and SHALL NOT be projected to cycle markdown such as `workflow.md` or `metrics.md` in 1.0 (PRD-C01-001 §5.2; ADR-013; Deferred D9).

#### Scenario: Persisting a stage transition
- **WHEN** the engine completes a stage transition
- **THEN** the new state is readable from SQLite via CLI query commands and no `workflow.md` / `metrics.md` write is required for correctness

#### Scenario: Project artifacts remain files
- **WHEN** Hero records artifact metadata or updates project context after meaningful outcomes
- **THEN** artifact content and `context/*.md` / `docs/` / `openspec/` remain ordinary project files on disk (Hero does not kidnap the project)

### Requirement: Store SHALL provide versioned schema migrations
The store package SHALL create the database if missing and apply ordered schema migrations tracked in-database so install and upgrade converge on the same schema (ADR-013; DEPLOY §3.1).

#### Scenario: Fresh database initialization
- **WHEN** a Hero command opens the store and `hero.db` does not exist
- **THEN** the schema is created at the current migration version and subsequent opens are no-ops for already-applied migrations

### Requirement: Store SHALL support append-only events
The store SHALL append operational events (`stage_started`, `approved`, `harness_invoked`, and similar) without in-place mutation of historical event rows (PRD-C01-001 §5.3; ADR-013).

#### Scenario: Listing recent events
- **WHEN** events are appended during a cycle
- **THEN** `hero events` can return them in chronological order from SQLite

### Requirement: Store tests SHALL use a real temporary database
Store behavior SHALL be verified with a real SQLite file under `t.TempDir()` (ADR-009), not a mocked repository.

#### Scenario: Repository round-trip test
- **WHEN** tests insert cycle, stage, metrics, and event rows
- **THEN** queries return the persisted values from the temporary database file
