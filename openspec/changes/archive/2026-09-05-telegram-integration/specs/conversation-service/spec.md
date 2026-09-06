# conversation-service Specification

## Purpose

Extract a transport-neutral conversation service from the TUI so Telegram IPC and Bubble Tea share one harness dispatch path (PRD-C09-001 §3.3; ADR-061).

## ADDED Requirements

### Requirement: A conversation service SHALL own harness dispatch independent of Bubble Tea

The package SHALL expose a `Service` that accepts user input (slash command or plain text), manages session/context for cycle and free chat, and invokes `HarnessAdapter.Execute` through the existing routing rules. TUI packages SHALL adapt UI events into service calls rather than owning business rules (ADR-061).

#### Scenario: Slash command reaches the same dispatcher
- **WHEN** the TUI submits `/hero-status` through the service
- **THEN** the same command routing used before extraction runs and no Telegram types are referenced

#### Scenario: Plain text starts a harness turn
- **WHEN** free-chat plain text is submitted through the service
- **THEN** one harness Execute begins with the configured freechat pair

### Requirement: Transcript rendering SHALL remain in the TUI layer

Message roles, wait spinner, stream deltas, and Esc cancellation rendering SHALL stay in `internal/tui`. The service SHALL emit structured events/results the TUI renders (design D3).

#### Scenario: Stream deltas still update the transcript
- **WHEN** the harness emits text deltas during an Execute
- **THEN** the TUI renders them without the service importing lipgloss or tea types

### Requirement: Cycle notifications SHALL publish through a narrow Notifier interface

The service or an adjacent publisher SHALL expose callbacks for cycle start/finish, stage start/finish, approval required, errors, and final results. Subscribers (TUI transcript, Telegram outbound) SHALL filter locally; the Notifier SHALL NOT include stream/thinking/tool events (PRD-C09-001 §3.3).

#### Scenario: Tool events are not notifier events
- **WHEN** a harness tool call occurs during Execute
- **THEN** no Notifier callback fires for Telegram-bound notifications

#### Scenario: Approval required fires a notifier event
- **WHEN** a stage enters human approval required state
- **THEN** subscribers receive an approval-required notification payload with cycle/stage identity
