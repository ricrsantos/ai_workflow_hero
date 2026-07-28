## MODIFIED Requirements

### Requirement: Runtime asset inventory SHALL follow one-file-per-command-and-agent mapping
The installation layout SHALL include exactly one asset file per Runtime command and one asset file per defined agent prompt, matching documented command and agent lists (PRD §5.9; ADR-011). The agent inventory SHALL include `browser_ui_agent` in addition to the existing V1 agents.

#### Scenario: Validating installed Runtime inventory
- **WHEN** installation or doctor validation checks Runtime assets
- **THEN** each required Runtime command and agent file exists at its expected path, including `browser_ui_agent`

#### Scenario: Embedded browser_ui_agent asset present
- **WHEN** the embedded Cursor agent assets are inventoried
- **THEN** `cursor/agents/browser_ui_agent.md` is present with Cursor YAML frontmatter `model: inherit`

## ADDED Requirements

### Requirement: Bootstrap templates SHALL include Browser UI Validation configuration
The embedded `workflow-config.yml` template SHALL include `stages.browser_ui_validation` with standard stage fields (`enabled`, `purpose`, `max_iterations`, `timeout_minutes`, `require_human_approval`), nested `visual_validation.enabled` and `visual_validation.reference_dir` (default `docs/ui/visual_reference`), and `agents.browser_ui_agent` model block fields matching other agents. Defaults SHALL be `stages.browser_ui_validation.enabled: false` and `visual_validation.enabled: false`. Workflow rules SHALL document that enabling the stage requires `scope.frontend: true`.

#### Scenario: Fresh install materializes browser_ui_validation keys
- **WHEN** a user runs `hero install --tools cursor` (or receives a non-customized template refresh on upgrade)
- **THEN** the cycle `workflow-config.yml` template contains `browser_ui_validation` stage keys and `browser_ui_agent` under `agents`

### Requirement: Metrics and workflow templates SHALL include Browser UI Validation
Embedded `workflow.md` and `metrics.md` templates SHALL include a Browser UI Validation / `browser_ui_agent` row consistent with other stages so Runtime can record status and metrics without inventing schema.

#### Scenario: Metrics template lists the new stage
- **WHEN** cycle metrics artifacts are created from templates
- **THEN** Browser UI Validation appears as a stage row alongside QA, Judge, and QA End-to-End

### Requirement: End-user documentation SHALL describe Browser UI Validation
Embedded `workflow-help.md` and the project README (EN + PT-BR) SHALL document the stage order including Browser UI Validation, Playwright prerequisite for Health, optional Visual Validation with PNG references, configuration keys, failure routing summary, and artifact path `.workflow-hero/cycles/current/browser-ui/`.

#### Scenario: User reads workflow help after install
- **WHEN** a user opens `.workflow-hero/docs/workflow-help.md` from a post-0.6.0 install
- **THEN** Browser UI Validation usage and `workflow-config.yml` keys are documented in both English and Portuguese sections as applicable

### Requirement: CLI default version SHALL be 0.6.0 for this release line
The Hero CLI default embedded version SHALL be `0.6.0` after this change lands, reflecting the new Runtime stage/agent feature.

#### Scenario: hero version reports 0.6.0
- **WHEN** a user runs `hero version` on a build of this change without overriding ldflags
- **THEN** the reported version is `0.6.0`
