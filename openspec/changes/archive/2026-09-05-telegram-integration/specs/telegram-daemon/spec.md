# telegram-daemon Specification

## Purpose

Local Telegram Bot API daemon: pairing, routing, queue, notifications (PRD-C09-001 §3.2–§3.4; ADR-059, ADR-063).

## ADDED Requirements

### Requirement: The daemon SHALL own Bot API connectivity exclusively

Only the daemon process SHALL call Telegram send/receive APIs. TUIs SHALL NOT long-poll Telegram directly (ADR-059/060).

#### Scenario: TUI sends outbound via IPC
- **WHEN** the TUI publishes a cycle notification
- **THEN** the daemon receives an `outbound` frame and performs the Bot API send

### Requirement: Inbound messages SHALL require a valid address prefix

User messages SHALL match `<address>:` before payload parsing. Unknown or malformed addresses SHALL receive a generic response without revealing project names or cycle state (PRD-C09-001 §3.2; ADR-063).

#### Scenario: Valid addressed slash command forwards as command
- **WHEN** Telegram receives `myproj: /hero-status`
- **THEN** the registered `myproj` TUI gets an `inbound` command frame

#### Scenario: Unknown address does not leak project list
- **WHEN** Telegram receives `unknown: hello`
- **THEN** the sender gets a generic error with no project identifiers

### Requirement: Offline targets SHALL queue durably for twenty-four hours

When the addressed TUI is not connected, the daemon SHALL persist the message with provider update id, timestamps, and status `pending` until delivery, cancellation, or expiry (PRD-C09-001 §3.3).

#### Scenario: Message expires after twenty-four hours
- **WHEN** a pending message is older than twenty-four hours
- **THEN** its status becomes `expired` and it is not delivered

### Requirement: Cancel-pending SHALL affect only undelivered queue rows

`<address>: /telegram-cancel-pending` SHALL transition pending messages for that address to `cancelled`. It SHALL NOT invoke cycle cancel, harness interrupt, or `/hero-cancel` (PRD-C09-001 §3.3).

#### Scenario: Cancel-pending does not stop active harness
- **WHEN** a pending message exists and the user sends cancel-pending while a harness turn is running
- **THEN** pending rows cancel but the active Execute continues

### Requirement: Pairing SHALL bind exactly one authorized chat

Pairing codes SHALL expire after ten minutes. Only a valid unexpired code SHALL bind the sender chat id into the vault. Unauthorized chats SHALL be ignored or generically rejected (PRD-C09-001 §3.4).

#### Scenario: Expired code does not bind
- **WHEN** `/start 123456` arrives eleven minutes after code issuance
- **THEN** pairing fails and no vault entry is written

### Requirement: Update handling SHALL be idempotent

The daemon SHALL record processed Telegram `update_id` values and ignore duplicates (PRD-C09-001 §3.3).

#### Scenario: Duplicate update is ignored
- **WHEN** the same Telegram update is delivered twice
- **THEN** only one harness turn or queue row is created

### Requirement: Notification filtering SHALL exclude intermediate harness activity

Outbound cycle notifications SHALL include stage/cycle/approval/error/final summaries only. Thinking, tool, activity, and stream deltas SHALL NOT be sent to Telegram (PRD-C09-001 §3.3; UI-C09-001 §4).

#### Scenario: Tool call does not notify Telegram
- **WHEN** a harness emits a tool start event
- **THEN** the daemon sends no Telegram message for that event
