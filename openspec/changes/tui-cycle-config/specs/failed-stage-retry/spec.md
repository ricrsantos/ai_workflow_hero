# failed-stage-retry Specification

## Purpose

Allow a user to correct a failed etapa’s configuration and requeue only that etapa without rewriting completed work or deleting operational history (PRD-C07-001 §4.8; ADR-052).

## ADDED Requirements

### Requirement: Engine SHALL provide an explicit Failed-to-Waiting retry transition

`Engine.RetryFailedStage` SHALL accept a cycle id and stage name. When the stage is `Failed`, it SHALL set status to `Waiting`, reset `Iteration` to 0, clear `StartedAt` and `CompletedAt`, apply the current workflow-config budgets/flags for that stage, and leave other stages unchanged. It SHALL reject any other status. The TUI SHALL request this transition; the engine SHALL own invariant checks. No new Cobra command is required (ADR-052; design D5).

#### Scenario: Failed stage returns to Waiting
- **WHEN** QA is `Failed` and Retry is invoked
- **THEN** QA status is `Waiting` and Research `Completed` is unchanged

#### Scenario: Counters reset for the next attempt
- **WHEN** a Failed stage with `Iteration=3` and a non-empty `StartedAt` is retried
- **THEN** `Iteration` is 0 and `StartedAt`/`CompletedAt` are empty so timeout accounting starts on the next `StartStage`

#### Scenario: Retry applies saved YAML budgets
- **WHEN** Save wrote `max_iterations: 5` for a Failed QA stage and sync skipped it
- **THEN** Retry sets QA `MaxIterations` to 5 from the current YAML

#### Scenario: Waiting stage cannot be retried
- **WHEN** Retry is invoked for a `Waiting` stage
- **THEN** the engine returns an error and the stage row is unchanged

### Requirement: Retry SHALL preserve events and metrics and record the transition

Retry SHALL append an event of type `stage_retried` with a JSON payload naming the stage. Existing events and metrics rows for prior attempts SHALL remain readable (PRD-C07-001 §4.8; ADR-052).

#### Scenario: History remains after retry
- **WHEN** a Failed stage has a prior `stage_completed` event and a metrics row
- **THEN** after Retry those rows still exist and a new `stage_retried` event is present

#### Scenario: Other stages are not rewritten
- **WHEN** QA is retried
- **THEN** Implementation’s status, iteration, and timestamps are unchanged
