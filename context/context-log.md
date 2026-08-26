# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

---

## 2026-08-26 — C7 Config TUI form readability and dirty-leave recovery

**Problem**: Text editing updated only a buffer printed at the bottom of the long Config form, so typed text and its cursor were often off-screen until Enter committed the field. The duplicate static summaries and ungrouped controls made the form difficult to scan. More seriously, pressing Escape with a dirty form captured all input for Save/Discard/Cancel, but that confirmation was rendered below the clipped form; the TUI therefore looked frozen.

**Change**: Text fields now render the live buffer, blue caret, and an editing background in the selected row; left/right, Home/End, Backspace, and Delete work in the field. Labels use the Chat composer blue (`colorAccentAI`), disabled controls use gray, and the selection background applies only to the label. Removed duplicate summaries and reorganized the editable fields into spaced Identity, Scope, per-stage, and Shared/Advanced sections. Dirty leave now replaces the form with an always-visible `[enter] Save  [d] Discard  [esc] Cancel` state, with matching fixed-footer help; Enter follows the normal save/validation path.

**Validation**: Focused Config regressions passed; `go test ./... -count=1` green.

---

## 2026-08-26 — C7 Config TUI navigation/editing fixes

**Problem**: The C7 Config screen had residual UX bugs from its first version: focused read-only (completed-stage) fields rendered fully muted so the cursor highlight was lost while tabbing; the quit shortcut was still `ctrl+q` in one spec doc instead of `alt+q`; and `esc`/`alt+n` could not leave the screen when clean (the non-dirty `Leave` branch was missing). Earlier in this same change `Toggle` was fixed from the literal `"space"` to `" "` and the footer/navbar hint updated to `alt+n events`.

**Change**: `renderConfigField` now applies the selection highlight to any focused field (including protected/inactive ones) instead of falling back to `mutedStyle`. Updated `docs/product/UI-C03-001` footer hint from `alt+y/r/i copy · ↑↓ scroll · alt+n screens · ctrl+q quit` to the current `alt+r/i copy · ↑↓ scroll · alt+q quit`. Added a regression test asserting a focused protected field keeps the selection highlight (color profile forced to ANSI, since non-TTY `go test` strips color).

**Validation**: `go test ./... -count=1` green; `go vet ./internal/tui/` clean.

---

## 2026-08-26 — OpenCode overnight hang workarounds

**Problem**: TUI stayed `streaming=true` overnight with SSE still open (or silent). OpenCode logs were INFO-only; no `session.idle`. Cause is OpenCode (`opencode-go` / `GET /event` half-open). Hero waited forever, did not persist session id until `executeDone`, `Cancel("")` missed `runCtx`, `/harness-reset` wiped the id, and recovered serve was bound to Execute `runCtx`.

**Decision**: Keep `opencode serve`. Do not migrate to `opencode run`.

**Change**: Probe `GET /session/{id}` after stall grace — idle completes Execute, 404 fails, busy waits. `HealthFailed` auto-cancels. Stall ignores `file.watcher` / LSP / `session.status` running. Emit `session.bound` and persist id immediately. `Cancel("")` cancels all in-flight. `ResumeSession` on Execute; OpenCode reset keeps the id. `ExecRunner.Start` uses `exec.Command`. Local `chunkTimeout: 180000` for `opencode-go`.

**Validation**: `go test ./...` green.

---

## 2026-08-26 — OpenCode question.asked interactive Chat

**Problem**: OpenCode `question.asked` was routed as activity with raw JSON in Chat; OpenCode blocked ~23min waiting for reply while Hero ignored composer during streaming.

**Change**: Harness `QuestionRequest`/`OnQuestionRequest`; OpenCode `handleQuestionAsked` + reply/reject HTTP; TUI composer answers (sequential multi-question, Enter/Esc) while streaming.

**Validation**: Adapter + TUI tests; `go test ./...` passed.

## 2026-08-26 — OpenCode SSE reconnect on unexpected EOF

**Problem**: Hero Chat with OpenCode harness showed `unexpected EOF` during long tool runs (`go test ./...`). OpenCode kept running (log showed step 11 + bash permissions) but Hero’s per-execute `GET /event` SSE closed before `session.idle`; next user message often failed with socket/connection errors while the session was still busy.

**Change**: Subscribe to `GET /event` **before** `prompt_async` (race on fast completions). SSE reconnect on disconnect. Fallback: `GET /session/{id}/message` when stream ends with no output. `ensureServe` re-validates cached base URL and restarts dead `opencode serve`; HTTP `do()` and `subscribeEvents` retry once on connection refused.

**Validation**: `TestEnsureServeRecoversDeadURL`, question + SSE tests; `go test ./...` passed.

## 2026-08-25 — `/hero-approve` must start the next TUI stage

**Problem**: After Research with `require_human_approval`, `/hero-approve` left Planning `Waiting`. TUI handoff only ran for `Running` stages, and the approve follow-up was just the slash — ORCH asked for `/hero-start` again.

**Change**: After an orchestrator turn, if the active named stage is Waiting, the TUI calls `hero stage start` and Executes that agent. Approve preamble/follow-up forbid a `/hero-start` CTA. Palette `/hero-approve` keeps `orchestrationLive` so the same handoff runs.

**Validation**: `TestApproveHandsOffToWaitingPlanning`; `go test ./...` passed.

## 2026-08-25 — Codex Research transcript glued without newlines

**Problem**: Codex emits one `agentMessage` item per status line (`→ …`). Deltas are concatenated per `itemId` only; Hero joined every item into one bubble with no separator, so Research opened as `documents.→ I'm using…`. `turn/completed` `lastAgentMessage` is often just the last (reworded) item and was appended when it diverged, duplicating the grill question.

**Change**: Insert `\n` when the `itemId` changes if the previous item has no trailing newline. Repair from `lastAgentMessage` only when it extends streamed text (still used when live deltas were dropped).

**Validation**: Colocated Codex adapter tests; `go test ./...` passed.

## 2026-08-25 — C8 TUI-direct stage Execute

**Change**: After `/hero-start`, ORCH starts a stage then STOPs. The TUI Executes named stage agents (`planning_agent`, scope Implementation agents, QA, Judge, Browser UI, E2E) on their YAML harness+model pair, then resumes ORCH. Nested Task fan-out remains inside the parent harness. Chat multiplexes tagged concurrent Executes; navbar chips named 4-letter codes plus generic `TASK`; Task **started** inserts a `[LABEL - model · harness]` launch line. Codex `collabToolCall` and OpenCode Task/`session.next.tool` events now set `AgentName`, `CallID`, and `Phase`.

**Validation**: Colocated TUI + adapter tests for Planning handoff, parallel Implementation, TASK vs CTX chips, sibling `executeDone`, and Task parse.

## 2026-08-25 — C7 implementation (`tui-cycle-config`)

**Change**: Added active-cycle Config navigation and an asynchronous TUI Config screen with keyboard focus/editing for identity, scope, stages, agent harness/model/properties, and save/retry actions. Agent model choices are constrained to the selected enabled harness via the existing cache/catalog model list; C5 property visibility uses `modelprops` capability snapshots, with unknown-capability warnings preserving YAML values. Nested subagent model choices use the parent harness. `workflowconfig.Document.Write` now loads the latest valid YAML, applies only managed values, validates, and atomically replaces the file without a revision-conflict dialog. Added managed diffs and `Engine`/`cycle.Service` failed-stage retry with an append-only `stage_retried` event.

**Validation**: `go test ./...` passed.

## 2026-08-25 — C7 QA remediation (`tui-cycle-config`)

**Change**: Config save validation and operational errors now preserve the visible draft and show field-level feedback. Agent controls are associated with their owning stage, so completed-stage controls are read-only. Added nested `same_of_agent` and subagent property controls, validation against known model/capability metadata, focused-field scrolling, and deferred dirty-exit navigation after Save or Discard. Failed-stage retry eligibility now uses the managed diff for that exact stage; retry always transitions Failed to Waiting even when the newly saved stage is disabled.

**Validation**: Added TUI interaction, workflow-config atomic failure, engine disabled-retry, and cycle service retry coverage; `go test ./... -count=1 -timeout=10m` passed.

## 2026-08-25 — C7 Planning SDD (`tui-cycle-config`)

**Decision**: OpenSpec change `tui-cycle-config` created from PRD/UI/ADR-C07-001. YAML remains source of truth. Save uses ADR-050 latest-file managed-node merge (idea-file reload/merge/cancel dialog out of scope). Retry is an engine Failed→Waiting transition with no new CLI command. Scope native → `generic_agent`. `openspec/config.yml` context regenerated from `documents.json` including C07 docs.

**Outcome**: SDD at `openspec/changes/tui-cycle-config/` (36 tasks). Persist slug with `hero cycle openspec-change tui-cycle-config`.

## 2026-08-25 — `/hero-start` blocked after finished `/hero-resume`

**Problem**: `/hero-resume` set `actionBusy` for the status timer but `executeDone` only called `setStatusResult` when the label was `/hero-start`. The palette then reported `busy — wait for /hero-resume to finish` even after the chat turn completed.

**Change**: Any busy Execute (resume, start without handoff, cancel, error) now clears `actionBusy` via `completeBusyExecuteStatus`. `/hero-start` → discover handoff still keeps the timer until the discover turn ends.

**Validation**: `go test ./internal/tui/` passed. `go test ./...` failed only in unrelated WIP `internal/workflowconfig` document tests.

## 2026-08-25 — `/hero-start` / `/hero-resume` visible wait in Chat

**Problem**: Preflight ran with a status timer but no in-chat spinner (`streaming` was false). Leftover grill history stayed on screen and Execute set `transcriptScrollOffset = 0`, so the viewport jumped to the top and `Waiting for harness…` was off-screen — looked frozen.

**Change**: `/hero-start` clears the transcript and shows `Preparing /hero-start…` during bootstrap/prepare. Execute follows the transcript bottom. `/hero-resume` keeps history, follows bottom, and sets status running with a timer.

**Validation**: `go test ./internal/tui/` passed.

## 2026-08-25 — Mixed-harness grill-me session resume

**Problem**: TUI Research with orchestrator Cursor + `discover_agent` Codex started a new Codex thread on every user answer. `persistHarnessSession` for discover never set `harnessSessionHarnessID`, so `harnessSessionIDForPair` treated the leftover Cursor id as a cross-harness resume and dropped the thread. Same TUI bug for OpenCode when harnesses differ.

**Change**: Bind discover (and orchestrator follow-up/close) to the runtime harness; `/hero-start` Prepare resets the registry adapter; Codex `thread/resume` failure on an unloaded id starts a new thread; `StopAppServer` clears session maps.

**Validation**: `go test ./internal/tui/ ./internal/adapters/codex/ ./internal/adapters/opencode/ ./internal/harnessmgr/` passed. `go test ./...` failed only in unrelated WIP `internal/workflowconfig` document tests (`document_test.go`), not in this change.

## 2026-08-24 — Chat wait spinner during thinking

**Change**: `transcriptContentLines` always draws `⠋ Waiting for harness…` at the end of the transcript while `streaming` is true (muted bar). The spinner no longer disappears after the first green agent text, so Codex/OpenCode/Cursor turns that continue with gray thinking or tools still show that Execute is in flight. Hidden on `executeDone` / cancel / failed-or-interrupted parent bubble.

**Validation**: `go test ./...` passed.

## 2026-08-24 — V2.8 TUI cycle configuration screen (idea consensus)

**Decision**: Add a future **Config** sidebar screen only while a cycle is active. It edits the existing `workflow-config.yml`; the Cursor IDE configuration flow remains unchanged. The screen is editable when no agent is running, progressively reveals stage/agent controls, selects harness → model → supported properties interactively, and saves atomically with a round-trip-safe YAML patch. Each save syncs title/objective and still-open stage budgets to SQLite.

**Artifact**: `docs/idea/v2_8_config/config-screen.md`.

**Decision (2026-08-25)**: For the planned TUI cycle configuration screen, an explicit property value selected in the UI takes precedence over a catalog or harness default. Defaults only initialize an untouched field; capability refresh must preserve a still-valid user choice.

## 2026-08-24 — Release v2.7.0 (local only)

**Outcome**: `go test ./...` green → version bump commit → tag `v2.7.0` on `main` → pushed tag (no GitHub Release). `scripts/release.sh` run locally for `dist/` artifacts.

**Notes**: Minor bump from 2.6.0 — `hero chat` free-chat mode; C6 Codex adapter idea archived.

## 2026-08-22 — `hero chat` free-chat mode

**Change**: Added CLI `hero chat` for a Chat-only TUI that does not require project install or git. Config/model/harness prefs live under `~/.workflow-hero/`; Execute workspace is cwd. Navbar shows only Chat; etapa/cycle hints hidden. Palette renames (global): `/hero-model`→`/model`, `/hero-harness`→`/harness`, `Refresh`→`/hero-refresh` (before Go to). Free-chat palette keeps only non-`/hero` items (no Go to). `/harness` in free chat toggles flags without projecting `.cursor`/`.opencode`/`.codex` into cwd.

**Validation**: `go test ./...` passed.

## 2026-08-22 — GitHub release v2.6.0

**Outcome**: `go test ./...` green → tag `v2.6.0` on `main` (`d9a9d48`) → pushed → `scripts/release.sh` → GitHub Release with 4 binaries + `checksums.txt`. URL: https://github.com/ricrsantos/ai_workflow_hero/releases/tag/v2.6.0

**Notes**: Minor bump from 2.5.0 — TUI redesign/layout polish, `/hero-update-config`, Codex stream/token fixes, harness status bar corrections.

## 2026-08-22 — Linear chat transcript

**Change**: Merged the separate You + green response chat boxes into one borderless linear transcript (full session history) with thin `│` accent bars per actor (user violet / agent green), Hero labels preserved (`You`, `[LABEL - model · harness]`). Composer stays bordered with solid accent. Unified `transcriptScrollOffset` + auto-follow; ↑↓ scrolls input then transcript. Updated UI-C03 / UI-C05 wording. Follow-up: composer content rows `2 → 3`.

**Validation**: `go test ./internal/tui/ -count=1` and `go test ./... -count=1` passed.

**Change**: Removed the chat-pane top row (`Chat · harness …`, cycle title, inline session id). Harness/model context stays in the composer and sidebar; cycle etapa hint stays in the status bar.

**Validation**: `go test ./internal/tui/ -count=1` passed.

## 2026-08-22 — Sidebar agents panel

**Change**: Moved the live agents summary (`agents: N` + label row) from the chat header box into the left nav sidebar between two full-width `colorBorder` rules (below **AI Hero**, above screen nav). Reserves 2 rows for agent labels. Header agents box remains only when the sidebar is hidden (narrow terminal).

**Validation**: `go test ./internal/tui/ -count=1` passed.

## 2026-08-22 — TUI chrome polish (borders, footer, rules)

**Fixes**: Chat/agents boxes overflowed by 2 cols (lipgloss `Width` is inner; borders add outside) so bottom borders wrapped — sized sidebar/chat boxes with `GetHorizontalFrameSize()`. Removed footer hint `alt+n screens`. Dropped the rule above the status area; the rule between status and footer now uses `colorBorder` (same as box borders).

**Validation**: `go test ./internal/tui/ -count=1` passed.

## 2026-08-22 — TUI left nav sidebar (Bonito style)

**Change**: Moved the horizontal `1. Chat │ 2. Status │ …` tab bar into a left framed sidebar (title **AI Hero**, `>` active marker, `alt+1-5` footer). Applied Bonito dark palette hex tokens in `styles.go`. Layout is `JoinHorizontal(sidebar, content)` above full-width status/footer chrome; sidebar auto-hides below 80 cols. Still Bubble Tea / Lip Gloss v1 (no Charm v2 migration).

**Validation**: `go test ./... -count=1` passed.

## 2026-08-22 — `/hero-config-update` reloads TUI model labels

**Problem**: Mid-cycle edits to `workflow-config.yml` or `hero.json` were used by Execute (disk reload) but Chat input `Build · model · harness` kept stale `runtimeModelSlug` / boot-time `chatModelSlug`. No poll desired.

**Fix**: Built-in slash `/hero-config-update` calls `syncDisplayModelFromDisk` (freechat from `hero.json`; active orch/discover/runtime agent from YAML) and refreshes the input label. Palette + composer dispatch; no harness Execute.

**Validation**: `go test ./... -count=1` passed.

## 2026-08-22 — Token usage: context bar + cycle SQLite

**Problem**: Freechat (and TUI stage executes) left the Chat context bar at 0 tokens. Cycle Costs/SQLite relied only on LLM Metrics Procedure (chars÷4 via `--metrics-json`). OpenCode never populated `ExecutionResult.Usage`; Codex `thread/tokenUsage/updated` could be dropped under notify backpressure.

**Fix**:
- OpenCode: extract `info.tokens` / step-finish tokens into `ExecutionResult.Usage`.
- Codex: `thread/tokenUsage/updated` is `notifyMustDeliver`.
- `harness.EstimateUsage` / `ResolveUsage` (chars÷4 fallback).
- TUI `executeDone`: context bar uses ResolveUsage; when `conversationStage` is set, accumulate into SQLite via `AccumulateStageHarnessMetrics`.
- Engine `persistMetrics` prefers prior harness tokens over agent estimates (still accepts agent cost when unset).

**Validation**: `go test ./... -count=1` passed.

## 2026-08-22 — Green pane response truncation (Codex/TUI)

**Problem**: Assistant replies in the Chat green pane sometimes ended mid-sentence when using Codex (and under TUI backpressure generally).

**Root causes**: (1) TUI `OnStreamDelta` dropped text deltas after a 2s backpressure timeout; (2) Codex `notifyQ` overflow reordered transcript events via unordered goroutines; (3) viewport clip made wrap boundaries look like incomplete answers.

**Fix**: Lossless delivery for text/thinking/warning/session deltas; `reconcileParentAgentOutput` on `executeDone` (prefix-superset, works with subagents); Codex must-deliver ordered enqueue + `textBuf` before `OnStreamDelta`; `turn/completed` `lastAgentMessage` repair; green pane `↓ more` / `…` clip markers.

**Validation**: `go test ./internal/tui/ ./internal/adapters/codex/ -count=1` (focused + package).

## 2026-08-21 — Codex skill YAML frontmatter

**Problem**: Codex warned that Hero-provisioned `.codex/skills/*/SKILL.md` lacked YAML frontmatter (`---` delimited `name` + `description`).

**Fix**: Added required frontmatter to `assets/codex/skills/{grilling,workflow-hero}/SKILL.md`; synced dogfood projection + checksums; regression test `TestCodexSkills_HaveRequiredYAMLFrontmatter`.

**Validation**: `go test ./... -count=1` passed.

## 2026-08-21 — Multi-harness TUI labels (model × harness mix)

**Problem**: During multi-harness cycles the Chat speaker/input could show a Cursor model with an OpenCode harness (e.g. `[ORCH - composer-2.5 · opencode]`). Display used freechat `chatHarnessID` while the model came from `runtimeModelSlug` (YAML orch).

**Fix**: Added `runtimeHarnessID` (parallel to `runtimeModelSlug`); `conversationHarnessTool` prefers it; orch/discover/start paths apply `AgentPairFor`; `executePairMsg` updates labels after `ResolveExecutePair` (incl. fallback); `liveAgent`/`convMessage` carry per-agent harness for nested speakers.

**Validation**: `go test ./... -count=1` passed.


## 2026-08-21 — Responsive `/hero-start` preflight and streaming

**Problem**: `/hero-start` performed SQLite status/config sync, model resolution, prompt reads, and preparation checks synchronously in Bubble Tea `Update`; high-frequency harness deltas then caused repeated full transcript wrapping/rendering.

**Outcome**: Moved `/hero-start` preflight and OpenCode/Codex preparation to cancellable `tea.Cmd` workers with request IDs and visible progress. Added `Ctrl+C`/quit handling during preflight, 25ms/64-event stream batching, and per-message response line wrapping/style caching. Existing stream/cancel/handoff behavior remains covered by TUI tests.

**Validation**: `GOCACHE=/tmp/hero-go-cache go test ./... -count=1 -p 1 -timeout=600s` passed with local-listener permission enabled; the restricted sandbox cannot run the repository's `httptest.NewServer` tests.

## 2026-08-21 — TUI footer anchoring on short terminals

**Problem**: The navigation footer text was fixed, but `renderFrame` enforced a minimum three-row content area. On short terminals that made the frame taller than the viewport, hiding the final footer hint group (`ctrl+q quit`).

**Fix**: Content height now may reach zero when fixed chrome consumes the available rows; the footer remains anchored and the rendered frame stays at the terminal height. Added a regression test covering an 80×10 Chat frame and the complete fixed footer.

---

## 2026-08-21 — TUI fixed navigation footer

**Problem**: The footer hints changed by screen/streaming state and the long line could wrap without reserving its rows, causing incomplete or overwritten navigation instructions.

**Outcome**: `internal/tui/screens.go` now renders one fixed footer string for every TUI state, wraps only between hint groups, and includes the wrapped footer in frame-height calculations. Chat response sizing and the idle agents box were tightened so the header/composer remain visible. Added coverage for fixed content, narrow wrapping, and footer anchoring; UI-C03 and context state now document the fixed footer.

---

## 2026-08-21 — Codex stdio deadlock + stall 3m

**Problem**: Live Hero↔Codex hang — Codex blocked on `anon_pipe_write` (~75KB JSONL), Hero stdout pipe full (~64KB), no TCP; sync `onNotify` + TUI `ch <-` backpressure stopped `readLoop` draining.

**Fix**: Codex `rpcConn` serial `notifyQ` (never block stdout reader); TUI stream chan 512 + timed backpressure drop; OpenCode+Codex `StallTimeout` **3m** (was 6m).

---

## 2026-08-21 — GitHub release v2.5.0

**Outcome**: `go test ./...` green → annotated tag `v2.5.0` on `main` (`6c898e7`) → pushed → `scripts/release.sh` → GitHub Release with 4 binaries + `checksums.txt`. URL: https://github.com/ricrsantos/ai_workflow_hero/releases/tag/v2.5.0

---

## 2026-08-21 — Codex stream UX: debug noise + agentMessage dedupe

**Problem**: Chat showed garbled then clean duplicate of the same agent answer (`stringField` TrimSpace on deltas + full `item/completed` re-emit). Noisy activities (`userMessage`, `agent message`, tokens, rate limits) and unrecognized-event warnings always appeared.

**Decision / Outcome**: Mirror OpenCode: `stringFieldRaw` for text/tool deltas; authoritative `item/completed` for agentMessage (suffix-only / skip); reasoning live deltas suppressed, thinking from completed snapshot only; activities + unrecognized warnings gated on `ExecuteRequest.Debug` (`hero --debug`). UI-C06 §5 updated. Tests in `events_test.go` + adapter/integration mocks.

---

## 2026-08-21 — OpenAI + Codex model catalog refresh

**Change**: Added `gpt-5.6-terra`, `gpt-5.6-luna`, `gpt-5.5`, `gpt-5.4-mini` (plus `openai/` OpenCode ids) to `assets/models/openai.yml` and `.workflow-hero/models/openai.yml`. Same native ids in `assets/models/codex.yml` / `.workflow-hero/models/codex.yml` with ChatGPT rates at 0.00, `th` (summary) available, `fs` na. OpenAI pricing from API docs; GPT-5.6 `cache_write` at 1.25× input on openai.yml only.

---

## 2026-08-21 — C6 §9: Integration + close (native complete)

**Problem**: Close Implementation: integration lock for upgrade→enable→Execute→orphan reap, SemVer 2.5.0, context landing, openspec-change persistence.

**Decision / Outcome**: Added `internal/integration/codex_c6_test.go` (§9.1: 2.4→2.5 upgrade leaves Codex off; `EnableHarnessWithProjection` projects `.codex/`; mock stdio Execute streams text+thinking; Codex orphan reap clears dead registry PID). Default `cmd/hero` version + `scripts/release.sh` tag example → **2.5.0** (ldflags injection unchanged). `hero status` shows OpenSpec change `codex-adapter` on C6. Tasks 9.1–9.4 checked; `go test ./...` green. Phase: native C6 complete → QA/Judge next.

---

## 2026-08-21 — C6 §8 parallel tracks (summary)

- **8A/8B**: `/hero-harness` Codex enable/disable + projection; `/hero-model` Codex harness step + `ListModels` + C5 properties; stage YAML untouched.
- **8C**: Chat `[LABEL - model · harness]` / `Build · model · harness`; Codex auth/CLI/app-server goldens; unknown events → yellow warning.
- **8D**: Codex-native ids in `assets/models/*.yml` (no invented ChatGPT USD rates); metrics warn on unknown id.
- **8E**: `/harness-reset` includes Codex; `PrepareHeroStart` sync/reset/probe tests.
- **8F**: Doctor warn-only `codex-cli` when enabled and missing from PATH.
- **8G**: workflow-help + README EN/PT-BR + DEPLOY 2.5.0; Cursor Runtime assets unchanged.
- **8H**: Cursor/OpenCode Execute untouched; mixed three-harness integration + OpenCode Prepare when Codex unused.

---

## 2026-08-21 — C6 §1–§7 (summary)

**Outcome**: hero.json + SQLite Codex registry; `/hero-new` three-harness injection; install/upgrade Codex opt-in (2.4→2.5 never auto-enables); `internal/adapters/codex` stdio JSON-RPC + stream/permission/auth/C5/usage; `assets/codex/` projection; harnessmgr + TUI boot/orphan/health; fallback + Execute routing + Prepare-on-`/hero-start`.

---

## 2026-08-20 — Releases / pre-C6

- **v2.4.1**: TUI UX patch (bordered prompt, clipboard, context bar).
- **v2.4.0**: OpenCode serve lifecycle + orphan reap.
- **C5 archived** (`model-properties-tui`); Judge approved without formal SDD verify (judge_agent empty in opencode harness).

---

## 2026-08-25 — C7 Research: TUI cycle configuration screen

**Outcome**: Completed interactive requirements grilling for the active-cycle Config screen described in `docs/idea/v2_8_config/config-screen.md`. Confirmed YAML remains the single source of truth; TUI precedence applies only to managed fields during parallel edits; unmanaged latest-file content is preserved; invalid/missing YAML is never replaced; completed stages are read-only; failed stages support explicit stage-specific retry after a saved configuration change, with fresh attempt counters and preserved events/metrics; chat language is non-empty free text; active stage timeout and iteration budgets remain positive.

**Artifacts**: `docs/product/PRD-C07-001-tui-cycle-config.md`, `docs/product/UI-C07-001-tui-cycle-config.md`, and proposed `docs/architecture/ADR-C07-001-tui-cycle-config.md`, all registered in `.workflow-hero/config/documents.json`. `PRD.md`, `UI.md`, `ADR.md`, `architecture-overview.md`, `TESTING.md`, and context state were updated.

---

## 2026-08-25 — C7 Config judge gaps

**Outcome**: Config asynchronously checks every enabled harness and renders a yellow warning with the availability cause. `browser_ui_agent` fields now require both frontend scope and Browser UI Validation. Missing capability metadata is flagged beside every visible parent/subagent and `fallback_model` model field, while keeping configured property values intact.

## 2026-08-26 — Codex project config and Harness trust

**Outcome**: Added `.codex/config.toml` with `approval_policy = "never"`, `sandbox_mode = "workspace-write"`, and `sandbox_workspace_write.network_access = true` for this repository's trusted development workspace.

**Decision**: Project trust is not configured in the project-local file because Codex skips `.codex/` layers while a project is untrusted. For stream/app-server Harnesses, inject `projects."<absolute-project-path>".trust_level = "trusted"` using a startup `-c` override (or a global Codex config) before app-server initialization. The app-server protocol also supports per-thread `approvalPolicy` and sandbox overrides, but no Harness implementation change was made in this task.

**Validation**: `codex-cli 0.149.0` accepted the project config and the trust override syntax with `--strict-config` for both the CLI and `codex app-server`.

## 2026-08-26 — Codex adapter automatic project trust and execution policy

**Outcome**: Updated `internal/adapters/codex` so every Hero-managed app-server starts from an absolute/canonical project path with `projects."<path>".trust_level="trusted"`, `approval_policy="never"`, `sandbox_mode="workspace-write"`, and workspace network access enabled as explicit CLI config overrides. `thread/start` and `turn/start` also carry the app-server-native `approvalPolicy: "never"`, `workspaceWrite` sandbox, project writable root, and network access.

**Decision**: Keep `.codex/config.toml` as the project-local fallback and retain `workspace-write`; do not use `--yolo`/`danger-full-access` or auto-approve residual permission requests. Host/Harness permissions and OS-level listener restrictions remain outside the Codex adapter boundary.

**Validation**: Added adapter tests for startup arguments and JSON-RPC policy payloads. `CC=/usr/bin/gcc GOCACHE=/tmp/hero-go-cache go test ./... -count=1 -p 1 -timeout=600s` passed, including the TUI package; local-listener tests required host permission.

## 2026-08-26 — Codex thread/start sandbox variant mismatch

**Outcome**: Hero-managed Codex Execute failed immediately with `unknown variant workspaceWrite, expected one of read-only, workspace-write, danger-full-access`. `thread/start.sandbox` is Codex `SandboxMode` (kebab-case). The adapter had reused the `turn/start.sandboxPolicy.type` camelCase value `workspaceWrite`.

**Decision**: Keep both wire formats: `thread/start.sandbox` and CLI `-c sandbox_mode=` send `workspace-write`; `turn/start.sandboxPolicy.type` remains `workspaceWrite`.

**Validation**: `go test ./internal/adapters/codex/` and `CC=/usr/bin/gcc GOCACHE=/tmp/hero-go-cache go test ./... -count=1 -p 1 -timeout=600s` passed.

## 2026-08-25 — Removed `alt+y` copy-prompt shortcut

**Outcome**: The `alt+y` "copy prompt" shortcut was removed from the TUI because the "you" and AI-response panes were unified, making a separate prompt-copy command redundant. Removed the `alt+y` key handling in `handleConversationKey` (streaming + global shortcut), the now-dead `copyChatPrompt` and `latestUserContent` helpers, and updated the fixed footer hint to `alt+r/i copy`. Updated `clipboard_test.go` and `screens_test.go` accordingly. `go test ./...` passes.

## 2026-08-25 — `alt+r` copies the whole chat box as plain text

**Outcome**: `alt+r` (copy response) now copies the entire unified chat box (You + every agent turn), not just the latest agent turn. Replaced `responsePlainText` (last-turn only) with `transcriptPlainText`, which walks the full transcript, keeps speaker headers (`[AGENT - model · harness]`, `You`) and thinking/tool markers, and omits the `│` accent bars and right-padding used to decorate the rendered box. Updated `clipboard_test.go` (new whole-conversation and no-decoration tests). `go test ./...` passes.

## 2026-08-26 — Codex chat stream integrity under TUI backpressure

**Problem**: In a long-lived TUI session, Codex responses could lose fragments in the middle of a sentence. The text callback eventually blocked behind the bounded chat channel while the TUI rebuilt a growing transcript; the previous final-output repair only handled a prefix/suffix relationship and therefore retained internal gaps.

**Outcome**: Added a per-Execute ordered/coalescing relay between harness callbacks and Bubble Tea. It makes callbacks return immediately, coalesces adjacent text/thinking deltas, retains transcript-critical events, drains before Execute completion, and is stopped on cancellation. Codex now records final text per `agentMessage` item and constructs `ExecutionResult.Output` from authoritative completed snapshots in wire order. A unique streamed item also accepts `turn/completed.lastAgentMessage` as a final fallback. TUI auto-follow is calculated once per stream batch rather than once per delta.

**Validation**: New Codex gap-repair tests and TUI relay backpressure/cancellation tests pass. `CC=/usr/bin/gcc GOCACHE=/tmp/hero-go-cache go test ./... -count=1 -p 1 -timeout=600s` passed all affected packages (Codex, TUI, Cursor, integration) but the full command still fails in two pre-existing OpenCode local-serve tests: `TestDiscoverModelPropertiesNormalized` and `TestIsManagedOpenCodeServeDetectsSpawnedServe`, both reporting no listening URL from the simulated serve process in this restricted sandbox.
