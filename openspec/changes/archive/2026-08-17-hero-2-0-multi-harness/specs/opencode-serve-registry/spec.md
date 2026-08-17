# opencode-serve-registry Specification

## Purpose
Persist OpenCode serve process metadata in project `hero.db` and reap orphans on TUI boot (ADR-035; PRD-C04-001 §4.8).

## ADDED Requirements

### Requirement: Serve registry SHALL live in project hero.db
Each Hero-started `opencode serve` SHALL be recorded with pid, port, url, harness id, and created_at in SQLite (design D13).

#### Scenario: Record on serve start
- **WHEN** OpenCode serve starts successfully
- **THEN** a row exists in `harness_serve_registry` with the child pid and url

### Requirement: TUI boot SHALL reap orphan serves
On `hero tui` start, Hero SHALL stop processes recorded in this project's registry whose PID is still `opencode serve` after an unexpected exit (ADR-035).

#### Scenario: Crash recovery
- **WHEN** a prior TUI exited without cleanup and the registry row's pid is still opencode serve
- **THEN** boot reaps that process before accepting user input

### Requirement: Normal shutdown SHALL stop Hero-owned serve
TUI quit and disabling OpenCode via `/hero-harness` SHALL stop the serve process Hero created (ADR-035).

#### Scenario: TUI quit
- **WHEN** the user exits the TUI while OpenCode serve is running
- **THEN** the child process is stopped and the registry row is cleared or marked inactive

#### Scenario: Disable OpenCode
- **WHEN** the user disables OpenCode via `/hero-harness`
- **THEN** the running serve child is stopped

### Requirement: Session binding SHALL be harness-scoped
SQLite stage/session metadata SHALL record harness id so a Cursor session id is never resumed as OpenCode (PRD-C04-001 §4.11).

#### Scenario: Cross-harness resume blocked
- **WHEN** a stage has a Cursor session id and the agent harness changes to opencode
- **THEN** Execute starts a new OpenCode session instead of resuming the Cursor id
