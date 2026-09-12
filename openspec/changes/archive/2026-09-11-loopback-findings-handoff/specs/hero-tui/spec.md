## ADDED Requirements

### Requirement: Chat SHALL show finding handoff and exact diagnostics
After a valid failed report is atomically persisted, Chat SHALL show the source, created/reopened IDs, owners, one-line issues, and loop-back routing without pasting the entire report. Implementation Execute SHALL display owned planned `task-*` and findings `find-*` separately. Completion-gate failures SHALL show diagnostic code, field path, offending ID/value, assigned IDs when relevant, and that nothing was persisted (UI-C15-001 §§3–5).

#### Scenario: QA failure lists IDs and route
- **WHEN** QA persists `find-qa-1` (reopened) and `find-qa-2` (open)
- **THEN** Chat shows both IDs, owners, and `Loop-back QA → Implementation`

#### Scenario: Unassigned ID error names the assigned set
- **WHEN** Implementation reports `task-03.2` but the wave assigned only `find-judge-1`
- **THEN** Chat shows `unassigned_id` and `assigned IDs: find-judge-1`

### Requirement: Status screen SHALL project the active findings flow
The existing Status screen SHALL keep the stage table first and add compact sections for loop-backs, findings, deferred ToDos, and disposition/CTAs. Owner labels remain BACK/FRNT/GEN. `deferred_todo` renders as `ToDo` in the compact table. Empty cycles MAY show `Findings none` and MUST NOT render noisy empty tables. No eighth navbar item is added (UI-C15-001 §6; ADR-090).

#### Scenario: Escalated Status lists CTAs
- **WHEN** Implementation is Escalated with open findings
- **THEN** Status shows `/hero-continue`, `/hero-add-todo`, `/hero-cancel`, and `/hero-finish`

#### Scenario: Deferred completion copy is explicit
- **WHEN** a cycle closes with `completed_with_deferred_todos`
- **THEN** Status/Chat states the disposition, deferred IDs, and that remaining validation stages were skipped

### Requirement: TUI SHALL provide add-todo and complete-todo dialogs
`/hero-add-todo` SHALL be enabled only for an Escalated loop with at least one open/reopened finding. Invoking it without IDs opens an unselected checklist; Enter with nothing selected leaves the dialog open. Partial review states the stage remains Escalated. Defer-all requires typed `DEFER` or the existing stricter terminal confirmation. `/hero-complete-todo` SHALL require explicit IDs, a non-empty note, and confirmation. Both commands reject multimodal attachments. `/hero-finish` with open findings requires a strong warning that they will not become ToDos (UI-C15-001 §§7–8,11).

#### Scenario: add-todo before escalation is blocked
- **WHEN** the user runs `/hero-add-todo` while no loop is Escalated
- **THEN** Chat shows that the command is available only for open findings in an Escalated loop and mutates nothing

#### Scenario: Projection failure copy offers retry
- **WHEN** ToDo projection is not completed
- **THEN** Chat says the cycle remains Escalated, no duplicate was created, and `/hero-add-todo` can retry

### Requirement: Research Chat SHALL offer pending ToDos before grilling
After objective/config and idea notes, Research SHALL emit a pending-ToDo check. If none exist it continues without an extra question. If items exist it presents objective matches separately from other candidates and records adoption only after deterministic persistence succeeds (UI-C15-001 §9).

#### Scenario: No pending ToDos skips the question
- **WHEN** Research starts and there are no pending ToDos
- **THEN** Chat shows `No pending project ToDos` and continues grilling

### Requirement: `/hero-todos` SHALL list pending and adopted structured items
`/hero-todos` SHALL remain read-only, SHALL list pending plus adopted items with IDs, SHALL omit resolved items, MUST NOT silently adopt, and SHALL retain the `/hero-sync` notice (UI-C15-001 §10; ADR-028).

#### Scenario: Adopted items appear until resolved
- **WHEN** `find-qa-1` is adopted by C16
- **THEN** `/hero-todos` lists it under Adopted and does not change its state
