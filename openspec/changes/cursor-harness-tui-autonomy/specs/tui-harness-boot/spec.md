# tui-harness-boot Specification

## Purpose
TUI selects and validates harness at launch without depending on `hero doctor` (ADR-027; UI-C03-001 §2).

## ADDED Requirements

### Requirement: TUI SHALL prompt for harness when cli.tools is empty
When `.workflow-hero/config/hero.json` has no `cli.tools` entries, the TUI SHALL prompt the user to select a harness before entering the main loop (design D5).

#### Scenario: First launch without tools
- **WHEN** `hero` starts and `cli.tools` is empty
- **THEN** the user sees a harness selection prompt

#### Scenario: V1 supported list
- **WHEN** the user is prompted
- **THEN** only supported harnesses for V1 are offered (cursor)

### Requirement: Validation failure SHALL abort TUI with remediation
After selection, the TUI SHALL call `IsAvailable`; on failure it SHALL print the error and exit non-zero with actionable guidance (e.g. `cursor agent login`) (UI-C03-001 §2).

#### Scenario: Auth failure aborts
- **WHEN** harness validation fails due to authentication
- **THEN** the TUI exits without entering the main screen loop

### Requirement: Successful selection SHALL persist cli.tools
When the user selects a harness and validation succeeds, the TUI SHALL write the tool id to `hero.json` → `cli.tools` (ADR-027).

#### Scenario: Persist cursor
- **WHEN** the user selects cursor and validation succeeds
- **THEN** `cli.tools` contains `"cursor"`
