## ADDED Requirements

### Requirement: Schema v11 SHALL add findings and ToDo tables without rewriting existing rows
Opening a schema-v10 database SHALL apply a forward-only transactional migration to version 11 that creates constrained findings, occurrence, ToDo, adoption, and projection-op storage plus cycle disposition columns. The migration MUST NOT recreate `hero.db`. Existing cycles, stages, events, metrics, conversation records, artifacts, process registries, and model caches SHALL remain readable and unchanged. Empty new tables SHALL produce empty additive JSON and existing visible behavior (PRD-C15-001 §11; ADR-083; ADR-087).

#### Scenario: v10 database migrates on open
- **WHEN** a project with schema version 10 is opened by the C15 store
- **THEN** migration v11 is applied, the new structures are available, and existing operational rows remain intact

#### Scenario: No synthetic findings are invented
- **WHEN** a migrated v10 database has no prior findings or ToDos
- **THEN** finding and ToDo queries return empty results and no placeholder rows are inserted
