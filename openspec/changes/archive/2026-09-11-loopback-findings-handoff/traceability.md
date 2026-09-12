# C15 traceability — loopback-findings-handoff

Maps PRD-C15-001 §14 acceptance criteria, UI-C15-001 §15 UX matrix, ADR-083–090, and PRD §10.4 component rows to OpenSpec tasks/specs and primary code paths. Verification date: 2026-09-11.

## ADR → tasks → code

| ADR | Decision (summary) | Tasks | Primary implementation |
|-----|-------------------|-------|------------------------|
| ADR-083 | Findings in SQLite, Go-owned | task-01.*, task-02.* | `internal/store/migrate.go`, `findings.go` |
| ADR-084 | Atomic failed close + findings + loop-back | task-05.*, task-06.1 | `internal/engine/handoff.go`, `internal/cycle/handoff.go`, `store.InTx` |
| ADR-085 | Assignment union task-* + find-* | task-08.* | `internal/tui/implementation_assignment.go`, `internal/cycle/assignment.go` |
| ADR-086 | Fail-closed structured reports | task-04.* | `internal/cycle/reports/` |
| ADR-087 | Deferred findings → durable ToDos + projection | task-03.*, task-10.* | `internal/store/todos.go`, `internal/todos/` |
| ADR-088 | Defer-all terminal disposition | task-06.2, task-13.3 | `internal/engine/implementation_close.go`, `internal/tui/todo_control.go` |
| ADR-089 | Research adoption; pending-only manual complete | task-14.*, task-07.2 | `internal/tui/research_session.go`, `internal/cycle/todo_cli.go` |
| ADR-090 | Status/Events project findings flow | task-11.*, task-12.* | `internal/cycle/status_view.go`, `internal/tui/status_screen.go`, `internal/tui/events_format.go` |

OpenSpec capability specs: `findings-lifecycle`, `durable-todos`, `structured-stage-reports`, `runtime-workflow-execution`, `sqlite-operational-store`, `cli-deterministic-command-suite`, `hero-tui`, `telegram-project-control`, `asset-bootstrap-and-layout`.

## PRD §10.4 component map

| Component (PRD) | Task(s) | Notes |
|-----------------|---------|-------|
| `internal/store/migrate.go` | task-01.1 | v11 migration + constraints |
| `internal/store` finding slice | task-02.* | |
| `internal/store` ToDo slice | task-03.* | |
| `internal/engine` | task-05.*, task-06.2 | |
| `internal/cycle/service.go` | task-06.* | façade in `handoff.go`, `assignment.go`, `todo_cli.go` |
| `internal/cycle/command.go` | task-07.* | `--findings-json`, add/complete-todo |
| `internal/tui/implementation_assignment.go` | task-08.* | |
| `internal/tui/stage_handoff.go` | task-09.* | |
| `internal/tui/research_session.go` | task-14.* | |
| `internal/tui/herocmd.go` | task-13.* | + `todo_control.go`, `palette.go`, `conversation.go` |
| `internal/tui/palette.go`, `slash_overlay.go` | task-13.*, task-16.2 | |
| `internal/tui/screens.go`, `output_view.go` | task-12.* | Status via `status_screen.go`; Events via `events_format.go` |
| `internal/status` | task-11.* | Table/JSON helpers; cycle `StatusView` is source of truth |
| `internal/todos` | task-10.* | |
| `internal/conversation` | task-15.1 | `cycle.StageAgentAuditBody` |
| `internal/telegram` + daemon | task-17.1 | `CompactFindingsStatus`, help, forwarding |
| `assets/{cursor,opencode,codex,claude}/agents` | task-16.1 | |
| `assets/.../commands` | task-16.2 | |
| installed harness projections | task-16.2 | install/upgrade golden layouts |
| workflow-help source | task-16.2 | embedded asset |
| tests/fixtures | all tasks | `internal/store/*_test.go`, `internal/tui/*_test.go`, v10→v11 fixture task-01.2 |

**Gap (documentation only):** PRD lists `internal/cycle/service.go`; orchestration APIs live across `internal/cycle` package files (`handoff.go`, `assignment.go`, `command.go`, `todo_cli.go`) — behavior matches §10.4 intent.

## PRD §14 acceptance criteria → evidence

| # | Criterion | Task / test anchor |
|---|-----------|-------------------|
| 1 | Four validation stages atomic fail → Implementation IDs | task-05.2, task-07.1; `engine/handoff_test.go` |
| 2 | Malformed close commits no partial state | task-05.2; engine + store tx tests |
| 3 | Mixed disjoint ordered assignments | task-08.*; `implementation_assignment_test.go` |
| 4 | find-* done; rediscovery reopens same ID | task-02.2; `findings_test.go` |
| 5 | unassigned_id on old task-* in findings-only wave | task-08.2; reports + assignment tests |
| 6 | Empty wave accepts only empty arrays | task-08.2, task-06.2 |
| 7 | Deferred not recreated; recurrence warning | task-02.2 |
| 8 | Partial triage stays Escalated until continue | task-13.1; `todo_control_test.go` |
| 9 | Defer-all disposition, no Execute | task-06.2, task-13.1 |
| 10 | Projection failure; retry idempotent | task-10.1; `internal/todos/*_test.go` |
| 11 | Research lists/adopts ToDos | task-14.1 |
| 12 | Adopting cycle resolve; cancel releases | task-03.2, task-14.1 |
| 13 | complete-todo note, pending-only, legacy promote | task-07.2, task-10.2, task-13.2 |
| 14 | Additive Status JSON → TUI/Telegram | task-11.1, task-12.*, task-17.1 |
| 15 | v10→v11 fixture intact | task-01.2 |
| 16 | Harness agent contracts consistent | task-16.1 |
| 17 | Quality gates §12 | task-19.2 |

## UI-C15-001 §15 UX matrix → evidence

| Scenario | Task / code |
|----------|-------------|
| QA two findings | task-12.1 `status_screen_test.go` |
| Judge reopen | task-02.2 + task-12.2 events |
| unassigned_id | task-08.2, UI §5 diagnostics |
| Empty verification OK | task-04.*, task-08.2 |
| nonempty_empty_assignment | task-04.2 |
| Partial ToDo triage | task-13.1 `todo_control.go` |
| All blockers deferred | task-13.1 defer-all + task-06.2 |
| Projection write fails | task-13.1 retry copy |
| Research ToDos | task-14.1 |
| Research adopts legacy | task-10.2, task-14.1 |
| Adopted manual complete rejected | task-13.2 |
| Pending manual complete | task-13.2 |
| No findings | task-12.1 `Findings none` |

## Remaining gaps

None blocking handoff. Non-blocking notes:

- Scheduler checkbox updates remain TUI-owned (by design); agents never write `tasks.md`.
- Historical findings drill-down in `/hero-cycles` explicitly out of scope (PRD §13).
