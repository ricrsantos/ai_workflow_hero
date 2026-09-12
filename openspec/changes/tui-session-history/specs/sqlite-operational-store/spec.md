## ADDED Requirements

### Requirement: Schema v12 SHALL add session-history tables without rewriting existing rows
Opening a schema-v11 database SHALL apply a forward-only transactional migration to version 12 that creates constrained `sessions`, `session_events`, `session_assets`, `session_leases`, and `session_delete_ops` storage plus the indexes required for `last_activity DESC`, case-insensitive title lookup, bounded event paging, and idempotent provider-event import. The migration MUST NOT recreate `hero.db`. Existing cycles, stages, events, metrics, audit `conversation` records, artifacts, process registries, model caches, findings, and ToDos SHALL remain readable and unchanged. The audit `conversation` table SHALL NOT be dropped or repurposed (PRD-C16-001 §5; ADR-091–092, ADR-098).

#### Scenario: v11 database migrates on open
- **WHEN** a project with schema version 11 is opened by the C16 store
- **THEN** migration v12 is applied, the new structures are available, and existing operational rows remain intact

#### Scenario: Audit conversation rows survive
- **WHEN** a v11 fixture contains `stage_agent_assignment` conversation entries
- **THEN** those rows remain readable after v12 and are not copied into `session_events`

#### Scenario: Empty History when no bindings exist
- **WHEN** a migrated v11 database has no orchestration or stage harness bindings
- **THEN** session queries return empty results and no placeholder rows are inserted
