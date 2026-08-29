# sqlite-operational-store Specification

## MODIFIED Requirements

### Requirement: Store SHALL record failed-stage retry as an append-only event

Retry SHALL insert an event with type `stage_retried` and a JSON payload naming the stage. No schema version bump is required. Existing events and metrics tables SHALL remain the history of prior attempts (ADR-052; PRD-C07-001 §4.8).

#### Scenario: Retry event is stored
- **WHEN** `RetryFailedStage` succeeds for `qa`
- **THEN** `ListEvents` includes a `stage_retried` row whose payload contains `"qa"`

#### Scenario: Metrics rows are not deleted
- **WHEN** a Failed stage has metrics from the failed attempt and is retried
- **THEN** those metric rows remain queryable after the status becomes `Waiting`

#### Scenario: Schema version is unchanged
- **WHEN** a v6 (or current) database is opened after this change
- **THEN** no new migration is required solely for retry events
