## ADDED Requirements

### Requirement: Telegram-originated turns SHALL share the Hero session and retain origin
Local TUI and Telegram-originated turns in the same conversation SHALL persist on the same Hero session. Restored events SHALL keep Telegram origin so Chat can render `←/→ [Telegram · addr]` labels. Last activity SHALL update when a Telegram turn is accepted (PRD-C16-001 §3.1, §3.3; UI-C16-001 §8, §10.4).

#### Scenario: Telegram message lands on the open session
- **WHEN** a paired Telegram user sends a plain prompt into the selected TUI conversation
- **THEN** the user event is stored with `origin=telegram` and the same Hero session ID as local turns

#### Scenario: Restored origin labels survive restart
- **WHEN** the TUI restarts and the user opens that session
- **THEN** the Telegram turn shows its origin marker and the History row last-activity timestamp includes that turn
