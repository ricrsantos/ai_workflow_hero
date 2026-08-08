## ADDED Requirements

### Requirement: Install and doctor SHALL integrate harness marker detection
`hero install` and `hero doctor` SHALL invoke harness filesystem marker detection and surface suggestions/warnings consistent with UI-C02-001 §5 without installing unsupported harness assets (PRD-C02-001 §5.3; ADR-022; capability `harness-marker-detection`).

#### Scenario: Doctor reports unsupported harness marker
- **WHEN** `.claude/` exists and `cli.tools` does not include a supported entry for it
- **THEN** doctor output includes a `⚠` warning describing the divergence and that the harness is unsupported in this Hero version

#### Scenario: Install still succeeds with extra markers
- **WHEN** install runs with `--tools cursor` and an unsupported marker directory is present
- **THEN** Cursor assets install successfully and a warning/suggestion is emitted for the unsupported marker
