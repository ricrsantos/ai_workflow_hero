# sqlite-operational-store Specification

## Purpose
TBD - created by archiving change slash-parity-tui-harness. Update Purpose after archive.
## Requirements
### Requirement: cycles table SHALL store openspec_change
Schema migration version 2 SHALL add `openspec_change TEXT NOT NULL DEFAULT ''` to `cycles`. Store APIs SHALL read and write this field on cycle records (ADR-013 extension; ADR-023; design D5).

#### Scenario: Migration applies on open
- **WHEN** a store at schema v1 is opened by a C2 binary
- **THEN** migration v2 runs and `openspec_change` is available (empty for existing rows)

#### Scenario: Update openspec_change
- **WHEN** the store updates a cycle’s `openspec_change` to a non-empty slug
- **THEN** a subsequent read returns that slug

