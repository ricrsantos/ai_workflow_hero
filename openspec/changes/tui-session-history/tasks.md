# Implementation Tasks: Persistent TUI Session History

Every task has an executable verification criterion. Artifacts stay English.
`[SERIES]` = ordering constraint. `[PARALLEL]` = independent `generic_agent` fan-out after prerequisites.
Scope: **native** → all tasks use `[agent:generic_agent]`. Apply `go-engineering` and `golang-tui` skills.

## Execution order and fan-out

```text
PARALLEL: task-01 schema v12 | task-07 optional harness capabilities
  -> PARALLEL after 01: task-02 sessions | task-03 events | task-04 leases | task-05 assets/delete-ops
  -> PARALLEL after 02: task-08 media retention | task-09 legacy migration
  -> task-06 conversation session service (after 02+03+04+05)
  -> PARALLEL after 06:
       task-10 History screen
       task-12 Chat persist/restore
  -> task-11 navbar (after 10)
  -> PARALLEL after 10+12: task-16 archive/delete UI | task-17 Telegram origin
  -> task-13 exact resume + lease TUI + historical continuation (after 06+07+12)
  -> PARALLEL after 12+07: task-14 interrupt recovery
  -> task-15 context fork (after 13)
  -> task-18 remote import (after 07+06+10)
  -> PARALLEL: task-19 docs/context
  -> task-20 final verification
```

UI/prompt work must not precede stable service contracts. Lifecycle rollout is not complete until v11→v12 and lease/delete recovery tests pass (PRD-C16-001 §5, §7).

## 1. Schema v12 migration — [SERIES foundation; PARALLEL with task-07]

- [x] 1.1 [task-01.1-migrate] [agent:generic_agent] Add transactional schema v12 in `internal/store/migrate.go` creating constrained `sessions`, `session_events`, `session_assets`, `session_leases`, and `session_delete_ops` plus indexes from design D1; never recreate `hero.db`; keep audit `conversation` intact (PRD-C16-001 §5; ADR-091–092).
- [x] 1.2 [task-01.2-v11-fixture] [agent:generic_agent] Add a real SQLite fixture that opens a schema-v11 database and proves cycles, stages, events, metrics, audit conversation, artifacts, registries, model caches, findings, and ToDos remain intact with empty new tables before legacy import (PRD-C16-001 §7.7).

## 2. Sessions aggregate store — [PARALLEL after task-01]

- [x] 2.1 [task-02.1-sessions-crud] [agent:generic_agent] Implement session create/get/update title/lifecycle, uniqueness of `(harness_id, native_session_id)` when native id is set, and cycle `ON DELETE SET NULL` in `internal/store` (PRD-C16-001 FR-01, FR-04; ADR-091).
- [x] 2.2 [task-02.2-list-search] [agent:generic_agent] Implement Active/Archived listing by `last_activity_at DESC, id` and case-insensitive title search with table-driven tests (PRD-C16-001 FR-03; UI-C16-001 §2).

## 3. Event store — [PARALLEL after task-01]

- [x] 3.1 [task-03.1-append] [agent:generic_agent] Implement append-only `session_events` with transactional `seq` allocation, `last_activity_at` update, Telegram origin fields, and rejection of cross-session routing (PRD-C16-001 FR-02; ADR-092).
- [x] 3.2 [task-03.2-paging-order] [agent:generic_agent] Cover concurrent fake stream callbacks keeping monotonic order, `provider_event_id` idempotency, and bounded newest-page reads (PRD-C16-001 §5, §7.4).

## 4. Lease store — [PARALLEL after task-01]

- [x] 4.1 [task-04.1-lease-cas] [agent:generic_agent] Implement acquire/heartbeat/release and stale compare-and-swap using an injected clock (5s heartbeat / 30s TTL) with busy-on-live-owner tests (PRD-C16-001 FR-07; ADR-094; design D5).

## 5. Assets and delete ops — [PARALLEL after task-01]

- [x] 5.1 [task-05.1-assets-ownership] [agent:generic_agent] Persist `session_assets` with `managed_copy` vs `external_source` and stable card metadata without storing image bytes (PRD-C16-001 FR-10; ADR-096).
- [x] 5.2 [task-05.2-delete-ops] [agent:generic_agent] Implement recoverable `session_delete_ops` so partial filesystem failure retries without unlinking external paths (PRD-C16-001 FR-10; design D8).

## 6. Conversation session service — [SERIES after task-02, task-03, task-04, task-05]

- [x] 6.1 [task-06.1-lifecycle-api] [agent:generic_agent] Extend `internal/conversation` with a Bubble-Tea-free session service for create, append, rename, list/search, archive/restore, lease, resume/fork, import, and delete, accepting context and injected dependencies (ADR-093).
- [x] 6.2 [task-06.2-create-on-first-turn] [agent:generic_agent] Create the session and first event atomically on the first accepted user or stage-agent prompt; empty surfaces create nothing; persistence failure blocks Execute (PRD-C16-001 FR-01, FR-02, FR-14).
- [x] 6.3 [task-06.3-titles] [agent:generic_agent] Apply deterministic Free Chat titles (48 graphemes / `Image conversation`) and cycle-aware `C{n} · {stage} · {LABEL}` titles without an LLM call (PRD-C16-001 §3.2; design D4).

## 7. Optional harness capabilities — [PARALLEL with task-01]

- [x] 7.1 [task-07.1-optional-interfaces] [agent:generic_agent] Add optional `RemoteHistoryReader` and `NativeSessionDeleter` interfaces in `internal/harness` without changing the mandatory `HarnessAdapter` contract (ADR-097).
- [x] 7.2 [task-07.2-adapter-truthfulness] [agent:generic_agent] Keep Cursor/Codex/Claude remote-history and native-delete unsupported; wire OpenCode `ReadRemoteHistory` only if existing session messages can be normalized without invention; never silently recreate a native session on exact-resume failure (PRD-C16-001 FR-09, FR-11; design D7).

## 8. Media retention amendment — [PARALLEL after task-02]

- [x] 8.1 [task-08.1-durable-retention] [agent:generic_agent] Key C14 directories by Hero session ID; skip age cleanup for registered sessions; delete only orphans and unregistered temp dirs older than seven days; never unlink `external_source` paths (PRD-C16-001 FR-10; ADR-096).

## 9. Legacy migration — [PARALLEL after task-02; uses task-01 migration path]

- [x] 9.1 [task-09.1-idempotent-legacy] [agent:generic_agent] Import valid v11 orchestration/stage bindings into History with `unavailable_legacy`, stable `legacy_source_key`, no invented events, no network, and no duplicates on second open (PRD-C16-001 FR-12; ADR-098).

## 10. History TUI screen — [PARALLEL after task-06]

- [x] 10.1 [task-10.1-history-screen] [agent:generic_agent] Add an async History child model (list/detail, Active/Archived, search, rename buffer, loading/error/dialogs) with all I/O as `tea.Cmd` and centralized `key.Binding` (UI-C16-001 §§2,5,9; golang-tui).
- [x] 10.2 [task-10.2-responsive-copy] [agent:generic_agent] Cover wide/stacked/narrow layouts, rune-safe truncation, and empty/busy/legacy/interrupted/fork/import copy from UI-C16-001 §§3–4 (PRD-C16-001 §7.8).

## 11. Navbar History item — [SERIES after task-10]

- [x] 11.1 [task-11.1-navbar-history] [agent:generic_agent] Insert History immediately after Chat in `nav_sidebar.go` with dynamic Alt ranges for project-no-cycle, active-cycle, and standalone `hero chat` (UI-C16-001 §1).

## 12. Chat persist and restore — [PARALLEL after task-06]

- [x] 12.1 [task-12.1-incremental-persist] [agent:generic_agent] Persist visible Chat/Telegram events incrementally from the execute path through the session service; never block `Update` (PRD-C16-001 FR-02, FR-14).
- [x] 12.2 [task-12.2-restore-transcript] [agent:generic_agent] Reconstruct Chat from newest event page including actor styling, agent labels, Telegram origin, asset cards, and interruption markers; occupancy stays per selected session (PRD-C16-001 FR-05; UI-C16-001 §8).
- [x] 12.3 [task-12.3-new-chat] [agent:generic_agent] Make `/new-chat` keep the prior session in History, release its lease, and open an unpersisted empty surface (PRD-C16-001 FR-13).

## 13. Resume, lease UX, historical continuation — [SERIES after task-06, task-07, task-12]

- [x] 13.1 [task-13.1-exact-resume] [agent:generic_agent] Open-from-History resumes stored harness/model/properties/native ID with no silent substitution (PRD-C16-001 FR-05).
- [x] 13.2 [task-13.2-lease-tui] [agent:generic_agent] Acquire/heartbeat/release leases from TUI commands; show busy copy; disable unsafe actions during Execute/preflight; allow History browse (PRD-C16-001 FR-07; UI-C16-001 §4).
- [x] 13.3 [task-13.3-historical-continuation] [agent:generic_agent] Opening a completed stage session shows `Historical continuation · workflow stage remains completed.` and does not mutate stage lifecycle (PRD-C16-001 FR-06).

## 14. Interrupt recovery — [PARALLEL after task-12 and task-07]

- [x] 14.1 [task-14.1-interrupt-recover] [agent:generic_agent] Mark interrupted sessions, keep partial events, check native status on reopen, reconnect when supported, otherwise offer cancel/recovery and gate the composer (PRD-C16-001 FR-08).

## 15. Context fork — [SERIES after task-13]

- [x] 15.1 [task-15.1-context-fork] [agent:generic_agent] On resume-unavailable, show the documented fork dialog and create a new session with a 32 KiB deterministic user/assistant context blob; leave the original row unchanged (PRD-C16-001 FR-09; design D6).

## 16. Archive and delete UI — [PARALLEL after task-10 and task-06]

- [x] 16.1 [task-16.1-archive-restore] [agent:generic_agent] Implement `a` archive/restore, lightweight confirm only when the selected session is currently open, and empty-Chat after archiving the current idle session (UI-C16-001 §6).
- [x] 16.2 [task-16.2-permanent-delete] [agent:generic_agent] Implement `d` confirmation defaulting to Cancel, local-first purge, remote warning, and empty-Chat after deleting the current idle session (UI-C16-001 §7; PRD-C16-001 FR-10).

## 17. Telegram origin persistence — [PARALLEL after task-12]

- [x] 17.1 [task-17.1-telegram-origin] [agent:generic_agent] Store Telegram turns on the same Hero session with origin/address and restore `←/→ [Telegram · addr]` labels after restart (PRD-C16-001 §3.1; UI-C16-001 §10.4).

## 18. Confirmed remote import — [SERIES after task-07, task-06, task-10]

- [x] 18.1 [task-18.1-confirmed-import] [agent:generic_agent] Prompt before first remote import, persist normalized events idempotently, and leave local state unchanged on unsupported/failed reads (PRD-C16-001 FR-11; UI-C16-001 §4).

## 19. Docs and context — [PARALLEL after service contracts; finalize after features land]

- [x] 19.1 [task-19.1-context] [agent:generic_agent] Update `context/current-state.md` and append `context/context-log.md` with C16 implementation outcomes; confirm architecture-overview/TESTING/DEPLOY describe the shipped History behavior (PRD-C16-001 §5).

## 20. Final verification — [SERIES]

- [x] 20.1 [task-20.1-traceability] [agent:generic_agent] Trace PRD-C16-001 FR-01–FR-14 and §7, UI-C16-001 §10, and ADR-091–098 to tasks/specs; resolve gaps before handoff.
- [x] 20.2 [task-20.2-full-verify] [agent:generic_agent] Run `openspec validate tui-session-history --strict`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt`, and `git diff --check`; write any test binaries only under `./temp/` and clean them; fix until green (PRD-C16-001 §7.8).
