# sqlite-operational-store Specification

## ADDED Requirements

### Requirement: Stages MAY store harness session identifiers
The operational store SHALL support persisting an optional harness session id per stage row for Cursor CLI `--resume` continuity across interações within an etapa (design D6).

#### Scenario: Session id persisted
- **WHEN** a stage starts harness-driven interactive work and receives a session id
- **THEN** the store can save and retrieve that id for the stage

#### Scenario: Session cleared on stage complete
- **WHEN** a stage completes
- **THEN** the harness session id for that stage may be cleared or left for audit per SDD
