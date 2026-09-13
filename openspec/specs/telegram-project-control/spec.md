# telegram-project-control Specification

## Purpose
Telegram help, command forwarding, and compact status rendering for findings, deferred ToDos, and the C15 control commands, using the shared status JSON contract.

## Requirements

### Requirement: Telegram help SHALL list add-todo and complete-todo
Telegram help SHALL include `/hero-add-todo` and `/hero-complete-todo` among project-control commands. These commands SHALL reject image or other attachments (PRD-C15-001 §10.3; UI-C15-001 §13).

#### Scenario: Help catalog names both commands
- **WHEN** a user requests Telegram `/help` without a selected project requirement beyond the existing catalog
- **THEN** the catalog text includes `/hero-add-todo` and `/hero-complete-todo`

#### Scenario: Attachments are rejected
- **WHEN** a Telegram user sends either command with an image attachment
- **THEN** the command is not forwarded as a mutation and the user is told attachments are not accepted

### Requirement: Telegram SHALL forward C15 commands only to a selected connected TUI
Telegram SHALL forward `/hero-add-todo` and `/hero-complete-todo` only to a selected, connected project TUI under existing addressing and auth rules (PRD-C15-001 §10.3).

#### Scenario: Forwarding requires selection
- **WHEN** the commands are sent without a selected connected project
- **THEN** Telegram does not mutate Hero state and prompts for selection using existing rules

### Requirement: Telegram status SHALL consume additive status JSON
Telegram `/status` SHALL render compact finding counts and the first actionable IDs from `hero status --json` without duplicating parsing logic. Detailed rows remain available through project Status/JSON (UI-C15-001 §13–14; ADR-090).

#### Scenario: Compact counts are shown
- **WHEN** status JSON reports one open finding `find-qa-1`
- **THEN** Telegram status includes the open count and `find-qa-1` without inventing a second parser

### Requirement: Telegram-originated turns SHALL share the Hero session and retain origin
Local TUI and Telegram-originated turns in the same conversation SHALL persist on the same Hero session. Restored events SHALL keep Telegram origin so Chat can render `←/→ [Telegram · addr]` labels. Last activity SHALL update when a Telegram turn is accepted (PRD-C16-001 §3.1, §3.3; UI-C16-001 §8, §10.4).

#### Scenario: Telegram message lands on the open session
- **WHEN** a paired Telegram user sends a plain prompt into the selected TUI conversation
- **THEN** the user event is stored with `origin=telegram` and the same Hero session ID as local turns

#### Scenario: Restored origin labels survive restart
- **WHEN** the TUI restarts and the user opens that session
- **THEN** the Telegram turn shows its origin marker and the History row last-activity timestamp includes that turn
