## ADDED Requirements

### Requirement: Opening a stage conversation SHALL NOT mutate workflow lifecycle
Resuming or continuing a Research, orchestration, or named stage-agent conversation from History SHALL NOT call stage start, change stage status, reset iterations, or dispatch scheduler transitions merely because the conversation was opened. Completed stages SHALL remain completed. Historical continuation is Chat/session navigation, not a workflow control command (PRD-C16-001 §3.5, §4 FR-06, §6; ADR-095).

#### Scenario: Completed Implementation stays completed
- **WHEN** Implementation is completed and the user opens a GEN History row
- **THEN** engine stage status remains completed and no Implementation Execute is dispatched by that navigation

#### Scenario: Active Execute is unchanged by History browse
- **WHEN** QA is Running and the user browses History without opening a different session for continuation
- **THEN** the Running QA Execute continues and stage status is unchanged
