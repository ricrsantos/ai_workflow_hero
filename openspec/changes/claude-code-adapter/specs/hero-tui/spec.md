## ADDED Requirements

### Requirement: TUI harness and model surfaces SHALL expose Claude as an opt-in fourth choice

Install and `/hero-harness` SHALL show Claude as a PATH-independent fourth checkbox; `/hero-model` SHALL list Claude native ids and supported effort values when enabled. Enable/disable copy SHALL name `.claude/` projection and files-kept behavior, and the last-harness guard SHALL remain active (UI-C13-001 §1).

#### Scenario: Fourth harness row
- **WHEN** the harness selector opens in a project with Claude disabled
- **THEN** a Claude row is available to select regardless of whether `claude` is currently on PATH

#### Scenario: Claude pair selection
- **WHEN** the user selects Claude in `/hero-model`
- **THEN** the next picker shows native Claude aliases/ids, persists the free-chat pair only after final confirmation, and leaves workflow YAML unchanged

#### Scenario: Turn identity
- **WHEN** a Claude agent or free-chat turn is rendered
- **THEN** the speaker/status identity includes the native model and `· claude`

### Requirement: Claude diagnostics and security gates SHALL remain visible at every verbosity

The TUI SHALL show missing CLI, minimum-version, authentication, incompatible-flag, permission, question, session, and warning states with actionable copy. Pending permission/question decisions SHALL remain visible in Compact, Standard, Detailed, and Debug profiles; permission waits SHALL pause watchdog accounting. Claude SHALL not appear as a persistent `/harness-reset` target (UI-C13-001 §§1–2).

#### Scenario: Missing or incompatible CLI
- **WHEN** an enabled Claude turn cannot find a compatible CLI
- **THEN** the TUI names the 2.1.261 minimum or missing capability and points the user to remediation and `/hero-continue`

#### Scenario: Permission remains visible in compact mode
- **WHEN** a Claude permission request is pending under Compact verbosity
- **THEN** the permission gate remains visible and actionable even if ordinary activity events are filtered

#### Scenario: Claude reset surface
- **WHEN** the user opens `/harness-reset`
- **THEN** Claude is absent because its child is turn-scoped, while existing persistent harness reset behavior is unchanged

### Requirement: Claude rendering SHALL preserve existing responsive terminal behavior

Claude identity, warning, session, and permission rows SHALL follow existing semantic styles and constrained-width rules. Narrow and no-color terminals SHALL retain enough information to understand the harness, action, and remediation without blocking the TUI Update loop (UI-C13-001 §3; go-tui guidance).

#### Scenario: Constrained turn label
- **WHEN** a Claude model id or warning is wider than the available terminal region
- **THEN** the TUI truncates or wraps intentionally using display width and preserves the harness label and primary action

