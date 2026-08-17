# sqlite-operational-store Specification

## Purpose
SQLite schema extensions for OpenCode serve registry and per-harness session binding (ADR-035; PRD-C04-001 §4.8, §4.11).

## MODIFIED Requirements

### Requirement: Schema SHALL include harness serve registry
`hero.db` SHALL store rows for Hero-managed harness serve processes with pid, port, url, harness, and created_at (design D13).

#### Scenario: Migration from v3
- **WHEN** an existing v3 database is opened by Hero 2.0.0
- **THEN** migration creates `harness_serve_registry` without data loss on cycles/stages

### Requirement: Stage session metadata SHALL record harness id
Stage or session storage SHALL bind `harness_session_id` to a harness id so cross-harness resume is prevented (PRD-C04-001 §4.11).

#### Scenario: Harness id on stage
- **WHEN** a stage starts an OpenCode session
- **THEN** the stored metadata includes harness `opencode`
