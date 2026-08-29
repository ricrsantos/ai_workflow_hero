# hero-tui Specification

## MODIFIED Requirements

### Requirement: Project TUI navigation SHALL include Config as the second item when a cycle is active

The left nav SHALL insert Config between Chat and Status when an active cycle exists. Shortcuts SHALL shift so Chat remains `alt+1`, Config is `alt+2`, Status `alt+3`, Artifacts `alt+4`, and Costs `alt+5`. Events SHALL remain reachable from the command palette when the footer cannot show a sixth shortcut. Free-chat mode SHALL continue to show Chat only (UI-C07-001 §2; PRD-C07-001 §4.1; ADR-049).

#### Scenario: Active-cycle nav order
- **WHEN** the project TUI has an active cycle and the sidebar is visible
- **THEN** labels appear in order Chat, Config, Status, Artifacts, Costs, Events

#### Scenario: Status shortcut moves to alt+3
- **WHEN** an active cycle exists and the user presses `alt+3`
- **THEN** the Status screen opens (not Config)

#### Scenario: Free chat ignores Config
- **WHEN** `freeChatMode` is true
- **THEN** Config is not listed and `alt+2` does not change the Chat screen

### Requirement: Existing Chat, palette, and harness flows SHALL keep current behavior

Adding Config SHALL not change Chat transcript, `/hero-model` property picking, `/hero-harness`, `/harness-reset`, Cursor/OpenCode/Codex Execute routing, or free-chat configuration stored in `hero.json.model_properties` (PRD-C07-001 §4.10; UI-C07-001 §12).

#### Scenario: /hero-model still writes freechat properties
- **WHEN** the user completes `/hero-model` property save from Chat
- **THEN** `hero.json.model_properties` is updated and `workflow-config.yml` agent blocks are not written

#### Scenario: Chat remains the default boot screen
- **WHEN** `hero tui` starts with an active cycle
- **THEN** the initial screen is Chat and Config is available via nav
