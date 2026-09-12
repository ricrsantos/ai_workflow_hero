# findings-lifecycle Specification

## Purpose
Scheduler-owned validation findings with stable IDs, deterministic fingerprints, append-only occurrences, and atomic failed-stage handoff into Implementation.

## Requirements

### Requirement: Findings SHALL use stable per-cycle source-namespace IDs
Hero SHALL allocate user-facing finding IDs in the namespaces `find-qa-N`, `find-judge-N`, `find-bui-N`, and `find-e2e-N` when a valid failure entry omits an ID. Agents MUST NOT invent IDs. An agent MAY supply `reopen_id` only for an existing `done` finding in the same cycle with the same source stage, owner, canonical file, requirement, and acceptance criteria (PRD-C15-001 §5.1; ADR-083).

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
Without a valid `reopen_id`, equality SHALL use a SHA-256 fingerprint of cycle, source stage, owner, canonical file, canonical requirement, and normalized acceptance criteria. Issue prose SHALL NOT be hashed. `reopen_id` takes precedence over fingerprint matching only when that contract matches the stored row. Reopen, rediscovery, done, and deferral MUST NOT overwrite the stored issue, file, requirement, or acceptance criterion. Different source stages SHALL NOT merge (PRD-C15-001 §5.1; ADR-083; design D2).

#### Scenario: Exact rediscovery of a done finding reopens the same ID
- **WHEN** a later valid report matches the fingerprint of a `done` finding and omits `reopen_id`
- **THEN** that ID becomes `reopened`, round increments, a `reopened` occurrence is appended, and the finding row's issue and acceptance stay unchanged

#### Scenario: Exact rediscovery of an open finding does not clone
- **WHEN** a later valid report matches the fingerprint of an `open` or `reopened` finding
- **THEN** Hero appends a `rediscovered` occurrence and does not create a second ID

#### Scenario: Deferred recurrence stays deferred
- **WHEN** a later valid report in the same cycle matches a `deferred_todo` finding
- **THEN** Hero appends a `deferred_recurrence` warning occurrence, does not reopen the finding, and does not treat it as an actionable failure

### Requirement: Only Go SHALL change finding status
Finding status SHALL be `open`, `done`, `reopened`, or `deferred_todo`. Only scheduler/service code MAY change status. `done` means the assigned Implementation agent reported and verified that exact ID. A `deferred_todo` finding is not assignable in its source cycle (PRD-C15-001 §5.3; ADR-083).

#### Scenario: Implementation completion marks a finding done
- **WHEN** a verified Implementation report includes assigned `find-qa-1` in `tasks_completed`
- **THEN** that finding status becomes `done` and a `done` occurrence is appended

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
