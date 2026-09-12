# durable-todos Specification

## Purpose
Durable project ToDos for deferred findings and legacy Pending lines, with adoption, release, manual completion, and recoverable current-state.md projection.

## Requirements

### Requirement: Deferred findings SHALL become structured pending ToDos
Finding-derived ToDos SHALL retain the finding ID. Structured rows SHALL store origin, safe summary, acceptance criterion when present, status `pending`/`adopted`/`resolved`, adopted cycle when any, timestamps, resolution note, and append-only adoption history (PRD-C15-001 §9.1; ADR-087).

#### Scenario: Deferring a finding creates a pending ToDo with the same ID
- **WHEN** the user defers `find-qa-2` at Escalated
- **THEN** the finding becomes `deferred_todo`, a pending ToDo with id `find-qa-2` exists, and a `deferred` occurrence is stored

#### Scenario: Legacy lines are promoted only when selected
- **WHEN** Research or `/hero-complete-todo` selects a markdown-only Pending line
- **THEN** Hero assigns the next project-scoped `todo-N` before adoption or completion and does not import unselected Pending lines

### Requirement: SQLite and current-state.md SHALL be reconciled recoverably
`hero.db` is authoritative for structured lifecycle. The `## Pending Features` section of `context/current-state.md` SHALL project pending and adopted items with a stable backtick-wrapped ID. `hero add-todo` and other mutating ToDo operations SHALL build a fsynced replacement candidate, persist transition plus projection intent, install and verify the file, and only then allow stage/cycle advancement. A crash or file-write failure MUST be retryable without duplicate rows or markdown lines. Until reconciliation succeeds, an Escalated loop remains Escalated (PRD-C15-001 §9.2; ADR-087; design D6).

#### Scenario: Projection failure blocks advancement
- **WHEN** the atomic install of `current-state.md` fails after SQLite intent is persisted
- **THEN** the cycle remains Escalated, no duplicate ToDo row exists, and retrying the same operation completes reconciliation

#### Scenario: Verified retry is idempotent
- **WHEN** the same defer operation is retried after status `verified`
- **THEN** Hero reports success without inserting a second ToDo row or a second markdown line

### Requirement: Research SHALL adopt pending ToDos before grilling
At the start of every Research session, after objective/config and active idea notes and before general grilling, Hero SHALL query pending ToDos, present IDs and summaries in the user's preferred chat language, identify items already implied by the objective, ask which remaining items enter this cycle, recommend leaving unrelated items out, and persist selected items as `adopted` by the active cycle through a deterministic service/CLI operation. No public `/hero-adopt-todo` command is added (PRD-C15-001 §9.3; UI-C15-001 §9; ADR-089).

#### Scenario: Selected pending item becomes adopted and remains visible
- **WHEN** the user confirms adoption of `find-qa-1` during Research
- **THEN** the ToDo status is `adopted` with the active cycle ID and the projection still shows the item until validation resolves it

#### Scenario: Persistence failure blocks Research finalization
- **WHEN** adoption persistence or projection verification fails
- **THEN** Research MUST NOT claim the item is adopted and MUST provide a retry

### Requirement: Adoption SHALL resolve only after validating completion
An adopted ToDo SHALL become `resolved` only after its adopting cycle completes successfully through all enabled validation stages. Cancel, emergency finish without validation, rejection/rollback, or another non-validating terminal outcome SHALL release unresolved adopted items to `pending` while preserving adoption history. A cycle completed with deferred ToDos SHALL NOT resolve unresolved adopted items merely by closing (PRD-C15-001 §9.4; ADR-089).

#### Scenario: Validated cycle resolves adopted ToDos
- **WHEN** cycle C16 adopted `find-qa-1` and later completes all enabled validation stages successfully
- **THEN** `find-qa-1` is `resolved` and removed from the Pending projection

#### Scenario: Cancel releases adopted ToDos
- **WHEN** the adopting cycle is cancelled with `find-qa-1` still adopted
- **THEN** the ToDo returns to `pending`, adoption history retains the released attempt, and the next Research session can offer it again

### Requirement: Manual completion SHALL accept pending items only
`/hero-complete-todo` SHALL accept structured and legacy ToDos, promote a legacy item to `todo-*` before completion, accept only `pending` items, reject an item adopted by an active cycle, require a non-empty safe resolution note, set status `resolved`, remove the item from Pending and default `/hero-todos` output, and retain audit history. Retry of the same resolved ID SHALL be idempotent; a conflicting new note SHALL be rejected (PRD-C15-001 §9.5; UI-C15-001 §11).

#### Scenario: Pending item is completed with a note
- **WHEN** the user confirms `/hero-complete-todo find-qa-2` with a non-empty safe note
- **THEN** the ToDo is resolved, leaves Pending, and Events retains the manual completion

#### Scenario: Adopted item cannot be completed manually
- **WHEN** `find-qa-1` is adopted by active cycle C16
- **THEN** `/hero-complete-todo find-qa-1` is rejected and names C16
