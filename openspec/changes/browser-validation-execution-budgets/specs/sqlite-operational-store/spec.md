# sqlite-operational-store Specification (delta)

## Purpose
Forward-only schema v16/v17 for budgets, blockers/coverage, screenshot manifests, and approved Planning coverage traceability. ADR-104–106.

## MODIFIED Requirements

### Requirement: Migration to v16 SHALL be transactional and additive
Migration SHALL create stage budget ledger, blocker/coverage, and screenshot-manifest tables with constraints and indexes in one transaction from v15; existing rows (cycles, stages, events, metrics, audit conversation, findings, ToDos, session history) SHALL remain intact; `hero.db` SHALL never be recreated. Budget rows and events SHALL contain no credentials.

#### Scenario: v15 database opens after upgrade
- **WHEN** a v15 `hero.db` with cycles, findings, and session history migrates
- **THEN** every prior row is preserved and the new tables start empty

### Requirement: Migration to v17 SHALL preserve and trace approved coverage
Migration from v16 to v17 SHALL transactionally add requirement/screen/evidence/reference metadata to `stage_coverage` and a `stage_coverage_plans` digest record. Existing v16 coverage rows SHALL retain safe empty/default metadata and SHALL NOT receive an inferred plan snapshot. The approved denominator and non-secret browser-plan digest SHALL be stored atomically; later scope changes SHALL require an explicit approved Planning update. Credentials and raw browser output SHALL NOT be stored.

#### Scenario: v16 coverage rows migrate without invented Planning approval
- **WHEN** a v16 database with existing coverage rows migrates to v17
- **THEN** the coverage rows remain intact with empty/default traceability fields and no approved-plan snapshot

#### Scenario: approved Planning denominator is immutable between approvals
- **WHEN** a browser validation stage starts or resumes after a Planning plan changes without approval
- **THEN** the engine rejects the changed plan without replacing the persisted denominator or consuming a validation attempt
