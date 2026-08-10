# hero-todos-command Specification

## Purpose
Display pending work from `context/current-state.md` with sync guidance (ADR-028; PRD-C03-001 §4.7).

## ADDED Requirements

### Requirement: hero-todos SHALL read only current-state.md
`/hero-todos` SHALL display pending items sourced exclusively from `context/current-state.md` (not live scans of `docs/product/` or `docs/architecture/`).

#### Scenario: Display pending features
- **WHEN** the user invokes `/hero-todos`
- **THEN** output lists items from the pending sections of `current-state.md`

### Requirement: hero-todos SHALL notify about hero-sync
Output SHALL include a notice that if `docs/product/` or `docs/architecture/` changed, the user should run `/hero-sync` then `/hero-todos` again (UI-C03-001 §6).

#### Scenario: Sync notice present
- **WHEN** `/hero-todos` completes successfully
- **THEN** the output includes guidance to run `/hero-sync` when docs may be stale
