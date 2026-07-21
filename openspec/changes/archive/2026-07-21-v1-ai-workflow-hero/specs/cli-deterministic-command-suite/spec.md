## ADDED Requirements

### Requirement: V1 CLI command suite SHALL be deterministic and complete
The system SHALL provide the V1 CLI commands `hero install --tools cursor`, `hero upgrade`, `hero uninstall`, `hero doctor`, `hero version`, `hero variables`, `hero update-models`, `hero status`, and `hero help` as deterministic operations with no agent reasoning (PRD §5.8, §6; ADR-003).

#### Scenario: Running supported deterministic commands
- **WHEN** a user runs any V1 CLI command listed in PRD §5.8
- **THEN** the command executes without invoking Runtime reasoning flows and returns deterministic results based on local state and documented inputs

### Requirement: Read commands SHALL support table and JSON output modes
For `hero status`, `hero variables`, and `hero doctor`, the CLI SHALL return a human-readable ASCII table by default and SHALL return machine-readable JSON when `--json` is provided (UI §3.1).

#### Scenario: Reading status in default and JSON modes
- **WHEN** the user runs `hero status` and then `hero status --json`
- **THEN** the first output is a human-readable table and the second output is a single JSON payload with no decorative color or icon formatting

### Requirement: Interactive commands SHALL support non-interactive flag overrides
Interactive prompts in CLI commands SHALL provide equivalent flags so fully specified invocations can run without prompts, while missing fields fall back to prompts (UI §4).

#### Scenario: Fully scripted install path
- **WHEN** the user provides all required install flags for `hero install --tools cursor`
- **THEN** the command completes with zero interactive prompts

### Requirement: CLI error messages SHALL follow the standard structure
Every CLI failure SHALL emit a structured error including a clear description, an actionable suggestion when applicable, and a non-zero exit code (UI §5).

#### Scenario: Invalid command precondition
- **WHEN** a command fails due to an invalid precondition (for example, invalid config or missing repository requirement)
- **THEN** output follows the standard error structure and the process exits non-zero

### Requirement: Install command SHALL enforce git prerequisite with explicit user control
`hero install --tools cursor` SHALL verify git repository presence and SHALL offer `git init` interaction when missing; declining SHALL abort install (PRD §6; ADR-004; DEPLOY §6).

#### Scenario: Install in a non-git directory
- **WHEN** the user runs `hero install --tools cursor` in a directory without git metadata
- **THEN** the command offers initialization and aborts installation if the user declines

### Requirement: Upgrade and uninstall SHALL preserve user-owned boundaries
`hero upgrade` SHALL not silently overwrite user-customized files and SHALL warn instead; `hero uninstall` SHALL remove only Hero-owned paths and SHALL preserve project artifacts (`AGENTS.md`, `context/`, `docs/`, `openspec/`) (PRD §6; DEPLOY §7).

#### Scenario: Upgrade with customized local asset
- **WHEN** `hero upgrade` detects a customized installed asset
- **THEN** the command reports the file and does not overwrite it automatically

#### Scenario: Uninstall execution
- **WHEN** the user runs `hero uninstall`
- **THEN** Hero-owned installation paths are removed and project artifact directories remain intact
