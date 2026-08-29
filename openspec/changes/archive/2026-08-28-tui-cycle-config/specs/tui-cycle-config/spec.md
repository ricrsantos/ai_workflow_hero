# tui-cycle-config Specification

## Purpose

Add a guided Config screen to the project TUI for the active cycle’s `workflow-config.yml`, with progressive disclosure, C5 harness/model/property pickers, save states, and Save and start (PRD-C07-001; UI-C07-001; ADR-049/053).

## ADDED Requirements

### Requirement: Config SHALL be available only for an active project cycle

The project sidebar SHALL show Config as the second item when an active cycle exists. `hero chat` SHALL continue to show Chat only. Shortcuts SHALL be Chat `alt+1`, Config `alt+2`, Status `alt+3`, Artifacts `alt+4`, Costs `alt+5`; Events remains palette-reachable when a sixth hint cannot be shown. Missing or invalid YAML SHALL render a red error state, not an editable form (UI-C07-001 §2; PRD-C07-001 §4.1; ADR-049).

#### Scenario: Config appears with an active cycle
- **WHEN** the TUI has an active cycle and the sidebar is visible
- **THEN** nav order is Chat, Config, Status, Artifacts, Costs, Events and Config is reachable with `alt+2`

#### Scenario: Config is absent in free chat
- **WHEN** the user runs `hero chat`
- **THEN** the sidebar shows Chat only and `alt+2` does not open Config

#### Scenario: Invalid YAML is not an editor
- **WHEN** the active YAML cannot be parsed
- **THEN** Config shows a red error with the path, reason, and a suggestion to correct the file manually

### Requirement: The form SHALL use progressive disclosure without deleting hidden YAML values

A disabled stage SHALL show only its toggle and a muted retained-configuration hint. Implementation SHALL show `backend_agent` / `frontend_agent` / `generic_agent` according to active scopes. Stage-specific agents SHALL appear only when their stage is enabled. Browser UI Validation SHALL require frontend scope before revealing visual validation and `browser_ui_agent`. Playwright SHALL require frontend scope. Nested `subagent` SHALL appear only when `same_of_agent` is false. Shared/Advanced SHALL hold orchestration, context, and fallback (PRD-C07-001 §4.3; UI-C07-001 §5).

#### Scenario: Disabled stage hides budgets
- **WHEN** Research is toggled off
- **THEN** purpose, budgets, approval, and discover-agent controls are hidden and their stored values remain in the draft

#### Scenario: Native scope reveals generic_agent
- **WHEN** implementation is enabled and only `scope.native` is true
- **THEN** the form shows `generic_agent` and does not show `backend_agent` or `frontend_agent`

#### Scenario: Subagent appears when not same-of-agent
- **WHEN** the user sets `same_of_agent` to false on a visible agent
- **THEN** subagent model/property controls appear and the model list is limited to the parent harness

### Requirement: Harness, model, and property pickers SHALL reuse C5 discovery and persist to YAML

Each visible agent/fallback SHALL select an enabled harness, then a native model for that harness, then supported `fs`/`th`/`ef` controls. Unavailable enabled harnesses SHALL warn in yellow and SHALL NOT be silently replaced. Explicit form property choices SHALL win over later catalog defaults. Values SHALL be written to the YAML agent/fallback block, not `hero.json.model_properties`. Missing capability metadata SHALL warn and preserve compatible YAML (PRD-C07-001 §4.4; UI-C07-001 §6).

#### Scenario: Unavailable harness is selectable with a warning
- **WHEN** Codex is enabled in `hero.json` but not available on PATH
- **THEN** the harness picker still lists Codex with a `⚠` warning and does not switch the agent to Cursor

#### Scenario: Properties hide when known-unsupported
- **WHEN** capability data is known and the model does not support `ef`
- **THEN** the reasoning-effort control is not shown for that agent

#### Scenario: Explicit property survives catalog refresh
- **WHEN** the user set thinking to an accepted value and a later refresh reports a different default
- **THEN** the form and subsequent Save keep the user’s value

### Requirement: Save actions and dirty-exit SHALL follow the documented states

Save SHALL validate, write, sync, and remain on Config. Save and start SHALL be enabled only for a valid editable form, save first, then follow `/hero-start`. Read-only busy and Saving SHALL disable save actions and show the reason. Leaving a dirty form SHALL prompt Save, Discard, or Cancel. Keyboard bindings SHALL use `key.Matches` (PRD-C07-001 §4.9; UI-C07-001 §§4,7,8; ADR-053).

#### Scenario: Save stays on Config
- **WHEN** the user activates Save on a valid dirty form
- **THEN** the YAML is written, the cycle is synced, and the screen remains Config with a green saved message

#### Scenario: Dirty leave can be cancelled
- **WHEN** the form is dirty and the user presses a nav shortcut
- **THEN** a dialog offers Save, Discard, and Cancel, and Cancel keeps the user on Config with the draft

#### Scenario: Busy state disables Save
- **WHEN** `/hero-start` preflight is running
- **THEN** Save and Save and start are unavailable and the form is muted read-only

### Requirement: Failed stages SHALL expose Retry only after a qualifying Save

Completed stages SHALL be read-only. Failed stages SHALL remain editable. After a successful Save whose managed diff includes that failed stage, Config SHALL show a stage-specific Retry action. Retry SHALL call the engine transition and MUST NOT appear for an unchanged failed configuration (PRD-C07-001 §4.8; UI-C07-001 §9).

#### Scenario: Completed stage cannot be edited
- **WHEN** Research is Completed
- **THEN** its controls are muted with a protected-stage label and cannot change YAML through the form

#### Scenario: Retry appears after a QA config save
- **WHEN** QA is Failed and the user saves a new QA timeout
- **THEN** Config shows Retry for QA and not for other stages

#### Scenario: Title-only save does not enable Retry
- **WHEN** QA is Failed and the user saves only a title change
- **THEN** Retry for QA stays hidden or disabled

### Requirement: Config rendering SHALL stay non-blocking and responsive

File, catalog, SQLite, retry, and preflight work SHALL run in `tea.Cmd`. `Update` SHALL NOT block. Views SHALL use ANSI-aware width, a scrollable form, and a too-small-window fallback rather than clipping Save or errors (UI-C07-001 §10; ADR-053).

#### Scenario: Load uses a command
- **WHEN** the user opens Config
- **THEN** the screen shows a loading state until a load message returns, without blocking the Bubble Tea loop

#### Scenario: Narrow terminal keeps actions
- **WHEN** the terminal cannot fit descriptions, validation, and the action row
- **THEN** descriptions collapse first and Save/error copy remain visible, or a centered too-small message is shown
