## ADDED Requirements

### Requirement: CLI suite SHALL include Hero 1.0 operational and TUI commands
In addition to the pre-1.0 deterministic suite, the CLI SHALL register `hero metrics`, `hero events`, `hero approve`, `hero reject`, `hero cancel`, `hero finish`, `hero continue`, `hero cycle new|archive|resume`, `hero tui`, and `hero run` as deterministic operations with no agent reasoning (PRD-C01-001 §5; ADR-014; UI-C01-001 §4).

#### Scenario: Help lists new commands
- **WHEN** the user runs `hero help`
- **THEN** the new 1.0 commands appear in help output

### Requirement: Upgrade SHALL migrate 0.9.x installs to SQLite-backed 1.0
`hero upgrade` SHALL create or migrate `.workflow-hero/hero.db`, refresh Runtime assets with existing checksum-safe non-overwrite behavior, and perform a one-shot import of an in-flight legacy `workflow.md` cycle when present; legacy cycle markdown SHALL cease to be canonical (PRD-C01-001 §3; ADR-018; DEPLOY §3.1). Soft dual-mode is out of scope (Deferred D11).

#### Scenario: Upgrade creates database
- **WHEN** a user on 0.9.x runs `hero upgrade` to 1.0
- **THEN** `hero.db` exists under `.workflow-hero/` and `hero status` reads from SQLite

#### Scenario: Legacy markdown not required at runtime
- **WHEN** upgrade completes
- **THEN** Runtime operation does not require reading or writing `workflow.md` / `metrics.md` as source of truth

### Requirement: Default CLI version SHALL be 1.0.0
The CLI default version string SHALL be `1.0.0` for the Hero 1.0 release line (DEPLOY §3; ADR-018).

#### Scenario: Version command
- **WHEN** the user runs `hero version` on an untagged dev build using the default
- **THEN** the reported default version is `1.0.0`

## MODIFIED Requirements

### Requirement: V1 CLI command suite SHALL be deterministic and complete
The system SHALL provide the deterministic CLI commands `hero install --tools cursor`, `hero upgrade`, `hero uninstall`, `hero doctor`, `hero version`, `hero variables`, `hero update-models`, `hero status`, `hero help`, and the Hero 1.0 operational commands listed in this capability (`metrics`, `events`, cycle control, `tui`, `run`) with no agent reasoning (PRD-C01-001 §5; ADR-003 as amended; ADR-014).

#### Scenario: Running supported deterministic commands
- **WHEN** a user runs any supported deterministic CLI command
- **THEN** the command executes without invoking Runtime reasoning flows and returns deterministic results based on local state and documented inputs

### Requirement: Read commands SHALL support table and JSON output modes
For `hero status`, `hero variables`, `hero doctor`, `hero metrics`, and `hero events`, the CLI SHALL return a human-readable ASCII table by default and SHALL return machine-readable JSON when `--json` is provided (UI §3.1; UI-C01-001 §4).

#### Scenario: Reading status in default and JSON modes
- **WHEN** the user runs `hero status` and then `hero status --json`
- **THEN** the first output is a human-readable table and the second output is a single JSON payload with no decorative color or icon formatting

#### Scenario: Reading metrics in JSON mode
- **WHEN** the user runs `hero metrics --json`
- **THEN** the output is a single JSON payload suitable for scripting

### Requirement: Upgrade and uninstall SHALL preserve user-owned boundaries
`hero upgrade` SHALL not silently overwrite user-customized files and SHALL warn instead; `hero uninstall` SHALL remove only Hero-owned paths (including `.workflow-hero/` which contains `hero.db`) and SHALL preserve project artifacts (`AGENTS.md`, `context/`, `docs/`, `openspec/`) (PRD §6; DEPLOY §7; ADR-018).

#### Scenario: Upgrade with customized local asset
- **WHEN** `hero upgrade` detects a customized installed asset
- **THEN** the command reports the file and does not overwrite it automatically

#### Scenario: Uninstall execution
- **WHEN** the user runs `hero uninstall`
- **THEN** Hero-owned installation paths including the SQLite store are removed and project artifact directories remain intact
