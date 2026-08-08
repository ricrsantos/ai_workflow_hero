## ADDED Requirements

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
