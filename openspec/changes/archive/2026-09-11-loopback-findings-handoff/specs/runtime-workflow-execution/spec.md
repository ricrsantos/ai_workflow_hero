## ADDED Requirements

### Requirement: Runtime SHALL union OpenSpec tasks with actionable findings for Implementation
For each active Implementation agent, the scheduler SHALL build one ordered, duplicate-free assignment from unchecked owned OpenSpec `task-*` IDs followed by owned findings with status `open` or `reopened`. OpenSpec remains authoritative for `task-*`; SQLite findings remain authoritative for `find-*`. A finding owner that is not in active Implementation scope SHALL fail before Execute. After a productive wave or user deferral empties both authorities, Runtime SHALL close Implementation directly and MUST NOT dispatch an empty wave. Starting/restarting Implementation with zero pending tasks and zero actionable findings MAY run the existing single verification wave (PRD-C15-001 §7.2–7.3; ADR-075 amended by ADR-085).

#### Scenario: Mixed assignment is owner-scoped
- **WHEN** generic_agent has unchecked `task-04.1` and open `find-qa-1`
- **THEN** that agent's assignment is exactly those IDs in that order

#### Scenario: Empty after deferral does not redispatch
- **WHEN** the last remaining finding is deferred and no OpenSpec tasks remain unchecked
- **THEN** Implementation closes without another Execute

### Requirement: Runtime SHALL support Escalated finding triage
When a loop is Escalated, `/hero-add-todo` SHALL let the user defer selected open/reopened findings to pending ToDos. Partial deferral SHALL leave the stage Escalated and SHALL require a separate `/hero-continue [N]` for remaining work. Deferring every remaining blocker after successful projection SHALL complete the cycle with status `completed` and disposition `completed_with_deferred_todos`, skip enabled downstream validation, and MUST NOT run an empty Implementation wave (PRD-C15-001 §8; ADR-088).

#### Scenario: Partial deferral stays Escalated
- **WHEN** the user defers `find-qa-2` and leaves `find-qa-1` open
- **THEN** the stage remains Escalated and `/hero-continue` is still required for remaining work

#### Scenario: Deferring every blocker records disposition
- **WHEN** the confirmed selection leaves zero unchecked OpenSpec tasks and zero open/reopened findings
- **THEN** the cycle completes with `completed_with_deferred_todos` and downstream validation stages are not executed as successful

### Requirement: Runtime SHALL release or resolve adopted ToDos at terminal outcomes
A successful validating cycle completion SHALL resolve ToDos adopted by that cycle. Cancel, emergency `/hero-finish`, rejection/rollback, or another non-validating terminal outcome SHALL release unresolved adopted items to `pending`. Emergency finish with open findings SHALL require strong confirmation that those findings will not become ToDos and MUST NOT write `completed_with_deferred_todos` (PRD-C15-001 §8.4, §9.4; ADR-088; ADR-089).

#### Scenario: Emergency finish does not create ToDos
- **WHEN** the user confirms `/hero-finish` while `find-qa-1` is open
- **THEN** the finding is not converted to a ToDo and completion disposition is not `completed_with_deferred_todos`
