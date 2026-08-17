# hero-harness-command Specification

## Purpose
TUI slash `/hero-harness` to enable/disable harnesses with provision-on-enable and last-harness guard (ADR-037; UI-C04-001 §3).

## ADDED Requirements

### Requirement: hero-harness SHALL appear in palette and Chat overlay
The TUI SHALL register `/hero-harness` with label "Manage harnesses" and execute immediately like other non-approval slashes (UI-C04-001 §8).

#### Scenario: Palette presence
- **WHEN** the user opens the command palette
- **THEN** `/hero-harness` is listed

### Requirement: hero-harness SHALL show enabled and available state
The command SHALL list each supported harness with Enabled and Available columns (PATH/CLI) (UI-C04-001 §3).

#### Scenario: OpenCode unavailable on PATH
- **WHEN** OpenCode is disabled and CLI is missing
- **THEN** the list shows OpenCode as disabled and unavailable

### Requirement: Enable SHALL provision projection
Enabling a harness SHALL set `enabled: true` and provision its projection immediately (ADR-036).

#### Scenario: Enable OpenCode
- **WHEN** the user confirms enabling OpenCode
- **THEN** the success line reads `OpenCode enabled (projected .opencode/)`

### Requirement: Disable last enabled harness SHALL error
The TUI SHALL reject disabling the last remaining enabled harness (ADR-037).

#### Scenario: Last harness guard
- **WHEN** only Cursor is enabled and the user tries to disable Cursor
- **THEN** the error matches UI-C04-001 §3 with suggestion to enable another harness first
