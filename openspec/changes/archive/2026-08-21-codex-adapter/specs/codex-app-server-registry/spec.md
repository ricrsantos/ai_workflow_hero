# codex-app-server-registry Specification

## Purpose
Persist Hero-managed Codex app-server child identity in project SQLite for orphan reap, graceful shutdown, and `/harness-reset` (ADR-044; PRD-C06-001 §4.3, §4.10).

## ADDED Requirements

### Requirement: Codex app-server rows SHALL live in project hero.db
When Hero starts a Codex app-server child, the adapter SHALL write a registry row with at minimum pid, harness id `codex`, project path, and created timestamp. Stdio transport has no HTTP URL — do not invent a serve URL field (ADR-044; design D13).

#### Scenario: Registry write on start
- **WHEN** Codex app-server starts successfully for a project
- **THEN** a row is persisted in `hero.db` identifying the Hero-owned child

#### Scenario: Registry cleared on stop
- **WHEN** Hero gracefully stops the managed Codex app-server
- **THEN** the corresponding registry row is removed or marked inactive

### Requirement: TUI boot SHALL reap Codex orphans
On `hero tui` start, Hero SHALL inspect registry rows for Codex and terminate orphaned Hero-owned app-server processes from unexpected prior exits (OpenCode serve-registry analog) (ADR-044).

#### Scenario: Crash recovery
- **WHEN** a prior TUI exited without stopping Codex and the PID still matches Hero identity
- **THEN** boot reaps the orphan before new Execute

#### Scenario: Foreign process not killed
- **WHEN** a running `codex app-server` was not started by Hero for this project
- **THEN** orphan reap does not terminate it

### Requirement: Disabling Codex or quitting TUI SHALL stop Hero-managed app-server
Normal TUI quit and `/hero-harness` disable Codex SHALL stop the child Hero created. The next Codex Execute SHALL recreate it (PRD-C06-001 §4.3).

#### Scenario: TUI quit stops child
- **WHEN** the user exits `hero tui` while Codex app-server is running
- **THEN** Hero stops the managed child it started

#### Scenario: Disable stops child
- **WHEN** the user disables Codex via `/hero-harness`
- **THEN** Hero stops the managed app-server and sets `enabled=false` without deleting `.codex/`
