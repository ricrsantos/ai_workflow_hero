# asset-bootstrap-and-layout Specification

## Purpose
Install/upgrade materialization for multi-harness projections and OpenCode asset paths (ADR-036; PRD-C04-001 §4.6, §4.12).

## MODIFIED Requirements

### Requirement: Install SHALL materialize projections for selected harnesses only
Core `.workflow-hero/` installs once; Cursor assets copy when cursor enabled; OpenCode projection when opencode enabled (ADR-036).

#### Scenario: Cursor-only install
- **WHEN** install selects only Cursor
- **THEN** `.cursor/` is written and `.opencode/` is not created

#### Scenario: Both harnesses install
- **WHEN** install selects Cursor and OpenCode
- **THEN** both `.cursor/` and `.opencode/` projections are written

### Requirement: workflow-config template SHALL include harness on all agents
`assets/templates/workflow-config.yml` SHALL include `harness` on every agent and `fallback_model` (PRD-C04-001 §4.3, §4.12).

#### Scenario: Template agents
- **WHEN** a new project is installed
- **THEN** the template workflow-config agents each have a `harness` field

### Requirement: Embedded assets SHALL include opencode projection source
`assets/opencode/` SHALL be embedded alongside `assets/cursor/` for provision-on-enable (design D7).

#### Scenario: Embed inventory
- **WHEN** asset inventory tests run
- **THEN** `assets/opencode/` paths are present in embed.FS
