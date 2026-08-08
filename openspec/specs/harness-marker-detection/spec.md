# harness-marker-detection Specification

## Purpose
TBD - created by archiving change slash-parity-tui-harness. Update Purpose after archive.
## Requirements
### Requirement: CLI SHALL detect known harness filesystem markers
On `hero install` and `hero doctor`, the system SHALL detect known harness directories at the project root — at minimum `.cursor/`, `.claude/`, `.windsurf/`, and `.codex/` — and compare the detected set to `.workflow-hero/config/hero.json` → `cli.tools` (PRD-C02-001 §5.3; ADR-022).

#### Scenario: Supported marker matches cli.tools
- **WHEN** `.cursor/` exists and `cli.tools` includes `cursor`
- **THEN** detection does not emit an unsupported-harness warning for Cursor

#### Scenario: Unsupported marker present
- **WHEN** `.claude/` exists and `cli.tools` only lists `cursor`
- **THEN** `hero doctor` emits a warn-only message that `.claude/` was detected but is unsupported in this Hero version and was not installed

### Requirement: Detection MUST be warn-only for unsupported harnesses
For markers without a Hero adapter/assets path, the system MUST warn or suggest only. It MUST NOT materialize assets, update `cli.tools` automatically to claim support, or install unsupported harness packs (ADR-022; Deferred D1).

#### Scenario: Install sees unsupported marker
- **WHEN** the user runs `hero install --tools cursor` in a tree that also has `.windsurf/`
- **THEN** install completes for Cursor and surfaces a suggestion/warning about the unsupported marker without installing Windsurf assets

### Requirement: Runtime sync SHALL surface harness detection via doctor
Because `hero sync` does not exist as a CLI command (ADR-003), `/hero:sync` Runtime guidance SHALL instruct the agent to run `hero doctor` after sync so harness-marker warnings are visible (PRD-C02-001 §5.3 reconciled with ADR-003; UI-C02-001 §5).

#### Scenario: Sync handoff includes doctor
- **WHEN** `/hero:sync` completes artifact generation
- **THEN** Runtime guidance tells the agent/user to run `hero doctor` (slash-first user messaging may still prefer `/hero:status` / help alongside doctor for review)

