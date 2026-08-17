# interactive-harness-install Specification

## Purpose
Replace `--tools` with interactive harness selection at install; reject `--tools` on install and upgrade (ADR-034; UI-C04-001 §2; PRD-C04-001 §4.5).

## ADDED Requirements

### Requirement: Install SHALL require interactive harness selection
`hero install` without TTY harness flags SHALL present a multi-select of **supported** harnesses (not PATH-filtered) and require at least one selection (design D4).

#### Scenario: User selects at least one harness
- **WHEN** the user checks Cursor and continues
- **THEN** install succeeds and reports which harnesses were enabled

#### Scenario: Zero harnesses selected
- **WHEN** the user attempts to continue with no harness checked
- **THEN** install shows inline validation and does not proceed

### Requirement: --tools flag SHALL error on install and upgrade
Passing `--tools` to `hero install` or `hero upgrade` SHALL exit with code 1 and the message from UI-C04-001 §2 (ADR-034).

#### Scenario: install --tools cursor
- **WHEN** the user runs `hero install --tools cursor`
- **THEN** the CLI prints that `--tools` is not supported in Hero 2.0 with a suggestion to use interactive install or `/hero-harness`

#### Scenario: upgrade --tools cursor
- **WHEN** the user runs `hero upgrade --tools cursor`
- **THEN** the CLI prints the same unsupported flag error

### Requirement: OpenCode-only install SHALL be valid
Install with only OpenCode selected SHALL succeed without requiring Cursor (PRD-C04-001 §4.5).

#### Scenario: OpenCode-only selection
- **WHEN** the user selects only OpenCode
- **THEN** install completes with `harnesses.opencode.enabled` true and Cursor disabled
