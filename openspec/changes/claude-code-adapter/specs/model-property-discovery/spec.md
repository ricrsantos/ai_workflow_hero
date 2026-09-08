## ADDED Requirements

### Requirement: Claude capability discovery SHALL be lazy and runtime-authoritative

Opening `/hero-model` SHALL use the local Claude catalog immediately and SHALL not start a Claude session merely to populate the picker. When an actual Claude turn emits `system/init`, its effective model and capabilities SHALL supersede the local snapshot for that turn and future cached resolution, subject to existing background-refresh behavior.

#### Scenario: Picker does not authenticate Claude
- **WHEN** `/hero-model` opens with Claude enabled but no active turn
- **THEN** the picker renders catalog values without launching a Claude child or authentication flow

#### Scenario: system/init supersedes catalog
- **WHEN** an executing Claude turn reports a resolved model or capability set in `system/init`
- **THEN** the TUI and metrics use that effective information for the turn rather than the stale alias metadata

