# cli-deterministic-command-suite Specification

## Purpose
CLI breaking changes for Hero 2.0.0: remove `--tools`, interactive harness install (ADR-034; PRD-C04-001 §4.5, §8).

## MODIFIED Requirements

### Requirement: install SHALL NOT accept --tools flag
The `hero install` command SHALL reject `--tools` with exit code 1 and UI-C04-001 §2 message (ADR-034).

#### Scenario: Deprecated flag
- **WHEN** `hero install --tools cursor` is invoked
- **THEN** the command fails before any filesystem writes

### Requirement: upgrade SHALL NOT accept --tools flag
The `hero upgrade` command SHALL reject `--tools` with the same error structure (ADR-034).

#### Scenario: Upgrade deprecated flag
- **WHEN** `hero upgrade --tools cursor` is invoked
- **THEN** the command fails with the unsupported flag message

### Requirement: install SHALL use interactive harness selection
Non-flag harness selection SHALL be interactive via huh multi-select requiring at least one harness (UI-C04-001 §2).

#### Scenario: Interactive success
- **WHEN** the user completes install interactively with OpenCode only
- **THEN** `hero.json` reflects opencode enabled without requiring `--tools`
