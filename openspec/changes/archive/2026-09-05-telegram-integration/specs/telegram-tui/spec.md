# telegram-tui Specification

## Purpose

TUI-side Telegram plugin UX: Settings, pairing modal, IPC client, transcript labels (UI-C09-001; PRD-C09-001 §4).

## ADDED Requirements

### Requirement: Settings SHALL expose Telegram controls only when the plugin is installed

The Settings sidebar SHALL include a Telegram section when the plugin is installed. It SHALL show installed/version, daemon connection, configuration state, editable project abbreviation, and display-only live instance suffix. It SHALL never render token or chat id (UI-C09-001 §1).

#### Scenario: Not installed shows install guidance
- **WHEN** the plugin is not installed
- **THEN** Settings shows `Install with: hero plugin install telegram`

#### Scenario: Configured state hides secrets
- **WHEN** pairing succeeded
- **THEN** Status reads `Configured` with no numeric chat id visible

### Requirement: Pairing modal SHALL be keyboard accessible with a ten-minute countdown

Pair opens a focused modal with numbered steps, visible pairing code, countdown, waiting/success/expiry states, and Cancel/Escape cleanup that invalidates the code (UI-C09-001 §2).

#### Scenario: Escape cancels pairing
- **WHEN** the user presses Escape during pairing
- **THEN** the modal closes and the code is invalidated

#### Scenario: Success closes modal
- **WHEN** the daemon confirms pairing
- **THEN** the modal closes and Settings shows success copy

### Requirement: Chat transcript SHALL label Telegram origin and destination

Remote input SHALL render with `← [Telegram · <address>]` and outbound replies with `→ [Telegram · <address>]` without altering harness speaker headers (UI-C09-001 §3).

#### Scenario: Remote plain text shows origin label
- **WHEN** an addressed plain-text message arrives from Telegram
- **THEN** the transcript shows the left-arrow Telegram label before the user content

### Requirement: IPC client lifecycle SHALL run without blocking Bubble Tea Update

Registration, reconnect, and daemon restart attempts SHALL execute in `tea.Cmd` goroutines with bounded exponential backoff. Daemon outage and recovery SHALL surface in Chat and Settings (UI-C09-001 §§1,4; design D7).

#### Scenario: Daemon disconnect shows retry copy
- **WHEN** the IPC connection drops unexpectedly
- **THEN** Chat shows `⚠ Telegram daemon disconnected; retrying…` and Settings reflects retry state

#### Scenario: Recovery shows success copy
- **WHEN** the daemon reconnects after backoff
- **THEN** Chat shows `✓ Telegram daemon reconnected.`
