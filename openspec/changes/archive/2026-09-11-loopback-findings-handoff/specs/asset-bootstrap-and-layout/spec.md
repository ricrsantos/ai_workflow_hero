## ADDED Requirements

### Requirement: Canonical agent assets SHALL carry C15 report contracts for every supported harness
Embedded Cursor, OpenCode, Codex, and Claude agent sources for orchestration, discover, implementation (backend/frontend/generic), QA, Judge, Browser UI, and QA End-to-End SHALL include complete field rules, valid enum values, owner rules, one passing example, one reopening example where applicable, empty-success arrays, and an explicit prohibition on stage transitions, OpenSpec checkbox edits, gap files, and `current-state.md` mutation. Installed projections SHALL be generated from those canonical assets so contracts do not drift by harness (PRD-C15-001 §6.1, §14.16).

#### Scenario: Judge assets no longer write gap files
- **WHEN** the canonical Judge agent for each supported harness is inspected
- **THEN** it instructs the agent to emit JSON and stop, and it does not tell the agent to write `judge-gaps.md` or run `hero stage loop-back`

#### Scenario: Implementation example includes a find-* ID
- **WHEN** the canonical generic/backend/frontend agent assets are inspected
- **THEN** they include a valid example where `tasks_completed` may contain a `find-*` ID from the explicit assignment

### Requirement: Runtime command inventory SHALL include add-todo and complete-todo
The installation layout SHALL include `hero-add-todo` and `hero-complete-todo` command assets for each supported harness, plus updates to start, status, todos, continue, finish, and help. Upgrade SHALL refresh those files under the existing checksum/conflict policy; customized assets are warned, not silently overwritten. Embedded workflow-help SHALL document escalation triage, Research adoption, and manual completion (PRD-C15-001 §10.3–10.4).

#### Scenario: Fresh or upgraded inventory contains the new commands
- **WHEN** doctor or install inventory is checked after C15 assets ship
- **THEN** `hero-add-todo` and `hero-complete-todo` exist for each enabled harness projection

#### Scenario: Customized Judge prompt is not overwritten
- **WHEN** an installed Judge asset checksum diverges from the embedded original
- **THEN** upgrade warns and does not silently replace the customized file
