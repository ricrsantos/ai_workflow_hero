# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

---

## 2026-08-20 — C6 Research: Codex Adapter (Hero 2.5.0)

**Problem**: Users cannot run OpenAI Codex as a Hero TUI harness (C4 left Codex out of scope).

**Decision / Outcome**: Research complete. OpenCodeAdapter is the behavioral spec; idea file `docs/idea/v2.5_codex_adapter/codex_adapter.md` yields on divergence. TUI-only. ChatGPT `codex login` outside TUI (no API key). PATH CLI, no version pin. Projection `assets/codex/` → `.codex/`. Out: MCP, images, web search. Docs: PRD-C06-001, ADR-043–048, UI-C06-001; DEPLOY/TESTING/indexes updated.

---

## 2026-08-20 — Release v2.4.1 (TUI UX patch)

**Problem**: Ship TUI conversation improvements accumulated since v2.4.0.

**Decision / Outcome**: Bumped default version to `2.4.1` in `cmd/hero/main.go`. Tagged `v2.4.1` on `main` after `go test ./...` green. Ships bordered prompt box, clipboard copy for assistant messages, context bar refinements, and model picker display fixes. Artifacts via `scripts/release.sh`; GitHub Release https://github.com/ricrsantos/ai_workflow_hero/releases/tag/v2.4.1

## 2026-08-20 — TUI shortcut hints consolidated in footer

**Problem**: Chat duplicated keyboard hints on three lines (property row, input row, footer).

**Decision / Outcome**: Removed `↑↓ scroll` from the property/context row; removed the hint row under the input box; footer carries Chat shortcuts plus compact `alt+n screens`. Tab bar shows numbered screens (`1. Chat │ 2. Status │ …`). `go test ./internal/tui/...` green.

## 2026-08-20 — TUI property status labels readability

**Problem**: Chat status-line property chips `[fs-na] [th-na] [ef-na]` were hard to read.

**Decision / Outcome**: Display-only prefixes changed to `[fast-*] [thinking-*] [effort-*]` via `propertyStatusLabelPrefix` in `internal/tui/model_properties.go`; internal keys (`fs`/`th`/`ef`) unchanged. Tests updated; `go test ./...` green.

## 2026-08-20 — Release v2.4.0 (OpenCode serve lifecycle)

**Problem**: Ship OpenCode `serve` process ownership so TUI quit, terminal close, and stale registry rows no longer leave orphan or zombie servers.

**Decision / Outcome**: Tagged `v2.4.0` on `main` after `go test ./...` green. Ships serve lifecycle manager (`TerminateManagedProcess`, registry `project_path`, signal cleanup), shutdown via live store, and zombie/orphan reap (`Pdeathsig`, Kill+Wait, `/proc/net/tcp` fallback). Artifacts via `scripts/release.sh`; GitHub Release with binaries + `checksums.txt`.

## 2026-08-20 — OpenCode zombie/orphan fix (v2.4)

**Problem**: Closing Cursor terminal left `opencode serve` running; restart showed `[opencode] <defunct>` zombie and spawned a second server on next prompt.

**Root cause**: (1) Kill without `Wait()` left zombies when Hero reaped/killed foreign PIDs. (2) No `Pdeathsig` — child survived abrupt parent death. (3) Startup reap skipped stale registry rows when PID was zombie/wrong but URL still live.

**Decision / Outcome**: Store `serveHandle` and `stopProcessHandle` (Kill+Wait) for Hero-owned children; `terminateAndReap` + `waitProcessExit` for registry orphans; Linux `Pdeathsig: SIGTERM` on spawn; `processZombie` detection; URL/port fallback reap via `/proc/net/tcp`. `go test ./...` green.

## 2026-08-20 — OpenCode serve shutdown fix (v2.4)

**Problem**: TUI quit (ctrl+q) left `opencode serve` running; second launch spawned another server. `harness_serve_registry` stayed empty.

**Root cause**: `bootHarness` opened its own SQLite store and `defer st.Close()` before returning the registry. The OpenCode adapter kept a closed store — `registerServe` failed silently, so shutdown had no PID/registry to terminate.

**Decision / Outcome**: `bootHarness` now accepts the caller's live `svc.Store` (no close on shared store). `stopOpenCodeServe` stops via registry singleton adapter (in-memory PID) then store registry + orphan reap. Added `TestEnsureServeRegistersWithLiveStore`. `go test ./...` green.

## 2026-08-20 — OpenCode serve lifecycle manager (v2.4)

**Problem**: `opencode serve` children could survive unexpected Hero exits; shutdown used immediate `Kill()` without identity checks; registry lacked `project_path`.

**Decision / Outcome**: Extracted `internal/adapters/opencode/server.go` — `TerminateManagedProcess` (SIGTERM → wait → SIGKILL), `IsManagedOpenCodeServe` (cmdline check), `ReapOrphanServers` (startup GC), `PruneStaleServeRegistry` + `RunServeWatchdog` (TUI session). Schema **v6** adds `harness_serve_registry.project_path`. TUI launch registers signal cleanup (SIGINT/SIGTERM/SIGHUP) with `sync.Once` stop hook. Hero only terminates processes it registered and that pass cmdline identity. `go test ./...` green.

## 2026-08-19 — Release v2.3.0 (harness health watchdog)

**Problem**: TUI Execute could hang indefinitely when a harness process died, the serve became unavailable, or streaming stalled without surfacing an error.

**Decision / Outcome**: Tagged `v2.3.0` on `main` after `go test ./...` green. Ships `HarnessHealth` / `HealthChecker` / `Watchdog` (Cursor + OpenCode adapters), TUI stall prompts (cancel/wait/restart), auto-cancel on failure, and empty-response warnings in Chat. Artifacts via `scripts/release.sh`; GitHub Release with binaries + `checksums.txt`.

## 2026-08-19 — AGENTS.md: architecture-overview maintenance policy

**Decision / Outcome**: Documented mandatory creation and update of `docs/architecture/architecture-overview.md` on architectural changes (synthetic, diagrams over prose). Updated repo `AGENTS.md`, `assets/templates/AGENTS.md`, `context_agent`, and `/hero-sync` to generate the overview on sync when missing and register it in `documents.json`.

## 2026-08-19 — Release v2.2.0 (OpenCode prepare + harness reset)

**Problem**: After v2.1.2, TUI still needed a way to restart harnesses after agent edits, and `/hero-start` could dispatch OpenCode agents with stale `.opencode/agents` frontmatter.

**Decision / Outcome**: Tagged `v2.2.0` on `main` after `go test ./...` green. Ships TUI `/harness-reset`, OpenCode `PrepareHeroStart` (sync agents, reset serve, probe), and upgrade/`update-models` conflict backup+replace. Artifacts via `scripts/release.sh`; GitHub Release with binaries + `checksums.txt`.

## 2026-08-19 — TUI `/harness-reset` slash command

**Problem**: After editing agents/skills, harnesses need a restart (OpenCode serve, Cursor in-flight runs) without leaving the TUI.

**Decision / Outcome**: TUI-only `/harness-reset` opens a picker of enabled harnesses. OpenCode: `StopServe` when Hero-managed (else warns not started). Cursor: cancel in-flight CLI via adapter. Clears session binding when matched. Not-initialized cases use yellow `statusWarn` (not error). Picker loads async with braille wait animation; Enter is ignored until the list is ready. `go test ./...` green.

---

## 2026-08-19 — Upgrade/update-models conflict backup + replace

**Problem**: `hero upgrade` and `hero update-models` skipped locally customized files (checksum mismatch) without applying updates.

**Decision / Outcome**: Added `internal/common/assetconflict` — on conflict, save `{filename}_{timestamp}.conflict` backup, write new content, warn per file. `upgrade.Result.Skipped` → `Replaced`. `update-models` now loads/writes `checksums.json` with the same logic. Exported `install.LoadChecksums` / `WriteChecksums`. `go test ./...` green.

---

**Decision / Outcome**: V2.1.1 harness stream normalization shipped: `StreamDelta` extended, OpenCode SSE full event map + permission flow, Cursor unknown-event warnings, TUI harness permission prompt. Plan: `docs/idea/V2.1.1_fix_adapters_streams/implementation-plan.md`; architecture: `docs/architecture/architecture-overview.md`.

---

## 2026-08-19 — OpenCode external_directory permission reply fix

**Problem**: Hero TUI stalled during corrections when OpenCode asked `external_directory` permission (e.g. reading `~/.nvm/.../opencode-ai/`). Infrastructure existed (`permission.asked` → `OnPermissionRequest` → status bar `Allow? [y/N]`) but reply routing was wrong: `directory` was sent in JSON body instead of query param; v2 events use `action`/`resources` not `permission`/`patterns`; prompt was easy to miss (status bar only).

**Decision / Outcome**: `replyPermission` now sends `{reply}` body + `?directory=` query (session-scoped fallback on 404); `handlePermissionAsked` maps v1+v2 fields and metadata filepath; SSE subscribe uses `?directory=`; TUI also inserts permission prompt into chat transcript. Tests for `external_directory` v2. `go test ./...` green.

---

## 2026-08-19 — Cursor adapter permission parity

**Problem**: Cursor adapter had no permission handling; only `--force`/`--trust`/`--sandbox disabled`. MCP tools could still stall with "User rejected" in headless mode; unknown `permission_request` NDJSON lines emitted generic warnings.

**Decision / Outcome**: Added `--approve-mcps`; `ParseStreamJSONWithOptions` handles `permission_request`/`permission` via `OnPermissionRequest` (no stream-json reply channel — CLI flags resolve permissions); detects denied tool results and emits actionable warnings. Tests in `parse_permission_test.go`. `go test ./...` green.

---

## 2026-08-18 — Release v2.1.1 (OpenCode harness stream fix)

**Problem**: OpenCode TUI chat dropped or truncated assistant responses when `session.next.text.*` and `message.part.updated` diverged.

**Decision / Outcome**: Tagged `v2.1.1` on `main` after `go test ./...` green. Harness stream normalization (`StreamDelta`, `events.go` content-based dedup, `hero --debug`, permission flow). Artifacts via `scripts/release.sh`; GitHub Release with binaries + `checksums.txt`.

---

## 2026-08-18 — OpenCode response stream regression fix

**Problem**: TUI showed only thinking + a tiny text fragment; full assistant response missing after delta-only emit policy.

**Decision / Outcome**: OpenCode v2 `session.next.text.*` events carry `assistantMessageID` — adapter sets `assistantMsgID` from them. Text dedup now uses `emittedText` content prefix (not byte length): token deltas accumulate only; `message.part.updated` and `text.ended` emit authoritative full text; divergent paths recover via full re-emit. `go test ./...` green.

---

## 2026-08-18 — Hero --debug chat event filtering

**Problem**: OpenCode observability events (`session.updated`, `plugin.added`, step markers, etc.) cluttered the TUI chat.

**Decision / Outcome**: Wired global `hero --debug` (`internal/common/debug`) into `ExecuteRequest.Debug`; OpenCode adapter suppresses debug-only activity unless debug is on. Streaming: `session.next.text.delta` live when no `message.part` for same textID; `*.ended` + `message.part.updated` authoritative; `assistantMessageID` from v2 events unlocks part filtering.

---

**Problem**: Hero OpenCode adapter and `event_streams_improvements.md` listed obsolete events (`tool.execute.*`, `shell.env`, `lsp.client.diagnostics`) and missed the v2 `session.next.*` family from `anomalyco/opencode` branch `dev`.

**Decision / Outcome**: Extracted canonical list from `packages/schema/src/event-manifest.ts` (87 schema events + transport). Updated `docs/idea/V2.1.1_fix_adapters_streams/event_streams_improvements.md` and `internal/adapters/opencode/events.go` (v2 text/reasoning/tool/shell streaming, `sync` unwrap, `permission.v2.asked`, activity map). Tests added for `session.next.text.delta` and `sync`. `go test ./...` green.

---

## 2026-08-18 — V2.1.1 harness stream events

**Problem**: OpenCode streaming ignored most SSE events (no tools/thinking/permissions); unknown events were dropped silently; harness permission stalls had no TUI feedback.

**Decision / Outcome**: Implemented normalized `StreamDelta` kinds + `OnPermissionRequest`; `internal/adapters/opencode/events.go` handles documented OpenCode event families; Cursor emits warnings for unknown NDJSON types; TUI shows warnings/activity and harness permission prompt in status bar. OpenSpec `harness-adapter` updated. `go test ./...` green.

---

## 2026-08-18 — Release v2.1.0 (C5 model properties)

**Problem**: Ship C5 model-property TUI (dynamic `fs`/`th`/`ef`, API/cache/catalog resolution, adapter transport) as a minor release after v2.0.2.

**Decision / Outcome**: Tagged `v2.1.0` on `main` after `go test ./...` green. Artifacts via `scripts/release.sh`; GitHub Release published with binaries + `checksums.txt`.

---

## 2026-08-18 — Chat slash overlay after property-picker Esc

**Problem**: Esc from `/hero-model` property picker broke Chat `/` autocomplete; Esc before properties or Enter (save) did not.

**Root cause**: `cancelPropertyDraft` set `pickingProps=false` before `closePalette()`, so `wasPicking` was false and `reloadPaletteItems()` was skipped — stale model-picker palette rows filtered out by `chatSlashOverlayItem`.

**Decision / Outcome**: Defer `pickingProps` reset to `closePalette()`; regression test `TestPropertyPickerEscapeRestoresChatSlashOverlay`.

---

## 2026-08-18 — C5 model-properties refresh + slug locks (TUI)

**Problem**: After selecting a model during `/hero-model` background refresh, the TUI showed `No catalog is available` because Cursor refresh persisted only model lists (no capabilities) and OpenCode cache applied only on the next picker open. Variant slugs (`cursor-grok-4.6-low`) did not lock fixed properties.

**Decision / Outcome**: `internal/adapters/cursor/capabilities.go` infers `fs`/`th`/`ef` from `ListModels` and applies slug locks; `modelprops.Service` persists inferred caps and adds `SnapshotCacheOnly`. TUI waits with braille animation when selection happens during refresh, then applies cache-only snapshot (catalog fallback only when refresh fails). Cursor snapshots apply slug locks in the picker (gray locked rows). Catalog YAML + base-slug lookup already landed earlier. `go test ./...` green.

---

## 2026-08-17 — OpenCode serve process leak (TUI)

**Problem**: Each TUI interaction with OpenCode spawned a new `opencode serve` child; dozens of processes accumulated and exhausted RAM/swap.

**Root cause**: `DefaultRegistry.Adapter()` returned a **new** `opencode.Adapter` on every call, so in-memory `baseURL`/`servePID` were always empty and `ensureServe` started another serve. `StopServe` on TUI quit used a fresh adapter (`servePID=0`), only cleared SQLite registry, and used Unix `kill` — processes survived (especially on Windows).

**Decision / Outcome**: Cache singleton adapters in `DefaultRegistry`; `ensureServe` adopts a live serve from `harness_serve_registry` via HTTP health check before spawning; `StopServe` kills all registry PIDs with `os.FindProcess` + `Kill()`; orphan reap uses URL liveness + cross-platform kill. Cursor adapter unchanged — one short-lived CLI process per Execute is by design (`--resume` for continuity).

---

## 2026-08-19 — Harness health watchdog + empty response warning (v2.3, TUI)

**Problem**: TUI Execute could wait indefinitely on a stalled harness with no user feedback; successful Execute with zero agent output left a blank response pane.

**Decision / Outcome**: Added `HarnessHealth` / `HealthChecker` / `Watchdog` in `internal/harness`; Cursor and OpenCode adapters implement `CheckHealth` (OpenCode uses `GET /global/health` with `/config/providers` fallback). TUI runs 10s health probes during streaming, prompts on `suspected_hang` (cancel/wait/restart), auto-cancels on `failed`, and warns on empty successful output via `convRoleWarning`. Cursor `stream-json` with no substantive output returns an explicit error. Scope is TUI harness Execute only — not Cursor IDE chat Runtime. `go test ./...` green.

---

## 2026-08-15 — OpenCode TUI chat hang ("Waiting for harness")

**Problem**: Chat with OpenCode harness stuck on "Waiting for harness…" — no stream deltas, no error.

**Root cause**: `OpenCodeAdapter` had three integration bugs vs real `opencode serve` 1.18.x: (1) `defaultServeURLResolver` hardcoded `:4096` instead of parsing `listening on http://…` from the child process; (2) `Execute` sent `model` as a string (`provider/model`) but the API expects `{providerID, modelID}`; (3) `/event` is SSE (`data: …`) with `message.part.updated` / `session.idle`, not raw JSON lines — decode loop never saw text or completion.

**Decision / Outcome**: Fixed adapter: stdout URL scan on `ExecRunner`, SSE reader, incremental part text deltas, HTTP error surfacing on failed `prompt_async`. Follow-up: ignore user `message.part.updated` events until assistant message id is known (OpenCode echoes user text in SSE before assistant reply). Boot: skip false `not in harness catalog` for OpenCode defaults — aggregate `ListModels` intentionally omits OpenCode at boot to avoid starting `opencode serve`. TUI palette/slash overlay order: Hero slashes first (user-specified), then Go to screens, then Refresh/Quit/imported commands.

---

**Problem**: TUI auto-selected `composer-2.5`; `/hero-model` listed only Cursor models; harness screen was unclear.

**Decision / Outcome**: Do not invent `freechat_default.model`. `/hero-model` lists only **enabled** harnesses (no availability checks on open; skip submenu when only one). Model list fetched on demand per harness. OpenCode `/config/providers` models parsed as object map. `/hero-harness` remains checkboxes with `(available)`/`(unavailable)`.

---

## 2026-08-15 — C4 finished (Hero 2.0.0 multi-harness)

**Problem**: Close cycle after Research→Judge; Browser UI / E2E were skipped.

**Decision / Outcome**: `hero finish` recorded `completed_at`. All 47 SDD tasks landed (OpenCode adapter + managed serve/HTTP, `/hero-harness`, `/hero-model` pair, native model ids, `--tools` removed). Judge gaps fixed: StopServe on quit/disable, no cross-harness session resume, two-step fallback only. Totals: 224200 tokens, ~$0.406. Archive next via `/hero-archive` (OpenSpec change `hero-2-0-multi-harness`).

---

## 2026-08-15 — C4 Research + Planning

**Problem**: Hero 1.x is Cursor-only; need TUI multi-harness without breaking IDE Runtime.

**Decision / Outcome**: PRD/ADR/UI C04 + SDD `hero-2-0-multi-harness`. TUI-only multi-harness; native model ids; `--tools` error; OpenCode via Hero-managed `opencode serve` + HTTP API; project `hero.db` orphan reap; enable provisions `.opencode/`; `models/*.yml` OpenCode ids. Cursor IDE ignores `harness`.

---

## 2026-08-15 — Architecture overview (codebase audit)

**Problem**: High-level architecture lived only in scattered ADRs and an untracked overview file.

**Decision / Outcome**: Living `docs/architecture/architecture-overview.md` registered in `documents.json`. C4 later added OpenCode / `harnessmgr` / schema v4 (see current-state).

---

## 2026-08-15 — TUI Chat context bar

**Problem**: Chat had no view of context-window fill.

**Decision / Outcome**: `context_window` in `assets/models/*.yml`; TUI scroll-hint bar from last Execute `usage`. Not a live mid-stream meter.

---

_Older 2026-08-14 TUI notes (iterations, orch/discover models, wrap panic, Alt+Enter, streaming nav) are in git history._

---

## 2026-08-17 — C5 started (model properties selection in TUI)

**Problem**: `/hero-start` for C5 ("seleção das propriedades dos modelos na TUI") in the Hero TUI.

**Decision / Outcome**: Configuration validated: scope `native` only (→ generic_agent on Implementation); Browser UI Validation and QA End-to-End disabled (gates N/A); research auto-advances (no human approval), planning requires approval. Chat language PT-BR.

**Exceptions**:
- `hero stage start --name configuration` → "not found": the SQLite store tracks only Research→E2E, no Configuration row. Configuration metrics will be folded into the Research close as a multi-entry metrics-json array (entry 1: configuration/orchestration_agent, entry 2: research/discover_agent).
- No pricing entry for `opencode-go/deepseek-v4-pro` (or any opencode-go id) in `.workflow-hero/models/`; `hero update-models` upstream ships no `opencode.yml`. Cost for orchestration-stage models is recorded as 0.00 pending catalog availability (tokens/duration still recorded).

---

## 2026-08-17 — C5 Research completed (dynamic model properties)

**Problem**: The TUI `/hero-model` flow selected only a harness/model pair, leaving model-specific fast, thinking, and reasoning-effort properties implicit and invisible in Chat.

**Decision / Outcome**: Requirements confirmed and documented in PRD-C05-001, ADR-C05-001 (ADR-038–042), and UI-C05-001. C5 will support `fs`, `th`, and `ef` with dynamic values obtained from an optional harness capability API, a project-scoped `hero.db` cache, or `assets/models/*.yml`. `/hero-model` refreshes enabled harnesses in background without OpenCode boot preloading; choices persist by harness/model in `hero.json`; adapters own API mapping and rejected properties fail explicitly; workflow agent YAML remains authoritative during stages. Planning is pending.

---

## 2026-08-17 — C5 Planning completed (model properties TUI)

**Decision / Outcome**: Created OpenSpec change `model-properties-tui` under `openspec/changes/model-properties-tui/`. The SDD defines 19 normalized/cache/persistence/TUI/adapter/workflow requirements and 22 independently testable native tasks. Catalog, SQLite, hero.json, adapter, and workflow-contract tracks can run in parallel after the shared contract; TUI picker/status tracks can run in parallel after state integration. The active cycle stores the OpenSpec slug for archive coupling.

---

## 2026-08-17 — C5 generic_agent model probe (pre-Implementation diagnostic)

**Problem**: User asked to probe the generic_agent identity before `/hero-start`. The TUI appeared frozen ("solicitou alguma permissão que não chegou").

**Decision / Outcome**: Task dispatch to generic_agent returned `completed` with answer `deepseek-v4-pro` (matches `agents.generic_agent.model`). On resume-interrogation the subagent confirmed it never executed any tool and had no pending permission request — the freeze was a TUI/harness rendering artifact, not a subagent permission stall. Probe passed: model routing resolves the configured `opencode-go/deepseek-v4-pro`.

---

## 2026-08-18 — C5 resume: sync .opencode/agents models with workflow-config.yml

**Problem**: Before restarting Implementation after `/hero-resume`, the `.opencode/agents/*.md` frontmatter models drifted from `workflow-config.yml` `agents.*` blocks (context_agent/qa_agent/end2end_qa_agent still pointed at `kimi-k2.7-code`; orchestration/generic/judge/browser at `deepseek-v4-pro`).

**Decision / Outcome**: Synced every agent frontmatter to the opencode harness IDs + reasoningEffort/thinking in the C5 workflow-config (orchestration/context/qa/judge/browser/end2end → `opencode/deepseek-v4-flash-free`; generic → `opencode-go/gpt-5.6-luna`; discover/planning/backend/frontend already matched). `reasoning_effort: na` → omit `reasoningEffort`; `thinking: na` → omit `thinking`. opencode.json untouched. Ready for `/hero-start`.

---

## 2026-08-18 — C5 Implementation completed (model properties TUI)

**Decision / Outcome**: Completed all 22 native `generic_agent` tasks in `openspec/changes/model-properties-tui/`. The implementation adds normalized `fs`/`th`/`ef` contracts, optional OpenCode discovery and Cursor-safe composition, schema-v5 project cache, embedded/installed catalog fallback, atomic per-pair `hero.json` persistence, background refresh at `/hero-model` open, the Bubble Tea property picker, responsive status labels/warnings, workflow-YAML projection, explicit rejection errors, and Runtime help/inventory assertions. Existing C4 Cursor/OpenCode/session/lazy-serve behavior remains green. `go test ./...`, `go vet ./...`, and targeted race checks passed; no browser or web work was introduced. The active cycle remains native-only with OpenSpec slug `model-properties-tui`.

---

## 2026-08-18 — C5 catalog metadata for Cursor and OpenCode Go

**Problem**: `/hero-model` showed `No catalog is available` and skipped the property submenu for common pairs (`cursor-grok-4.6-low`, `opencode-go/gpt-5.6-luna`) because embedded/installed YAML had pricing only (Cursor) or a single OpenCode fixture.

**Decision / Outcome**: Added C5 `properties` blocks to `assets/models/cursor.yml` and `assets/models/opencode.yml` (mirrored in `.workflow-hero/models/`). Cursor base rows (`composer-2.5`, `cursor-grok-4.5`, `cursor-grok-4.6`) define `fs`/`ef`; variant slugs inherit via new `CatalogValues` base-suffix stripping. OpenCode Go catalog covers `deepseek-v4-pro`, `gpt-5.6-luna`, and `glm-5.3`. Plan todos updated; `go test ./...` green.

---

## 2026-08-18 — C5 Judge escalation: judge_agent returns empty in opencode harness

**Problem**: During /hero-start resume, Judge was dispatched to judge_agent via Task. Six attempts all returned `completed` with an EMPTY task_result: (1) full SDD-coverage prompt, (2) resume of that session, (3) fresh full prompt, (4) fallback frontmatter without `thinking: false`, (5) minimal frontmatter (model only, same shape as working qa_agent), (6) one-word smoke test.

**Decision / Outcome**: The agent itself fails to emit any output in this harness — not the prompt, not the model config (qa_agent with identical `opencode/deepseek-v4-flash-free` config returns fine; agent registry healthy). Per iteration-exhaustion rules, Judge closed as Failed (Human Approval = Escalated) and the cycle waits for `/hero-continue` (after fixing judge_agent) or user decision (/hero-approve, /hero-reject, /hero-cancel, /hero-finish). Fallback per ADR-008 was applied to judge_agent.md frontmatter (now `model: opencode/deepseek-v4-flash-free` only) — no effect, re-sync may be needed. QA minor findings (t.Skip condicional em model_properties_test.go:393; branch duplicado em property_picker.go:169) permanecem não tratados (não bloqueantes).

---

## 2026-08-18 — C5 Judge iteration 2 still empty; waiting for user direction

**Problem**: /hero-continue 1 granted an extra iteration (engine events: escalated reason=timeout → continued extra=1 → stage_started judge iteration=2/4). The engine's un-fail path was `hero run --stage judge` (escalate→continue→start; the Cursor push itself hung and was killed by shell timeout). Fresh judge_agent dispatch on iteration 2 returned EMPTY again — 7th consecutive empty return (full prompt x3, resume x1, fallback configs x2, smoke test x1). judge_agent cannot emit output in the opencode harness; qa_agent with identical model config works.

**Decision / Outcome**: Stage left Running (2/4) while waiting for user direction. Options presented: (a) authorize substitute agent (qa_agent or general) for SDD coverage verification, (b) /hero-approve without formal Judge (QA passed; indirect coverage evidence exists), (c) fix judge_agent/harness (restart TUI, re-sync .opencode/agents) then /hero-continue, (d) /hero-reject, /hero-cancel, /hero-finish. No substitute dispatched without explicit authorization.

---

## 2026-08-18 — C5 Judge approved by user without formal verification

**Decision / Outcome**: User ran /hero-approve, accepting the current state without formal Judge SDD-coverage verification (judge_agent broken in the opencode harness — 7 empty returns). QA verdict was PASS with coverage evidence for all 22 SDD tasks; architecture validated against ADR-038–042. Judge closed as Completed on user approval. Remaining known items: 2 minor QA findings (t.Skip condicional em internal/tui/model_properties_test.go:393; branch duplicado em internal/tui/property_picker.go:169) and judge_agent.md frontmatter left in fallback state (model only — diverges from workflow-config reasoning_effort/thinking; re-sync via /hero-sync for next cycle).

---

## 2026-08-18 — C5 archived (model-properties-tui)

**Problem**: Initial `hero cycle archive` failed — OpenSpec `MODIFIED` deltas referenced requirement headers absent from base specs (`harness-adapter` first failure).

**Decision / Outcome**: Changed C5 delta sections from `## MODIFIED Requirements` to `## ADDED Requirements` in `openspec/changes/model-properties-tui/specs/{harness-adapter,hero-tui,runtime-workflow-execution,sqlite-operational-store}/spec.md`. Retry succeeded: `openspec archive model-properties-tui -y` merged 19 requirements; Hero archived to `.workflow-hero/cycles/archive/C5-2026-08-18-implementa-o-da-sele-o-das-propriedades/`. Resume with `/hero-resume C5`. No active cycle remains.

---

## 2026-08-20 — TUI Chat user-prompt scroll box + clipboard shortcuts

**Problem**: Long user prompts rendered as full-width blue text above the response pane, overflowing the terminal with no scroll. No way to copy prompt, response, or composer text from Chat.

**Decision / Outcome**: Replaced flat `You:` history with a bordered scroll box (2 visible lines, `#0000CC` accent bar distinct from Build input blue; same in-box text colors). Added `historyScrollOffset`; ↑↓ scroll chains input → user box → response. **Alt+y** / **Alt+r** / **Alt+i** copy plain text via OSC 52 + `atotto/clipboard` (`internal/tui/clipboard.go`). Footer and input hints updated. UI-C03-001 §3 Chat table amended. `go test ./...` green.

**Follow-up (same day)**: User prompt status label `You` now uses `chatInUser` (`#0000CC` foreground) to match the darker accent bar; prompt body text unchanged (`chatInText`).

---

## 2026-08-19 — opencode.yml completed with all 27 OpenCode models

**Problem**: `assets/models/opencode.yml` had only 3 model rows (deepseek-v4-pro, gpt-5.6-luna, glm-5.3), so most `opencode/` (Zen free) and `opencode-go/` models had no pricing/capability row.

**Decision / Outcome**: Completed `assets/models/opencode.yml` (mirrored to `.workflow-hero/models/`) with all 27 model IDs exactly as `opencode models` exposes them: 7 `opencode/` free (big-pickle, deepseek-v4-flash-free, hy3-free, laguna-s-2.1-free, mimo-v2.5-free, nemotron-3-ultra-free, nemotron-3.5-lightning-free) + 20 `opencode-go/` paid (existing 3 kept unchanged + deepseek-v4-flash, glm-5.1, glm-5.2, grok-4.5, hy3, kimi-k2.6, kimi-k2.7-code, kimi-k3, mimo-v2.5, mimo-v2.5-pro, minimax-m2.7, minimax-m3, muse-spark-1.2-contributor, qwen3.6-plus, qwen3.7-plus, qwen3.7-max, qwen3.8-max). Pricing/cache/context from the local opencode server `/provider` API (cross-checked against models.dev TOML via subagent web research); `fs`/`th`/`ef` properties from models.dev `reasoning_options` (effort values, thinking toggles; `fs` fast unavailable for all — no built-in fast variant). Golden rows (deepseek-v4-pro/luna/glm-5.3) untouched. `go test ./...` green.

---

## 2026-08-18 — Model catalog pricing/capabilities aligned in assets/models/*.yml

**Problem**: `assets/models/*.yml` diverged from the expected pattern (pricing + `properties` blocks) and carried stale/wrong data (e.g. `grok-4.5` cache_read 0.50, grok context 256000, `glm-5.2` 200000, `kimi-k3` 256000, outdated OpenAI/Google prices); several files had no `properties`.

**Decision / Outcome**: Researched 2026 pricing/capabilities on provider/Cursor/OpenCode docs and rewrote the 8 catalogs (mirrored to `.workflow-hero/models/`). Added `properties` (`fs`/`ef`/`th`) to anthropic/google/moonshot/openai/xai/zhipu/cursor; added `th` to cursor base rows; added pricing to opencode-go rows (properties kept unchanged per test constraints). Unsupported props use `values: ["na"]`/`default: "na"` (scalar `na` breaks yaml.v3 `[]string` unmarshal — must be a list); unfindable data (grok-3-mini, retired) uses `["not found"]`. Fixed values: grok-4.5 cache_read 0.30 + context 500000, grok-4.6 context 500000, cursor fast Grok variants $4/$1/$12, GLM-5.2/Kimi K3 context 1000000 (kimi-k3 1048576), Sonnet 5 $2/$2.50/$0.20/$10/1M, gpt-5.3-codex $1.75/$0.175/$14, gpt-5-mini $0.25/$0.025/$2, gemini-3.1-pro $2/$0.20/$12, deepseek-v4-pro off-peak $0.66/$0.022/$1.98/1M, gpt-5.6-luna $0.20/$0.02/$0.25/$1.20/1.05M. Updated `internal/tui/contextbar_test.go` grok windows 256000→500000. `go test ./...` green.

---

## 2026-08-19 — Release v2.1.2 (model catalog expansion)

**Problem**: Model catalogs in `assets/models/` were incomplete or stale after 2.1.1 — OpenCode had only 3 rows; provider pricing/capabilities needed alignment across anthropic, cursor, google, moonshot, openai, xai, zhipu.

**Decision / Outcome**: Tagged `v2.1.2` on `main` after `go test ./...` green. Shipped full OpenCode catalog (27 models), pricing/properties alignment across 8 provider YAML files, xai catalog adjustments. Artifacts via `scripts/release.sh`; GitHub Release with binaries + `checksums.txt`.

---

## 2026-08-19 — TUI /hero-start OpenCode agent sync + serve reset

**Problem**: OpenCode subagents read model/properties from `.opencode/agents/*.md` frontmatter at launch; edits require a serve restart. Manual frontmatter sync before `/hero-start` was error-prone (see 2026-08-18 C5 resume entry).

**Decision / Outcome**: Added `internal/adapters/opencode` **`SyncAgentDefinition`**, **`PrepareHeroStart`**, and adapter **`ResetServe`** (2s delay between stop and restart). TUI `/hero-start` runs prepare asynchronously when OpenCode is enabled and any workflow agent uses `harness: opencode`: sync all matching agent files from `workflow-config.yml`, reset serve, probe the first agent without a request-level model; failure stops start and tells the user to exit the TUI and retry. Cursor-only projects skip prepare synchronously. `go test ./...` green.
