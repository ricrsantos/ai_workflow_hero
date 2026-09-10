# Context Log

> Short-term project memory for this repository (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Permanent facts belong in `context/current-state.md`.

## 2026-09-10 — Test binary artifact isolation

**Change**: Added `/hero-telegram-daemon` and `/temp/` to `.gitignore`. `docs/testing/TESTING.md` now prohibits repository-root binaries, requires temporary binaries from tests, validation, and local build checks to use `./temp/`, and requires cleanup on both success and failure. Updated local build examples in `README.md`, `docs/deployment/DEPLOY.md`, and the embedded user help to use `./temp/hero`; added `TESTING.md` to the bilingual README documentation maps.

**Document hierarchy**: `docs/testing/TESTING.md` remains a living, unnumbered document and is present in the active `.workflow-hero/config/documents.json` registry and the generated `openspec/config.yml` context, alongside the existing `AGENTS.md`, `ADR.md`, and architecture-overview references. The empty `assets/config/documents.json` remains the bootstrap template.

**Validation**: `git diff --check`, registry JSON parsing, reference assertions, and the isolated OpenCode usage test passed. The full `go test ./... -count=1` run still reproduces the unrelated `internal/adapters/opencode/TestExtractOpenCodeUsageAccumulatesStepFinishes` failure (`context=13`, want `24`); no root executable or `./temp` artifact remained.

## 2026-09-10 — Stabilized OpenCode usage test

**Change**: Replaced the unordered `map` fixture in `TestExtractOpenCodeUsageAccumulatesStepFinishes` with an ordered slice so the `step-finish` event with `20 + 4` tokens is deterministically last. This matches the adapter contract: billed input/output accumulate, while `ContextTokens` represents the last model-call occupancy.

**Root cause**: Map iteration order made the test flaky. When the `20 + 4` event was processed first, the later `10 + 3` event correctly left `ContextTokens=13`, but the test always expected `24`.

**Validation**: The focused test passed 30 consecutive runs and `go test ./...` passed after the change.

## 2026-09-09 — Development Telegram auto-update

**Change**: Added ADR-076 and the development-only `/auto-update` flow. The TUI command commits safe working-tree changes and sets `needs-update.txt`; a systemd user timer runs `hero-update.sh` every five minutes. The updater invokes the new `scripts/build_update.sh`, which builds only `./cmd/hero` for the host OS/architecture, atomically replaces the installed binary, signals running TUIs through versioned Telegram IPC, and preserves their terminal/process identity. Existing `build_dev.sh` and `release.sh` were not changed.

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
