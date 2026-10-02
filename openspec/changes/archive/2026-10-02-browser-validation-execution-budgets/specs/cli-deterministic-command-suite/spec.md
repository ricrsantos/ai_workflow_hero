# cli-deterministic-command-suite Specification (delta)

## Purpose
`/hero-screenshot` control semantics and credential-lifecycle messaging. PRD FR-04, FR-13; UI §5.

## MODIFIED Requirements

### Requirement: /hero-screenshot SHALL be a TUI-owned deterministic control
`/hero-screenshot` with latest/list/id/todos selectors SHALL be permitted despite the busy composer and available through Telegram; it SHALL retrieve generated captures from the active cycle and SHALL never invoke a harness, start a capture, interrupt a test, or wait for an agent. Selected-project addressing SHALL remain mandatory. Missing cycle/captures/ID, disconnected Telegram, or unreadable images SHALL produce precise explanations.

#### Scenario: Unknown screenshot ID
- **WHEN** `/hero-screenshot shot-999` names no ready capture
- **THEN** the control lists that the ID is unknown (distinct from having no captures) without contacting any harness

### Requirement: Lifecycle commands SHALL report credential retention and cleanup honestly
Archive SHALL retain safe screenshots and remove `.env.hero` only after prerequisites; cleanup failure SHALL report archive pending with the file-access problem and safe retry, never completed removal. Uninstall SHALL explicitly report retained credentials.

#### Scenario: Cleanup failure during archive
- **WHEN** `.env.hero` cannot be removed after otherwise successful archive steps
- **THEN** the CLI reports pending archive with the resolve-and-retry instruction
