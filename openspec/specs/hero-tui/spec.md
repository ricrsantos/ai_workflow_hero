# hero-tui Specification

## Purpose
TBD - created by archiving change slash-parity-tui-harness. Update Purpose after archive.

## Requirements

### Requirement: TUI Hero action labels SHALL use slash vocabulary
Palette items that invoke Hero cycle actions SHALL use `/hero:*` labels per UI-C02-001 §2 (e.g. `/hero:approve`, `/hero:reject`, `/hero:cancel`, `/hero:finish`). Screen navigation entries (`Go: …`) MAY remain non-slash. Execution MUST continue through existing `cycle.Service` / CLI API paths (rename only) (PRD-C02-001 §5.1; ADR-020; ADR-015 naming parity).

#### Scenario: Approve label is slash form
- **WHEN** the user opens the command palette
- **THEN** the approve action is labeled `/hero:approve` (not “Approve stage”)

#### Scenario: Empty cycle hint prefers slash
- **WHEN** there is no active cycle
- **THEN** the empty-state hint mentions `/hero:new` as the primary guidance

### Requirement: TUI MAY expose archive resume and help as slash actions
When archive, resume, or help actions are exposed in the palette, their labels SHALL be `/hero:archive`, `/hero:resume`, and `/hero:help` respectively (UI-C02-001 §2).

#### Scenario: Archive action label
- **WHEN** archive is available in the palette
- **THEN** its label is `/hero:archive`

### Requirement: TUI SHALL group imported harness commands separately from Hero actions
Imported non-Hero commands SHALL appear under a distinct group/prefix (e.g. “Harness commands”) filterable like other palette items (UI-C02-001 §3; capability `harness-command-import`).

#### Scenario: Filter finds imported command
- **WHEN** an imported command `/opsx-propose` is present and the user filters `opsx`
- **THEN** that item appears in the filtered palette results

### Requirement: `/hero-model` SHALL open a property submenu after model selection when editable metadata exists

The existing harness-first/model-second C4 flow SHALL remain intact. After a model row is selected, the TUI SHALL build the best available C5 snapshot immediately. If at least one supported property is selectable, it SHALL open a property screen headed `/hero-model · <Harness> · properties`; if none is selectable, it SHALL save the pair immediately and return to Chat. Pending background refresh SHALL not block the flow (PRD-C05-001 §4.1.1–3; §4.2.4–7; §4.4.8; UI-C05-001 §§2–3; ADR-042).

#### Scenario: Full capability model opens properties
- **WHEN** the user selects a model with editable `fs`, `th`, or `ef` metadata
- **THEN** the TUI opens `/hero-model · <Harness> · properties` instead of closing the model picker

#### Scenario: No selectable property skips the submenu
- **WHEN** the selected model has no available editable C5 property
- **THEN** the pair is saved immediately and the property screen is not shown

#### Scenario: Cached values do not block on refresh
- **WHEN** live capability refresh is still pending as the user selects a model
- **THEN** the property screen uses the best cache/catalog values and remains interactive

### Requirement: The property picker SHALL use Hero's established keyboard and disabled-row conventions

The main picker SHALL show friendly rows `Fast model`, `Thinking`, and `Reasoning effort` with current values and a footer containing `ENTER to save`. Available boolean properties SHALL use Space to toggle. Multi-value properties SHALL open a secondary list with arrow navigation and Enter confirmation. Unsupported properties SHALL stay visible, disabled, and gray. Main Enter SHALL save the complete draft; Escape from the main or secondary picker SHALL cancel the complete model/property selection (PRD-C05-001 §4.4; UI-C05-001 §3; ADR-042).

#### Scenario: Boolean property toggles with Space
- **WHEN** the Fast model row is available and selected and the user presses Space
- **THEN** its draft value toggles without writing `hero.json`

#### Scenario: Multi-value property uses a secondary list
- **WHEN** the user presses Enter on an available Thinking row with multiple accepted values
- **THEN** a secondary value list opens, arrow navigation changes the draft choice, and Enter returns to the main property picker

#### Scenario: Unsupported row is visible but inert
- **WHEN** a model does not expose Reasoning effort
- **THEN** the row remains visible with `na`/unavailable text in gray and Space/Enter cannot edit it

#### Scenario: Main footer communicates the commit point
- **WHEN** the main property picker is rendered
- **THEN** its interaction guidance clearly includes `ENTER to save` and `esc cancel`

### Requirement: Chat SHALL render an effective C5 property line beside scroll and context information

When a model is selected, Chat SHALL show a line below the green response pane, beside the response scroll hint and context bar, even when the transcript is empty. It SHALL render the stable labels `[fs-<value>] [th-<value>] [ef-<value>]`. Validated configured `th`/`ef` values SHALL be green; `na`, unavailable, and unvalidated values SHALL be gray; `fs` SHALL be green only when validated and enabled, and gray when disabled or unavailable. Normal Chat and `/hero-new` SHALL use freechat values; workflow execution SHALL use the active agent projection. Capability source details SHALL not become permanent chrome; only relevant fallback warnings expose them (PRD-C05-001 §4.6; UI-C05-001 §§4,6; ADR-042).

#### Scenario: Complete freechat properties are visible in an empty Chat
- **WHEN** a freechat model is selected with `fs=true`, `th=max`, and `ef=high` before the first message
- **THEN** the line shows `[fs-true] [th-max] [ef-high]` beside the scroll/context information, with the configured labels green

#### Scenario: Fast-off and unavailable values are semantically gray
- **WHEN** fast is configured as `false` or a property is unavailable
- **THEN** the textual label remains visible and is rendered gray, including `[fs-false]` or `[fs-na]`

#### Scenario: Narrow terminal preserves all status content
- **WHEN** the terminal is too narrow for the scroll hint, all property labels, and the context bar on one row
- **THEN** the status renderer wraps at rune-safe boundaries and does not hide any property label or the context bar

### Requirement: Property warnings and terminal accessibility SHALL use semantic status behavior

Missing catalog, stale-cache fallback, and invalidated-value warnings SHALL use the existing yellow `⚠` warning style in the same status area used for execution errors, without being fatal. A warning SHALL clear on the next user action, while an adapter property rejection remains an explicit execution error. Labels and `na` text SHALL preserve meaning without color; no-color/non-TTY semantic fallback and rune-safe rendering SHALL remain safe with multi-byte icons and model names (PRD-C05-001 §4.6.8–10; UI-C05-001 §§5,7–8; ADR-042).

#### Scenario: Missing catalog warning is yellow and actionable
- **WHEN** no live, cache, or catalog metadata exists for the selected model
- **THEN** the status area shows a yellow warning equivalent to `No catalog is available for the selected model. Model properties will use their default values.` and Chat remains usable

#### Scenario: Warning clears after user action
- **WHEN** a stale-data warning is visible and the user presses a picker/navigation key
- **THEN** the warning is cleared without changing the selected property draft unless the action commits it

#### Scenario: No-color output remains understandable
- **WHEN** color is disabled for a rendered Chat/status fixture
- **THEN** all three textual labels, `na` values, and warning wording remain present without relying on ANSI color

#### Scenario: Multi-byte content does not panic
- **WHEN** the status line wraps a multi-byte warning, icon, or model name in a narrow terminal
- **THEN** rendering completes without slicing invalid UTF-8 or dropping property labels
