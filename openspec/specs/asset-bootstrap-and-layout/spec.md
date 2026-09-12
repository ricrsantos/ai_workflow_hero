## Purpose

TBD - Asset bootstrap, Hero-owned layout, template rendering, documents registry, model pricing files, and doctor integrity checks.

## Requirements

### Requirement: Install SHALL bootstrap Hero-owned asset layout from embedded assets
`hero install --tools cursor` SHALL copy Hero commands, agents, skills, templates, and config scaffolding from embedded assets into project-local Hero paths without requiring external downloads (PRD §5.8; DEPLOY §6; ADR-001).

#### Scenario: Fresh install into supported project
- **WHEN** a user runs `hero install --tools cursor` in a valid project directory
- **THEN** required Hero-owned directories and files are created from embedded assets

### Requirement: Runtime asset inventory SHALL follow one-file-per-command-and-agent mapping
The installation layout SHALL include exactly one asset file per Runtime command and one asset file per defined agent prompt, matching documented command and agent lists (PRD §5.9; ADR-011). The agent inventory SHALL include `browser_ui_agent` in addition to the existing V1 agents.

#### Scenario: Validating installed Runtime inventory
- **WHEN** installation or doctor validation checks Runtime assets
- **THEN** each required Runtime command and agent file exists at its expected path, including `browser_ui_agent`

#### Scenario: Embedded browser_ui_agent asset present
- **WHEN** the embedded Cursor agent assets are inventoried
- **THEN** `cursor/agents/browser_ui_agent.md` is present with Cursor YAML frontmatter `model: inherit`

### Requirement: Template rendering SHALL support only simple placeholders
Template processing SHALL support only direct placeholder replacement in the form `{{path.key}}` and SHALL NOT require or interpret loop or conditional templating constructs (ADR-006).

#### Scenario: Rendering a template with scalar placeholders
- **WHEN** a template contains simple placeholders such as `{{project.name}}`
- **THEN** placeholders resolve from config values and no loop syntax is required for output generation

### Requirement: Documents registry and planning context SHALL remain schema-consistent
Hero-generated planning and document references SHALL be represented through `.workflow-hero/config/documents.json` and consumed to compose planning context as documented in ADR Appendix and ADR-007.

#### Scenario: Preparing planning context from documents registry
- **WHEN** planning context is prepared for OpenSpec operations
- **THEN** authoritative document references are derived from `documents.json` entries instead of hardcoded lists

### Requirement: Model pricing references SHALL be maintained as structured provider files
`hero update-models` SHALL refresh `models/*.yml` from the official pre-structured source and SHALL avoid scraping or parsing raw pricing pages with reasoning (PRD §5.8, §5.10; DEPLOY §8; ADR-003).

#### Scenario: Updating model reference files
- **WHEN** a user runs `hero update-models`
- **THEN** provider model files are rewritten from the structured upstream data source and remain compatible with metrics estimation inputs

### Requirement: Doctor SHALL verify installation integrity and version consistency
`hero doctor` SHALL verify expected file presence, config syntax, git prerequisite, and consistency between installed metadata and running CLI version where documented (DEPLOY §7; ADR Appendix).

#### Scenario: Running doctor on inconsistent install
- **WHEN** installed metadata or required paths are inconsistent
- **THEN** doctor reports structured failure diagnostics and actionable guidance

### Requirement: Install and doctor SHALL integrate harness marker detection
`hero install` and `hero doctor` SHALL invoke harness filesystem marker detection and surface suggestions/warnings consistent with UI-C02-001 §5 without installing unsupported harness assets (PRD-C02-001 §5.3; ADR-022; capability `harness-marker-detection`).

#### Scenario: Doctor reports unsupported harness marker
- **WHEN** `.claude/` exists and `cli.tools` does not include a supported entry for it
- **THEN** doctor output includes a `⚠` warning describing the divergence and that the harness is unsupported in this Hero version

#### Scenario: Install still succeeds with extra markers
- **WHEN** install runs with `--tools cursor` and an unsupported marker directory is present
- **THEN** Cursor assets install successfully and a warning/suggestion is emitted for the unsupported marker

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

### Requirement: Canonical agent assets SHALL carry C15 report contracts for every supported harness
Embedded Cursor, OpenCode, Codex, and Claude agent sources for orchestration, discover, implementation (backend/frontend/generic), QA, Judge, Browser UI, and QA End-to-End SHALL include complete field rules, valid enum values, owner rules, one passing example, one reopening example where applicable, empty-success arrays, and an explicit prohibition on stage transitions, OpenSpec checkbox edits, gap files, and `current-state.md` mutation. Installed projections SHALL be generated from those canonical assets so contracts do not drift by harness (PRD-C15-001 §6.1, §14.16).

#### Scenario: Judge assets no longer write gap files
- **WHEN** the canonical Judge agent for each supported harness is inspected
- **THEN** it instructs the agent to emit JSON and stop, and it does not tell the agent to write `judge-gaps.md` or run `hero stage loop-back`

#### Scenario: Implementation example includes a find-* ID
- **WHEN** the canonical generic/backend/frontend agent assets are inspected
- **THEN** they include a valid example where `tasks_completed` may contain a `find-*` ID from the explicit assignment

### Requirement: Runtime command inventory SHALL include add-todo and complete-todo
The installation layout SHALL include `hero-add-todo` and `hero-complete-todo` command assets for each supported harness, plus updates to start, status, todos, continue, finish, and help. Upgrade SHALL refresh those files under the existing checksum/conflict policy; customized assets are warned, not silently overwritten. Embedded workflow-help SHALL document escalation triage, Research adoption, and manual completion (PRD-C15-001 §10.3–10.4).

#### Scenario: Fresh or upgraded inventory contains the new commands
- **WHEN** doctor or install inventory is checked after C15 assets ship
- **THEN** `hero-add-todo` and `hero-complete-todo` exist for each enabled harness projection

#### Scenario: Customized Judge prompt is not overwritten
- **WHEN** an installed Judge asset checksum diverges from the embedded original
- **THEN** upgrade warns and does not silently replace the customized file
