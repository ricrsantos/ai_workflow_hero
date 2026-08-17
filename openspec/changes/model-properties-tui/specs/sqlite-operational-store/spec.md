# sqlite-operational-store Specification

## Purpose

Extend the project operational store with normalized model-list/capability cache data for C5 while preserving SQLite as the sole Hero operational store (PRD-C05-001 §4.2/§5; ADR-039).

## MODIFIED Requirements

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
