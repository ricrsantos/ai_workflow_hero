# hero-tui Specification

## MODIFIED Requirements

### Requirement: Settings SHALL include Telegram plugin state when installed

When the optional Telegram plugin is installed, Settings SHALL expose the Telegram section described in UI-C09-001 §1 with semantic colors/icons and actionable recovery guidance. When not installed, Chat and other TUI behavior SHALL remain unchanged (UI-C09-001 §§1,5).

#### Scenario: Telegram section hidden without plugin
- **WHEN** the plugin is not installed
- **THEN** the Settings focus order excludes Telegram-specific rows beyond the install guidance

#### Scenario: Daemon error uses existing warning styling
- **WHEN** the daemon cannot start
- **THEN** Settings shows a warning-colored message with remediation steps and no secret values

### Requirement: Chat transcript SHALL distinguish Telegram traffic from harness speakers

Telegram-originated and Telegram-bound lines SHALL use the directional labels from UI-C09-001 §3 while preserving existing harness headers such as `[FREE - model · harness]` (UI-C09-001 §3).

#### Scenario: Harness header unchanged after Telegram input
- **WHEN** Telegram delivers plain text that triggers a harness response
- **THEN** the agent block still uses the existing speaker header format
