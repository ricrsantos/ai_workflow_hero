# hero-tui Specification

## MODIFIED Requirements

### Requirement: TUI Hero action labels SHALL use slash vocabulary
Palette items that invoke Hero cycle actions SHALL use **`/hero-<name>`** hyphen labels per UI-C03-001 §4 (e.g. `/hero-approve`, `/hero-start`). Screen navigation entries (`Go: …`) MAY remain non-slash. Execution MUST continue through `cycle.Service` / harness paths as appropriate (ADR-024 amends ADR-020).

#### Scenario: Approve label is hyphen slash form
- **WHEN** the user opens the command palette
- **THEN** the approve action is labeled `/hero-approve` (not `/hero:approve` or “Approve stage”)

#### Scenario: Empty cycle hint prefers slash
- **WHEN** there is no active cycle
- **THEN** the empty-state hint mentions `/hero-new` as primary guidance

### Requirement: TUI SHALL expose all Hero slash commands
The palette SHALL include actions for all installed `hero-*.md` Runtime commands, including `/hero-new`, `/hero-start`, `/hero-sync`, `/hero-status`, `/hero-continue`, `/hero-back`, `/hero-cycles`, and `/hero-todos` (UI-C03-001 §4; PRD-C03-001 §4.3).

#### Scenario: Start command present
- **WHEN** the user opens the command palette during an active cycle
- **THEN** `/hero-start` or context-appropriate Hero commands are available per SDD wiring

### Requirement: TUI MAY expose archive resume and help as slash actions
When archive, resume, or help actions are exposed in the palette, their labels SHALL be `/hero-archive`, `/hero-resume`, and `/hero-help` respectively (UI-C03-001 §4).

#### Scenario: Archive action label
- **WHEN** archive is available in the palette
- **THEN** its label is `/hero-archive`

### Requirement: TUI SHALL group imported harness commands separately from Hero actions
Imported non-Hero commands SHALL appear under a distinct group/prefix filterable like other palette items (unchanged from C2; UI-C03-001).

#### Scenario: Filter finds imported command
- **WHEN** an imported command `/opsx-propose` is present and the user filters `opsx`
- **THEN** that item appears in the filtered palette results
