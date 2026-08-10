# hero-cycles-command Specification

## Purpose
List project development cycles with per-etapa metrics (ADR-028; PRD-C03-001 §4.6).

## ADDED Requirements

### Requirement: hero-cycles SHALL aggregate SQLite and archive cycles
`/hero-cycles` (Runtime) and the TUI `/hero-cycles` action SHALL list cycles from the operational SQLite store and from `.workflow-hero/cycles/archive/` when not present in SQLite (design D7).

#### Scenario: Active cycle from SQLite
- **WHEN** the user invokes `/hero-cycles` with an active cycle in `hero.db`
- **THEN** output includes that cycle with etapa rows and metrics

#### Scenario: Legacy archive folder
- **WHEN** an archived cycle exists only as a folder with `metrics.md`
- **THEN** output includes that cycle using parsed legacy metrics when possible

### Requirement: Output SHALL include per-etapa and total metrics
Formatted output SHALL show tokens, duration, and cost per etapa and cycle totals where data exists (UI-C03-001 §5).

#### Scenario: Per-etapa metrics row
- **WHEN** metrics exist for a completed etapa
- **THEN** the cycle listing includes input tokens, output tokens, cost, and duration for that etapa
