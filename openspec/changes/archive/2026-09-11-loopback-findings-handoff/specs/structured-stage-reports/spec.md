## Purpose

Typed validation and Implementation JSON contracts that fail closed with field-specific diagnostics before any Hero operational state mutates.

## ADDED Requirements

### Requirement: Validation agents SHALL emit one JSON object and stop
QA, Judge, Browser UI Validation, and QA End-to-End SHALL emit one JSON object. They MUST NOT call `hero stage close`, `hero stage loop-back`, or another transition; MUST NOT edit OpenSpec checkboxes; MUST NOT write `qa-gaps.md`, `judge-gaps.md`, or another operational gap file; MUST NOT edit `context/current-state.md`; and MUST NOT invent `find-*` IDs not provided in their current context. Success SHALL return empty failure/gap arrays (PRD-C15-001 §6.1; ADR-086).

#### Scenario: Successful QA returns empty failures
- **WHEN** QA finds no defects
- **THEN** the report status is successful and `failures` is an empty array

#### Scenario: Judge implementation gaps are separate from SDD ambiguity
- **WHEN** Judge sets `sdd_ambiguity` true
- **THEN** no finding is created and the existing `/hero-back` versus `/hero-approve` decision path is used

### Requirement: Failure entries SHALL carry routing and verification fields
Each failure/gap entry SHALL include `owner` when not derivable, `file` and/or `requirement`, non-empty `issue`, non-empty `acceptance_criteria`, optional safe `evidence` paths, and optional valid `reopen_id`. QA and QA End-to-End require a valid owner. Browser UI maps `failure_class=frontend` to `frontend_agent` and `failure_class=backend` to `backend_agent`; Visual failures are frontend. Missing visual reference PNGs SHALL warn and MUST NOT create findings. Judge implementation gaps require an explicit owner, defaulting only when exactly one Implementation agent is active; otherwise the report fails closed. An invalid or unavailable owner rejects the entire report (PRD-C15-001 §5.4, §6.2–6.4).

#### Scenario: QA entry without acceptance criteria is rejected
- **WHEN** a QA failure omits `acceptance_criteria`
- **THEN** the diagnostic is `missing_field` at `failures[0].acceptance_criteria` and nothing is persisted

#### Scenario: Missing visual PNG is a warning only
- **WHEN** Browser Visual Validation lacks a reference PNG
- **THEN** the stage may warn and MUST NOT allocate a `find-bui-*` ID for that absence

### Requirement: Implementation reports SHALL cover the exact assignment including find-* IDs
Implementation reports SHALL keep `tasks_completed` and `tasks_remaining` as disjoint arrays whose union equals the exact assigned IDs, which MAY mix `task-*` and `find-*`. IDs not assigned to that report SHALL be rejected. A truly empty verification assignment SHALL accept only empty arrays. Historical field names remain; they mean assignment item IDs (PRD-C15-001 §6.5, §7.3; ADR-085).

#### Scenario: Mixed assignment completes both prefixes
- **WHEN** the assignment is `task-03.2` and `find-qa-1` and both appear in `tasks_completed` with empty remaining and passing gates
- **THEN** the report is accepted

#### Scenario: Old task ID in a findings-only wave is unassigned
- **WHEN** the assignment is only `find-judge-1` and `tasks_completed` contains `task-03.2`
- **THEN** the diagnostic is `unassigned_id` for `tasks_completed[0]` and no result is applied

#### Scenario: Empty verification wave rejects a nonempty ID
- **WHEN** Implementation was started with no task and no finding IDs and the report lists any ID
- **THEN** the diagnostic is `nonempty_empty_assignment` and nothing is applied

### Requirement: Invalid reports SHALL name a stable diagnostic before mutation
Go decoders SHALL reject unknown fields, invalid JSON, invalid enums, missing required fields, invalid owners, invalid `reopen_id`, duplicate IDs, overlapping completed/remaining arrays, assignment union mismatch, false acceptance gates, and nonempty IDs on an empty assignment. Each rejection SHALL expose a stable code, JSON field path when applicable, offending value, and concise rule. No finding, task checkbox, stage, event, or conversation-result mutation SHALL occur until the entire report passes. Raw invalid output MAY be kept only in the existing safe audit channel (PRD-C15-001 §7.3; UI-C15-001 §5; ADR-086).

#### Scenario: Union mismatch names the missing ID
- **WHEN** assignment is `find-qa-1` and `find-qa-2` but `find-qa-2` is absent from completed and remaining
- **THEN** the diagnostic is `assignment_union_mismatch` and status is unchanged

#### Scenario: False gate applies nothing
- **WHEN** `acceptance_gates.completed_tasks_verified` is false on a complete report
- **THEN** the diagnostic is `false_acceptance_gate` and no checkbox or finding status changes
