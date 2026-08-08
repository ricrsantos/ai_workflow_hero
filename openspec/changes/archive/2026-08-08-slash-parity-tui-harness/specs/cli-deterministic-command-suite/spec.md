## ADDED Requirements

### Requirement: CLI SHALL expose openspec_change on the active cycle
The deterministic CLI SHALL provide a non-interactive way to set, clear, and read the active cycle’s `openspec_change` value (e.g. `hero cycle openspec-change <name>`, `--clear`, and inclusion in `hero status` / `--json`) without agent reasoning (PRD-C02-001 §5.4; ADR-014; ADR-023).

#### Scenario: Set openspec change name
- **WHEN** the user runs `hero cycle openspec-change slash-parity-tui-harness` with an active cycle
- **THEN** the store persists that name and subsequent `hero status --json` includes it

### Requirement: cycle archive SHALL support OpenSpec coupling flags
`hero cycle archive` SHALL support `--force` and `--skip-openspec` (aliases) and optional `--openspec-change <name>` override for the invocation, implementing the archive orchestration in ADR-023 (PRD-C02-001 §5.4).

#### Scenario: Archive help documents force flags
- **WHEN** the user inspects `hero cycle archive --help`
- **THEN** `--force` / `--skip-openspec` and `--openspec-change` are documented

#### Scenario: Forced archive after OpenSpec failure
- **WHEN** OpenSpec archive fails and the user re-runs with `--force`
- **THEN** Hero cycle archive completes and manual OpenSpec instructions are printed
