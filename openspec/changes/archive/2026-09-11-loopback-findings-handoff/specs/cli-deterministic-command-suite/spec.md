## ADDED Requirements

### Requirement: CLI SHALL accept atomic failed-stage findings JSON
`hero stage close --name <source> --failed --findings-json '<report>'` SHALL accept the full validated report for `qa`, `judge`, `browser_ui_validation`, and `qa_end_to_end`. Validation occurs before mutation. Structured errors SHALL be JSON-safe and include diagnostic code, field path, and message. Standalone `hero stage loop-back` remains available and MUST NOT create findings (PRD-C15-001 §7.1; ADR-084).

#### Scenario: Failed QA close with findings-json succeeds atomically
- **WHEN** a user or TUI runs `hero stage close --name qa --failed --findings-json` with a valid failed report
- **THEN** findings, failed close, and loop-back are persisted together

#### Scenario: Malformed findings-json returns a structured error
- **WHEN** `--findings-json` is not valid JSON
- **THEN** the command exits non-zero with `invalid_json` and no store mutation

### Requirement: CLI SHALL expose deterministic ToDo verbs
The deterministic CLI SHALL provide `hero add-todo` and `hero complete-todo` without agent reasoning. `hero add-todo` is legal only for open/reopened findings in the currently Escalated loop. `hero complete-todo` requires a non-empty safe resolution note and pending IDs. `/hero-todos` remains a read-only view (PRD-C15-001 §8.1, §9.5; ADR-028 amended by ADR-087).

#### Scenario: add-todo is rejected before escalation
- **WHEN** `hero add-todo` is invoked while no loop is Escalated
- **THEN** the command fails with a precise reason and mutates nothing

#### Scenario: complete-todo requires a note
- **WHEN** `hero complete-todo find-qa-2` is invoked without a resolution note
- **THEN** the command fails and the ToDo remains pending

### Requirement: Status JSON SHALL add findings and ToDo blocks additively
`hero status` and `hero status --json` SHALL include additive finding counts/rows, short loop-back history, ToDo counts, available actions, and nullable completion disposition. Existing fields SHALL remain unchanged. Missing data SHALL be empty arrays/zero counts or null disposition (PRD-C15-001 §10.1; UI-C15-001 §14; ADR-090).

#### Scenario: JSON stays backward compatible
- **WHEN** a cycle has no findings
- **THEN** existing status fields still parse and findings counts are zero with empty items
