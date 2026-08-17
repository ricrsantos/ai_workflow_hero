# hero-tui Specification

## Purpose
TUI changes for Hero 2.0.0 multi-harness: `/hero-harness`, `/hero-model` pair picker, harness labels, boot behavior (UI-C04-001; ADR-037).

## MODIFIED Requirements

### Requirement: TUI SHALL expose hero-harness slash command
The command palette and Chat `/` overlay SHALL include `/hero-harness` executing immediately (UI-C04-001 §3, §8).

#### Scenario: Slash table entry
- **WHEN** the user filters the palette for `harness`
- **THEN** `/hero-harness` appears with label "Manage harnesses"

### Requirement: hero-harness SHALL use checkboxes with availability
`/hero-harness` SHALL show each supported harness as a checkbox with `(available)` or `(unavailable)` in parentheses. Space toggles; Enter applies; Esc cancels.

#### Scenario: Checkbox list
- **WHEN** Cursor is enabled and OpenCode is disabled
- **THEN** the picker shows `[x] Cursor (…)` and `[ ] OpenCode (…)`

### Requirement: hero-model SHALL use harness then model submenu
`/hero-model` SHALL show a harness list first, then models for the selected harness, and persist the pair — superseding model-only selection from C3 (ADR-037; UI-C04-001 §4).

#### Scenario: Two-step picker
- **WHEN** the user opens `/hero-model` with both harnesses enabled
- **THEN** the first screen lists harness names and the second screen lists that harness's models

### Requirement: Chat speaker labels SHALL include harness
Green pane and status lines SHALL use `[LABEL - model · harness]` format (UI-C04-001 §5).

#### Scenario: QA agent on OpenCode
- **WHEN** qa_agent runs with `anthropic/claude-sonnet-4` on opencode
- **THEN** the speaker shows `[QA - anthropic/claude-sonnet-4 · opencode]`

### Requirement: TUI harness boot SHALL use enabled harnesses not cli.tools alone
Boot flow SHALL read `harnesses.<id>.enabled`, distinguish enabled vs available, and warn when none available (UI-C04-001 §7; supersedes C3 cli.tools-only boot for new installs).

#### Scenario: Boot with enabled unavailable harness
- **WHEN** OpenCode is enabled but CLI is missing
- **THEN** the TUI enters with a warning rather than aborting solely for unavailable CLI

### Requirement: Fallback and stop messages SHALL name harness pairs
TUI copy for fallback and hard stop SHALL follow UI-C04-001 §6.

#### Scenario: Fallback warning in Chat
- **WHEN** fallback from cursor to opencode occurs
- **THEN** the Chat shows the ⚠ fallback lines with both harness/model pairs
