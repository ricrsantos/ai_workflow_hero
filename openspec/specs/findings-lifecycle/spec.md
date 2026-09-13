# findings-lifecycle Specification

## Purpose
Scheduler-owned validation findings with stable IDs, deterministic fingerprints, append-only occurrences, and atomic failed-stage handoff into Implementation.

## Requirements

### Requirement: Findings SHALL use stable per-cycle source-namespace IDs
Hero SHALL allocate user-facing finding IDs in the namespaces `find-qa-N`, `find-judge-N`, `find-bui-N`, and `find-e2e-N` when a valid failure entry omits an ID. Agents MUST NOT invent IDs. An agent MAY supply `reopen_id` only for an existing `done` finding in the same cycle with the same source stage, owner, canonical file, requirement, acceptance criteria, and repro package+test (PRD-C15-001 §5.1; ADR-083).

#### Scenario: Scheduler allocates the next QA ID
- **WHEN** a valid QA failure entry omits an ID and the cycle already has `find-qa-1`
- **THEN** Hero persists `find-qa-2` and does not ask the agent to choose an ID

#### Scenario: Invalid reopen_id is rejected
- **WHEN** a report supplies `reopen_id=find-qa-1` but that finding is not `done` in the same cycle with matching source and owner
- **THEN** the report is rejected with `unknown_reopen_id` and no finding or stage change is persisted

#### Scenario: reopen_id with a different acceptance is rejected
- **WHEN** a report supplies `reopen_id=find-qa-1` for a `done` finding whose stored acceptance (or file/requirement) differs from the report
- **THEN** the report is rejected with `unknown_reopen_id` and Hero does not mutate that finding; the agent must omit `reopen_id` so a new ID can be allocated

### Requirement: Finding equality SHALL be deterministic
Without a valid `reopen_id`, equality SHALL use a SHA-256 fingerprint of cycle, source stage, owner, canonical file, canonical requirement, and normalized acceptance criteria. When `repro.package` and `repro.test` are present they SHALL be hashed too, so a different Go test is a new ID. Legacy rows with empty stored repro keep the pre-v14 six-field hash until the first incoming repro locks onto the row and rewrites the fingerprint. Issue prose SHALL NOT be hashed. `reopen_id` takes precedence over fingerprint matching only when that contract **and** repro identity match the stored row. Reopen, rediscovery, done, and deferral MUST NOT overwrite the stored issue, file, requirement, or acceptance criterion. Different source stages SHALL NOT merge (PRD-C15-001 §5.1; ADR-083; design D2).

#### Scenario: Exact rediscovery of a done finding reopens the same ID
- **WHEN** a later valid report matches the fingerprint of a `done` finding and omits `reopen_id`
- **THEN** that ID becomes `reopened`, round increments, a `reopened` occurrence is appended, and the finding row's issue and acceptance stay unchanged

#### Scenario: Exact rediscovery of an open finding does not clone
- **WHEN** a later valid report matches the fingerprint of an `open` or `reopened` finding
- **THEN** Hero appends a `rediscovered` occurrence and does not create a second ID

#### Scenario: Deferred recurrence stays deferred
- **WHEN** a later valid report in the same cycle matches a `deferred_todo` finding
- **THEN** Hero appends a `deferred_recurrence` warning occurrence, does not reopen the finding, and does not treat it as an actionable failure

### Requirement: Repro modes SHALL follow the project's verification policy
The repro identity of a finding SHALL use one of three modes: `go_test` (a Go package plus `Test*` name and source), `command` (a repo-relative target plus filter token, re-run through the argv template in `verification.repro.command`), or `evidence` (no automated re-run; a non-empty `evidence` array). The available modes SHALL come from `verification.repro` in workflow-config.yml, defaulting to `go_test` when the project root holds a `go.mod` and to `evidence` when no automated mode exists. Evidence mode SHALL stay closed for stages that can express the failure automatically, while Browser UI Validation keeps it by default because rendering and visual-diff failures have no deterministic re-run. Agent-supplied tokens SHALL be charset-restricted and interpolated into a project-owned argv without a shell (PRD-C15-001 §6; ADR-084).

#### Scenario: Non-Go project uses its own command
- **WHEN** `verification.repro.command` is configured and a QA failure entry uses `repro.mode: "command"`
- **THEN** the report is accepted and the scheduler re-runs that command with the finding's target and filter before accepting `done`

#### Scenario: Mode not enabled is rejected with a usable diagnostic
- **WHEN** a report uses a mode this project has not enabled
- **THEN** the report is rejected with `invalid_enum` on `repro.mode` listing the allowed modes, and nothing is persisted

#### Scenario: Browser UI keeps the evidence escape hatch
- **WHEN** Browser UI Validation reports a visual or rendering failure with `repro.mode: "evidence"` and non-empty `evidence`
- **THEN** the finding is persisted and routed normally, and the scheduler records that it has no automated gate

### Requirement: The validation loop SHALL have scheduler-owned ceilings
The scheduler SHALL stop the validation → Implementation loop instead of letting it run until the iteration budget is exhausted. One finding SHALL travel at most 3 rounds, an Implementation stage SHALL run at most 8 waves per iteration, and the stage iteration/timeout budget SHALL be re-checked between waves. Reaching any ceiling SHALL Escalate Implementation so the user decides with `/hero-continue`, `/hero-add-todo`, `/hero-cancel`, or `/hero-finish`.

#### Scenario: A finding that keeps coming back stops the loop
- **WHEN** an actionable finding reaches round 4 at Implementation dispatch
- **THEN** no wave is dispatched, Implementation becomes Escalated, and the user is shown the finding IDs and the available actions

#### Scenario: Budget is re-checked between waves
- **WHEN** an Implementation wave makes progress and the stage timeout or iteration budget is already spent
- **THEN** the scheduler escalates instead of starting the next wave

### Requirement: Only Go SHALL change finding status
Finding status SHALL be `open`, `done`, `reopened`, or `deferred_todo`. Only scheduler/service code MAY change status. `done` means the assigned Implementation agent reported and verified that exact ID. A `deferred_todo` finding is not assignable in its source cycle (PRD-C15-001 §5.3; ADR-083).

#### Scenario: Implementation completion marks a finding done
- **WHEN** a verified Implementation report includes assigned `find-qa-1` in `tasks_completed` and the locked repro passes for its mode (`go test <package> -count=1 -run ^TestName$` for `go_test`; the configured `verification.repro.command` for `command`; nothing to re-run for `evidence`)
- **THEN** that finding status becomes `done` and a `done` occurrence is appended

#### Scenario: Scheduler rejects done when the repro still fails
- **WHEN** an Implementation report includes `find-qa-1` in `tasks_completed` but the locked repro fails, is missing, or `go test` reports no tests ran
- **THEN** Hero does not mark the finding done (`repro_test_failed`) and does not check the corresponding OpenSpec task boxes for that close

#### Scenario: Repro verification never blocks the TUI
- **WHEN** an Implementation wave claims one or more `find-*` IDs as done
- **THEN** the scheduler re-runs their repros in a background command bounded by a total budget, keeps the handoff live while it runs, and applies the verdict only to the wave that requested it

#### Scenario: Validation reports require a repro in an enabled mode
- **WHEN** a QA/Judge/BUI/E2E failure entry omits `repro`, uses a `repro.mode` that this project has not enabled, or breaks that mode's shape (a `go_test` `source` that does not declare `func TestName(`, an `evidence` entry carrying package/test/source or an empty `evidence` array)
- **THEN** the report is rejected fail-closed (`missing_field` / `invalid_enum`) and no finding or stage change is persisted

#### Scenario: Agents cannot mutate findings
- **WHEN** a validation agent writes markdown or calls a store helper directly
- **THEN** that write is not a legal status transition; only the Go scheduler APIs change finding status

### Requirement: Failed validation close SHALL persist findings and loop-back atomically
Closing QA, Judge, Browser UI Validation, or QA End-to-End as failed SHALL validate the whole report before opening a transaction, then in one SQLite transaction create/append/reopen findings, mark the source stage Failed, apply loop-back stage resets, store finding IDs on the loop-back event, and append audit data. If validation or any transactional write fails, none of those changes commit. A failed close without at least one actionable new/open/reopened finding SHALL be rejected (PRD-C15-001 §7.1; ADR-084).

#### Scenario: Valid QA failure loops back with IDs
- **WHEN** QA emits a valid failed report with one new finding
- **THEN** one transaction persists `find-qa-1`, closes QA as Failed, reopens Implementation, and the loop-back event payload includes `find-qa-1`

#### Scenario: Invalid report commits nothing
- **WHEN** a failed close payload is missing `acceptance_criteria` or has an invalid owner
- **THEN** no finding row, stage status change, or loop-back event is committed

#### Scenario: Failed close without actionable findings is illegal
- **WHEN** a validation stage is closed `--failed` with an empty failure array or only deferred recurrences
- **THEN** the operation is rejected with `no_actionable_finding` and the stage remains unchanged
