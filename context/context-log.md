# Context Log

> Short-term project memory for this repository (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Permanent facts belong in `context/current-state.md`.

## 2026-09-12 — Freeze QA→Implement finding contract (handoff)

**Problem**: C16 QA/Implement loop reopened the same `find-qa-*` IDs with mutated issue/acceptance each round. Implementation patched the previous sentence; QA then rewrote the card. `reopen_id` won without matching file/requirement/acceptance, and persist overwrote the assignment contract.

**Fix**: Store `ValidateReopenID` / `PersistFindingTx` require a matching stored contract. Reopen, rediscovery, done, and defer update status/round/occurrences only. Decoder passes file/requirement/acceptance into the reopen validator (`unknown_reopen_id` on drift). Implementation finding blocks include a frozen-contract line plus occurrence history. QA/Judge/BUI/E2E agent prompts (assets + four harness trees) forbid ID reuse for a new residual. ADR-083 amended; PRD-C15-001 §5.1; living `findings-lifecycle` spec updated.

**Validation**: `go test ./internal/store/ ./internal/cycle/reports/ ./internal/engine/ ./internal/tui/ ./internal/cycle/`; `go test ./...`.

## 2026-09-12 — Restore passive TUI harness health (inviolable)

**Outcome**: `handleHarnessHealthResult` no longer calls `cancelStreamCmd` on `HealthFailed`. Degraded / suspected / failed warn only. Reconnecting still uses the connection-dropped copy and maps Failed→Degraded. `TestHealthFailedDoesNotCancelStream` plus hang-path tests lock the contract. `AGENTS.md` Project Constraints records the rule as inviolable (spec `openspec/specs/harness-adapter/spec.md`).

**Validation**: `go test ./internal/tui/ ./internal/harness/ ./internal/adapters/cursor/ ./internal/adapters/codex/`; `go test ./...`.

## 2026-09-12 — Cursor session-idle watchdog cancelled ORCH handoff (C16)

**Problem**: C16 Implementation wave completed (`generic_agent` Cursor, 14m, findings marked done). ~550ms later TUI showed `WARNING: session idle` + ORCH `Interrupted`. `.workflow-hero/logs/tui.log`: `execute complete` then `tui stream cancel failed` (`no in-flight execution for session ""`) then `tui conversation interrupted` then scheduler `retry_start` with Implementation still Running.

**Cause**: Cursor `CheckHealth` mapped `HasInFlight()==false` + non-running status to `SessionAlive=false` / `ProcessAlive=false` ("session idle"). Watchdog `HealthFailed` auto-cancels. A probe in flight during the long GEN turn finished after Execute returned and after `resumeOrchestratorAfterStageHandoff` started ORCH — cancelling the new turn before `cursor agent execute start`.

**Fix**: Known Cursor sessions that are completed/cancelled/idle stay alive in `CheckHealth`. TUI health results include `harnessHealthGeneration` (incremented in `resetHarnessWatchdog`); mismatched probes are ignored.

**Cycle state**: C16 active; Implementation Running 6/6; QA Waiting 5/5; 30 findings done. Resume needs rebuilt TUI then `/hero-start`. Same race can recur on the next long Cursor handoff until the TUI process is replaced.

**Validation**: `go test ./internal/adapters/cursor/ ./internal/tui/ ./internal/harness/`; `go test ./...`.

## 2026-09-12 — Implementation loop-back find-qa-16..23 (generic_agent)

**Outcome**: Closed QA contract defects for C16 session history:
- find-qa-16: durable media dirs keyed by Hero session ID (provisional ID reused as SQLite PK); retention keep-list aligned.
- find-qa-17: schema v13 `session_delete_op_managed_paths` + `ResumeIncompleteDelete` purge-before-complete.
- find-qa-18: conversation/session/context SQLite I/O moved behind tea.Cmd; blocking Update test added.
- find-qa-19: two-phase quit persists interrupt before Quit; LiveStreamAttacher + OpenCode attach; unsupported adapters stay gated.
- find-qa-20: history diagnostics log `redact.Error` only.
- find-qa-21: history truncation/padding via ANSI-safe display width helpers.
- find-qa-22: atomic remote import + transcript available; import errors stay on History; stable OpenCode message IDs.
- find-qa-23: Go time layouts `2 Jan 15:04` / `2 Jan 2006 15:04`.

**Validation**: `go test ./...` PASS; `openspec validate tui-session-history --strict` PASS.

## 2026-09-12 — find-qa-18 async conversation context I/O (generic_agent)

**Outcome**: Session/cycle context reads and chat-session store clears (`ClearOrchestrationSession`, stage binding clear, `ConversationContext`, orchestration/stage bindings) run in `tea.Cmd` handlers (`conversation_context.go`) for `/new-chat`, history resume, `enterConversation`, `submitChatFollowUp`, and `/hero-new` `PrepareWorkflowConfig`. `TestUpdatePathsDoNotBlockOnSlowConversationContextIO` injects `testContextIODelay` to prove Update stays fast while Cmd performs SQLite work. `testMode` without delay keeps synchronous helpers for existing unit tests.

**Validation**: `go test ./internal/tui/ -run TestUpdatePathsDoNotBlock` and full `go test ./internal/tui/`.

## 2026-09-12 — find-qa-19 quit-while-streaming + live recover attach (generic_agent)

**Outcome**: Two-phase shutdown sets `pendingQuitAfterInterrupt`, cancels the stream only, then on `streamCancelDoneMsg` runs `syncFinalizeSessionInterruptCmd` (persist + `MarkInterrupted`) before `tea.Quit`/restart. Recovery no longer fakes reconnect via `ResumeSession` when harness reports `running` without `LiveStreamAttacher`; OpenCode implements attach via resume + SSE (`readExecuteSSE`). Composer stays gated on `heroSessionRecoverBusy` through attach.

**Validation**: `go test ./internal/tui/ -run 'TestSessionRecover|TestQuitWhile' ./internal/harness/ -run TestLiveStream`

## 2026-09-12 — find-qa-22 remote import atomicity + TUI error path (generic_agent)

**Outcome**: `store.UpdateSessionTranscriptState` + `ImportRemoteSessionEvents` commit remote append, `remote_import_confirmed`, and `transcript_state=available` atomically. `SessionService.ImportRemoteHistory` reads remote first then imports in one tx (rollback on any failure). History `historyImportMsg` errors keep the user on History with `actionErr` (no Chat open). OpenCode `stableOpenCodeMessageID` replaces index-based fallback for empty `info.id`. Tests: transcript available, atomic rollback, TUI no-open-on-error, reorder-stable provider IDs.

**Validation**: `go test ./...`

## 2026-09-12 — find-qa-17 session delete managed-path manifest (generic_agent)

**Outcome**: Schema v13 adds `session_delete_op_managed_paths`; `DeleteSessionLocalFirst` persists `managed_copy` paths before session cascade delete. `ListManagedPathsForDeleteOp` + `SessionService.ResumeIncompleteDelete` purge from manifest then advance op; TUI startup retry uses resume instead of skipping purge. Tests cover manifest persistence, crash/reopen resume, external paths excluded, purge failure leaves `intent`.

**Validation**: `go test ./internal/store/... ./internal/conversation/... ./internal/tui/...`

## 2026-09-12 — C16 Implementation wave verify (generic_agent)

**Outcome**: Confirmed all assigned `tui-session-history` tasks (01.1–20.2) against shipped code. Fixed gofmt on `internal/harness/capabilities_test.go`. Full verify green: `openspec validate tui-session-history --strict`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt`, `git diff --check`. Traceability at `openspec/changes/tui-session-history/traceability.md`; current-state already documents C16 History.

**Validation**: openspec strict + go test(+race) + vet + gofmt + diff --check

## 2026-09-12 — C15 Implementation report rejected (`unknown_field: metrics`)

**Outcome**: Aligned C15 stage-agent contracts with `internal/cycle/reports` allowlists. Implementation `backend`/`frontend`/`generic` examples no longer put `metrics` in the report JSON (allowlist + `complete`/`partial`/`blocked`); QA/Judge/BUI/E2E keep metrics out of the decoded object (`Metrics (orchestrator only)`). Orchestrator Metrics Procedure estimates tokens without reading C15 JSON. TUI `implementationReportDiagnosticCode` maps `unknown_field` (and other decoder codes) instead of collapsing to `invalid_report`. Tests: decoder rejects `metrics`; embedded JSON fences decode; Runtime assets forbid `metrics` inside C15 examples.

**Validation**: `go test ./...`

## 2026-09-11 — C16 Implementation wave 1 (full assignment verify)

**Outcome**: Verified all 34 `generic_agent` tasks for `tui-session-history` against the shipped codebase (schema v12 store, SessionService, optional harness capabilities, History/Chat TUI, retention, legacy import, Telegram origin, confirmed remote import, traceability). Corrected architecture-overview stale schema **v11** / "C16 target" wording to describe shipped v12 History behavior. TESTING.md / DEPLOY.md §3.5 / traceability.md already matched.

**Validation**: `openspec validate tui-session-history --strict`; `go test ./...`; `go test -race ./...`; `go vet ./...`; `gofmt`; `git diff --check`

## 2026-09-11 — C16 tasks 13–16 resume/lease/fork/archive TUI

History open acquires/releases leases, attempts exact harness resume, shows fork dialog on failure, restores Chat with stored harness/model/native binding and historical-continuation banner for completed stages; interrupt recovery checks native Status and gates composer; archive/delete confirm flows empty Chat when the current idle session is affected. Implemented in `chat_session_history.go` plus `history_screen.go`, `chat_session.go`, `timers.go`.

## 2026-09-11 — C16 task-17/18/19/20 Telegram origin, remote import, docs, verify

**Outcome**: Telegram turns persist `origin`/`origin_address` on user and assistant events (`chat_session.go` `assistantPersistOrigin`, restore via `eventsToTranscript` + `telegramOriginLabel`). Confirmed remote import: `ShouldOfferRemoteImport`, History import dialog wired to `ImportRemoteHistory` (`session_remote.go`, `session_import.go`); failed reads leave local events unchanged. Updated `context/current-state.md`, `traceability.md` (FR-01–14 / ADR-091–098), confirmed TESTING/DEPLOY C16 sections describe shipped History behavior.

**Validation**: `openspec validate tui-session-history --strict`; `go test ./...`; `go test -race ./...`; `go vet ./...`; `gofmt`; `git diff --check`

## 2026-09-11 — C16 task-10/11 History TUI + navbar

**Outcome**: `internal/tui/history_screen.go` adds async History child model (list/detail, active/archived, search, rename, delete/archive dialogs, UI-C16 §4 copy, wide/stacked/narrow layouts). Navbar inserts History after Chat with `alt+1-8` / free-chat `alt+1-3`; palette `Go to - History`. `sessionService` injected from project store in `newModel`. Resume-from-History in Chat remains stubbed.

**Validation**: `go test ./internal/tui/ -count=1 -timeout 120s`

## 2026-09-11 — C16 task-12 Chat persist/restore + /new-chat

**Outcome**: `internal/tui/chat_session.go` persists visible Chat events through `SessionService` (first-turn gate in execute worker blocks harness on failure; stream inserts via async `tea.Cmd` queue; assistant/assets/native bind synced at execute completion). Transcript restore rebuilds user/agent styling, Telegram origin, assets, interruption markers, and per-session occupancy from `ListEventsNewest` (200). `/new-chat` releases the prior lease and clears in-memory Hero/native ids without deleting the History row. History open triggers `loadChatTranscriptCmd`.

**Validation**: `go test ./internal/tui/ ./internal/conversation/ -count=1 -timeout 180s`

## 2026-09-11 — C16 task-08.1 / task-09.1 retention + legacy import

**Outcome**: `internal/media` `CleanupExpiredSessions` retains directories whose names match registered Hero session IDs (`RegisteredSessionIDs` / `ListRegistered`); unregistered orphans still age out at 7 days. TUI startup (`mediaStartupCleanupCmd`) and shutdown (`launch.go`) pass `ListRegisteredSessionIDs` from the project store. `internal/store.MigrateLegacySessionBindings` runs on every `Open` after schema migration, importing valid orchestration/stage harness+native pairs with D4 titles, `transcript_state=unavailable_legacy`, no `session_events`, idempotent via `legacy_source_key`.

**Validation**: `go test ./internal/media/ ./internal/store/`

## 2026-09-11 — C16 task-06 conversation session service

**Outcome**: `internal/conversation` gained `SessionService` (lifecycle + fork/import/delete helpers), D4 title functions (`TitleFreeChat`, cycle-aware orchestration/research/stage titles), and table-driven tests. `EnsureFirstTurn` atomically creates session+first event; empty surfaces create no row. Fixed `session_bindings_legacy_test.go` `ListSessionsFilter` typo.

**Validation**: `go test ./internal/conversation/ ./internal/store/`

## 2026-09-11 — C16 task-07 optional harness capabilities

**Outcome**: `internal/harness/capabilities.go` adds `NormalizedEvent`, optional `RemoteHistoryReader` / `NativeSessionDeleter`, and `ErrExactResumeUnavailable`. OpenCode implements `ReadRemoteHistory` via existing `fetchSessionMessages` normalization; Cursor/Codex/Claude omit both optional interfaces (tests assert type-assert fails). OpenCode and Codex no longer start a replacement native session when explicit `SessionID` resume fails.

**Validation**: `go test ./internal/harness/... ./internal/adapters/...`

## 2026-09-11 — C16 Planning: tui-session-history SDD

**Outcome**: OpenSpec change `tui-session-history` created at `openspec/changes/tui-session-history/` (proposal, design D1–D12, 8 spec deltas, 34 `generic_agent` tasks, traceability). `openspec validate tui-session-history --strict` passes. Linked with `hero cycle openspec-change tui-session-history`. `openspec/config.yml` context regenerated from `documents.json` for C16.

**Locked decisions**: Schema v12 tables (`sessions`, `session_events`, `session_assets`, `session_leases`, `session_delete_ops`); Hero UUID identity; cycle FK `ON DELETE SET NULL`; lease heartbeat 5s / TTL 30s; fork context 32 KiB user+assistant text; titles without LLM; C14 registered dirs retained until delete; optional `RemoteHistoryReader` / `NativeSessionDeleter` with no invented Cursor/Codex/Claude delete support. Parallel Implementation agents get distinct History rows; nested TASK stays on the parent session.

**Fan-out**: schema v12 ∥ harness capabilities → store slices → session service → History UI ∥ Chat persist → navbar/resume/fork/Telegram/import → docs → final verify.

## 2026-09-11 — TUI scheduler never-idle after loop-back

**Defect**: C15 persisted findings and loop-back, but `resumeOrchestratorAfterStageHandoff` set `stageHandoffInterventionRequired` whenever `Complete` was false. After QA/Judge fail, if the orchestrator called `hero stage start` and STOPped, Implementation was Running with no Execute. `/hero-start` was the only recovery. Other idle paths (Escalated, PendingApproval, StartStage budget, orch Execute error, silent `startStageAgentSessions` when not Running) also returned without launching or asking.

**Fix**: `ensureStageProgress` (`internal/tui/stage_progress.go`) is the idle gate. Scheduler-handled loop-back clears intervention and `stageHandoffDoneKey`. Waiting/Running (new iteration) dispatches named agents; matching `doneKey` without intervention does not redispatch (avoids a planning loop); matching `doneKey` with intervention posts `/hero-start` (or `/hero-back` for Judge). Escalated/PendingApproval/start failure post CTAs. `EventStageStarted`/`EventApprovalRequired` while idle re-enter the gate. Orchestrator Execute errors always call `maybeHandoffAfterExecute`. User cancel holds auto-dispatch until `/hero-start`. Loop-back preamble no longer asks for `/hero-start`.

**Validation**: `go test ./internal/tui/ -count=1`; `go test ./...` green.

## 2026-09-11 — C15 finished via /hero-finish

**Outcome**: Cycle C15 (`loopback-findings-handoff`) completed with all enabled stages done: Research 1/3, Planning 1/3, Implementation 3/4 (two QA loop-backs), QA 3/3 (one `/hero-continue` after iteration budget), Judge 1/3. Browser UI Validation and QA End-to-End stayed skipped (disabled; native scope). `hero finish` recorded `completed_at` in SQLite. OpenSpec change `loopback-findings-handoff` remains linked until `/hero-archive`.

**Decisions that landed**: Validation agents emit typed JSON only; Go atomically persists findings, failed close, and loop-back. Implementation assignments union OpenSpec `task-*` with SQLite `find-*`. Escalated users may defer selected findings via `/hero-add-todo`; `/hero-complete-todo` resolves pending items with an audit note. Cancel/Reject release unresolved adopted ToDos; Finish resolves them (emergency-release if open findings remain). `current-state.md` Pending Features projection is recoverable via `todo_projection_ops`.

**Metrics** (last active `hero metrics` snapshot): 5278751 in / 259178 out tokens (5537929 total), ~$2.326925. Finish-turn orchestrator estimate (`cursor-grok-4.6-high`): 30934 in / 1225 out, ~$0.069218, 120000 ms — not stored as a stage row because no stage was pending approval. Project totals written to `.workflow-hero/metrics-summary.md`.

**Next**: `/hero-archive` (OpenSpec archive first; Hero folder date from store `completed_at`).

## 2026-09-11 — C15 QA loop-back: Reject releases adopted ToDos

**Defect**: `Service.Reject` did not release unresolved adopted ToDos or reconcile `context/current-state.md` (PRD-C15-001 §9.4; ADR-089).

**Fix**: `Reject` now calls `releaseAdoptedTodosBeforeTerminal` before engine reject (same path as `Cancel`). Regression: `TestRejectReleasesAdoptedTodos` covers pending status, adoption history, verified projection op, and pending line in `current-state.md`.

**Validation**: `go test ./internal/cycle/ -run TestRejectReleasesAdoptedTodos`; `go test ./...` green.

## 2026-09-11 — C15 final verification (tasks 19.1–19.2)

**Traceability**: Added `openspec/changes/loopback-findings-handoff/traceability.md` mapping PRD §14, UI §15, ADR-083–090, and §10.4 components to tasks and code. No blocking gaps.

**Fix**: Updated `internal/install/testdata/codex_projection_layout.golden` for C15 `hero-add-todo` / `hero-complete-todo` command assets.

**Validation**: `openspec validate loopback-findings-handoff --strict`; `go test ./...`; `go test -race ./...`; `go vet ./...`; `gofmt -l .` (clean on touched files); `git diff --check`.

**Context**: `current-state.md` marks TUI Status board, Escalated add/complete-todo dialogs, and Telegram C15 surfaces as shipped.

## 2026-09-11 — C15 TUI Status and Events (tasks 12.1–12.2)

**Change**: `internal/tui/status_screen.go` extends Status with loop-backs, findings board (owner labels BACK/FRNT/GEN, `deferred_todo` as `ToDo`), ToDo counts, completion disposition, Escalated CTAs, and focused-row detail from SQLite. `internal/tui/events_format.go` renders lifecycle events ID-first on the Events screen. `internal/store` adds lifecycle event constants and append-on-mutate for finding create/reopen/defer/done and ToDo adopt/release/manual complete. Tests: `go test ./internal/tui/ -count=1 -timeout 120s`.

## 2026-09-11 — C15 TUI control commands (tasks 13.1–13.3)

**Change**: Added `internal/tui/todo_control.go` — Escalated-gated `/hero-add-todo` checklist, partial vs defer-all review, typed `DEFER` confirmation, projection-failure retry copy; `/hero-complete-todo` pending/adopted gates, required note, secret warning, idempotent success messaging; `/hero-finish` strong `FINISH` confirmation when open/reopened findings exist (orchestrator path unchanged, no `completed_with_deferred_todos`). Wired palette items, chat slash dispatch, `herocmd.go` inline parsers. Tests: `go test ./internal/tui/ -count=1`.

## 2026-09-11 — C15 Telegram surface (task-17.1-telegram)

**Change**: `internal/telegram` — help lists `/hero-add-todo` and `/hero-complete-todo`; `IsProjectControlCommand` + daemon routing rejects free-chat targets and attachments; `CompactFindingsStatus` from additive `cycle.StatusView` JSON; TUI `telegram_status.go` appends compact block on cycle `/status`. Tests: `go test ./internal/telegram/...` pass.

## 2026-09-11 — C15 implementation summary (context task-18.1)

**Shipped (verified in tree)**: SQLite schema v10→v11 with findings, occurrences, structured ToDos, adoptions, projection ops, and cycle completion disposition. `internal/store` transactional findings/ToDo APIs; `internal/cycle/reports` typed validation decoders; `internal/engine` atomic `CloseStageFailedWithFindings` and deferred-terminal close paths; `internal/cycle` handoff façade, mixed assignment union, `add-todo` / `complete-todo` / `stage close --failed --findings-json`; `internal/todos` recoverable `current-state.md` projection; TUI implementation assignment union, stage-handoff parse/apply, Research ToDo injection/adoption; `cycle.StatusView` additive JSON (`findings`, `loopBacks`, `todos`, `availableActions`, `completionDisposition`); canonical four-harness agents/commands including `/hero-add-todo` and `/hero-complete-todo`.

**Docs**: `context/current-state.md` consolidated; `architecture-overview.md` schema/command flow aligned to shipped core. Superseded by final verification entry above (TUI Status/control and Telegram now shipped).

## 2026-09-11 — C15 deterministic CLI verbs (tasks 07.1–07.2)

**Change**: Extended `hero stage close` with `--findings-json` (requires `--failed`) calling `Service.CloseStageFailedWithFindings`; report validation errors emit JSON-safe diagnostics (`code`, `field`, `value`, `message`) via `internal/cycle/cli_errors.go`. Added `hero add-todo` and `hero complete-todo` wired through `AddFindingTodos` / `CompleteManualTodos` (`internal/cycle/todo_cli.go`) with Escalated gate, pending/adopted guards, required `--note`, idempotency keys, and `ReconcileTodoProjection` after store mutations. `DeferFindingTodoIdempotent` now marks findings `deferred_todo` in the same transaction. Tests: `command_test.go`, `todo_cli_test.go`.

**Validation**: `go test ./cmd/... ./internal/cycle/ ./internal/store/ -count=1` passes.

## 2026-09-11 — C15 Status JSON (task-11.1-json)

**Change**: Extended `cycle.StatusView` and `Service.Status()` with additive `findings`, `loopBacks`, `todos`, `completionDisposition`, and `availableActions` (`internal/cycle/status_view.go`). Added `store.CountTodosByStatus` and `internal/cycle/status_view_test.go`.

**Validation**: `go test ./internal/cycle/ -count=1` passes.

## 2026-09-11 — C15 assignment audit envelope (task-15.1-audit)

**Change**: Extended `cycle.StageAgentAuditBody` in `internal/cycle/stage_agent_audit.go` to normalize mixed `task-*`/`find-*` assignment IDs via `internal/cycle/reports`, mark raw results with `result_validated=false`, and add `RecordStageAgentResultValidated` for raw body plus normalized `tasks_completed`/`tasks_remaining`. Tests in `internal/cycle/service_test.go`.

**Validation**: `go test ./internal/cycle/ -count=1 -run Audit` passes.

## 2026-09-11 — C15 harness assets (tasks 16.1–16.2)

**Change**: Updated canonical agents under `assets/{cursor,opencode,codex,claude}/agents/` (orchestration, discover, backend/frontend/generic, qa, judge, browser_ui, end2end) with C15 report contracts aligned to `internal/cycle/reports` (field rules, pass/reopen examples, empty success arrays, bans on stage/checkbox/gap-file/current-state mutation; Judge no longer instructs gap files or `hero stage loop-back`). Added `hero-add-todo` and `hero-complete-todo` command assets per harness; refreshed start/status/todos/continue/finish/help and `assets/docs/workflow-help.md`. Bumped `internal/common/assets_test.go` command inventory to 18 files. Helper script: `scripts/c15_asset_bootstrap.py`.

**Validation**: `go test ./internal/common/ -run 'RuntimeAssets|Asset' -count=1` passes.

## 2026-09-11 — C15 findings store slice (tasks 02.1–02.2)

**Change**: Added `internal/store/findings.go` (fingerprint canonicalization, `PersistFinding`/`PersistFindingTx`, ID allocation, status/actionable queries, `ValidateReopenID`, scheduler helpers `MarkFindingDoneTx`/`SetFindingDeferredTodoTx`) and `internal/store/findings_lifecycle_test.go` (create, rediscovery, reopen via fingerprint/`reopen_id`, deferred recurrence, unsafe content rejection, namespace IDs).

**Validation**: `go test ./internal/store/ -count=1` passes.

## 2026-09-11 — C15 ToDo store slice (tasks 03.1–03.2)

**Change**: Added `internal/store/tx.go` (`InTx`), `internal/store/todos.go` (finding/legacy ToDos, adoption history, cycle completion disposition, projection-op upsert/idempotent defer+complete), and `internal/store/todos_test.go` (pending→adopted→resolved, release with history, idempotency without duplicate rows or conflicting notes).

**Validation**: `go test ./internal/store/ -count=1` passes.

## 2026-09-11 — Telegram `/status` line order

**Problem**: The daemon prefixes outbound text with the instance address. The
new `Agent state` line was emitted first, so idle replies began with
`aiwkhero: Agent state: idle` and then repeated `idle` on the next line.

**Change**: `internal/tui/telegram_status.go` now keeps the primary status
headline first and inserts `Agent state` as the second line. This produces
`aiwkhero: idle` followed by `Agent state: idle`, with the same ordering for
cycle and active free-chat responses.

**Validation**: Focused Telegram status tests and `go test ./...` pass.

## 2026-09-11 — Telegram `/status` agent state

**Change**: Added an explicit `Agent state: working|idle` line to the
Telegram `/status` response. The value follows the Chat's live execution,
agent, and `/hero-start` preflight state; existing cycle, model, timer, and
context fields remain unchanged. Automatic idle reports remain suppressed.

**Validation**: Added focused TUI coverage for idle, streaming, live-agent,
and `/hero-start` preflight states. Focused tests and `go test ./...` pass.

## 2026-09-11 — C14 archived

**Change**: `/hero-archive` ran `openspec archive tui-multimodal-images -y` (merged 18 spec additions; archived as `2026-09-11-tui-multimodal-images`), then Hero archived cycle C14 to `.workflow-hero/cycles/archive/C14-2026-09-11-implementa-o-da-capacidade-de-lidar-com` using store `completed_at` 2026-09-11. No current cycle remains. Metrics for C14 were already in `.workflow-hero/metrics-summary.md` from `/hero-finish` (2101873 tokens, ~$0.5692).

## 2026-09-11 — C14 implementation verification formatting

**Change**: Applied the required gofmt comment-column alignment in
`internal/tui/model_picker_refresh_test.go` after the QA loop-back. Behavior and
task checkboxes were unchanged.

**Validation**: `gofmt -d internal/tui/model_picker_refresh_test.go`,
`go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...`,
`git diff --check`, and strict OpenSpec validation all pass.

## 2026-09-11 — C14 multimodal loop-back hardening

**Problem**: QA identified edge regressions around image-bearing Free Chat
turns: composer chips could disappear before capability/adapter rejection,
free-chat policy lookup could read the execution work directory instead of the
Hero config root, and streamed assets could follow the mutable active-agent
index rather than their producing Execute.

**Change**: Deferred chip removal until a successful image Execute; rejected
turns therefore preserve validated chips. Permission lookup now uses the
configured Hero project root while materialization still uses the execution
workspace. Stream asset routing carries the Execute's transcript index,
associates fallback assets with an agent turn, invalidates transcript layout,
and removes the populated-transcript global-tail fallback. Downloads-prefilled
save behavior with explicit overwrite confirmation remains covered. The
Claude Unix process was normalized with `gofmt`.

**Validation**: Added focused regressions for pre-admission chip retention,
split config/workspace policy resolution, and concurrent stream asset ownership.
Focused TUI tests, `gofmt`, `go test ./...`, `go test -race ./...`, `go vet
./...`, and strict OpenSpec validation all passed.

## 2026-09-11 — Cursor `/model` false “reset to na” on Grok 4.6 High

**Problem**: Selecting `cursor-grok-4.6-high` in the TUI showed `⚠ /model — The selected value is no longer supported by this model and was reset to na.` The model itself was valid; effort is baked into the slug.

**Cause**: `EffectiveValues` treated any saved value on an unavailable property as invalidated. Cursor slug locks mark `ef`/`fs` unavailable while keeping the locked default (`high`, `false`). Matching saved values were wiped to `na`.

**Change**: Unavailable properties now keep a saved value that matches the lock, surface the locked default when unset, and warn only on a real conflict (e.g. `cursor-grok-4.6-low` with saved `ef: high`). ADR-040 points 4–5 clarified.

**Validation**: `go test ./...`.

## 2026-09-11 — TUI Chat lag after long sessions

**Problem**: After ~12h of C14 in one `hero tui` process, typing in the composer and scrolling the agent pane were extremely slow.

**Cause**: `View` rebuilt the entire transcript on every keystroke and timer tick. `transcriptVisibleLines` called `buildConversation(0)`, which walked all messages, wrapped, and Lip Gloss-painted every row. Per-message `responseLines` lived on the View copy and did not persist across frames.

**Change**: Heap-backed `transcriptLayout` cache shared by View copies; wait spinner appended outside the cache; chrome height measured without rebuilding history.

**Validation**: `go test ./internal/tui/` (43s). Restart the running TUI to pick up the binary.

## 2026-09-11 — Idea: loop-back findings handoff (tobe)

**Problem**: C14 Implementation stayed Running after QA loop-back because ADR-075 assigned only unchecked OpenSpec IDs. Informal `qa-gaps.md` / `judge-gaps.md` and `stages.summary` reasons never enter the scheduler. The same hole exists for Judge, Browser UI Validation, and QA End-to-End.

**Decision**: Non-normative idea at `docs/idea/tobe/loopback-findings-handoff.md` (Discover ignores `tobe/`). Findings live in `hero.db`; agents emit JSON only; scheduler owns status. OpenSpec `task-*` stays planning. Escalation gains `/hero-add-todo` (defer finding to `current-state.md` Pending and continue); `/hero-todos` stays read-only. Status / `hero status` / `/hero-status` show loop-back + findings + deferred todos. Next dedicated cycle; do not fold into C14.

**Validation**: Idea file written; no code or schema change.

## 2026-09-10 — Release Hero v3.2.0

**Change**: Tagged `v3.2.0` (minor after `v3.1.1`). Release commit bumps default `main.version`, `current-state`, and architecture overview. GitHub Release ships cross-compiled Hero + Telegram daemon artifacts and `checksums.txt`.

**Validation**: `go test ./...`; `./scripts/release.sh`.

## 2026-09-10 — Context bar occupancy vs billed run totals

**Problem**: The Chat context bar (and Telegram `Context`) filled too quickly
in freechat and during cycles. Previous fixes stopped summing Executes, but
Cursor/Claude `result.usage` is the billed sum of every model call in the
tool loop. `WithContextTokens()` then added cache on top of that aggregate,
so occupancy looked like window overflow after a few tool uses.

**Change**: `ContextTokens` is last-call occupancy only. Billed
`Input`/`Output`/cache stay for Costs. Inclusive cache (input already
contains cache) is not added twice. Cursor uses the last stream `usage`
event; Claude uses the last assistant `message.usage`. A billed result is
occupancy only for a single-call turn (no tools). Otherwise the TUI falls
back to chars÷4 of that session's transcript and never reconstructs the bar
from billed totals. OpenCode last-step and Codex `last` keep occupancy via
`WithCallOccupancy()`.

**Validation**: Harness occupancy tests (Cursor last-usage vs result sum,
Claude last-assistant vs result sum, Codex inclusive cache), TUI billed-
aggregate fallback tests, `go test` on harness/adapter/tui packages.

## 2026-09-10 — Exclude `~/.workflow-hero` from project root discovery

**Problem**: After `hero chat` (and later Telegram plugins) created
`~/.workflow-hero/`, `FindProjectRoot` walked up from any cwd under `$HOME`
and treated the home directory as an installed project. Plain `hero` /
`hero tui` then opened the full project TUI without a local `hero install`.

**Change**: `cycle.FindProjectRoot` skips a match when the candidate root is
the OS user home directory (global free-chat/plugin state only). `hero chat`
still uses `OpenFreeChatService` against `~/.workflow-hero` unchanged.
Installed projects under `$HOME` continue to resolve via walk-up.

**Validation**: New `TestFindProjectRoot_SkipsUserHomeWorkflowHero` and
`TestFindProjectRoot_FindsInstalledProjectUnderHome`; `go test ./...`.

## 2026-09-10 — Harness connection closed auto-reconnect

**Problem**: When a harness transport dropped mid-turn (notably Codex
`app-server connection closed`), the TUI only saw a failed `executeDoneMsg`.
OpenCode already recovered inside Execute; Codex/Cursor did not, and the
health watchdog could cancel during recovery.

**Change**: Added shared `harness.ErrConnectionClosed` + lifecycle stream
deltas (`connection.closed` / `connection.reconnected`). Codex Execute now
detects RPC close, restarts app-server, `thread/resume`s the session, and
continues with a bounded continuation prompt. OpenCode emits the shared
deltas on SSE reconnect. Cursor retries `IsTransportFailure` with the same
SessionID. TUI keeps reconnect warnings visible, sets `harnessReconnecting`,
and skips HealthFailed auto-cancel while reconnecting; Codex CheckHealth
reports reconnecting as degraded.

**Validation**: `go test ./...` passes, including
`TestExecute_ReconnectsAfterConnectionClosed` and transport-failure unit tests.

## 2026-09-10 — Telegram `/tail` command

**Problem**: No remote way to inspect the tail of the agent's most recent chat
response from Telegram.

**Change**: Added the Telegram-only `/tail [n]` command (`internal/tui/telegram_tail.go`).
It returns the last n lines of the most recent non-empty `convRoleAgent`
transcript message, defaulting to 10 and capped at 100 (`n` in `[1, 100]`);
trailing newlines are not counted as empty lines and shorter responses are
returned whole. Wired into `handleTelegramInbound` alongside `/status`/`/interrupt`
so it never starts a harness turn, and documented in `CommandHelpText`.

**Validation**: `go test ./...` passes; new `parseTelegramTail` table tests plus
`telegramTailText` and inbound-handler coverage in `telegram_tail_test.go`.

## 2026-09-10 — Claude enable now provisions root CLAUDE.md context

**Problem**: Enabling the Claude harness provisioned `.claude/` but never created
the root `CLAUDE.md`, because `ProvisionClaude` only writes the projection and
`ApplyClaudeContext` was dead code — only tests called it. The enable flow
(`EnableHarnessWithProjection`, install `Run`, and the `/hero-harness` picker)
never surfaced the managed-context decision required by `claude-projection`
spec / ADR-073.

**Change**: Added `install.EnableClaudeHarness` (enable + provision + context),
an `Options.ClaudeContext` decision consumed by `Run` (warn-only on missing
`AGENTS.md`/malformed markers), a huh confirm in `hero install` when Claude is
selected, and a follow-up TUI palette (`Claude · managed context`) after enabling
Claude via `/hero-harness` with `insert/update` and `leave unchanged` choices
(`actionClaudeContextInsert`/`Leave`). Esc on the decision = leave unchanged.

**Validation**: `go test ./...` passes; new coverage for `EnableClaudeHarness`
(create + leave-unchanged), install-time `ClaudeContext` (create + no-create),
and the TUI enable→decision→create/leave flows. Also strengthened the managed
block to explicitly name `context/current-state.md` and `context/context-log.md`
alongside the existing `@AGENTS.md` import and `.claude/skills/workflow-hero/`
skill reference (locked by a test assertion).

## 2026-09-10 — Coupled Telegram plugin auto-update

**Problem**: The development updater built and installed only Hero, leaving an
already-installed Telegram daemon on the previous revision. That violated the
current contract that the optional plugin, once enabled, follows the Hero
revision.

**Change**: `build_update.sh` now builds both host-targeted executables.
`hero-update.sh` detects an existing Telegram plugin without auto-installing
one, stages and atomically replaces its daemon and manifest with Hero, rolls
back on a pre-commit failure, stops only the daemon process captured before the
swap, and then restarts TUIs. Tests cover the coupled install, manifest version,
old-daemon shutdown, optional-plugin preservation, and temporary-file cleanup.

**Validation**: `go test ./... -count=1`, targeted race tests, `go vet ./...`,
shell syntax checks, `git diff --check`, and a host-targeted build of both
artifacts completed successfully; generated validation binaries were removed
from `./temp`.

## 2026-09-10 — Resilient development auto-update restart

**Problem**: The updater installed the new Hero binary but the TUI did not
restart. `hero-update.sh` waited indefinitely for an ACK from a legacy Telegram
daemon that silently ignored the restart frame; the TUI and daemon also kept
running from deleted old binary inodes after replacement.

**Change**: Made direct `/proc`-matched `SIGUSR2` the authoritative TUI restart
path, with a five-second, context-aware IPC compatibility fallback. Current
IPC registrations now expose additive daemon version/capability metadata and
unknown message types return an explicit error. Added daemon pidfile creation
with ownership-safe cleanup, Linux recovery of legacy deleted-inode daemon
processes, idempotent TUI client close, atomic daemon/manifest writes, and
daemon stop during plugin install/uninstall and local `build_dev.sh`/`release.sh`
installation. Successful installs clear the update flag even when notification
is unavailable; the systemd service has a five-minute execution limit.

**Validation**: Focused updater, IPC, daemon, plugin, TUI, and script tests;
shell syntax checks; and `gofmt` passed. The full `go test ./...`, race, vet,
diff, and final documentation checks are run before handoff.

## 2026-09-10 — Test binary artifact isolation

**Change**: Added `/hero-telegram-daemon` and `/temp/` to `.gitignore`. `docs/testing/TESTING.md` now prohibits repository-root binaries, requires temporary binaries from tests, validation, and local build checks to use `./temp/`, and requires cleanup on both success and failure. Updated local build examples in `README.md`, `docs/deployment/DEPLOY.md`, and the embedded user help to use `./temp/hero`; added `TESTING.md` to the bilingual README documentation maps.

**Document hierarchy**: `docs/testing/TESTING.md` remains a living, unnumbered document and is present in the active `.workflow-hero/config/documents.json` registry and the generated `openspec/config.yml` context, alongside the existing `AGENTS.md`, `ADR.md`, and architecture-overview references. The empty `assets/config/documents.json` remains the bootstrap template.

**Validation**: `git diff --check`, registry JSON parsing, reference assertions, and the isolated OpenCode usage test passed. The full `go test ./... -count=1` run still reproduces the unrelated `internal/adapters/opencode/TestExtractOpenCodeUsageAccumulatesStepFinishes` failure (`context=13`, want `24`); no root executable or `./temp` artifact remained.

## 2026-09-10 — Stabilized OpenCode usage test

**Change**: Replaced the unordered `map` fixture in `TestExtractOpenCodeUsageAccumulatesStepFinishes` with an ordered slice so the `step-finish` event with `20 + 4` tokens is deterministically last. This matches the adapter contract: billed input/output accumulate, while `ContextTokens` represents the last model-call occupancy.

**Root cause**: Map iteration order made the test flaky. When the `20 + 4` event was processed first, the later `10 + 3` event correctly left `ContextTokens=13`, but the test always expected `24`.

**Validation**: The focused test passed 30 consecutive runs and `go test ./...` passed after the change.

## 2026-09-09 — Development Telegram auto-update

**Change**: Added ADR-076 and the development-only `/auto-update` flow. The TUI command commits safe working-tree changes and sets `needs-update.txt`; a systemd user timer runs `hero-update.sh` every five minutes. The initial updater invoked `scripts/build_update.sh` for only `./cmd/hero`; this historical behavior was superseded by the coupled Hero/Telegram artifact update recorded above.

**Validation**: `go test ./...`, `go vet ./...`, shell syntax checks, `git diff --check`, and a real host-target `build_update.sh` build passed.

## 2026-09-10 — Development auto-update uninstaller

**Change**: Added `scripts/uninstall_update_dev.sh`. It stops/disables the systemd user timer, reloads the user manager, removes the updater service/timer, helper, state, lock, and timer-wants link, and deliberately preserves the installed `hero` binary and `hero.previous` backup.

**Validation**: Added a temp-directory contract test covering the cleanup scope and preserving the Hero binary/backup; shell syntax and the full Go test suite pass.

## 2026-09-09 — Claude stream-json event mapping

**Change**: The Claude NDJSON normalizer now maps official Agent SDK / CLI `stream-json` types that previously fell through as unknown. User-visible work and errors emit always: tool progress, task lifecycle, hooks/`api_retry`, local command output, informational/permission-denied/worker/auth/rate-limit/result errors, extra assistant content blocks, and control-protocol permission/elicitation warnings. Protocol noise (`stream_event`, compact/plugin/session catalog frames, thinking-token estimates, keep-alives, control ACKs, prompt suggestions, memory/notification) stays `hero --debug` only, matching Codex/OpenCode. `control_request can_use_tool` is a warning, not a TUI permission gate, because Hero answers ask via the MCP bridge rather than stdin `control_response`.

**Validation**: `go test ./internal/adapters/claude` and `go test ./...` passed.

## 2026-09-09 — C13 archived via /hero-archive

**Change**: `hero cycle archive` archived completed C13. OpenSpec change `claude-code-adapter` was already archived (`openspec/changes/archive/2026-09-09-claude-code-adapter`); CLI skipped `openspec archive`. Hero path: `.workflow-hero/cycles/archive/C13-2026-09-09-implementa-o-do-adapter-para-o-claude-co` (date from store `completed_at`). `metrics-summary.md` already had C13 totals (5006554 tokens, ~$0.7840). No active cycle remains.

**Validation**: `hero status --json` before archive showed C13 `completed` with `openspec_change: claude-code-adapter`. After archive, `hero metrics` reports no active cycle. Resume with `/hero-resume` C13.

## 2026-09-09 — Completed cycles remain archiveable

**Problem**: `/hero-finish` correctly changed C13 to `completed`, but `cycle.Service.Status()` and the TUI archive precondition only looked for `active`, so `/hero-status` appeared empty and `/hero-archive` was rejected before reaching `hero cycle archive`.

**Change**: Added the read-only `GetCurrentCycle()` lookup for active or latest completed cycles. Status now exposes a completed cycle until archive; active-only lifecycle mutations and the TUI Config screen remain protected. `/hero-archive` has its own precondition that accepts both `active` and `completed`, and all four Runtime projections document the finish → archive transition.

**Validation**: `go test ./...`; `go run ./cmd/hero status --json` reports C13 with `status: completed` and its stage rows. C13 was not archived during this correction.

## 2026-09-09 — C13 finished via /hero-finish

**Problem**: Implementation 9/9 was still Running after the TUI completion gate refused the `generic_agent` report (unassigned `task-06.3-projection-lifecycle` on a fully checked `tasks.md`). QA and Judge were Waiting after the last Judge loop-back. The user issued `/hero-finish`.

**Change**: `hero finish` with implementation-wave metrics (`gpt-5.6-terra`, 19500 in / 4500 out tokens, ~$0.093, 900000 ms). Did not close Implementation/QA/Judge as completed stages. Recorded cycle `completed_at` for archive dating. Updated `current-state.md` and `metrics-summary.md`. OpenSpec change `claude-code-adapter` remains linked until `/hero-archive`.

**Validation**: `hero status` — no active cycle. Adapter work including `PrepareHeroStart` / `SyncAgentDefinition` remains on disk.

## 2026-09-09 — C13 Implementation gate refused: unassigned task ID

**Problem**: Implementation 9/9 stayed Running. TUI gate: reports invalid or missing required gate fields. `generic_agent` returned `status: complete` with gates true, but `tasks_completed: ["task-06.3-projection-lifecycle"]`. All OpenSpec checkboxes are already `[x]`, so the wave assignment was empty (verification-only). Claiming an unassigned ID fails closed.

**Change**: Did not close or start another stage. PrepareHeroStart / SyncAgentDefinition and TUI Claude prepare wiring are on disk. Next `/hero-start` wave must report empty `tasks_completed` / `tasks_remaining` to match the empty assignment, or reopen an unchecked task if more coding remains.

**Validation**: `hero status` — Implementation Running 9/9; QA Waiting 7/7; Judge Waiting 5/5.

## 2026-09-09 — C13 Implementation /hero-continue: extra iteration granted, stage started

**Change**: `/hero-continue` defaulted to `--extra 1` while Implementation was Escalated 8/8 (`iteration_budget`). `hero continue --extra 1` then `hero stage start --name implementation` (iteration 9/9 Running). Did not dispatch `generic_agent` (TUI handoff). Remaining gap: Claude `/hero-start` Prepare (see judge-gaps.md).

**Validation**: `hero status` — Implementation Running 9/9; QA Waiting 7/7; Judge Waiting 5/5.

## 2026-09-09 — C13 Judge failed; Implementation escalated (iteration_budget)

**Problem**: Judge iter 5/5 found 1 remaining SDD gap: Claude `/hero-start` Prepare does not sync managed model/effort/skill fields in `.claude/agents/<agent>.md` and does not fail closed on invalid CLI or marked-agent preparation. OpenCode/Codex Prepare is wired; Claude is not. Prior iteration-4 remainders are landed. `sdd_ambiguity: false`.

**Change**: Closed Judge `--failed` with metrics. `hero stage loop-back --from judge`. `hero stage start --name implementation` escalated (`iteration_budget` 8/8). Did not dispatch stage agents. Waiting for `/hero-continue` in the Hero TUI.

**Validation**: `hero status` — Implementation Escalated 8/8; QA Waiting 7/7; Judge Waiting 5/5. Artifact: `.workflow-hero/cycles/current/judge-gaps.md`. Metrics: `cursor-grok-4.6-high`, 31250 in / 1550 out tokens, ~$0.0718, 426000 ms.

## 2026-09-09 — C13 QA 7/7 closed without JSON; Judge started

**Problem**: TUI `qa_agent` (`opencode-go/deepseek-v4-pro`) returned preamble only (TESTING.md / parallel build+lint / cached `go test ./...` green / intended fresh+logging review) with no JSON Output Format (`tests_passed` unset). QA was Running 7/7 so close was allowed.

**Change**: Closed QA as pass from the explicit cached-pass signal (picker 4-harness gap already fixed). Did not re-dispatch agents. `hero stage start --name judge` for the next TUI wave (Waiting 4/5). Metrics estimate: model `opencode-go/deepseek-v4-pro`, 6250 in / 106 out tokens, ~$0.004335, 90000 ms.

**Validation**: `hero status` after close+start — QA Completed Auto; Judge Running.

## 2026-09-09 — Harness session isolation (schema v10)

**Problem**: During a mixed-harness cycle, QA on OpenCode emitted `ses_f810…`.
`persistHarnessSession` overwrote `orchestrationSessionID` because
`orchestrationLive` was still true. Resume copied that id into Cursor
`--resume`, which requires a UUID. Cancel then used the global session against
the wrong adapter (`no in-flight execution`).

**Change**: SQLite schema v10 adds `cycles.orchestration_session_id` and
`cycles.orchestration_harness_id` as an atomic pair. Stage rows remain the
named stage-agent session only. TUI persist updates the orchestrator slot only
for `orchestration_agent`. Resume is fail-closed when the owner is missing or
mismatched; `bindSessionToRuntimeHarness` no longer relabels. Cursor Execute
rejects non-UUID resume ids before launching.

**Validation**: store v9→v10 migration, TUI QA-stream vs orch resume, cancel
session identity, Cursor foreign-id guard; `go test ./...`.

## 2026-09-09 — Telegram address-token validation

**Problem**: Unprefixed Telegram messages that contained `:` (e.g. pasted errors
starting with `…agente: aiwkhero: …`) were misparsed as addressed inbound. The
daemon replied `Unknown address…` even when `/select` pointed at a live
instance; prefixing `aiwkhero:` made the same text deliver correctly.

**Change**: `parseAddressed` now accepts only leading tokens matching the
allocated abbrev charset (`[a-z0-9][a-z0-9_-]*`). Invalid left sides fall
through to `/select` routing with the full original text. Updated UI-C09,
ADR-063, architecture overview, and current-state.

**Validation**: router + daemon regression tests for the reported prose;
`go test ./internal/telegram/...` and `go test ./...`.

## 2026-09-09 — Telegram /help command catalog

**Change**: Added daemon-owned `/help` that returns the shared
`internal/telegram.CommandHelpText` catalog (routing, control, cycle-config,
permission, queue, and Hero slash commands). It works without `/select`, does
not forward a harness turn, and is also intercepted for addressed
`<addr>: /help`. TUI keeps a matching handler as defense in depth. Docs updated
in PRD/UI/ADR-C09, architecture overview, README, and workflow-help.

**Validation**: package + daemon + TUI help tests; `go test ./...`.

## 2026-09-09 — Telegram /interrupt and /kill

**Decision**: Keep existing `/interrupt` as the remote equivalent of Chat
`Ctrl+C`. Add `/kill` as a last-resort force exit of the selected TUI only.

**Change**: `/kill` is intercepted on the Telegram IPC client goroutine before
`Program.Send`, best-effort acks delivery, sends `Killing TUI.`, then
`SIGKILL`s the TUI process (injectable in tests). Update-path handling remains
as defense in depth. Documented in PRD/UI/ADR-C09, architecture overview,
README (EN/PT), and `workflow-help.md`. Architecture boundaries unchanged.

**Validation**: `TestIsTelegramKillCommand`, inbound force-kill test, client
ack/outbound-before-kill test; full `go test ./...` after implementation.

## 2026-09-08 — Telegram /hero-config keep-first options

**Problem**: The Telegram cycle-config wizard exposed "manter configuração atual"
at inconsistent numbered positions (e.g. approval at 3, models at 2, scope/stages
as free text only).

**Change**: Every numbered wizard step that offers keep-current now lists it as
option **1** — scope, stages, stage approval, models question, parent model,
and subagent choice. Scope/stage selections shift to items 2+; stage parsing now
matches the visible stage list. Added `telegramConfigKeepFirst` and
approval-specific yes/no helpers.

**Validation**: Updated wizard tests plus
`TestTelegramConfigKeepOptionIsAlwaysFirst`; `go test ./...` passed.

## 2026-09-08 — Telegram status flood

**Problem**: During a long cycle, Telegram started sending Hero status in a
tight loop (DoS-like). The 2026-09-06 idle/duplicate-timer fix did not cover
the two remaining senders of `telegramAutoReportText`.

**Change**: Remote text that arrives while Execute is live is stored in
`telegramPendingTurns` (deduped by text+origin) and gets **one** immediate
status. The 500ms inbound Tick retry is gone; the queue drains one turn after
`executeDone` / cancel when the TUI is no longer streaming. Auto-report now
schedules with wall-clock `max(at, now)` plus `lastAutoReportAt`, so a stale
1s tick cannot keep `nextAutoReportAt` in the past. `handleTimerTick` rejects
generation mismatches including generation 0; unused `statusTickCmd` was
removed.

**Validation**: TUI tests for queued-turn once-only status, drain after
Execute, stale-tick / burst / generation-0 auto-report, and existing idle
manual `/status`. `go test ./...` passed.

## 2026-09-08 — Context window occupancy vs billed tokens

**Problem**: The Chat context bar and Telegram `Context` treated billed
`input+output` as window fill. Cursor/Claude omit cache from `input`, OpenCode
sums every tool-loop step's full prompt, missing usage fell back to the last
prompt only, and ordinary freechat during an active cycle could resume the
stage-agent session. Earlier fixes oscillated between summing turns (too high)
and replacing with the latest billed turn (too low).

**Change**: `harness.Usage` now carries cache fields and `ContextTokens`
(occupancy after this turn). Adapters fill occupancy from the last model call,
including cache; OpenCode keeps billed step sums but occupancy is the last
step. The TUI stores occupancy per freechat vs cycle-agent session and assigns
it, never summing billed turns. Freechat has its own in-memory session id and
does not write cycle metrics or stage session ids. When usage is absent,
occupancy is chars÷4 of that session's transcript. Cycle Costs remain the
billed accumulator.

**Validation**: Adapter occupancy tests (Cursor/Claude cache, OpenCode last
step, Codex `last` not `total`), TUI session-isolation and transcript-estimate
tests, `go test ./internal/tui` and adapter packages.

## 2026-09-08 — Multi-agent Implementation ownership and completion contract

**Change**: Standardized implementation ownership across Cursor, Codex,
OpenCode, and Claude. Every task line carries exactly one canonical owner
(`backend_agent`, `frontend_agent`, or `generic_agent`). Ownerless legacy work
is routable only with exactly one active implementation agent; missing or invalid
ownership fails closed when multiple agents are active. The scheduler prepares
per-agent prompts with the linked tasks file, exact IDs, dependencies,
acceptance criteria, and verification commands rather than dispatching a global
checklist. Claude prompt routing resolves the projected `.claude/agents/` path.

**Completion contract**: Reports identify the expected stage and agent, and
their `tasks_completed`/`tasks_remaining` arrays must form the disjoint union of
that agent's assigned IDs. The TUI/runtime scheduler is the only checkbox writer
and performs the validated update atomically. Subsequent waves are selective:
only agents with remaining assigned IDs are re-dispatched; an empty wave is not
launched after progress clears the checklist. If Implementation starts with no
pending tasks, one verification wave may still collect the required reports and
gates. Assignment audits retain normalized ordered `task_ids` in the existing
conversation JSON. Cancellation removes executions from the accepted set and
stops relays before asynchronous cancellation, so late completion messages
cannot mutate stage state.

**Validation**: `go test ./...`, `go vet ./...`,
`go test -race ./internal/tui ./internal/cycle -count=1 -timeout=240s`,
`openspec validate claude-code-adapter --strict`, and `git diff --check` passed.

## 2026-09-08 — C13 QA /hero-continue: extra iteration granted, stage started

**Change**: `/hero-continue` defaulted to `--extra 1` while QA was Escalated 5/5 (`iteration_budget`). `hero continue --extra 1` then `hero stage start --name qa` (iteration 6/6 Running). Did not dispatch `qa_agent` (TUI handoff). Judge remains Escalated 4/4.

**Validation**: `hero status` — QA Running 6/6; Judge Escalated 4/4; Implementation Completed 6/6 Auto.

## 2026-09-08 — C13 QA TUI return incomplete; QA and Judge Escalated (iteration_budget)

**Problem**: After Implementation 6/6 closed, QA was Escalated 5/5 (`iteration_budget`). The TUI still ran `qa_agent` (`opencode-go/deepseek-v4-pro`). Returned text was preamble only ("I'll start the QA validation…") with no JSON Output Format (`tests_passed` unset). `hero stage close --name qa` failed (`Escalated, expected Running`). `hero stage start --name qa` failed (`run continue/cancel/finish first`). `hero stage start --name judge` then escalated Judge (`iteration_budget` 4/4).

**Change**: Did not auto-grant iterations, did not close QA as pass/fail, and did not dispatch stage agents. Waiting for `/hero-continue` in the Hero TUI. Estimated metrics (not persisted): model `opencode-go/deepseek-v4-pro`, ~25000 in / 25 out tokens, ~$0.01655, ~90000 ms.

**Validation**: `hero status` — Implementation Completed 6/6 Auto; QA Escalated 5/5; Judge Escalated 4/4.

## 2026-09-08 — C13 Judge failed; loop-back blocked on Implementation iteration_budget

**Problem**: Judge (`cursor-grok-4.6-high`) found 5 remaining SDD gaps (disconnected production ask stdio/MCP, adapter foreign `--resume`, TUI NativeModel unused, Doctor unsupported-version copy / Status permissionPaused, four-harness acceptance). Loop-back from Escalated Judge 3/3 was refused.

**Change**: `/hero-continue --extra 1` moved Judge to Waiting 3/4. Started Judge iter 4 and closed `--failed` with metrics (no re-dispatch). Loop-back reopened Implementation/QA/Judge. `hero stage start --name implementation` escalated (`iteration_budget` 5/5). Waiting for `/hero-continue` in the Hero TUI.

**Validation**: `hero status` — Implementation Escalated 5/5; QA Waiting 5/5; Judge Waiting 4/4. Artifact: `.workflow-hero/cycles/current/judge-gaps.md`.

## 2026-09-08 — C13 QA 5/5 closed; Judge escalated (iteration_budget)

**Problem**: TUI `qa_agent` (`opencode-go/deepseek-v4-pro`) returned preamble only (TESTING.md / suite / cached pass / intended fresh+race re-run) with no JSON Output Format. QA was Running 5/5 so close was allowed.

**Change**: Closed QA as pass from the explicit cached-pass signal (picker 4-harness gap already fixed in Implementation 4). Did not re-dispatch agents. `hero stage start --name judge` escalated (`iteration_budget` 3/3). Waiting for `/hero-continue` in the Hero TUI.

**Validation**: `hero status` — QA Completed 5/5 Auto; Judge Escalated 3/3; Implementation Completed 5/5 Auto.

## 2026-09-08 — C13 QA /hero-continue: extra iteration granted, stage started

**Change**: `/hero-continue` defaulted to `--extra 1` while QA was Escalated 4/4 (`iteration_budget`). `hero continue --extra 1` then `hero stage start --name qa` (iteration 5/5 Running). Did not dispatch `qa_agent` (TUI handoff).

**Validation**: `hero status` — QA Running 5/5; Judge Waiting 3/3; Implementation Completed 5/5 Auto.

## 2026-09-08 — C13 QA TUI handoff incomplete while Escalated (iteration_budget)

**Problem**: After Implementation 5/5, QA remained Escalated 4/4 (`iteration_budget`). The TUI still ran `qa_agent` (`opencode-go/deepseek-v4-pro`). The returned text was preamble only (started reading TESTING.md / current-state; mentioned build/vet) with no JSON Output Format (`tests_passed` unset). `hero stage close --name qa` failed (`Escalated, expected Running`). `hero stage start --name qa` failed (`run continue/cancel/finish first`).

**Change**: Did not auto-grant iterations, did not close QA as pass/fail, and did not start Judge. Waiting for `/hero-continue` in the Hero TUI. Estimated metrics (not persisted): model `opencode-go/deepseek-v4-pro`, ~25000 in / 70 out tokens, ~$0.0166, ~90000 ms.

**Validation**: `hero status` — Implementation Completed 5/5 Auto; QA Escalated 4/4; Judge Waiting 3/3.

## 2026-09-08 — C13 Implementation closed after Judge loop-back (iteration_budget)

**Problem**: Judge iter 3 failed and looped back; Implementation was Escalated 4/4 (`iteration_budget`). The TUI still ran `generic_agent`. `hero stage close` requires Running.

**Change**: `hero continue --extra 1` (Implementation Waiting). Started Implementation iter 5, closed with the passing TUI report (no re-dispatch). Metrics estimate: `gpt-5.6-terra`, 52500/18750 tokens, $0.33, 1464000 ms. Next `hero stage start --name qa` failed (`iteration_budget` 4/4) and left QA Escalated. Did not dispatch stage agents (TUI handoff). Waiting for `/hero-continue` in the Hero TUI.

**Validation**: `hero status` — Implementation Completed 5/5 Auto; QA Escalated 4/4; Judge Waiting 3/3.

## 2026-09-08 — C13 implementation loop-back: Claude execution, projection, diagnostics, and catalog

**Change**: Completed the C13 loop-back slices: execution-scoped Claude ask bridge wiring with one-time-token environment injection and callback forwarding; Claude model aliases, dated native IDs, 1M selectors, unknown pricing, and provider-scoped C5 lookup; opt-in embedded `.claude/` projection and preserved marker-delimited `CLAUDE.md`; lifecycle integration; fourth-harness labels/reset exclusion; configured-marker detection; and Claude Doctor/Status diagnostics.

**Safety**: The bridge accepts only the permission callback contract and always cleans up with the turn. No credentials or token values are logged. Claude remains disabled until explicitly enabled; projection removal preserves non-Hero user files and unmarked root instructions.

**Validation**: `go test ./...`, `go test ./internal/tui`, `go vet ./...`, `openspec validate claude-code-adapter --strict`, and `git diff --check` passed.

## 2026-09-08 — C13 QA closed after loop-back, Judge started

**Problem**: After Implementation 4/4, QA was Escalated (iteration_budget 3/3). The TUI still ran `qa_agent`; `hero stage close` requires Running.

**Change**: `hero continue --extra 1` (QA Waiting). Started QA iter 4, closed with the passing TUI report (no re-dispatch). Metrics: `opencode-go/deepseek-v4-pro`, 1600/525 tokens, $0.002096, 284000 ms. Auto-advanced; `hero stage start --name judge` (Running 3/3). Did not dispatch stage agents (TUI handoff).

**Validation**: `hero status` — QA Completed 4/4 Auto; Judge Running 3/3. Full `go test ./...` green; picker test uses `len(install.SupportedHarnessIDs)`. Pre-existing flake in `TestConversationCancelDuringStreamWithoutSessionID` noted, not a C13 regression.

## 2026-09-08 — C13 implementation iteration 4: picker registry assertion

**Change**: Updated `TestHarnessPickerPersistsAutoProjectPermissionProfileInline`
to compare its rendered `Permissions:` headings against
`len(install.SupportedHarnessIDs)`, rather than a stale fixed count of three.
The picker test now remains correct as supported harnesses are added, including
the C13 Claude entry.

**Validation**: Focused picker test and `go test ./...` passed.

## 2026-09-08 — C13 QA iteration 3 failed: loop-back to Implementation

**Problem**: After `/hero-continue`, QA ran as iteration 3/3 (`qa_agent`, `opencode-go/deepseek-v4-pro`). Build, vet, gofmt, and logging passed. One test failed: `internal/tui` `TestHarnessPickerPersistsAutoProjectPermissionProfileInline` still expects exactly 3 `Permissions:` headings; C13 added `claude` as a 4th supported harness.

**Change**: Closed QA as Failed with metrics. Wrote `.workflow-hero/cycles/current/qa-gaps.md`. `hero stage loop-back --from qa`, then `hero stage start --name implementation` (iteration 4/4). Did not dispatch stage agents (TUI handoff).

**Validation**: `hero status` — Implementation Running 4/4; QA Waiting (iteration kept at 3). Next QA `stage start` may escalate on iteration budget.

## 2026-09-08 — C13 QA escalated after Implementation iter 3 (iteration_budget)

**Problem**: After Implementation iter 3 closed, the engine escalated QA (`iteration_budget`) instead of `stage start` (QA already used 2/2). The TUI still launched `qa_agent` (`opencode-go/deepseek-v4-pro`). OpenCode serve restarted mid-turn; the agent output stopped after build/vet pass with no JSON verdict (`tests_passed` unset). `hero stage close --name qa` failed (`Escalated, expected Running`). `hero stage start --name qa` failed (`run continue/cancel/finish first`).

**Change**: Did not close QA and did not start Judge. Waiting for `/hero-continue` in the Hero TUI. Estimated metrics (not persisted): model `opencode-go/deepseek-v4-pro`, ~25000 in / 85 out tokens, ~$0.0167, ~32000 ms.

**Validation**: `hero status` — QA Escalated 2/2; Judge Waiting 2/3; Implementation Completed 3/4.

## 2026-09-07 — Context window usage correction

**Problem**: The context bar and Telegram `Context` accumulated normalized
`input+output` from every completed Execute. When a turn's input already
included prior conversation history, that history was counted again and the
window appeared to fill too quickly. Telegram idle status also ignored the
selected-model window passed to its formatter.

**Change**: The TUI now keeps the latest completed Execute's normalized
`input+output` as the current-context approximation; cycle metrics remain
cumulative independently. Telegram status now uses the explicit model window
provided by each status path, including the selected free-chat model for idle.

**Validation**: Context/Telegram regression tests, full `go test ./...`,
`go vet ./...`, and `git diff --check` pass.

## 2026-09-07 — Telegram remote interruption

**Requirement**: Add `/interrupt` as the Telegram equivalent of pressing `Esc`
in Chat, cancelling the process currently running in the TUI.

**Change**: `/interrupt` is intercepted before Telegram wizards and shared
slash dispatch. It reuses the TUI cancellation command for active Executes,
including concurrent executions, and cancels `/hero-start` preflight without
creating a harness turn. Idle requests receive a concise no-process response.

**Validation**: Focused Telegram interruption and existing stream-cancellation
tests, `go test ./...`, `go vet ./...`, and `git diff --check` pass.

## 2026-09-07 — Telegram status payload refinements

**Requirement**: A manual Telegram idle status must include the selected Chat
model, current context usage/window size, and the Session/AI wk/AI rp counters.
Cycle status must identify the cycle by title without sending its objective
summary.

**Change**: Idle status now renders `Model`, `Session`, `AI wk`, `AI rp`, and
`Context: used/max`; the context usage remains visible even when the catalog
does not know the maximum. Cycle status no longer emits `Objective`, while its
title, state, current stage, active agents, and counters remain available.

**Validation**: Focused Telegram status tests, `go test ./...`, `go vet ./...`,
and `git diff --check` pass.

## 2026-09-07 — Telegram cycle-config approval and subagent defaults

**Problem**: `/hero-config` skipped the `require_human_approval` choices for stages. A new parent agent/model selection could also leave a previously dedicated subagent mode active, and the remote selector needed to preserve the parent harness invariant.

**Change**: Added a sequential yes/no/keep prompt for every enabled stage; disabled stages retain their approval value. A parent model selection now sets `same_of_agent: true`, so the subagent inherits the selected parent model until the user explicitly chooses a dedicated model. Dedicated subagent model selection remains restricted and validated against the parent harness.

**Validation**: Telegram config regression tests cover approval prompts, disabled-stage preservation, parent-model reset, nested property selection, and same-harness enforcement. Focused tests pass; full `go test ./...` is the remaining release check.

## 2026-09-07 — Telegram cycle-config scope and subagent correction

**Problem**: The Telegram cycle model review handled only top-level agent pairs, and its summary enumerated stale `agents.*` blocks even when their stage or implementation scope was disabled. This made nested subagent settings invisible and could show an out-of-scope `backend_agent`.

**Change**: The review queue now comes from `ManagedConfig.RequiredAgentNames()`, followed by an explicit subagent question for every active named agent. The subagent flow supports keeping the current mode, reusing the parent model, or choosing a dedicated model; dedicated selection is constrained to the parent harness and writes only the cycle draft. The summary uses the same active-agent projection, so stale out-of-scope blocks are hidden. Added regression coverage for queue filtering, summary filtering, state transitions, parent-harness restriction, nested properties, and `hero.json` isolation.

**Reset and validation**: Cancelled the active cycle with `hero cancel` so the user can retest from a new `/hero-new`; the current YAML and cycle artifacts were not deleted. `go test ./...`, `go vet ./...`, and focused Telegram tests pass.

## 2026-09-07 — Release Hero v3.0.7

**Change**: Incremented the patch version for the Telegram cycle-config subagent review and active-stage/scope filtering fix. Release artifacts include the regression coverage and updated product/architecture context.

**Validation**: `go test ./...`, `go vet ./...`, and `git diff --check` pass before tagging.

## 2026-09-06 — Telegram status lists active agents and models

**Change**: Extended TUI-owned Telegram `/status` and automatic reports to include an `Agents` block during active turns. Each row reports the operating agent name and model from the live execution state; the unnamed Free Chat parent is normalized to the stable name `harness`. Idle status remains the compact `idle` response.

**Validation**: Focused Telegram status tests and full `go test ./...` pass; `go vet ./...` and `git diff --check` also pass.

## 2026-09-06 — Release Hero v3.0.4

**Change**: Incremented the patch version from `v3.0.3` to `v3.0.4` for the Telegram active-agent/model status enhancement. Release artifacts include the Hero CLI and platform-matched Telegram daemon binaries.

**Validation**: Release gate `go test ./...` and `go vet ./...` pass before tagging.

## 2026-09-06 — Telegram cycle configuration wizard

**Change**: Added Telegram-only `/hero-config`, `/hero-config-show`, and `/hero-config cancel` handling in the TUI edge. `/hero-config` loads the active `workflow-config.yml` asynchronously, guides title/objective/language/scope/stages, optionally reviews the required cycle-agent and fallback models, and keeps all answers in an address-scoped draft until explicit save. Cycle-agent model choices reuse the existing numbered Telegram `/model` selector and its C5 properties, while free-chat `hero.json` remains untouched. `/hero-config-show` renders a compact non-secret canonical/draft summary, and `/hero-new` replies now advertise the setup commands.

**Persistence**: Extracted the Config screen's atomic YAML write, enabled-harness validation, cycle synchronization, and retry-diff calculation into a shared TUI helper used by local Config and Telegram. Cancel and pre-write validation failures leave the workflow file unchanged.

**Validation**: `go test ./internal/tui -count=1` and `go test ./...` pass, including wizard routing, canonical show, model-draft isolation, and atomic save tests.

## 2026-09-05 — Telegram `/model` uses a numbered remote wizard

**Problem**: Telegram `/model` reused the local slash dispatcher and opened the TUI palette, leaving the remote user unable to choose a harness, model, or reasoning properties.

**Change**: Added address-scoped Telegram selection state in `internal/tui`. It sends numbered harness and model lists, then each selectable C5 property list, validates numeric replies, and atomically saves the resulting free-chat pair/properties without opening the local picker.

**Validation**: `go test ./internal/tui/...` pass.

## 2026-09-05 — Telegram inbound only after TUI restart

**Problem**: Replies reached Telegram, but inbound text still appeared in Chat only after quitting and reopening the TUI. Every TUI start spawned a new daemon that unlinked the live socket, so two processes polled `getUpdates`. The idle process stole updates and queued them; flush happened on the next register.

**Change**: Dial an existing daemon instead of always spawning. `Listen` refuses to steal a live socket. `getUpdates` runs only while a TUI is registered. Last-client shutdown waits 3s so a reconnect reuses the same poller.

**Validation**: `go test ./...` pass.

## 2026-09-05 — Telegram inbound stuck until TUI restart; replies still not sent

**Problem**: Live Telegram messages did not appear in Chat until the TUI was restarted (then the queued question arrived). The harness answer stayed in the TUI and was never sent back. Production `executeDone` is wrapped in `conversationBatchMsg`, which discarded the outbound `tea.Cmd`. Inbound after the first turn depended on re-issuing `waitTelegramMsg`, which did not survive that batch.

**Change**: `conversationBatchMsg` keeps nested cmds (Telegram `outbound` included). Launch relays daemon frames with `tea.Program.Send`. ACK/outbound IPC writes stay in cmds; `ipc.Conn.Send` is mutex-serialized; daemon Bot API send no longer blocks the IPC serve loop.

**Validation**: `go test ./...` pass.

## 2026-09-05 — Telegram Project ID not persisted

**Problem**: Changing Settings Project ID (e.g. `aiwkhero`) only updated in-memory TUI state. Boot always derived the abbreviation from the directory basename (`ai_workflow_hero`), so reopening Settings reset the field.

**Change**: Persist `telegram.project_abbrev` in `.workflow-hero/config/hero.json` on Enter. `startTelegram` loads the saved value when present, otherwise falls back to the directory name. Credentials remain vault-only (ADR-062).

**Validation**: `go test ./...` pass.

## 2026-09-05 — Telegram harness replies stayed in the TUI

**Problem**: Inbound Telegram text ran a harness turn and the transcript showed `→ [Telegram · addr]`, but the final agent output was never sent on IPC `outbound`. Lifecycle `Notifier` only covers cycle/stage/approval/error/final events, not conversation replies.

**Change**: On `executeDone` of a Telegram-originated turn (no remaining sibling Executes), send the harness `Output` (or error text) to the daemon as `outbound`, chunked under the Bot API size limit. Local composer turns are unchanged.

**Validation**: `go test ./...` pass.

## 2026-09-05 — README Telegram setup

**Change**: Added bilingual README sections **Telegram plugin** / **Plugin Telegram** (BotFather, `hero plugin install telegram`, TUI Retry/Pair, addressed messages). Also listed the plugin in Features and the post-install CLI commands.

**Validation**: README EN + PT-BR sections reviewed in place.

## 2026-09-05 — Telegram Settings stuck on Disconnected

**Problem**: Plugin installed but Settings showed `Daemon: Disconnected` with only `| Pair |`, which refused to pair. `startTelegram` wrote Connected/Disconnected into `telegramMsgCh`, but `Init` never issued `waitTelegramMsg`, so the TUI never learned the daemon was up. Pair gated on the stale `connected=false` flag.

**Change**: `Init` batches the telegram listener. The client spawns the daemon at start (with a 2s respawn cooldown, detached session). Disconnected Settings shows `| Retry |` plus recovery copy; Pair appears after Connected.

**Validation**: `go test ./...` pass.

## 2026-09-05 — Settings Pair button showed no pairing UI

**Problem**: Enter on Settings `| Pair |` did not start pairing or show instructions. The handler opened a token form (`pairState=token`) without sending `pair_start`, rendered a few dim lines instead of a full-screen dialog (alt-screen looked empty), and navbar/Tab stole keys before the modal. Disconnected Pair only appended a Chat transcript notice, invisible on Settings.

**Change**: Pair/Replace send `pair_start` immediately and open a centered instruction dialog (UI-C09-001 §2). The masked token field appears only on daemon `missing-token`. Pairing keys are handled before navbar/Tab. Disconnected/success/expiry feedback uses the Settings status bar.

**Validation**: `go test ./...` pass.

## 2026-09-05 — Release Hero v3.0.1

**Change**: Patch release — `hero plugin install telegram` downloads the platform-matched daemon from the matching Hero GitHub Release into `~/.workflow-hero/plugins/telegram/` (no local copy next to `hero` required).

**Validation**: `go test ./...` pass; `./scripts/release.sh`; GitHub Release `v3.0.1`.

## 2026-09-05 — Telegram plugin install downloads from GitHub Releases

**Change**: `hero plugin install telegram` now fetches the platform-matched `hero-telegram-daemon` from the Hero GitHub Release that matches the running binary version (`internal/plugin/release.go`), installs it under `~/.workflow-hero/plugins/telegram/`, and no longer requires a local copy next to the `hero` executable.

**Validation**: `go test ./internal/plugin/...` pass. Released as Hero **v3.0.1**.

## 2026-09-05 — Release Hero v3.0.0 / hero-telegram-daemon 0.1

**Change**: Cut `v3.0.0` — first Hero release shipping the optional Telegram plugin and platform-matched `hero-telegram-daemon` binaries (`scripts/release.sh` builds 4 OS/arch pairs + checksums). Includes C9 conversation service, IPC daemon, TUI Settings Telegram section, log rotation, and doctor/status plugin health.

**Validation**: `go test ./...` pass before tag; `./scripts/release.sh`; GitHub Release `v3.0.0` with 8 binaries + `checksums.txt`.

## 2026-09-05 — Settings TUI redesign

**Problem**: Settings rendered Chat verbosity and Telegram as one flat selectable list, so the plugin heading looked like another verbosity profile.

**Change**: Split the screen into `CHAT VERBOSITY` (navbar-style `>` on the applied profile and a full-width focus bar while navigating) and `TELEGRAM PLUGIN` (status badge, copyable `hero plugin install telegram`, `| Copy command |` or daemon/Project ID + piped action buttons). Focus order is only radios, copy command or Project ID, and Pair/Replace/Clear/Test. Enter applies the focused profile’s value (not the global cursor index). No in-TUI plugin install.

**Validation**: `go test ./...` pass.

## 2026-09-05 — C9 /hero-continue 2: QA closed, Judge started

**Problem**: QA was Escalated 2/2 after a passing TUI `qa_agent` run; `hero stage close` required Running.

**Change**: `hero continue --extra 2` (QA Waiting 2/4). Started QA iter 3, closed with the prior passing report (no re-dispatch), metrics persisted (`gpt-5.6-terra`, 31250/140 tokens, $0.06418, 240000 ms). Auto-advanced; `hero stage start --name judge` (Running 2/3). Did not dispatch stage agents (TUI handoff).

**Validation**: `hero status` — QA Completed 3/4 Auto; Judge Running 2/3.

## 2026-09-05 — TUI focus, resume, Judge loop-back, and transcript attribution

**Change**: Enter on the navbar now transfers focus to the chosen screen. `/hero-resume` performs the deterministic `cycle.Resume` transition asynchronously and immediately enters the existing `/hero-start` bootstrap. Judge prompts now require `hero stage loop-back --from judge` for implementation gaps, reopening completed Implementation and downstream QA before agents rerun. Stage-handoff labels are rendered as `Hero` system messages instead of user input.

**Validation**: focused navbar, resume, stage-handoff, and transcript tests pass; full suite pending.

## 2026-09-05 — Chat footer interruption hint

**Change**: Shortened the fixed Chat footer hint from `enter newline or command` to `enter newline` and added `esc interrupt chat`, making the existing stream-interrupt shortcut visible without increasing the footer footprint materially.

**Validation**: TUI footer and composer hint tests pass.

## 2026-09-05 — C9 Judge-gap completion: conversation boundary and E2E lock

**Change**: Routed every TUI harness Execute through `conversation.Service.SubmitWith` and a per-turn edge dispatcher, preserving Bubble Tea stream rendering while making the service the sole route to the adapter. Expanded the Telegram integration lock to use injected Bot API/vault/IPC for pairing, dual instance suffixes, addressed live routing, offline queue/reconnect, daemon-owned pending cancellation, and outbound notification prefixing.

**Validation**: focused conversation, TUI, and integration tests plus `go test ./...` pass. The `telegram-integration` OpenSpec link remains persisted on cycle C9.

## 2026-09-05 — C9 QA passed on TUI re-run; engine Escalated (2/2)

**Problem**: After Implementation iter 3 closed, the engine escalated QA (`iteration_budget`) instead of `stage start` (QA already used 2/2). The TUI still executed `qa_agent` (`gpt-5.6-terra`). Agent report: `tests_passed: true`, logging pass, lint `go vet` pass; staticcheck/golangci-lint incompatible with Go 1.26. Coverage: engine 78.5%, daemon 57.4%, IPC 64.1%, envhygiene 86.9%.

**Change**: Did not close QA (`hero stage close` requires Running) and did not start Judge. Waiting for `/hero-continue` in the Hero TUI. Estimated metrics (not persisted): model `gpt-5.6-terra`, 31250 in / 140 out tokens, ~$0.0642.

**Validation**: `hero status` — QA Escalated 2/2; Judge Waiting; Implementation Completed 3/4.

## 2026-09-05 — C9 Implementation iter 3: logging fix + Telegram TUI wiring

**Problem**: QA iter 2 failed logging on `LoopBackToImplementation` and Judge left 12 Telegram SDD gaps (Implementation iter 2 returned empty). Backend packages existed (conversation, logrotate, plugin CLI, vault, IPC, daemon) but the TUI and operational paths were not wired.

**Change** (this Implementation pass):
- `Engine.LoopBackToImplementation` now logs every failure path at `error` via a named-return defer and adds `debug` operational logs (request, waiting-shortcut, downstream reset). Default level stays `info`.
- Wired TUI to `conversation.Service`: the model holds `convService` and classifies every composer turn and Telegram inbound through it; the engine publishes `conversation.Event`s through a `Notifier` that the TUI adapter forwards to the daemon outbound path (cycle/stage/approval/error/final only).
- TUI Telegram client (`internal/tui/telegram*.go`): `tea.Cmd`-driven IPC with register/unregister/reconnect + bounded backoff + daemon respawn, Settings Telegram section (installed/version, daemon, configured state, editable abbrev, live suffix), Pair/Replace/Clear/Test actions, keyboard pairing modal with token step + 10-minute code countdown, and transcript `← / → [Telegram · addr]` labels. Added IPC `clear`/`test` frames and daemon handlers.
- Log rotation: TUI slog now writes `.workflow-hero/logs/tui.log` (10 MB × 10) with one-time legacy migration; install/upgrade call `install.MigrateTuiLog`. `.workflow-hero/logs/` added to the managed `.gitignore` block + template.
- Release: `scripts/release.sh` now builds `hero-telegram-daemon` per GOOS/GOARCH with checksums; contract test extended.
- Doctor/status report Telegram plugin health (installed / daemon binary / version match).
- `docs/architecture/architecture-overview.md` updated; integration lock tests added (`internal/integration/telegram_test.go`); `hero cycle openspec-change telegram-integration` persistence verified.

**Validation**: `go build ./...` and `go test ./...` green. Focused TUI Telegram tests, doctor/status/plugin/envhygiene/logrotate/engine/integration tests pass. Did not dispatch stage agents (TUI handoff).

## 2026-09-05 — C9 QA iter 2 failed: loop-back logging coverage

**Problem**: QA (`qa_agent`, `gpt-5.6-terra`) reported `tests_passed: false` / `logging: fail`. `go test ./...`, build, vet, and architecture passed. `LoopBackToImplementation` logs success at info but failure paths return without error-level logs and the changed path has no debug operational logging.

**Change**: Closed QA as Failed with metrics. `hero stage loop-back --from qa` with assignment in `.workflow-hero/cycles/current/qa-gaps.md` (logging fix + leftover Judge gaps from empty Implementation iter 2). Started Implementation (next iteration 3/4). Did not dispatch stage agents (TUI handoff). QA remains 2/2 so the next QA `stage start` may escalate.

**Validation**: `hero status` after close+loop-back+start: Implementation Running; QA Waiting (iteration kept at 2).

## 2026-09-05 — C9 Implementation iter 2 empty (OpenCode restart)

**Problem**: After Judge loop-back, TUI executed `generic_agent` (`opencode-go/deepseek-v4-pro`) for the 12 gaps in `.workflow-hero/cycles/current/judge-gaps.md`. OpenCode serve restarted mid-turn; the agent returned empty output. Working tree has no TUI Telegram wiring (`conversation.Service` unused; no IPC/Settings/pairing/transcript labels; logs/gitignore/release/doctor/e2e lock still missing). Diff remains loop-back engine/docs from the prior orchestrator turn.

**Change**: Closed Implementation without `--failed` (require_human_approval false) from disk artifacts and auto-advanced to QA. Coverage gaps stay for Judge.

**Validation**: `hero status` Implementation Running 2/4 before close; no new `internal/tui` Telegram references.

## 2026-09-05 — C9 Judge loop-back to Implementation

**Problem**: Judge found 12 SDD gaps but Implementation was Completed, so `hero stage start --name implementation` refused. PRD §5.4 requires QA/Judge failure to return to implementation agents.

**Change**: Added `Engine.LoopBackToImplementation` and `hero stage loop-back --from <qa|judge|browser_ui_validation|qa_end_to_end> --reason '...'`. Reopens Implementation and later enabled stages to Waiting, keeps iteration counters, clears StartedAt (timeout clock restarts). Judge report written to `.workflow-hero/cycles/current/judge-gaps.md`.

**Validation**: engine + cycle service tests for loop-back; then `hero stage start --name implementation` for C9 iteration 2.

## 2026-09-05 — C9 Judge: 12 SDD coverage gaps

**Problem**: Judge (`opencode-go/deepseek-v4-pro`) compared `openspec/changes/telegram-integration` (39 tasks) with the tree. Backend packages exist (conversation, logrotate, plugin CLI, vault, IPC, daemon). TUI/operational wiring does not: conversation.Service unused, TUI still owns dispatch, no IPC client, no Settings/pairing/transcript labels, tui.log not migrated, `.gitignore`/release/doctor/docs/context/e2e lock missing. No SDD ambiguity.

**Change**: Closed Judge as Failed with metrics. PRD loop-back is Implementation (`generic_agent`), but `hero stage start --name implementation` refused (`Completed`). Engine has no CLI to reopen a completed stage. Browser UI / E2E remain skipped. Cycle C9 stays active.

**Validation**: `hero status` shows Judge Failed 1/3; Implementation/QA Completed. Did not dispatch stage agents (TUI handoff).

## 2026-09-05 — OpenCode thinking "off" rejected by Console Go

**Problem**: Judge (`opencode-go/deepseek-v4-pro`) failed immediately with TUI `opencode session error: session error`. OpenCode log: `thinking: invalid type: string "off", expected struct ThinkingOptions`. Agent frontmatter and prompt options sent C5 `th=off` as a string; DeepSeek V4 requires `{type: disabled|enabled}`. Nested `session.error` objects were flattened to the generic "session error" text.

**Change**: OpenCode adapter maps thinking to `{type: disabled}` (`off`/`false`) or `{type: enabled}` (`max`/`true`). Agent sync writes the same object into `.opencode/agents` frontmatter. SSE `session.error` unwraps nested `error.message` so the TUI shows the provider text.

**Validation**: `go test ./...` passes, including native payload, agentdef frontmatter, and nested session.error tests.

## 2026-09-05 — C9 QA: flaky tui TempDir cleanup

**Outcome**: Cycle C9 QA (`go test ./...`) failed once in `internal/tui` (`TestConversationCancelDuringStreamWithoutSessionID` left `.workflow-hero` non-empty during `TempDir` cleanup). The isolated test passed 5/5 reruns; git diff was only `.workflow-hero/hero.db`. QA auto-closed (no human approval) and Judge started. Treat as a cleanup race, not an implementation gap from this cycle.

## 2026-09-04 — OpenCode Execute continues after serve restart

**Problem**: During C9 Implementation the OpenCode serve died mid-turn (`go test ./...` tool left `running`, no `session.idle`). Hero restarted serve and reconnected SSE, then waited forever because `GET /session/{id}` has no `status` on OpenCode 1.18.23 and message recovery requires assistant text. Ctrl+C is ignored by design (Esc / Alt+Q).

**Change**: After SSE disconnect, if `opencode serve` generation increased, Execute inspects the last assistant message. A completed turn with text is recovered. A dead/incomplete turn is aborted and continued on the same session with a short continuation prompt (original task is not re-sent). A plain SSE blip without process restart does not re-prompt. Limit: two continues per Execute.

**Validation**: `go test ./...` passes, including continue-after-restart, recover-completed-after-restart, and SSE-blip-does-not-continue tests.

## 2026-09-04 — Cursor TUI login false positive

**Problem**: During C9 Planning, the TUI reported `Cursor Agent CLI authentication required` three times and told the user to run `cursor agent login`. The CLI had already emitted `system/init` with `session_id`, model, and `apiKeySource: "login"` and run for 30s–2.5min. The same session later resumed successfully (also seen 2026-08-20). Cause: `IsAuthFailure` scanned the entire stream-json stdout for `"cursor agent login"`, which appears in docs/tool results; `AuthError.Detail` then used the first stdout line (the init JSON). A related bug treated `NonRetriableError` as retriable because the needle was `retriableerror`.

**Change**: Execute classifies auth failure only when the process failed and no Cursor `session_id` was established. `IsAuthFailure` scans stderr plus non-JSON stdout and dropped the overly broad `"cursor agent login"` / `"unauthorized"` needles. `AuthError.Detail` skips NDJSON lines. `IsRetriableFailure` strips `NonRetriableError` before matching `RetriableError`. Cycle C9 was not cancelled.

**Validation**: `go test ./... -count=1` passes, including stream-json login-phrase success, init-JSON detail skip, authenticated-session exit, and NonRetriableError no-retry tests.

## 2026-08-30 — Release v2.9.2

**Outcome**: Tagged `v2.9.2` (patch bump from `v2.9.1`). Ships Config model catalog picker, welcome dialog surface fill, and Config property/catalog cascade (including Luna `max`).

**Validation**: `go test ./...` green before tag; `scripts/release.sh` artifacts published to GitHub Releases.

## 2026-08-30 — Config model catalog picker

**Problem**: Config model fields cycled one catalog entry per Enter/Space. Harnesses with large catalogs (Cursor/Codex) made choosing a model slow and easy to overshoot.

**Change**: Enter or Space on a model field now opens a slash-overlay-style window listing that harness's models in alphabetical order. Up/Down move the cursor (with a scrolling 8-row window), Enter applies the highlighted model and normalizes thinking/effort/fast, and Escape closes without changing the draft. Tab dismisses the picker and focuses the navbar. Harness and property fields still cycle in place.

**Validation**: `go test ./... -count=1` passes, including picker open/navigate/select/escape, long-catalog scroll, subagent model fields, and harness-field regression coverage.

## 2026-08-29 — Welcome inner black gaps and Config property wheels

**Problem**: The post-`/hero-new` checklist still showed black bars inside the
dialog: nested foreground-only styles reset the surface fill on short wrapped
lines, and the button row's `PlaceHorizontal` leftover cells were unstyled.
Config thinking/effort controls cycled a hardcoded `na/true/false` and
`na/low/medium/high` list, so GPT-5.6 Luna stopped at `xhigh` instead of
catalog `max`.

**Change**: Welcome inner rows now share the surface background and are filled
to the content width before the bordered box is placed. Config property
wheels read C5 snapshot accepted values (plus YAML `na`), normalize
thinking/effort/fast when harness or model changes, and validate models
through the same catalog cascade as the picker. Catalog merge now expands a
partial live/cache effort list when it is missing later rungs such as `max`.

**Validation**: `go test ./... -count=1` passes, including welcome inner-fill
coverage and Luna effort/thinking Config cycle tests.

## 2026-08-29 — TUI welcome backdrop and Config model choices

**Problem**: The post-`/hero-new` guidance dialog left the cells outside its
centered panel unstyled, which showed as black gaps in dark terminals. The
cycle Config screen consulted only the boot-time model rows; boot intentionally
does not launch OpenCode or Codex, so their agent models could appear absent
and could not be changed.

**Change**: The centered welcome dialog now paints all placement whitespace
with Hero's surface background. Config now resolves a model choice through the
same local boot/cache/catalog cascade as the model picker, preserves an
unknown configured value, and starts the enabled-harness C5 refresh
asynchronously after the Config document loads.

**Validation**: Added TUI regression coverage for the full-screen backdrop and
for changing a Codex agent model when boot has only Cursor rows. Focused tests
pass; the full `internal/tui` suite was also run.

## 2026-08-29 — Release v2.9.1

**Outcome**: Tagged `v2.9.1` (patch bump from `v2.9.0`). Ships TUI timer/watchdog fixes, `auto-all` harness permission profile, idea-folder auto-archive on cycle archive, and removes accidental upgrade conflict backup files.

**Validation**: `go test ./...` green before tag; `scripts/release.sh` artifacts published to GitHub Releases.

## 2026-08-29 — Archive active idea notes with cycles

**Change**: `hero cycle archive` now moves every direct file and subfolder under
`docs/idea/` into `docs/idea/archive/`, preserving relative structure. The root
`README.md`, `tobe/`, and the existing `archive/` directory remain untouched.
The archive preflights destination collisions and rolls back idea moves when a
later Hero cycle filesystem step fails.

**Validation**: `go test ./internal/cycle -count=1` passes. The full
`go test ./... -count=1` suite passes all affected packages and is limited by
the two known restricted-sandbox OpenCode serve-spawn tests that cannot expose
a listening URL.

## 2026-08-29 — TUI Chat verbosity Settings

**Change**: Added the persistent Settings screen to the navbar. It offers Compact, Standard, Detailed, and Debug profiles; the default/legacy value is Debug, preserving existing transcript output. The setting is saved as `chat_verbosity` in `hero.json`. Settings is the final normal navigation item and moves immediately before the conditional Config item. Shortcut range labels now follow the visible nav list (`alt+1-6` normally; `alt+1-7` with Config; `alt+1-2` for free chat).

**Behaviour**: Profiles filter transcript detail only: Compact shows agent text; Standard adds tools/Task lifecycle; Detailed adds thinking, activities, and warnings; Debug shows all currently emitted rows. Permission/question gates, session failure handling, live-agent state, and warning status remain active regardless of the selected profile.

**Validation**: Focused install/TUI Settings/navigation tests pass. Full `go test ./... -count=1 -p 1` passes all other packages; the two pre-existing restricted-sandbox OpenCode serve-spawn tests still fail because they cannot expose a listening URL.

## 2026-08-29 — Unified Harness Manager permissions

**Change**: Collapsed the former `/harness` → enabled-Harness list → individual permission profile screens into one interactive Harness Manager. Each Harness has two indented, checkbox-style permission rows: `Ask every time` and `Automatic in project`. Space toggles the focused Harness or permission row, while Enter saves the full draft. A disabled Harness leaves its permission rows visible but muted and non-interactive, preserving the stored profile. On save, no marked permission falls back to `Ask every time`; if both are marked, `Automatic in project` takes precedence. Permission changes still restart long-lived OpenCode/Codex servers so their native settings are reloaded.

**Validation**: Focused Harness Manager tests, `go test ./internal/tui -count=1`, and `go test ./... -count=1` pass with the isolated Go cache.

## 2026-08-29 — TUI post-cycle welcome dialog

**Change**: After `/hero-new` successfully finishes `PrepareCycle`, the TUI now immediately refreshes active-cycle chrome and opens a clean, centered English guidance dialog. It explains Harness authentication, Skills parity, idea notes in `docs/idea/`, and cycle configuration through `workflow-config.yml` or Config. `Go to Config` opens the existing editable Config screen; `Close`/Esc returns to Chat. Tab or left/right switches the selected action. The dialog is transient and is shown once per successful new cycle within that TUI process; it is not persisted or shown after restart. Small terminals receive a concise resize fallback.

**Validation**: `go test ./internal/tui -run 'TestCycleWelcome' -count=1`, `go test ./internal/tui -count=1`, and `go test ./... -count=1` pass with an isolated Go cache because the restricted environment cannot write the default compiler cache.

## 2026-08-28 — Auto-ignore TUI slog log

**Problem**: `hero` redirects slog to `.workflow-hero/tui.log`, but install/upgrade
gitignore hygiene only ensured secrets patterns and skipped projects that already
had a Hero block or an existing `.env` ignore.

**Change**: `assets/templates/gitignore-secrets` now includes
`.workflow-hero/tui.log`. `internal/common/envhygiene` patches existing Hero
blocks (insert before `# END Hero secrets hygiene`) or appends a small runtime
block when needed. `hero tui` / default `hero` entry also runs env hygiene on
boot so older projects pick up the ignore without reinstalling.

**Validation**: `go test ./...` passes.

## 2026-08-28 — Accumulated TUI token usage

**Problem**: TUI Chat replaced the session context-token counter with the
latest completed Execute's usage. Cycle-stage attribution also relied on
mutable global speaker state, which could misattribute parallel stage-agent
results.

**Change**: Completed Execute usage now accumulates `input+output` for the
current Chat session. `/new-chat` and successful `/harness-reset` clear the
counter, and invalidated late completions cannot re-add usage to that new
counter. Each tagged Execute captures its stage, agent, model, and prompt for
usage fallback and cycle metrics attribution. OpenCode step usage is summed
within one Execute, while Codex app-server v2 cumulative snapshots are
normalized to their `last` turn before the TUI/cycle accumulator consumes them.
Existing cycle metrics aggregation remains additive by cycle, stage, and
agent; a late result from a reset session still records consumed cycle usage.
Nested Runtime Task usage remains represented by the parent agent's Metrics
Procedure estimate.

**Validation**: Focused TUI, Codex adapter, and harness tests pass with a
writable temporary Go build cache.

## 2026-08-28 — Codex turn callback isolation

**Change**: Codex app-server exposes one notification/request callback pair
per JSON-RPC connection. The adapter now serializes turns on the same
connection, while retaining concurrency across different harness adapters,
so parallel stage executions cannot replace one another's event and usage
routing. Queued turns honor cancellation.

**Validation**: Codex adapter and affected TUI/cycle/engine tests pass. The
unfiltered repository suite still reaches all packages and fails only the two
previously documented restricted-sandbox OpenCode serve-spawn tests, which
cannot expose a listening URL in this environment.

## 2026-08-28 — Alt-oriented TUI and navbar focus navigation

**Change**: Removed TUI Ctrl aliases and standardized modified shortcuts on Alt. Tab/Shift+Tab now switch shell focus between screen content and the visible navbar; the navbar keeps a wrapping luminous Up/Down cursor separate from the `>` active-screen marker, and Enter activates the highlighted screen. Chat Build/Plan moved from Tab to Alt+M. Config now uses Alt+S, Alt+Enter, and Alt+R; redundant Ctrl+P/N navigation aliases were removed in favor of arrow keys.

**Validation**: Added behavioral tests for focus transfer, luminous selection, marker stability, Enter activation, wrapping, hidden-navbar behavior, dirty Config leave protection, edit commit on Tab, Alt bindings, and the ignored legacy control quit key. `go test ./internal/tui -count=1` passes. The repository suite passes when skipping the two pre-existing restricted-sandbox OpenCode serve-spawn tests; an unfiltered `go test ./...` fails only those same documented cases because they cannot expose a listening URL in this environment.

## 2026-08-28 — AI working and response-gap timers

**Change**: Renamed the execution timer label to `AI wk` and added `AI rp`
directly below it. `AI rp` is transient TUI state: it is zero and stopped at
boot, starts when the first harness response is placed in Chat, and restarts on
every subsequent harness response. It continues after Execute completion so a
growing value exposes an absent response. Session metadata and local watchdog
alerts do not reset it.

**Validation**: Focused AI response-timer/sidebar-layout tests and
`go test ./internal/tui -count=1` pass. `go test ./... -count=1 -p 1` passes
every other package; the known restricted-sandbox OpenCode spawn tests
(`TestDiscoverModelPropertiesNormalized` and
`TestIsManagedOpenCodeServeDetectsSpawnedServe`) cannot expose a listening URL.

## 2026-08-28 — Sidebar timer value alignment

**Problem**: The `Session` and `AI` labels were aligned, but `AI` reserved one
column less before its `HH:MM:SS` value.

**Change**: Both timer rows now use the same fixed label field, and the layout
test asserts that the two counter values start in the same rendered column.

**Validation**: `go test ./internal/tui -count=1` passes. `go test ./... -count=1 -p 1`
passes all other packages; the known restricted-sandbox OpenCode spawn tests
(`TestDiscoverModelPropertiesNormalized` and
`TestIsManagedOpenCodeServeDetectsSpawnedServe`) cannot expose a listening URL.

## 2026-08-28 — Chat Session starts before cycle restore

**Problem**: An ordinary first Chat prompt left `Session` at zero after opening
the project TUI when SQLite already contained an active cycle. The timer only
considered whether a cycle row existed, not whether this TUI had restored a
cycle session.

**Change**: Ordinary Chat now starts the process-local Session timer unless
`/hero-start` or `/hero-resume` has restored the cycle timer (or `/hero-new`
is creating one). The first prompt is covered by a regression test.

**Validation**: `go test ./internal/tui -count=1` passes. `go test ./... -count=1 -p 1`
passes all other packages; the known restricted-sandbox OpenCode spawn tests
(`TestDiscoverModelPropertiesNormalized` and
`TestIsManagedOpenCodeServeDetectsSpawnedServe`) cannot expose a listening URL.

## 2026-08-28 — TUI timer label

**Change**: Renamed the blue navbar timer label from `Sessão` to `Session`.
The existing lifecycle remains unchanged: zero at TUI boot, first free-chat
prompt, and cycle transitions according to ADR-058.

**Validation**: `go test ./internal/tui` passes. The full suite passes outside
the two pre-existing restricted-sandbox OpenCode spawn cases documented in the
previous entry.

## 2026-08-28 — TUI navbar hint and Session timer lifecycle

**Problem**: The `alt+1-6` hint was attached to the navigation rows, and a fresh TUI restored the persisted cycle Session timer before the user explicitly resumed/started a cycle. Free-chat prompts also reset the Session timer on every turn.

**Change**: Anchored the shortcut hint to the last row of the navbar navigation area, immediately above the timer divider, with responsive clipping for short terminals. TUI startup now begins Session at zero; `/hero-start` and `/hero-resume` explicitly request persisted cycle recovery, while `/hero-new` starts a zeroed timer. Free chat starts at the first prompt and keeps the same process-local timer across later prompts. AI timing remains per Execute.

**Validation**: Focused layout/timer tests and the complete `internal/tui` suite pass with `GOCACHE=/tmp/hero-go-cache CC=/usr/bin/gcc CXX=/usr/bin/g++`. Full `go test ./... -count=1 -p 1` passes all other packages; the two known restricted-sandbox OpenCode serve-spawn tests still fail because their simulated process exits before exposing a listening URL.

## 2026-08-27 — TUI test helper and conditional navigation

**Outcome**: Test-only models skip the 30-second health probe, and command draining stops after the first business message. Esc cancels active Executes/preflight; the protected quit binding was later standardized on Alt+Q. Sidebar numbering follows visible screens (`alt+1-5`, or `alt+1-6` with Config).

**Validation**: TUI and focused navigation/cancellation tests passed.

## 2026-08-27 — `hero chat` OpenCode workspace routing

**Outcome**: Free chat stores configuration under `~/.workflow-hero/` but executes in cwd. All OpenCode session-scoped calls now use the same `directory` query as event subscription/recovery, preventing hangs and `context canceled` failures.

**Validation**: Adapter and full-suite checks passed apart from the two known restricted-sandbox OpenCode serve-spawn tests.

## 2026-08-26 — Chat composer and harness wait UX

**Outcome**: Enter inserts ordinary newlines while recognized slash commands execute; Alt+Enter submits ordinary prompts. Composer caret movement follows visual lines, and watchdog alerts are suppressed during permission/question waits.

**Validation**: TUI and repository tests passed except the known restricted-sandbox OpenCode serve-spawn cases.

## 2026-08-25 — C7 Config and C8 TUI-direct execution

**Outcome**: Added the active-cycle Config form with managed YAML saves/retry and TUI-direct named stage Executes after orchestrator handoff. Parallel Implementation agents and nested Task labels are represented in Chat.

**Validation**: Feature and repository tests passed during the implementation cycle.

## 2026-08-28 — Per-harness project-local approval profiles

**Decision**: User confirmed a simple profile per enabled harness: `Ask every time` is the default for new and legacy configuration; `Automatic in project` is opt-in. The automatic preset must not become unrestricted yolo: network, MCP, shell, and external-directory access stay in the native approval path.

**Implementation**: Added `harness.PermissionProfile` to normalized Execute requests and persisted `harnesses.<id>.permission_profile` in `hero.json`. `/hero-harness` now continues from enable/disable selection into an enabled-harness profile manager. OpenCode starts its managed server with an inline process-only permission override, Codex retains `on-request` and auto-approves only workspace-confined file changes, and Cursor uses sandboxed `--auto-review` without auto-approving MCPs. Existing absent values read as `ask` without forced migration writes.

**Validation**: `go test ./...` passes with an isolated Go cache and writable temporary OpenCode data directory (the execution sandbox makes the default Go/ccache and OpenCode user-data locations read-only).

## 2026-08-29 — Release v2.9.0

**Outcome**: Tagged `v2.9.0` (minor bump from `v2.8.0`). Ships TUI settings screen, checklist window, harness config adjustments, auto-hide Config after archive, TUI label/status-bar polish, and `docs/idea` folder support.

**Validation**: `go test ./...` green before tag; `scripts/release.sh` artifacts published to GitHub Releases.

## 2026-08-29 — AI rp tracks harness responsiveness independently of transcript detail

**Decision**: `AI rp` measures elapsed time without response content from the harness, rather than time since the last response visible in the Chat detail profile. Hidden thinking, tool, activity, or warning response content therefore resets it.

**Implementation**: Removed the transcript-verbosity condition from TUI response-timer resets and added coverage for Compact mode hiding thinking.

## 2026-08-29 — Harness Manager visual grouping and unrestricted profile

**Decision**: User authorized an unrestricted per-harness `auto-all` profile. It is labeled `Auto approve every time (Yolo)` and may approve shell, network, MCP, and external-path operations through the native harness mechanisms.

**Implementation**: `/harness` now groups each harness's three exclusive approval choices under `Permissions:` with blank separation between harnesses. Cursor maps `auto-all` to force/MCP approval with sandbox disabled, OpenCode to `permission: allow`, and Codex to no-approval danger-full-access plus automatic replies to residual requests.

## 2026-08-29 — Pause watchdog during interactive harness callbacks

**Decision**: A permission or question callback is an expected harness pause. Its user-wait duration must not count toward the harness inactivity timeout.

**Implementation**: Added watchdog pause/resume accounting around interactive callback lifecycle, preserving the active time before the callback and resuming it after the response.

## 2026-08-28 — Release v2.8.0

**Outcome**: Tagged `v2.8.0` (minor bump from `v2.7.0`). Ships C7 TUI cycle configuration, C8 TUI-direct stage Execute, shared Session/AI timers, OpenCode question mapping and hang workarounds, Codex stream/permission improvements, and per-harness project-local approval profiles (`ask` / `auto-project`).

**Validation**: `go test ./...` green before tag; `scripts/release.sh` artifacts published to GitHub Releases.

## 2026-09-04 — C09 Telegram remote interface research

**Decision**: Telegram will be an optional official Hero plugin distributed in
the same releases. A local daemon, one per OS user and machine, exclusively
owns Bot API traffic and pushes messages to concurrent TUIs through private
versioned IPC. The TUI and Telegram share a transport-neutral conversation
service; SQLite is durable queue/audit state, never a live-event polling bus.

**Requirements confirmed**: Pair exactly one authorized chat with a
single-use 10-minute code in a Settings modal; keep token and chat id in the
OS credential vault; identify project instances by a user-chosen base
abbreviation plus stable `_2+` suffixes and Free Chats as `free_N`; queue
unavailable-target messages for 24 hours; daemon-owned
`/telegram-cancel-pending` cancels an address's pending queue without touching
cycle execution. Project logs move to `.workflow-hero/logs/tui.log`, retain
10 × 10 MB, and daemon diagnostics use a global rotating user log. Install and
upgrade preserve `.gitignore` while ignoring the project log directory.

**Research artifacts**: PRD-C09-001, ADR-C09-002 (ADR-059–064), UI-C09-001,
plus DEPLOY/TESTING/index/architecture-overview updates and documents registry
entries.

## 2026-09-05 — Telegram instance selection commands

**Decision**: The authorized Telegram chat selects a live *instance* (not only
the project base name), so concurrent project TUIs remain unambiguous. `/list`
returns sorted numbered instance addresses; `/select n` persists the chosen
address in the daemon SQLite store without retaining the credential-vault-only
chat id. Selection is cleared when the authorized chat is replaced or cleared.

**Implementation**: Unprefixed ordinary text and slash commands now route to
the selected live instance. If it disconnects, the daemon reports an actionable
`/list` + `/select` error rather than queuing the turn. Explicit addressed
input remains compatible and keeps its existing offline durable queue behavior.
The daemon replies `OK, Received.` after accepting a live delivery to a TUI.

**Validation**: Added daemon/store/router coverage for deterministic listing,
selection persistence, selected routing, disconnection errors, and live-delivery
confirmation. `go test ./internal/telegram/daemon -count=1`,
`go test ./internal/tui -count=1`, and `go test ./...` pass.

## 2026-09-05 — Telegram status and auto report

**Implementation**: Added TUI-owned Telegram `/status`: it reports `idle`,
active cycle/current-stage data, or `Waiting for harness`, with Session, AI wk,
AI rp, and context-window counters. Telegram turns that start or wait for a
harness turn emit the same immediate status. Settings now persists
`telegram.auto_report_minutes` per project (`0` disabled; `1–300` minutes),
and the existing non-blocking Bubble Tea timer sends periodic status while
Telegram remains connected and paired.

**Validation**: Added focused install/TUI coverage. The Telegram status tests
cover the manual idle response, skipped idle auto reports, and one non-idle
report per interval; `go test ./internal/tui -count=1` and `go test ./...`
pass.

## 2026-08-28 — Discover auto-loads active `docs/idea` files

**Decision**: Research should consider optional design notes under `docs/idea/` at session start. Top-level `archive/` and `tobe/` are excluded; empty folder is fine.

**Implementation**: Added `internal/ideadocs` (`ListActive`, `PromptSection`), TUI injection in `startDiscoverResearchSession`, CLI `hero cycle idea-files` (`--json`), and `discover_agent.md` responsibility to run the command in Cursor IDE. Documented layout in `docs/idea/README.md`.

**Validation**: `go test ./internal/ideadocs/... ./internal/cycle/... ./internal/tui/...` and full `go test ./...`.

## 2026-08-28 — Hide Config after `/hero-archive`

**Implementation**: Added `syncActiveCycleChrome()` to reconcile navbar/palette when the active cycle ends: hides Config, switches label to `alt+1-5`, leaves Config screen for Chat, and clears config draft state. Called on archive success (eager `Status()` sync) and on `refreshDataMsg` when cycle presence drops.

**Validation**: `go test ./internal/tui/...` and full `go test ./...`.

## 2026-09-05 — Telegram daemon announces instance disconnections

**Implementation**: The daemon now removes an instance through one idempotent
lifecycle path for both `unregister` and unexpected IPC socket loss. After
removal, it sends `<address>: disconnected.` to the authorized Telegram chat
with a two-second request deadline, then schedules the existing last-client
shutdown when appropriate.

**Validation**: Added clean-unregister integration coverage and an unexpected
socket-drop daemon test. `go test ./internal/telegram/daemon ./internal/integration`
and `go vet ./internal/telegram/daemon ./internal/integration` pass.

## 2026-09-05 — Release Hero v3.0.3

**Change**: Incremented the patch version for the Telegram instance
disconnection notification and prepared the matching Hero and daemon release
artifacts.

## 2026-09-06 — Telegram auto status skips idle and duplicate timer startup

**Problem**: A periodic Telegram report reused the manual `/status` renderer,
so an idle TUI emitted `idle`; the initial timer command could also be started
from `Init` and again during the first state refresh, allowing duplicate timer
loops around Telegram status delivery.

**Change**: Split manual status rendering from automatic/turn status rendering.
Only the manual Telegram `/status` path may emit `idle`; automatic reports and
immediate status for remote harness turns stay silent while idle and advance
their next interval. Timer startup is now owned by stateful `Update` paths,
including Telegram connection registration, so `Init` cannot create an
untracked second loop.

**Validation**: Added TUI coverage for manual idle, skipped idle auto reports,
one non-idle report per interval, and retained the existing TUI/Telegram test
suites. Full `go test ./...` remains required before completion.

## 2026-09-06 — Telegram Always send project setting

**Requirement**: Add a project-local Telegram Settings toggle that forwards
completed TUI harness responses to Telegram, while keeping the existing
Telegram-origin-only behavior as the default.

**Change**: Added `telegram.always_send` to `hero.json`, exposed it as the
`Always send` row beside `Auto report`, and routed completed final turn replies
through the existing outbound path when enabled. Intermediate stream, thinking,
tool, and sibling-task output remains local; Telegram-originated replies remain
unchanged regardless of the toggle.

**Validation**: Added persistence, Settings rendering/focus/toggle, local
forwarding, and default-off coverage. `go test ./internal/install ./internal/tui
-count=1`, `go test ./...`, `go test -race ./internal/tui ./internal/telegram/...`,
and `go vet ./...` pass.

## 2026-09-06 — Release Hero v3.0.5

**Change**: Incremented the patch version for the Telegram `Always send`
project setting and prepared the matching Hero and daemon release artifacts.

## 2026-09-06 — Ideia ativa: Claude Code adapter

**Decision**: A próxima proposta de harness Claude Code usará a CLI headless
diretamente de Go (`claude -p` + NDJSON), em vez de uma bridge Node/TypeScript
ou automação do TUI. Será TUI-only, como OpenCode/Codex. A projeção nativa será
`assets/claude/` → `.claude/`, e o Hero passará a criar/gerir um bloco marcado
em `CLAUDE.md` que importa `@AGENTS.md`, sem duplicar instruções. O perfil
`ask` exige bridge MCP temporária; sua compatibilidade de protocolo é um spike
obrigatório antes da implementação.

**Artifact**: `docs/idea/v3.1_claude_adapter/claude_adapter.md` registra
assets, catálogo/modelos/propriedades, permissões, health/watchdog,
verbosidade, critérios de aceitação e referências oficiais em PT-BR.

## 2026-09-06 — Preflight determinístico do `/hero-new`

**Problema**: uma execução do `/hero-new` via Telegram terminou sem criar
`.workflow-hero/cycles/current/workflow-config.yml`. O fallback do engine usou
o template global, permitindo que o ciclo fosse criado com configurações
incorretas e deixando `/hero-config` sem o arquivo canônico esperado.

**Mudança**: o TUI agora prepara o arquivo antes do turno do Runtime. Quando
ele não existe, `workflowconfig.EnsureCurrent` cria uma cópia atômica do
template e importa, por deep merge, `workflow_config`, `fallback_model`,
`stages` e `agents` do ciclo arquivado mais recente; título, objetivo, escopo e
outras chaves do template permanecem resetados. Arquivos existentes são
preservados e validados. `cycle.Service.PrepareCycle` valida novamente e
passa o caminho explícito de `cycles/current` ao engine. O sincronismo de
`/hero-start` também deixou de aceitar o template global como fallback.

**Compatibilidade**: o prompt específico do TUI autoriza o agente a criar ou
atualizar o YAML e mantém a proibição de executar Shell/CLI. Os comandos
compartilhados do Cursor não foram alterados.

**Validação**: novos testes cobrem primeiro ciclo, importação do maior ciclo
arquivado, preservação do arquivo existente, preflight do TUI e falha fechada
no sync. `go vet` dos pacotes afetados e `go test ./...` passaram.

## 2026-09-06 — Release Hero v3.0.6

**Change**: Incremented the patch version for deterministic `/hero-new`
workflow-config preflight, archived-config import, and fail-closed current
configuration synchronization. The release includes the TUI/engine regression
coverage and the current project context.

## 2026-09-07 — C12 cancelled before Telegram-driven restart

**Decision**: At the user's request, cancelled the active Claude Code adapter
cycle during Research so the Telegram execution flow can be improved before a
new cycle starts from zero.

**Rollback**: Ran `hero cancel` with the cancellation reason, restored the
tracked C12 changes, removed the untracked C12 documents and current
`workflow-config.yml`, and kept `.workflow-hero/hero.db` changed so the
cancelled event remains persisted. `hero status --json` now reports no active
cycle.

**Next**: Improve and validate `/hero-new`, `/hero-config`, and `/hero-start`
through Telegram, then create a fresh Claude Code adapter cycle from Research.

## 2026-09-07 — Telegram native permissions and child lifecycle relay

**Problem**: OpenCode native `permission.asked` callbacks were rendered only
in the local TUI, so a Telegram-driven execution could not answer them. A
separate `hero stage close` child process persisted pending cycle approval in
SQLite but had no parent TUI `Engine.Notifier`. OpenCode resume/recovery paths
also called the default `ask` serve setup and could downgrade a configured
`auto-all` process.

**Change**: Added keyed TUI permission tracking and the correlated
`/hero-permission <id> allow|deny` Telegram command, with local `y`/`n` and
`/interrupt` cancellation preserved. OpenCode carries the normalized
permission profile in the Execute context through resume, SSE reconnect, and
HTTP recovery; Prepare uses the configured profile and long-lived serve
processes receive the private lifecycle socket environment. Added
`internal/lifecycle`, a per-TUI private Unix relay; CLI services inherit its
endpoint and publish append-only event IDs, allowing the TUI to forward child
cycle approvals without SQLite polling or Telegram-specific engine code.

**Validation**: Added tests for permission correlation/cancellation, lifecycle
socket delivery and buffering, cycle service notifier wiring, event-ID
correlation, OpenCode profile/environment propagation, and resumed execution.

## 2026-09-09 — Release Hero v3.1.1

**Change**: Incremented the patch version for Telegram `/interrupt` and `/kill`
commands, Telegram status-loop and project-prefix fixes, TUI models/wizard/token
fixes, harness session-id isolation, Claude adapter dogfooding on this repo, and
`release.sh` local binary install parity with `build_dev.sh`.

**Validation**: `go test ./...`

## 2026-09-08 — Release Hero v3.1.0

**Change**: Incremented the minor version for the C13 Claude Code adapter: opt-in
fourth TUI harness with supervised `claude -p` NDJSON turns, native session
resume, constrained ask permission bridge, native model catalog, `.claude/`
projection, marked `CLAUDE.md` ownership, install/upgrade/uninstall lifecycle,
Doctor/Status diagnostics, and cycle-manager execution fixes.

**Validation**: `go test ./...`

## 2026-09-07 — Release Hero v3.0.8

**Change**: Incremented the patch version for Telegram native-permission
forwarding, child CLI lifecycle-event relay, cycle approval delivery, and
OpenCode Yolo-profile preservation across resume/recovery.

## 2026-09-07 — C13 Claude Code adapter Research completed

**Decision**: C13 adds Claude Code as an opt-in fourth TUI harness while
preserving the existing deterministic engine, feature-based adapter boundary,
and Cursor-only IDE Runtime. The minimum supported installed CLI is 2.1.261;
Linux/macOS remain the only target platforms.

**Scope**: A turn-scoped `claude -p` NDJSON adapter, native session resume,
SIGINT-first cancellation, five-minute watchdog, an `ask` permission MCP
bridge validated by a mandatory fake-process spike, `.claude/` projection,
managed `CLAUDE.md` block importing `@AGENTS.md`, native catalog/properties,
and TUI/install/Doctor/Status/Telegram integration.

**Artifacts**: PRD-C13-001, ADR-C13-001 (ADR-070–074), and UI-C13-001 were
registered in `documents.json`; TESTING.md, DEPLOY.md, architecture overview,
and current state were updated. The user requested `golang-tui` and
`go-engineering` guidance for implementation.

## 2026-09-07 — C13 Claude Code adapter Planning completed

**Decision**: The planning stage produced the OpenSpec SDD at
`openspec/changes/claude-code-adapter/`. The design makes the fake-process
protocol spike a hard gate, keeps Claude execution turn-scoped and supervised,
uses the existing harness/session contract without a new daemon or database
registry, fails closed for unsupported ask-mode transport, and preserves
user-owned `.claude`/`CLAUDE.md` content through marked projection updates.

**Artifacts**: `proposal.md`, 12 spec delta directories, `design.md`, and
`tasks.md` with 42 independently testable tasks. The task graph identifies
parallel groups for shared state, adapter core, projection, catalog,
permission/lifecycle, diagnostics, and Telegram work, followed by mixed-harness
acceptance and final verification.

**Validation**: `openspec validate claude-code-adapter --strict` passed and
OpenSpec reports 4/4 planning artifacts complete. No Go source was changed in
Planning; `go test ./...` remains an Implementation/QA gate.

**Approval**: The active cycle stores the `claude-code-adapter` slug. Human
approval is required before Implementation; use `/hero-approve`,
`/hero-reject`, `/hero-cancel`, or `/hero-finish` in the Hero TUI.

## 2026-09-07 — C13 implementation: Claude protocol compatibility gate

**Change**: Added `internal/adapters/claude` protocol-gate primitives and
deterministic fake-launcher tests. The gate validates the 2.1.261 minimum,
required stream-json command surface, working directory/environment/process
group command contract, and line-by-line NDJSON fixtures for init/resume,
partial text/thinking, tool, subagent, retry, hook/plugin, usage, result,
authentication, stderr, unknown, and malformed events.

**Security**: Captured MCP permission-prompt request, allow/deny decision, and
termination fixtures are validated fail-closed. A one-time token helper rejects
replay, and unsupported `ask` transport returns the explicit
`ErrAskUnsupported` error without profile downgrade. No credential or live
Claude process is used.

**Validation**: `go test ./internal/adapters/claude` passed. Full repository
verification remains the final C13 task after the dependent adapter work.

## 2026-09-07 — C13 implementation: state, registry, and supervised adapter core

**Change**: Added `claude` as a disabled-by-default supported harness state and
registered a lazy Claude adapter without changing existing Cursor, OpenCode, or
Codex instances. Implemented injected PATH/probe/process/clock/token seams,
one-child-per-turn stream-json execution, required CLI compatibility probes,
incremental NDJSON normalization, early native-session stream metadata, final
result/usage repair, local catalog listing, five-minute health timeout, and
SIGINT-first process-group cancellation with bounded kill escalation.

**Safety**: Ask mode remains explicitly fail-closed until the execution-scoped
permission bridge is implemented; it never launches a permissive substitute.
Unknown protocol payloads produce bounded redacted warnings. Claude's local
catalog supplies aliases only and contains no invented prices.

**Validation**: Added fake-process adapter tests for argv, stream normalization,
resume single-launch behavior, ask rejection, cancellation, availability, and
catalog discovery. `go test ./...` passed.

## 2026-09-07 — C13 QA iteration 2 closed

**Outcome**: After Implementation loop-back, QA ran as iteration 2/2
(`qa_agent`, `opencode-go/deepseek-v4-pro`). The TUI finished the agent with a
partial report: build and vet pass; full `go test ./...` and logging review
were started. No structured failure JSON was returned, so the stage was
auto-closed (`require_human_approval: false`) rather than looped back.

**Next**: Judge is the next enabled stage (Browser UI Validation and QA
End-to-End remain skipped).

## 2026-09-07 — C13 Judge iteration 2 failed: loop-back to Implementation

**Outcome**: Judge (`judge_agent`, `cursor-grok-4.6-xhigh`) reported
`all_requirements_met: false` with no SDD ambiguity. Adapter core and alias
catalog remain landed. Remaining gaps: ask bridge, `assets/claude/` projection
and `CLAUDE.md`, native catalog IDs, TUI/Doctor/Status/Telegram, `.claude/`
marker still `Supported: false`, mixed-harness suite, and final verification.

**Change**: Closed Judge as Failed with metrics, ran
`hero stage loop-back --from judge`, then `hero stage start --name implementation`
(iteration 3/4). Did not dispatch stage agents (TUI handoff). Artifact:
`.workflow-hero/cycles/current/judge-gaps.md`. QA stays Waiting at 2/2, so the
next QA `stage start` may escalate on iteration budget.

## 2026-09-07 — C13 adapter normalizer and permission bridge core extended

**Change**: Added runtime-native model and effective-property metadata to the
shared execution result, populated from Claude `system/init`. Claude now
normalizes explicit permission/question frames into the shared stream kinds and
provides a one-request stdio MCP bridge codec protected by its injected
one-time token. The codec validates only `approval_prompt` JSON-RPC requests,
forwards a redacted permission request through the existing callback contract,
and returns a validated allow/deny response preserving the original input.

**Safety**: The bridge exposes no generic MCP methods, resources, credentials,
plugins, or raw request payloads. Replay and malformed requests are rejected;
callback failure produces denial rather than an approval fallback. The Claude
ask command now selects the proven permission-prompt tool only when a decision
callback is supplied. The temporary MCP process/config transport and TUI/
Telegram lifecycle wiring remain a separate unfinished C13 task.

**Validation**: Added deterministic bridge and adapter metadata tests; `go test
./...` passed.

## 2026-09-08 — C13 loop-back: live Claude ask bridge and diagnostics

**Change**: Replaced the disconnected in-process permission pipe with a
per-execution localhost bridge. Claude receives a 0600 temporary,
`--strict-mcp-config` file that starts only Hero's hidden stdio MCP helper;
that helper handles MCP negotiation and forwards only `approval_prompt` calls
to the parent bridge. The one-time token stays in the helper environment,
never in Claude's environment or logs, and all success, failure, and cancel
paths close the listener and remove the config. Claude rejects identifiable
Codex/OpenCode/Cursor session IDs before launch. TUI completed-turn labels now
use the native model reported by `system/init`.

**Diagnostics**: Doctor describes an unconfigured `.claude/` directory as
user-managed, not unsupported. SQLite schema v9 persists an active stage's
permission-pause state so Status can report it without starting a harness.

**Validation**: Added live bridge round-trip, foreign-session rejection, and
native-model identity coverage. `go test ./...` passed.

## 2026-09-08 — TUI Implementation completion gate (ADR-075)

**Problem**: C13 exposed a deterministic judge → implementation → QA loop.
The TUI treated the first successful stage-agent process return as completion
of the whole Implementation stage, even when the OpenSpec checklist still had
pending work. `Escalated` was also treated as executable, and stage-agent
assignments/results were not retained for audit.

**Change**: Added a fail-closed Implementation contract across Cursor, Codex,
OpenCode, and Claude agent assets. TUI assignments now include the linked
`tasks.md`, exact pending task lines, dependency/parallel pointers, and
verification commands. Reports require `complete|partial|blocked`, completed
and remaining task arrays, tests, a non-empty boolean acceptance-gate map, and
recovery details for partial/blocked outcomes. The TUI closes Implementation
only when every report and the on-disk checklist pass; productive partial waves
may continue in the same stage iteration up to a defensive limit of eight.
Empty, malformed, blocked, unlinked, or no-progress results keep the stage
Running and require explicit `/hero-start` retry after intervention. An
`Escalated` stage cannot dispatch before `/hero-continue` returns it to Waiting
and `StartStage` records Running.

**Auditability**: Reused SQLite `conversation` without a schema migration.
`stage_agent_assignment` and `stage_agent_result` entries preserve the exact
prompt/result inside a JSON envelope with stage, agent, and wave; empty results
are retained as evidence rather than discarded.

**Architecture**: Accepted ADR-075 amends C8. Go reads deterministic checklist
state and enforces the gate; dependency reasoning and nested Task fan-out stay
with implementation agents, preserving the CLI/Runtime boundary.

**Validation**: Added report parsing, required-field, assignment, checklist
completion/progress/no-progress, escalation, retry-copy, and SQLite round-trip
tests. `go test ./...`, `go test -race ./internal/tui ./internal/cycle`, `go vet ./...`,
`openspec validate claude-code-adapter --strict`, and `git diff --check` passed.

## 2026-09-08 — build_dev.sh local install

**Change**: `scripts/build_dev.sh` now cross-compiles the Telegram daemon alongside
Hero, copies linux/amd64 artifacts to `/home/ricardo/installable/hero/hero` and
`/home/ricardo/.workflow-hero/plugins/telegram/hero-telegram-daemon`, and updates
the Telegram plugin `manifest.json` version/installed_at via python3.

**Validation**: `go test ./scripts/...`; `./scripts/build_dev.sh` completed successfully.

## 2026-09-08 — Telegram model list parity

**Change**: `internal/tui/telegram_model_selection.go` now mirrors local `/model` refresh behavior, always fetches live harness model lists, merges them with catalog/cache via `modelChoicesForHarness` (`internal/tui/model_picker.go`), and paginates Telegram model prompts. `configModelChoices` delegates to the same helper.

**Validation**: `go test ./internal/tui/...` (Telegram model/config wizard regressions for grok-4.5 merge, live list, pagination, and refresh); `go test ./...`.

## 2026-09-08 — TUI interrupt key remap

**Change**: Chat stream interruption moved from `Esc` to `Ctrl+C`. `Esc` now
focuses the navbar (overlay dismiss unchanged). Footer hints and Telegram
`/interrupt` parity docs updated.

**Validation**: `go test ./internal/tui/...`

## 2026-09-09 — C13 deterministic bridge and event-fixture completion

**Change**: Expanded the frozen Claude NDJSON stream fixture with explicit
permission and question frames. The result-assembler golden test now verifies
their normalized metadata plus text/thinking/tool, retry, subagent, hook,
plugin, usage, final-result repair, and bounded redacted unknown-event warning
behavior. Added a fake-child `ask` test that inspects the temporary strict MCP
configuration while the turn is starting: it contains only the private Hero
stdio helper, has mode `0600`, and keeps the one-time token out of Claude's
environment. Adapter callback coverage confirms permission and question
frames reach the existing shared contracts, and boundary tests reject Cursor,
OpenCode, and Codex resume IDs before a Claude process starts.

**Validation**: `go test ./internal/adapters/claude ./internal/tui
./internal/status ./internal/doctor ./internal/integration ./internal/harnessmgr
./internal/install ./internal/modelprops` passed.

## 2026-09-09 — TUI Execute map race corrected

**Problem**: `startConversationExecute` captured a value-copy of the Bubble
Tea model, but its background worker still read the shared `executes` map while
`Update` handled `executePairMsg`. The required race check consistently caught
the concurrent map read/write.

**Change**: Snapshot the current turn's `Freechat` flag before launching the
worker and pass that scalar into `executeConversationTurn`; the worker no
longer touches `executes`. This preserves the TUI-only ownership of mutable
model state while retaining the same routing behavior.

**Validation**: Targeted approve/reject race reproductions and
`go test -race ./internal/tui ./internal/cycle` passed.

## 2026-09-09 — C13 Judge-gap recovery verification

**Outcome**: Revalidated the live strict MCP permission bridge, normalized
permission/question event fixtures, foreign-session guard, runtime native-model
identity, permission-pause Status reporting, Telegram permission denial path,
and deterministic four-harness execution/cancel/fallback acceptance coverage.
The bridge remains execution-scoped and exposes only its temporary
`approval_prompt` helper; no credential, plugin, or general MCP transport is
introduced.

**Validation**: `openspec validate claude-code-adapter --strict`, `go test
./...`, `go vet ./...`, `go build ./cmd/hero`, `git diff --check`, targeted
C13 package tests, and `go test -race ./internal/tui ./internal/cycle` passed.

## 2026-09-09 — C13 Claude `/hero-start` preparation recovery

**Change**: Added the Claude Prepare-on-start path. It performs only a
PATH/version/required-flag compatibility probe, then preflights every projected
Claude agent before atomically updating marker-delimited `model`, supported
`effort`, and embedded skill fields. User body text and unmarked frontmatter
remain untouched. A failed probe or invalid/missing marker produces actionable
`/hero-start` copy and completes no agent-file writes. The asynchronous TUI
prepare command now invokes this path after OpenCode/Codex and only when an
enabled workflow agent uses Claude.

**Validation**: Added Claude preparation and TUI scheduling tests; `go test
./...`, `openspec validate claude-code-adapter --strict`, and `git diff --check`
passed.

## 2026-09-09 — Claude adapter debug-only stream events

**Change**: Classified Claude `system.status` and `stream_event` as known
observability frames instead of unrecognized warnings. They now emit debug-only
`StreamKindActivity` deltas when `ExecuteRequest.Debug` is set from global
`hero --debug`. Unknown Claude events and the bounded suppression notice follow
the same rule, matching Codex/OpenCode behavior and keeping normal Chat output
free of harness protocol noise.

**Validation**: Extended `internal/adapters/claude/normalizer_test.go`; `go test
./...` passed.

## 2026-09-09 — Cursor harness trust false-positive fix

**Problem**: TUI showed `cursor agent workspace trust required` with a
`system/init` JSON detail even though Hero already passes `--trust` on every
Execute and the workspace had `.workspace-trusted`. Cause matched the C9 auth
false-positive: `IsTrustFailure` scanned full stream-json stdout (including
assistant text mentioning "workspace trust") and interpolated the init line as
detail. Remediation also incorrectly suggested bare `cursor agent --trust`.

**Change**: Hardened `IsTrustFailure` to scan stderr + non-JSON stdout only
(same pattern as `IsAuthFailure`). Added typed `TrustError` with
`trustFailureDetail` that skips NDJSON. Updated `TrustHint` and the TUI
remediation line. Expanded unit/Execute tests for NDJSON chatter vs plain
`Workspace Trust Required` warnings (including exit 0).

**Validation**: `go test ./internal/adapters/cursor ./internal/tui` passed.
`go test ./...` still reports a pre-existing failure in
`internal/adapters/opencode` (`TestExtractOpenCodeUsageAccumulatesStepFinishes`);
untouched by this change.

## 2026-09-10 — TUI multimodal image architecture direction

**Decision**: Selected the shared multimodal contract approach for a future
Hero cycle. User and model images will be represented as typed attachment/asset
references through the conversation, harness, adapter, and TUI boundaries;
each adapter owns its native protocol translation and unsupported capabilities
fail explicitly. Direct provider API calls are not the selected direction.

**Proposal**: Added the non-normative PT-BR design note
`docs/idea/tobe/tui-imagens-multimodais.md`. It covers bidirectional flows,
domain types, adapter-specific paths, TUI capture/cards/previews, storage,
security, testing, incremental delivery, and decisions that the future
PRD/UI/ADR/OpenSpec cycle must close. No runtime architecture or code changed.

**Validation**: documentation-only review and `git diff --check`.

## 2026-09-10 — Telegram model list uses live harness authority

**Change**: Fixed Telegram `/model` and `/hero-config` model selection so a
successful adapter `ListModels` response is the complete selectable list.
Catalog, cache, boot rows, and the configured current model are used only as
the existing fallback when live listing fails. This prevents stale Codex
catalog entries such as `gpt-5.3-codex` from being offered when the Codex
app-server does not return them.

**Tests**: Replaced the test that locked in catalog/live merging with live-only
assertions and added a Codex regression test. `go test ./...` passed.

## 2026-09-10 — TUI transcript stays at the bottom across resize

**Problem**: A terminal dimension event could be processed as `scrollTranscript(0)`.
When the viewport first grew and later returned to its previous height, the offset
remained below the new maximum and `transcriptFollowBottom` was cleared. This made
the chat appear to jump upward after returning to the TUI window.

**Change**: Separated transcript offset clamping from manual-scroll semantics.
`WindowSizeMsg` now clamps the offset while preserving the prior follow-bottom
state and recomputes the bottom offset when that state was active. Manual scrolling
continues to opt out of auto-follow.

**Validation**: Added a regression test covering grow-and-restore resize events;
`gofmt`, `go test ./...`, and `git diff --check` passed.

## 2026-09-10 — C14 multimodal Free Chat implementation

**Change**: Implemented the C14 shared multimodal contract and end-to-end
reference flow. `internal/harness` now carries typed attachments, assets,
capabilities, asset-stream deltas, and stream/final repair. `internal/media`
validates image headers and limits, materializes UUID-named 0600 session files
under XDG data, writes a redacted metadata manifest, deduplicates by SHA-256,
cleans expired sessions, admits only the transport/model capability
intersection, renders asynchronous Unicode mosaics, and keeps optional
terminal preview protocols disabled by default. Free Chat owns picker,
clipboard, path-paste, slash, chip, image-only submit, asset-card, save, and
session-ephemeral behavior; Research and workflow composers remain unchanged.

Codex App Server, OpenCode SSE, Cursor stream-json, and Claude stream-json
helpers now cover ordered image input, explicit native/degraded failures,
model/tool output normalization, turn-scoped tool-written image detection,
session materialization, and hash dedupe. Tests use fake processes, fake SSE,
tiny image fixtures, and temporary directories only; image bytes and secrets
are not logged.

**Validation**: `gofmt`, `go test ./...`, focused adapter/media/TUI tests, and
strict OpenSpec validation are the required handoff checks. No real provider
account is used.

## 2026-09-10 — C14 QA regression hardening

**Change**: Corrected the C14 loop-back regressions. Capability admission now
returns validated attachment chips to the Free Chat composer when a model
rejects images. External source paths consult the selected harness permission
profile before media materialization. Streamed and final assets are owned by
the producing transcript turn, so cards render after that turn's response
instead of at a global transcript tail. Asset saves now prefill the Downloads
directory with the original name and require an explicit overwrite choice.
The Claude Unix process file was also normalized with `gofmt`.

**Validation**: Added focused TUI regressions; `go test ./...`,
`go test -race ./...`, `go vet ./...`, strict OpenSpec validation, `gofmt`,
and `git diff --check` passed. No image bytes or sensitive paths were logged.

## 2026-09-11 — C14 implementation loop-back wiring

**Problem**: The C14 contracts and adapter translation helpers existed, but the
live multi-harness TUI never constructed its media registry. Adapter media
capabilities therefore stayed zero, model-side discovery/catalog facts were not
intersected, and expanded asset mosaics kept stale dimensions after resize
without a progress state.

**Change**: Production TUI models now keep a `media.Registry`, register each
adapter transport, consume explicit catalog media blocks and lazy native model
discovery, admit attachment requests before Execute, and apply the admitted
intersection through the shared adapter setter. Cursor, OpenCode, Codex, and
Claude expose transport/discovery hooks; legacy catalog rows remain unknown.
Window resize schedules fresh mosaic commands for expanded cards, and pending
cards render a non-blocking spinner driven by the existing wait tick. No image
bytes, credentials, or sensitive paths are logged or written to context files.

**Validation**: Focused harness, catalog, adapter, and TUI tests pass, including
the asynchronous mosaic spinner/resize regression. Full `go test ./...` and
strict OpenSpec validation are the final handoff checks.

## 2026-09-11 — C14 Implementation gate refused: unassigned loop-back IDs

**Problem**: Implementation 4/4 stayed Running. TUI gate: reports invalid or
missing required gate fields. `generic_agent` returned `status: complete` with
canonical gates true and `tasks_completed: ["task-03.1", "task-03.2",
"task-05.1", "task-11.2"]`. OpenSpec `tasks.md` is already 38/38 `[x]`, so the
Judge loop-back wave assignment was empty (verification-only). Claiming those
unassigned (and abbreviated) IDs fails closed. `judge-gaps.md` is prompt
context only and is not the scheduler assignment.

**Change**: Did not close Implementation or start QA. Live registry/admission
and async mosaic resize/spinner wiring remain on disk from this wave. Next
`/hero-start` verification wave must report empty `tasks_completed` /
`tasks_remaining` to match the empty assignment.

**Validation**: `hero status` — Implementation Running 4/4; QA Waiting 3/3;
Judge Waiting 1/3.

## 2026-09-11 — C14 finished via /hero-finish

**Problem**: Implementation 4/4 was still Running after the TUI completion gate
refused the `generic_agent` report (unassigned abbreviated loop-back IDs on a
fully checked `tasks.md`). QA and Judge were Waiting after the last Judge
loop-back. The user issued `/hero-finish`.

**Change**: `hero finish` with last-wave metrics (`gpt-5.6-luna`, 46250 in /
8000 out tokens, ~$0.01885, 1980000 ms). Did not close Implementation/QA/Judge
as completed stages (Implementation stayed Running). Recorded cycle
`completed_at` for archive dating. Updated `current-state.md` and
`metrics-summary.md`. Stored cycle totals from the last active `hero metrics`
snapshot: 1967096 in / 134777 out tokens (~2101873 total), ~$0.5692.
OpenSpec change `tui-multimodal-images` remains linked until `/hero-archive`.
Live registry/admission and async mosaic resize/spinner wiring remain on disk.

**Validation**: `hero status` — C14 `completed`; OpenSpec still
`tui-multimodal-images`. `hero metrics` reports no active cycle.

## 2026-09-11 — C15 Research: deterministic loop-back findings and ToDos

**Problem**: C13/C14 validation loop-backs could reopen Implementation after the
OpenSpec checklist was already complete. The scheduler then had an empty
assignment while agents reported old task IDs; the gate rejected them with
generic copy. Prose gap artifacts were not scheduler state, and escalation had
no controlled path to preserve selected blockers for a later cycle.

**Decisions**: The user confirmed the complete active idea scope. Schema v11
will add scheduler-owned findings with stable `find-*` IDs, append-only
occurrences, structured ToDos, and adoption history. A typed validation report
must pass completely before one SQLite transaction persists findings, closes
the failed source stage, and performs loop-back. Implementation assignments
union owned unchecked `task-*` with owned open/reopened `find-*`; exact contract
errors name the field and offending ID. Agents emit JSON only and receive
canonical examples across every harness projection.

At Escalated, `/hero-add-todo` supports explicit partial triage; remaining work
requires separate `/hero-continue`. Deferring every blocker reconciles SQLite
and `current-state.md`, skips an empty wave/downstream validation, and closes the
cycle as `completed` with disposition `completed_with_deferred_todos`.
`/hero-finish` remains an explicitly warned emergency exit. Research startup
offers pending ToDos and records selected items as adopted; validated cycles
resolve them, while cancellation/non-validating outcomes release them to
pending. `/hero-complete-todo` manually resolves pending (never actively
adopted) structured or legacy items with confirmation and a required audit note.
Unrelated pending items were explicitly excluded from C15.

**Artifacts**: Created and registered
`docs/product/PRD-C15-001-loopback-findings-handoff.md`,
`docs/product/UI-C15-001-loopback-findings-handoff.md`, and
`docs/architecture/ADR-C15-001-loopback-findings-handoff.md` (ADR-083–090).
Updated PRD/ADR indexes, architecture overview target flow, DEPLOY migration
requirements, TESTING coverage, and current project state. No runtime code was
implemented during Research.

## 2026-09-11 — C15 Planning: loopback-findings-handoff SDD

**Problem**: Convert approved PRD-C15-001 / UI-C15-001 / ADR-083–090 into an
implementable OpenSpec change with single-owner native tasks.

**Decisions**: Change slug `loopback-findings-handoff` (persisted via
`hero cycle openspec-change`). New capabilities: findings-lifecycle,
durable-todos, structured-stage-reports, telegram-project-control. Modified:
sqlite-operational-store, runtime-workflow-execution,
cli-deterministic-command-suite, hero-tui, asset-bootstrap-and-layout.
Fingerprint is SHA-256 of NFC/whitespace-collapsed file+requirement+acceptance
with cycle/source/owner. Failed validation close shares one Store transaction
with finding writes and loop-back; CLI never nests tx. Projection uses
`todo_projection_ops` and stays Escalated until verified. Status JSON is
additive on `StatusView`. Browser UI / E2E stay disabled this cycle but their
contracts still ship. All 34 tasks owned by `generic_agent`; validators parallel
with schema v11; UI/assets after service contracts.

**Validation**: `openspec validate loopback-findings-handoff --strict` passed.
No product code implemented.

## 2026-09-11 — C15 Implementation: schema v11 migration foundation

**Change**: Bumped `internal/store` to schema version 11 with forward-only
transactional migration creating `findings`, `finding_occurrences`, `todos`,
`todo_adoptions`, `todo_projection_ops`, and `cycles.completion_disposition*`
per design D1. Added `TestMigrateV10ToV11PreservesOperationalRows` seeding v10
operational data and asserting intact rows plus empty new tables.

**Verification**: `go test ./internal/store/ -count=1` passed.

## 2026-09-11 — C15 Implementation: structured report validators (task-04.1–04.2)

**Change**: Added `internal/cycle/reports` with typed decoders for QA, Judge,
Browser UI Validation, QA End-to-End, and Implementation JSON; owner derivation
(BUI `failure_class`, Judge default owner when exactly one implementation agent
is active), Judge `sdd_ambiguity` isolation (empty gaps, no findings path),
empty-success arrays on pass, and `ReopenIDValidator` / `ActionableFindingChecker`
hooks for store-backed checks without importing `store`.

**Diagnostics**: Stable codes (`invalid_json`, `unknown_field`, `missing_field`,
`invalid_enum`, `invalid_owner`, `unknown_reopen_id`, `duplicate_id`,
`overlapping_arrays`, `assignment_union_mismatch`, `unassigned_id`,
`false_acceptance_gate`, `nonempty_empty_assignment`, `no_actionable_finding`)
with field path, value, and rule on `DiagnosticError`.

**Verification**: `go test ./internal/cycle/... -count=1` passed.

## 2026-09-11 — C15 Implementation: assignment union merge and gates (task-08.1–08.2)

**Change**: Merged OpenSpec tasks with actionable findings in
`internal/tui/implementation_assignment.go`; TUI dispatch loads findings via
`Store.ListActionableFindings`, rejects out-of-scope finding owners before
Execute, and validates reports through exported `reports.ValidateAssignmentUnion`.
Updated assignment prompt copy for mixed `planned`/`findings` IDs and empty
verification waves.

**Verification**: `go test ./internal/tui/ -count=1 -run 'Assignment|Implementation|Union|Finding'` and `go test ./internal/cycle/reports/ -count=1` passed; full `./internal/tui/` green.

## 2026-09-11 — C15 Implementation: engine atomic failed-close handoff (task-05)

**Change**: Added `Engine.CloseStageFailedWithFindings` with report decode before
`store.InTx`, tx-aware store helpers (`UpdateStageTx`, `AppendEventTx`,
`AddConversationTx`, `UpsertMetricTx`, `GetMetricTx`, `GetStageTx`, `ListStagesTx`),
`PredictFindingActionable` for deferred/rediscovery checks, loop-back payloads with
`finding_ids`, and fail-closed tests (invalid report, mid-tx hook, standalone loop-back).

**Verification**: `go test ./internal/engine/ ./internal/store/ ./internal/cycle/reports/ -count=1` passed.

## 2026-09-11 — C15 Implementation: cycle service handoff façade (task-06.1–06.2)

**Change**: Added `internal/cycle/handoff.go` (delegates atomic failed close to engine without nested `InTx`; assignment union via `internal/cycle/assignment.go`; ToDo defer/complete/adopt/release/resolve and projection-op hooks; `CloseImplementationWhenAssignmentEmpty` and `CompleteCycleWithDeferredTodos`). Engine gained `implementation_close.go` for empty-assignment Implementation close and deferred-work terminal disposition (`EventCycleCompletedDeferredTodos`). Store: `ListAdoptedTodoIDsForCycle`, `ReleaseAdoptedTodosForCycle`.

**Verification**: `go test ./internal/cycle/ ./internal/engine/ ./internal/store/ -count=1` passed.

## 2026-09-11 — C15 Implementation: Research ToDo adoption (task-14.1-research)

**Change**: `internal/store.ListPendingTodos`; cycle `ListPendingTodos` / `AdoptTodosForResearch` and CLI `hero adopt-todo`; TUI `research_todos.go` injects pending-ToDo adoption context after idea notes in `startDiscoverResearchSession`; `tuiDiscoverResearchPreamble` and canonical `discover_agent.md` reference `hero adopt-todo` and persistence-failure gating before grilling.

**Verification**: `go test ./internal/tui/ -count=1 -run Research -timeout 60s` passed.

## 2026-09-11 — C15 Implementation: current-state projection (task-10.1, task-10.2)

**Change**: Added recoverable projection in `internal/todos` (`projection.go`, `legacy.go`): build candidate from SQLite pending/adopted rows plus unmatched Pending prose, fsync under `.workflow-hero/tmp/`, advance `todo_projection_ops`, atomic install to `context/current-state.md`, verify IDs/hashes before `verified`. `PromoteSelectedLegacyLine` allocates `todo-N` only for an explicitly selected legacy line. Store gained `ListTodosForProjection`; cycle `ReconcileTodoProjection` delegates to `todos.ReconcileProjection`; `AddFindingTodos` reconciles after defer and defers cycle completion until actionable findings are empty.

**Verification**: `go test ./internal/todos/ ./internal/store/ ./internal/cycle/ -count=1` passed.

## 2026-09-11 — C15 Implementation: TUI stage handoff parse/apply (task-09.1–09.2)

**Change**: `internal/tui/stage_handoff.go` decodes QA/Judge/BUI/E2E validation reports via `internal/cycle/reports`, calls `CloseStageFailedWithFindings` on failed closes, blocks orchestrator stage-close/loop-back on scheduler-handled failures, applies verified Implementation `task-*` OpenSpec checks and `find-*` `MarkFindingDoneTx`, emits UI-C15-001 §§3–5 Chat copy, and closes empty Implementation workloads without another wave. Added `ValidationDecodeContext` on cycle/engine, `reports.ReportJSONFromText`, and chat preambles for scheduler-owned validation failures.

**Verification**: `go test ./internal/tui/ -count=1 -timeout 120s` passed.

## 2026-09-11 — C15 QA loop-back: eight generic_agent defects

**Change**: Closed the QA loop-back for findings-only Implementation. `store.GetTodo` now maps `sql.ErrNoRows` to `ErrNotFound` so manual complete can promote legacy Pending prose. Projection suppresses promoted/resolved legacy prose via `ListResolvedLegacySummaries` while preserving unmatched lines. TUI complete-todo preselect allows unknown legacy prose through to the service. Removed unused soft Implementation report parsers; typed `DecodeImplementation` remains the only path. Regression coverage: no-active-cycle complete, legacy promote+project, Research adopt projection, Cancel release / Finish resolve hooks, open-finding rediscovery actionable prediction, findings-only partial wave progress, unknown-field report rejection.

**Verification**: `go test ./... -count=1` and `openspec validate loopback-findings-handoff --strict` passed.

## 2026-09-11 — C16 Research: persistent TUI session history

**Problem**: Free Chat identity/transcript is process-local, cycle/stage rows retain
only partial native session bindings, and the TUI has no durable surface to find,
name, resume, archive, restore, or delete past conversations.

**Decisions**: History is project-scoped and covers every conversation executed or
observed by Hero TUI, including Telegram origin and distinct parallel stage-agent
sessions; external IDE chats remain out. Sessions are created on first accepted
turn, named deterministically, persisted incrementally as normalized visible events,
and resumed only with the original harness/model/properties. Failed native resume
offers an explicit context fork. Interrupted turns remain visible and recover through
native status/reconnect where supported. One database lease permits one continuation
owner. Active/archive views support name search, rename, restore, and confirmed
permanent deletion. Local deletion removes only Hero-owned data/assets and succeeds
even when best-effort provider deletion fails; user source files are never removed.
Legacy harness bindings migrate idempotently without invented transcript content;
remote transcript import requires confirmation.

**Architecture/UI**: Proposed schema v12 session aggregate/event/asset/lease/import
state (ADR-091–098), without repurposing the cycle audit `conversation` table.
`internal/conversation` owns lifecycle for TUI/Telegram; Bubble Tea performs all I/O
through commands. History follows Chat in the navbar and uses responsive list/detail
states. ADR-080 is amended for durable sessions: linked managed assets do not expire
by age. The user explicitly required Implementation to use the repository
`golang-tui` and `go-engineering` skills.

**Artifacts**: Created and registered
`docs/product/PRD-C16-001-tui-session-history.md`,
`docs/product/UI-C16-001-tui-session-history.md`, and
`docs/architecture/ADR-C16-001-tui-session-history.md`; updated PRD/UI/ADR indexes,
architecture overview, DEPLOY migration requirements, TESTING coverage, and current
project state. No product code was implemented during Research.

**2026-09-11 — C16 Implementation (store schema foundation)**: Bumped
`internal/store` schema to **v12** with forward-only migration creating
`sessions`, `session_events`, `session_assets`, `session_leases`, and
`session_delete_ops` plus D1 indexes; audit `conversation` unchanged. Added
`TestMigrateV11ToV12PreservesOperationalRowsAndEmptySessionTables` (openCapped v11
fixture → Open v12). `go test ./internal/store/...` passes. Session service/CRUD and
D11 legacy backfill not in this slice.

## 2026-09-11 — C16 Implementation wave (generic_agent)

**Outcome**: Shipped schema v12 + store session/events/leases/assets/delete-ops + SessionService/titles + optional harness RemoteHistoryReader/NativeSessionDeleter (OpenCode history only) + durable media retention + idempotent legacy bindings + History screen/navbar + Chat persist/restore/new-chat + exact resume/lease/historical continuation/interrupt/fork/archive/delete + Telegram origin labels + confirmed remote import path. `openspec validate tui-session-history --strict` and `go test ./...` green.

**Validation**: openspec validate --strict; go test ./...; go vet on touched pkgs; gofmt clean on session/history files.
