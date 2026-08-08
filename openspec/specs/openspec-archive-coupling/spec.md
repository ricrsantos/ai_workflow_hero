# openspec-archive-coupling Specification

## Purpose
TBD - created by archiving change slash-parity-tui-harness. Update Purpose after archive.
## Requirements
### Requirement: Cycle record SHALL persist OpenSpec change name
The SQLite `cycles` table SHALL include an `openspec_change` TEXT field (empty when unset). Planning (or any caller that knows the change slug) SHALL persist it via a deterministic CLI API such as `hero cycle openspec-change <name>` (PRD-C02-001 §5.4; ADR-023; design D5).

#### Scenario: Planning records change name
- **WHEN** Planning creates OpenSpec change `slash-parity-tui-harness`
- **THEN** the active cycle’s `openspec_change` is set to `slash-parity-tui-harness` and `hero status --json` includes that value

#### Scenario: Clear openspec_change
- **WHEN** the user runs `hero cycle openspec-change --clear`
- **THEN** the active cycle’s `openspec_change` becomes empty

### Requirement: Hero archive SHALL run OpenSpec archive first
`hero cycle archive` SHALL resolve the OpenSpec change name in order: stored `openspec_change` → if empty, active dirs under `openspec/changes/` excluding `archive/` (0 = skip OpenSpec; 1 = auto; N = fail closed until `--openspec-change` or stored name). When a name is resolved, it SHALL run `openspec archive <name> -y` (merge specs; not `--skip-specs` by default) and only then archive the Hero cycle (PRD-C02-001 §5.4; ADR-023).

#### Scenario: Stored name archives OpenSpec then Hero
- **WHEN** `openspec_change` is `slash-parity-tui-harness` and `openspec archive … -y` succeeds
- **THEN** the Hero cycle is archived afterward

#### Scenario: Zero active OpenSpec changes
- **WHEN** `openspec_change` is empty and `openspec/changes/` has no non-archive entries
- **THEN** Hero archive proceeds without invoking OpenSpec

#### Scenario: Multiple active OpenSpec changes without stored name
- **WHEN** `openspec_change` is empty and more than one active change directory exists
- **THEN** archive fails closed with an actionable message until the user sets the name or passes `--openspec-change`

### Requirement: OpenSpec failure SHALL block Hero archive unless forced
If OpenSpec archive fails (including missing `openspec` on PATH), the CLI MUST NOT archive the Hero cycle unless `--force` or `--skip-openspec` is set. On force, the CLI MUST print manual instructions including `openspec archive <name> -y` (PRD-C02-001 §5.4; UI-C02-001 §4).

#### Scenario: OpenSpec fails without force
- **WHEN** `openspec archive` exits non-zero and neither force flag is set
- **THEN** the command exits non-zero and the cycle remains unarchived

#### Scenario: Force skips OpenSpec and archives Hero
- **WHEN** OpenSpec fails and the user passes `--force` (or `--skip-openspec`)
- **THEN** Hero cycle archive proceeds and output includes the manual OpenSpec command

### Requirement: Runtime archive SHALL offer the force path
`/hero:archive` Runtime guidance MUST, on OpenSpec failure, offer retry or force Hero archive and include the manual `openspec archive <name> -y` instruction (UI-C02-001 §4; ADR-023).

#### Scenario: Chat archive failure UX
- **WHEN** OpenSpec archive fails during `/hero:archive`
- **THEN** the user is shown failure reason, force option, and the manual OpenSpec command

