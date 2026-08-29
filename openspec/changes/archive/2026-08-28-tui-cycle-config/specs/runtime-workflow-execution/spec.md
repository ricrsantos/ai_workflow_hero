# runtime-workflow-execution Specification

## MODIFIED Requirements

### Requirement: Config Save SHALL reuse existing cycle synchronization

After a successful Config write, the TUI SHALL call the existing `SyncCycleConfig` path. Synchronization SHALL update the active cycle title, objective, configuration snapshot, and still-open stage budgets/flags. Completed and Failed stages SHALL remain unchanged by sync. TUI SHALL NOT write SQLite stage rows directly for ordinary configuration changes (PRD-C07-001 §4.7; ADR-051).

#### Scenario: Open stage budgets update after Save
- **WHEN** Planning is `Waiting` and Config saves a new `max_iterations`
- **THEN** SQLite Planning `MaxIterations` matches the YAML value

#### Scenario: Completed stage is protected
- **WHEN** Research is `Completed` and Config saves a new Research timeout in YAML
- **THEN** SQLite Research timeout and status remain unchanged after sync

#### Scenario: Failed stage waits for Retry
- **WHEN** QA is `Failed` and Config saves a new QA timeout
- **THEN** sync leaves QA `Failed` until `RetryFailedStage` runs

### Requirement: Save and start SHALL follow the existing /hero-start path

Save and start SHALL persist a valid configuration first, then invoke the same preflight, Prepare, session routing, transcript, and cancellation behavior as `/hero-start`. Harness availability failures SHALL be reported by that path, not by inventing a second start implementation (PRD-C07-001 §4.9; UI-C07-001 §8).

#### Scenario: Save and start runs preflight after a successful write
- **WHEN** the user activates Save and start on a valid editable form
- **THEN** the YAML is saved and the TUI enters the existing `/hero-start` preparing/preflight flow

#### Scenario: Cursor IDE /hero-start is unchanged
- **WHEN** the user runs `/hero-start` from Cursor IDE
- **THEN** Runtime still reads `workflow-config.yml` directly and does not depend on the TUI Config screen

### Requirement: Failed stages SHALL only re-enter Waiting through Retry

`StartStage` SHALL continue to reject `Failed`. The Config Retry action is the supported requeue path and then Save and start / `/hero-start` MAY execute the Waiting stage through normal orchestration (PRD-C07-001 §4.8; ADR-052).

#### Scenario: StartStage still rejects Failed
- **WHEN** a stage is `Failed` and `StartStage` is called without Retry
- **THEN** the engine returns an error and status stays `Failed`

#### Scenario: Save and start can run a retried stage
- **WHEN** QA was retried to `Waiting` and the user runs Save and start
- **THEN** the existing start path may select QA as a runnable Waiting stage
