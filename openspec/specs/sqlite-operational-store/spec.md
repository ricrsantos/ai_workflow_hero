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

### Requirement: Schema v5 SHALL persist project-scoped model metadata and refresh timestamps

Opening a schema-v4 database SHALL apply a migration that creates project-scoped storage for normalized model lists and per-harness/native-model capabilities, including accepted values, defaults, availability, and an RFC3339 refresh timestamp. The cache SHALL not be global and SHALL not be stored in `hero.json`; existing v4 operational data SHALL remain readable (PRD-C05-001 §4.2.8–10; §5.1; ADR-039).

#### Scenario: v4 database migrates on open
- **WHEN** a project with schema version 4 is opened by the C5 store
- **THEN** migration v5 is applied, the cache structures are available, and existing cycles/stages/events remain readable

#### Scenario: Cache is isolated by project
- **WHEN** two projects cache the same harness/model pair with different values
- **THEN** reading one project's store never returns the other project's capabilities

#### Scenario: Cache round trip preserves timestamp and metadata
- **WHEN** normalized capabilities are upserted for a harness/model pair
- **THEN** a subsequent read returns the same dynamic values/default/availability and the persisted refresh timestamp

### Requirement: Cache writes SHALL make successful API data authoritative and support stale fallback

Store helpers SHALL replace the cached accepted-value arrays/defaults for a model/property on a successful API refresh. When live refresh fails, callers SHALL be able to read the cached row regardless of age and distinguish that fallback using its timestamp; a failed refresh SHALL not delete valid cached metadata (PRD-C05-001 §4.2.9–10; ADR-039).

#### Scenario: Successful refresh replaces cached values
- **WHEN** the cache contains `ef=[low,high]` and a live response succeeds with `ef=[medium]`
- **THEN** the cache returns only `[medium]` and the new default/timestamp

#### Scenario: Failed refresh retains old data
- **WHEN** live discovery fails after a cache row was written
- **THEN** the old row remains readable with its original timestamp so the resolver can mark it stale

### Requirement: Schema v11 SHALL add findings and ToDo tables without rewriting existing rows
Opening a schema-v10 database SHALL apply a forward-only transactional migration to version 11 that creates constrained findings, occurrence, ToDo, adoption, and projection-op storage plus cycle disposition columns. The migration MUST NOT recreate `hero.db`. Existing cycles, stages, events, metrics, conversation records, artifacts, process registries, and model caches SHALL remain readable and unchanged. Empty new tables SHALL produce empty additive JSON and existing visible behavior (PRD-C15-001 §11; ADR-083; ADR-087).

#### Scenario: v10 database migrates on open
- **WHEN** a project with schema version 10 is opened by the C15 store
- **THEN** migration v11 is applied, the new structures are available, and existing operational rows remain intact

#### Scenario: No synthetic findings are invented
- **WHEN** a migrated v10 database has no prior findings or ToDos
- **THEN** finding and ToDo queries return empty results and no placeholder rows are inserted
