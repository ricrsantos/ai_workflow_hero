# asset-bootstrap-and-layout Specification (delta)

## Purpose
Committed placeholder example and upgrade hygiene for test credentials. PRD FR-02; DEPLOY §C17.

## MODIFIED Requirements

### Requirement: Install and upgrade SHALL provision only placeholder examples
Install/upgrade SHALL provision root `.env.hero.example` with placeholders only (the explicitly user-approved exception to the `.env.example`-only policy), never overwrite a customized example or any application `.env`, never create credential values, and never copy `.env.hero` into checksums, backups, snapshots, or release artifacts. Existing IDE resources SHALL NOT be removed by this browser cycle.

#### Scenario: Upgrade with customized example
- **WHEN** upgrade runs and root `.env.hero.example` was customized
- **THEN** the customized file is preserved and no credential values are created
