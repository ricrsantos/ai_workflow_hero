# sqlite-operational-store Specification

## MODIFIED Requirements

### Requirement: harness_serve_registry SHALL store Codex app-server identity
The operational store SHALL persist Codex app-server registry rows with pid, harness=`codex`, project path, and identity metadata sufficient for orphan reap without storing a fabricated HTTP URL (ADR-044; PRD-C06-001 §4.10).

#### Scenario: Codex row persisted
- **WHEN** Codex app-server starts for a project
- **THEN** SQLite contains a registry row queryable by harness id `codex`

#### Scenario: Schema migration is forward compatible
- **WHEN** an existing project upgrades to Hero 2.5.0
- **THEN** migrations add Codex registry support without dropping OpenCode serve rows

### Requirement: Stage and session records SHALL bind Codex harness id
SQLite stage/session metadata SHALL record harness id `codex` for Codex executions so resume logic never crosses harness boundaries (PRD-C06-001 §4.3).

#### Scenario: Codex execution records harness
- **WHEN** a Codex Execute completes for a stage
- **THEN** the stored session metadata includes harness id `codex`
