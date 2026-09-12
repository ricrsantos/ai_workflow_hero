# Implementation Tasks: Loop-back Findings and Deferred ToDos

Every task has an executable verification criterion. Artifacts stay English.
`[SERIES]` = ordering constraint. `[PARALLEL]` = independent `generic_agent` fan-out after prerequisites.
Scope: **native** → all tasks use `[agent:generic_agent]`. Apply `go-engineering` and `golang-tui` skills.

## Execution order and fan-out

```text
PARALLEL: task-01 schema v11 | task-04 report validators
  -> PARALLEL after 01: task-02 findings store | task-03 todo store
  -> task-05 engine atomic handoff (after 01+02+04)
  -> task-06 cycle service façade (after 03+05)
  -> PARALLEL after 06:
       task-07 CLI verbs
       task-10 projection (also needs 03)
       task-11 status JSON
       task-14 research adoption
       task-16 agent/command assets (also needs 04)
  -> PARALLEL after 02+04: task-08 assignment union
  -> task-09 stage handoff (after 06+07+08)
  -> task-15 conversation audit (after 08)
  -> PARALLEL after 11: task-12 TUI Status/Events
  -> PARALLEL after 07+10: task-13 TUI add-todo/complete-todo/finish dialogs
  -> task-17 Telegram (after 11+13)
  -> PARALLEL: task-18 docs/context
  -> task-19 final verification
```

UI/prompt work must not precede stable service contracts. Lifecycle rollout is not complete until migration and projection recovery tests pass (PRD-C15-001 §10.4).

## 1. Schema v11 migration — [SERIES foundation; PARALLEL with task-04]

- [x] 1.1 [task-01.1-migrate] [agent:generic_agent] Add transactional schema v11 in `internal/store/migrate.go` creating constrained `findings`, `finding_occurrences`, `todos`, `todo_adoptions`, and `todo_projection_ops` plus `cycles.completion_disposition` / `completion_disposition_json` per design D1; never recreate `hero.db` (PRD-C15-001 §11; ADR-083).
- [x] 1.2 [task-01.2-v10-fixture] [agent:generic_agent] Add a real SQLite fixture that opens a schema-v10 database and proves cycles, stages, events, metrics, conversation, artifacts, process registries, and model caches remain intact with empty new tables (PRD-C15-001 §14.15).

## 2. Findings store slice — [PARALLEL after task-01]

- [x] 2.1 [task-02.1-persist] [agent:generic_agent] Implement finding/occurrence persistence, per-namespace ID allocation, status queries, and fingerprint uniqueness in `internal/store` (PRD-C15-001 §5.1–5.2; ADR-083).
- [x] 2.2 [task-02.2-lifecycle] [agent:generic_agent] Cover create, exact rediscovery of open/reopened, reopen of done via fingerprint or `reopen_id`, deferred_todo recurrence warning, and secret/safe-path rejection with table-driven tests (PRD-C15-001 §5.3, §14.4, §14.7).

## 3. ToDo store slice — [PARALLEL after task-01]

- [x] 3.1 [task-03.1-todos] [agent:generic_agent] Implement ToDo/adoption/disposition persistence, finding-derived IDs, `todo-N` allocation, and idempotency keys in `internal/store` (PRD-C15-001 §9.1; ADR-087).
- [x] 3.2 [task-03.2-todo-invariants] [agent:generic_agent] Test pending→adopted→resolved, release back to pending with history retained, and retry of the same idempotency key creating neither duplicate rows nor conflicting notes (PRD-C15-001 §9.4–9.5, §14.12).

## 4. Structured report validators — [PARALLEL with task-01]

- [x] 4.1 [task-04.1-decode] [agent:generic_agent] Add typed decoders for QA, Judge, Browser UI, QA End-to-End, and Implementation reports including owner derivation, `sdd_ambiguity` isolation, and empty-success arrays (PRD-C15-001 §6; ADR-086).
- [x] 4.2 [task-04.2-diagnostics] [agent:generic_agent] Fail closed with codes `invalid_json`, `unknown_field`, `missing_field`, `invalid_enum`, `invalid_owner`, `unknown_reopen_id`, `duplicate_id`, `overlapping_arrays`, `assignment_union_mismatch`, `unassigned_id`, `false_acceptance_gate`, `nonempty_empty_assignment`, and `no_actionable_finding`; each error names field path and value (PRD-C15-001 §7.3; UI-C15-001 §5).

## 5. Engine atomic handoff — [SERIES after task-01, task-02, task-04]

- [x] 5.1 [task-05.1-tx] [agent:generic_agent] Factor `store.InTx` and tx-aware engine helpers so failed-stage close, finding writes, downstream reset, and loop-back event share one transaction; CLI/service must not nest transactions (PRD-C15-001 §7.1; ADR-084; design D3).
- [x] 5.2 [task-05.2-fail-closed] [agent:generic_agent] Reject a failed validation close with no actionable finding; prove malformed JSON / invalid owner / missing acceptance criterion / injected mid-tx failure commit no partial state; keep standalone `hero stage loop-back` finding-free (PRD-C15-001 §14.1–14.2).

## 6. Cycle service façade — [SERIES after task-03 and task-05]

- [x] 6.1 [task-06.1-atomic-api] [agent:generic_agent] Expose shared orchestration APIs on `internal/cycle` for atomic failure, assignment inputs, ToDo mutations, Research adoption, adoption release/resolve hooks, and deferred-work disposition (PRD-C15-001 §10.4; ADR-084–089).
- [x] 6.2 [task-06.2-empty-close] [agent:generic_agent] After a productive wave or user deferral that leaves zero unchecked tasks and zero open/reopened findings, close Implementation without dispatching an empty wave; defer-all sets `completed_with_deferred_todos` and skips downstream validation (PRD-C15-001 §7.3, §8.3, §14.9).

## 7. Deterministic CLI verbs — [PARALLEL after task-06]

- [x] 7.1 [task-07.1-findings-json] [agent:generic_agent] Extend `hero stage close --name <source> --failed --findings-json` to accept the full validated report and return JSON-safe structured errors (PRD-C15-001 §7.1; ADR-084).
- [x] 7.2 [task-07.2-todo-verbs] [agent:generic_agent] Add deterministic `hero add-todo` and `hero complete-todo` with Escalated/pending gates, required note, and idempotent retry behavior (PRD-C15-001 §8.1, §9.5).

## 8. Implementation assignment union — [PARALLEL after task-02 and task-04]

- [x] 8.1 [task-08.1-merge] [agent:generic_agent] Merge ordered unchecked owned `task-*` with open/reopened owned `find-*` in `internal/tui/implementation_assignment.go`; reject finding owners outside active scope before Execute (PRD-C15-001 §7.2; ADR-085).
- [x] 8.2 [task-08.2-union-gate] [agent:generic_agent] Validate exact assignment union, reject unassigned old `task-*` in a findings-only wave with `unassigned_id`, and accept only empty arrays on a true empty verification wave (PRD-C15-001 §14.3, §14.5–14.6; UI-C15-001 §4–5).

## 9. TUI stage handoff — [SERIES after task-06, task-07, task-08]

- [x] 9.1 [task-09.1-parse] [agent:generic_agent] Parse validation JSON in `internal/tui/stage_handoff.go`, call the atomic close API, and prevent agent-driven `stage close` / `loop-back` / OpenSpec checkbox / gap-file / current-state writes (PRD-C15-001 §6.1, §10.4).
- [x] 9.2 [task-09.2-apply] [agent:generic_agent] Apply verified Implementation results: check OpenSpec boxes for `task-*`, mark findings `done` for `find-*`, keep remaining IDs, apply nothing on invalid reports, and show Chat copy from UI-C15-001 §§3–5 (PRD-C15-001 §7.3).

## 10. current-state.md projection — [PARALLEL after task-03 and task-06]

- [x] 10.1 [task-10.1-reconcile] [agent:generic_agent] Implement recoverable projection in `internal/todos` using `todo_projection_ops`, atomic candidate install, and verify-before-advance; crash/retry tests must not duplicate rows or markdown lines (PRD-C15-001 §9.2, §14.10; design D6).
- [x] 10.2 [task-10.2-legacy] [agent:generic_agent] Promote selected legacy Pending lines to `todo-*` only on adopt or manual complete; preserve unmatched non-Hero prose; do not silently import every Pending line (PRD-C15-001 §9.1; ADR-087).

## 11. Status query contract — [PARALLEL after task-06]

- [x] 11.1 [task-11.1-json] [agent:generic_agent] Extend `cycle.StatusView` / `hero status` / `--json` with additive findings counts/rows, loop-backs, ToDo counts, available actions, and nullable `completionDisposition` without renaming existing fields (PRD-C15-001 §10.1, §14.14; UI-C15-001 §14; ADR-090).

## 12. TUI Status and Events — [PARALLEL after task-11]

- [x] 12.1 [task-12.1-status-board] [agent:generic_agent] Render Status finding board, loop-back history, deferred ToDos, Escalated CTAs, and `Findings none` empty state inside the existing viewport with keyboard focus/detail (UI-C15-001 §6; golang-tui).
- [x] 12.2 [task-12.2-events] [agent:generic_agent] Render finding/ToDo lifecycle events as readable ID-first rows and append corresponding store events (UI-C15-001 §12; PRD-C15-001 §10.2).

## 13. TUI control commands — [PARALLEL after task-07 and task-10]

- [x] 13.1 [task-13.1-add-todo] [agent:generic_agent] Dispatch `/hero-add-todo` from `herocmd.go`, palette, and slash overlay with Escalated gate, checklist, partial-vs-defer-all review, typed `DEFER` (or stricter) confirmation, and projection-failure retry copy (UI-C15-001 §7).
- [x] 13.2 [task-13.2-complete-todo] [agent:generic_agent] Dispatch `/hero-complete-todo` with pending-only gate, required note, adopted rejection, secret warning, and retry-idempotent success (UI-C15-001 §11).
- [x] 13.3 [task-13.3-finish-warning] [agent:generic_agent] When `/hero-finish` runs with open/reopened findings, require strong confirmation that they will not become ToDos and do not write `completed_with_deferred_todos` (PRD-C15-001 §8.4; UI-C15-001 §8).

## 14. Research ToDo adoption — [PARALLEL after task-06 and task-10]

- [x] 14.1 [task-14.1-research] [agent:generic_agent] In `internal/tui/research_session.go` and discover-agent context, query/present/adopt pending ToDos after idea notes and before grilling; persistence failure blocks requirement finalization (PRD-C15-001 §9.3; UI-C15-001 §9; ADR-089).

## 15. Assignment audit envelope — [PARALLEL after task-08]

- [x] 15.1 [task-15.1-audit] [agent:generic_agent] Preserve mixed `task-*`/`find-*` IDs and validated/raw result semantics on `cycle.StageAgentAuditBody` / conversation records (PRD-C15-001 §7.2, §10.4).

## 16. Canonical harness assets — [PARALLEL after task-04; finalize after CLI contracts]

- [x] 16.1 [task-16.1-agents] [agent:generic_agent] Update canonical orchestration, discover, backend, frontend, generic, qa, judge, browser_ui, and end2end agents for Cursor, OpenCode, Codex, and Claude with field rules, enums, passing and reopening examples, empty-success arrays, and bans on stage/file mutation; remove Judge gap-file/loop-back instructions (PRD-C15-001 §6.1, §14.16).
- [x] 16.2 [task-16.2-commands-help] [agent:generic_agent] Add `hero-add-todo` and `hero-complete-todo` command assets; update start/status/todos/continue/finish/help plus `.workflow-hero/docs/workflow-help.md` source; track checksums under existing customization protection (PRD-C15-001 §10.3–10.4).

## 17. Telegram surfaces — [SERIES after task-11 and task-13]

- [x] 17.1 [task-17.1-telegram] [agent:generic_agent] List both commands in Telegram help, forward them only to a selected connected project TUI, reject attachments, and render compact finding counts plus first actionable IDs from status JSON (PRD-C15-001 §10.3; UI-C15-001 §13).

## 18. Docs and context — [PARALLEL after service contracts; finalize after features land]

- [x] 18.1 [task-18.1-context] [agent:generic_agent] Update `context/current-state.md` and append `context/context-log.md` with C15 implementation outcomes; confirm architecture-overview/TESTING/DEPLOY already describe the shipped behavior (PRD-C15-001 §10.4).

## 19. Final verification — [SERIES]

- [x] 19.1 [task-19.1-traceability] [agent:generic_agent] Trace PRD-C15-001 §14, UI-C15-001 §15, ADR-083–090, and §10.4 component map to tasks/specs; resolve gaps before handoff.
- [x] 19.2 [task-19.2-full-verify] [agent:generic_agent] Run `openspec validate loopback-findings-handoff --strict`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt`, and `git diff --check`; write any test binaries only under `./temp/` and clean them; fix until green (PRD-C15-001 §12, §14.17).
