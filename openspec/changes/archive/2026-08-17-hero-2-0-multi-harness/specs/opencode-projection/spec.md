# opencode-projection Specification

## Purpose
Provision OpenCode on-disk layout from Hero `assets/` on enable; disable keeps files (ADR-036; PRD-C04-001 §4.6).

## ADDED Requirements

### Requirement: Enable SHALL provision projection from assets
Enabling OpenCode (install or `/hero-harness`) SHALL write `.opencode/agents`, `.opencode/commands`, `.opencode/skills` from embedded `assets/opencode/` and track checksums (design D7).

#### Scenario: Enable via hero-harness
- **WHEN** the user enables OpenCode in the TUI
- **THEN** `.opencode/` is created and the success line mentions projected `.opencode/`

### Requirement: Disable SHALL NOT delete projection files
Disabling OpenCode SHALL set `enabled: false` only and SHALL NOT remove `.opencode/` files (ADR-036).

#### Scenario: Disable keeps files
- **WHEN** the user disables OpenCode after provisioning
- **THEN** `.opencode/` files remain on disk

### Requirement: AGENTS.md SHALL NOT be copied into opencode projection
Root `AGENTS.md` SHALL NOT be duplicated into `.opencode/` (ADR-036).

#### Scenario: Projection inventory
- **WHEN** OpenCode projection is written
- **THEN** `AGENTS.md` is not among the projected paths

### Requirement: Checksum non-overwrite SHALL apply to opencode paths
Customized `.opencode/` files SHALL follow the same checksum non-overwrite rules as `.cursor/` on upgrade (PRD-C04-001 §4.1).

#### Scenario: Customized opencode file on upgrade
- **WHEN** upgrade runs and a `.opencode/` file was customized
- **THEN** Hero warns and does not overwrite that file
