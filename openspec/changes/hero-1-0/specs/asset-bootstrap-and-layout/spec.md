## ADDED Requirements

### Requirement: Install SHALL initialize the SQLite operational store
`hero install --tools cursor` SHALL create `.workflow-hero/hero.db` (or open and migrate it) as part of Hero-owned bootstrap so a fresh install is SQLite-ready (PRD-C01-001 §3; ADR-013; DEPLOY §6).

#### Scenario: Fresh install creates database
- **WHEN** a user runs `hero install --tools cursor` in a valid project directory
- **THEN** `.workflow-hero/hero.db` exists with the current schema version

### Requirement: Doctor SHALL verify SQLite store integrity
`hero doctor` SHALL verify presence and openability of the Hero SQLite store and report actionable failures when the database is missing or corrupt, in addition to existing install integrity checks (DEPLOY §7; ADR-013).

#### Scenario: Doctor on missing database
- **WHEN** `hero.db` is missing from an otherwise installed project
- **THEN** doctor reports a structured failure recommending upgrade or re-init

### Requirement: Runtime assets SHALL omit canonical cycle markdown ops instructions
Embedded Runtime commands, skills, and orchestration assets SHALL instruct CLI API persistence and SHALL NOT require agents to create or update `workflow.md` / `metrics.md` as the source of truth (PRD-C01-001 §5.4; ADR-012; ADR-013). Contract tests SHALL assert this semantic.

#### Scenario: Asset contract forbids markdown ops writes
- **WHEN** Runtime asset contract tests run
- **THEN** orchestration/stage-close instructions reference CLI persistence and do not mandate writing `workflow.md` / `metrics.md` as operational state

### Requirement: Idea archive layout SHALL remain non-normative inspiration
The tree `docs/idea/archive/v1/` SHALL remain the archived idea location; product requirements SHALL come from cycle PRD/ADR/UI docs (ADR-019). Implementation MAY fix stale documentation links only.

#### Scenario: Archive path present
- **WHEN** documentation references historical idea drafts
- **THEN** links point to `docs/idea/archive/v1/` (or equivalent) and not a competing living product spec under `docs/idea/v1/`

## MODIFIED Requirements

### Requirement: Install SHALL bootstrap Hero-owned asset layout from embedded assets
`hero install --tools cursor` SHALL copy Hero commands, agents, skills, templates, and config scaffolding from embedded assets into project-local Hero paths without requiring external downloads, and SHALL initialize the SQLite operational store under `.workflow-hero/` (PRD-C01-001 §3; DEPLOY §6; ADR-001; ADR-013).

#### Scenario: Fresh install into supported project
- **WHEN** a user runs `hero install --tools cursor` in a valid project directory
- **THEN** required Hero-owned directories and files are created from embedded assets and the SQLite store is present

### Requirement: Doctor SHALL verify installation integrity and version consistency
`hero doctor` SHALL verify expected file presence, config syntax, git prerequisite, SQLite store integrity, and consistency between installed metadata and running CLI version where documented (DEPLOY §7; ADR Appendix; ADR-013).

#### Scenario: Running doctor on inconsistent install
- **WHEN** installed metadata, required paths, or the SQLite store are inconsistent
- **THEN** doctor reports structured failure diagnostics and actionable guidance
