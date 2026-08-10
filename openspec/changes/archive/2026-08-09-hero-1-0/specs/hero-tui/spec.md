## ADDED Requirements

### Requirement: CLI SHALL launch a Bubble Tea TUI entry point
The system SHALL provide `hero tui` that launches an interactive Bubble Tea application (using huh where discrete prompts fit) sharing the same engine/store as CLI commands (PRD-C01-001 §5.1; ADR-017; UI-C01-001 §2–§3).

#### Scenario: Launch on TTY
- **WHEN** the user runs `hero tui` in an interactive TTY on a Hero-installed project
- **THEN** the TUI starts and can display current cycle status from SQLite

#### Scenario: Non-TTY refusal
- **WHEN** the user runs `hero tui` without a TTY (or in an unsupported degraded environment)
- **THEN** the command exits with an actionable error and does not hang

### Requirement: TUI SHALL provide the minimum 1.0 screens
The TUI SHALL include screens for cycle status, approvals, artifacts, costs/metrics, recent events, and a basic command palette (UI-C01-001 §3.3; ADR-017). Full multi-panel richness is out of scope (Deferred D10).

#### Scenario: Navigate to approvals and act
- **WHEN** a stage is pending approval
- **THEN** the user can approve or reject from the Approvals screen and see status update without opening cycle markdown files

#### Scenario: Command palette navigation
- **WHEN** the user opens the command palette
- **THEN** they can jump to the minimum screens / common actions via keyboard-first navigation

### Requirement: TUI SHALL drive the full cycle with parity to chat
The TUI SHALL NOT be monitor-only: it MUST support progressing the cycle (approvals and stage progression) over the shared core, including optional adapter dispatch via `hero run` semantics when available (PRD-C01-001 §5.1; ADR-015; Deferred D13 rejected).

#### Scenario: Complete approval flow in TUI
- **WHEN** the user approves a stage in TUI
- **THEN** SQLite state matches what `hero status` would show after an equivalent chat `/hero:approve`

### Requirement: TUI visual language SHALL follow Hero UI conventions
TUI presentation SHALL reuse color/icon semantics from UI.md and adopt Claude Code–inspired interaction patterns without cloning branding (UI-C01-001 §3.1–§3.2; UI.md §2).

#### Scenario: Status icons convey stage state
- **WHEN** stages are shown on the status screen
- **THEN** status is distinguishable using documented semantic icons/colors (respecting `NO_COLOR` where applicable for non-TUI CLI; TUI may refuse when color/TTY constraints require)
