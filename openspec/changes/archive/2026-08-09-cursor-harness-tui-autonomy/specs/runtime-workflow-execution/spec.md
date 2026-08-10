# runtime-workflow-execution Specification

## MODIFIED Requirements

### Requirement: User-facing Runtime vocabulary SHALL use hyphen slash commands
User-facing Runtime/orchestrator strings SHALL prefer `/hero-<name>` (hyphen) for Hero commands (ADR-024 amends ADR-020). Clean-session handoff after `/hero-new` tells users to run `/hero-start`.

#### Scenario: Start handoff uses hyphen form
- **WHEN** configuration completes and the orchestrator guides the user
- **THEN** the primary CTA references `/hero-start` not `/hero:start`

### Requirement: Runtime SHALL provide hero-cycles and hero-todos commands
Hero Runtime assets SHALL include `hero-cycles.md` and `hero-todos.md` discoverable as `/hero-cycles` and `/hero-todos` in Cursor chat (ADR-028).

#### Scenario: Cycles command asset exists
- **WHEN** Hero assets are installed
- **THEN** `hero-cycles.md` is present under `.cursor/commands/`

#### Scenario: Todos command asset exists
- **WHEN** Hero assets are installed
- **THEN** `hero-todos.md` is present under `.cursor/commands/`
