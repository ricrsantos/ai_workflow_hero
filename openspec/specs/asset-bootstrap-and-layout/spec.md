## Purpose

TBD - Asset bootstrap, Hero-owned layout, template rendering, documents registry, model pricing files, and doctor integrity checks.
## Requirements
### Requirement: Install SHALL bootstrap Hero-owned asset layout from embedded assets
`hero install --tools cursor` SHALL copy Hero commands, agents, skills, templates, and config scaffolding from embedded assets into project-local Hero paths without requiring external downloads (PRD §5.8; DEPLOY §6; ADR-001).

#### Scenario: Fresh install into supported project
- **WHEN** a user runs `hero install --tools cursor` in a valid project directory
- **THEN** required Hero-owned directories and files are created from embedded assets

### Requirement: Runtime asset inventory SHALL follow one-file-per-command-and-agent mapping
The installation layout SHALL include exactly one asset file per Runtime command and one asset file per defined agent prompt, matching documented command and agent lists (PRD §5.9; ADR-011).

#### Scenario: Validating installed Runtime inventory
- **WHEN** installation or doctor validation checks Runtime assets
- **THEN** each required Runtime command and agent file exists at its expected path

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

