# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

---

## 2026-08-14 — Release v1.2.1

**Problem**: Chat response pane and transcript used inconsistent speaker labels (`Agent`, `[Orchestrator]`) instead of the same 4-letter codes as the agents box.

**Decision / Outcome**: Green-pane status and transcript headers use `[LABEL - model]` (`[ORCH - composer-2.5]`, `[QA - composer-2.5]`, `[HARN - grok-4.6]`); status follows the last live agent while streaming. Tagged `v1.2.1`.

---

## 2026-08-14 — Chat green-pane speaker labels

**Problem**: Harness replies in the green response pane printed a fixed `Agent` word (`[Agent - composer-2.5]` / status line `Agent · model`). Origin was not the same 4-letter code as the agents box.

**Decision / Outcome**: Transcript headers and the green-pane status line use `[LABEL - model]` with `agentShortLabel` (`[QA - composer-2.5]`, `[ORCH - composer-2.5]`, `[HARN - grok-4.6]`). Status follows the last live agent while streaming.

---

## 2026-08-14 — TUI navigation while agent streaming

**Problem**: While `streaming == true` on the Chat screen, all navigation keys (`ctrl+1–6`, `alt+1–6`) were swallowed. Also, `handleConversationMsg` (stream deltas, done, cancel) only ran when `m.screen == screenConversation`, so the stream goroutine was effectively orphaned when the user navigated away from Chat.

**Decision / Outcome**:
1. **Stream messages always processed** — removed `if m.screen == screenConversation` guard in `Update`; `handleConversationMsg` now always runs regardless of active screen. `executeDoneMsg` / `streamCancelDoneMsg` also auto-clear `confirmPending`.
2. **Navigation while streaming** — `handleConversationKey` forwards `ctrl+1–6` / `alt+1–6` to `handleKey` while `streaming == true`. `goListScreen` does not touch `streaming` or `convStreamCh`, so the goroutine keeps running.
3. **Confirmation dialog for destructive actions** — added `confirmPending / confirmMsg / confirmAction / confirmActionN` to `model`. When a destructive palette action (`/hero-new`, `/hero-start`, `/hero-cancel`, `/hero-finish`, `/hero-archive`, `/hero-back`) or `ctrl+q` is requested while streaming, a yellow footer prompt `"Agent is running. <action> will interrupt it. Continue? [y/N]"` is shown. `y` cancels the stream then dispatches the action via `confirmResumeMsg`; any other key dismisses. Non-destructive actions (`/hero-approve`, `/hero-reject`, `/hero-sync`, `/hero-status`, `/hero-continue`) remain silently blocked with `setStatusBusyBlocked`.
4. Added 8 new tests covering all behaviours; full test suite passes.

---

## 2026-08-14 — Release v1.2.0

**Problem**: Users could not navigate TUI tabs while an agent was streaming; stream updates were dropped off the Chat screen; destructive actions during streaming were silently blocked.

**Decision / Outcome**: Tab navigation (`ctrl+1–6` / `alt+1–6`) works while streaming; stream messages always processed regardless of screen; destructive palette actions and `ctrl+q` show a yellow `[y/N]` confirmation footer. Tagged `v1.2.0`.

---

## 2026-08-14 — Release v1.1.1

**Problem**: Chat was first in the tab bar but boot still opened Status; the Chat `/` overlay hid `Go to` items and inserted every slash into the composer.

**Decision / Outcome**: Boot opens Chat with the composer focused. Chat `/` overlay lists the full palette; only `/hero-approve` `/hero-reject` `/hero-cancel` `/hero-continue` `/hero-finish` `/hero-back` insert-then-send — other items execute immediately. Tagged `v1.1.1`.

---

## 2026-08-14 — TUI boot Chat + selective slash overlay

**Problem**: Chat was first in the tab bar but boot still opened Status. Typing `/` in Chat hid `Go to - *` and inserted every slash into the composer.

**Decision / Outcome**: Boot opens Chat with the composer focused. The Chat `/` overlay lists the full palette (including `Go to`). Only `/hero-approve` `/hero-reject` `/hero-cancel` `/hero-continue` `/hero-finish` `/hero-back` insert-then-send; all other items execute immediately like the palette on other screens.

---

## 2026-08-14 — Release v1.1.0

**Problem**: Chat was last in the tab bar; the response pane showed only the parent harness session; nested Task work was a `→ Task name` line with no live agent count.

**Decision / Outcome**: Tab order is Chat | Status | … | Events (`alt+1` = Chat; boot still Status). Cursor `stream-json` parser tracks open Tasks (`AgentName`/`Model`/`CallID`/`Phase`); TUI labels `[Orchestrator]` / `[QA - model]` with blank lines around subagent blocks and skips `result.Output` replacement when those blocks exist. Chat header has a live `agents: N` box with 4-letter labels (`ORCH`, `BACK`, `HARN`, …). Nested Task text is best-effort: attributed while a Task is open, else Task `result.content`. Tagged `v1.1.0`.

---

## 2026-08-13 — Release v1.0.8

**Problem**: TUI Approvals had no history; Artifacts was empty; `/hero-archive` failed when `openspec` was only on the user shell PATH (nvm/fnm); manual OpenSpec archive left Hero archive blocked.

**Decision / Outcome**: Approvals history + Artifacts disk discovery; scroll/refresh on Status/Approvals/Artifacts/Costs/Events; Cursor Execute `--sandbox disabled` + user bin PATH; `hero cycle archive` resolves `openspec` via user bins and skips CLI when change dir is already archived. Tagged `v1.0.8`.

---

## 2026-08-13 — TUI `/hero-archive` OpenSpec PATH + default model

**Problem**: TUI `/hero-archive` failed with `openspec binary not found on PATH` even when `openspec` worked in the user’s login shell. Manual `openspec archive` still left Hero archive blocked because `openspec_change` stayed set. Chat looked like QA End-to-End because the cycle stage was waiting there.

**Decision / Outcome**: Cursor Execute uses `--sandbox disabled` and prepends nvm/fnm/volta/`~/.local/bin` to PATH. `hero cycle archive` also searches those dirs and runs `openspec` with matching `node` on PATH. If the linked change dir is already gone (manual archive), skip the OpenSpec CLI and continue Hero archive. `/hero-archive` stays on `orchestration_agent` + `/hero-model` (fresh session; no stage-agent Task).

---

## 2026-08-13 — TUI Approvals history + Artifacts discovery

**Problem**: Approvals showed only “No stage pending approval” (no history). Artifacts was always empty because nothing registered rows in SQLite.

**Decision / Outcome**: Approvals lists pending stage plus chronological requested/approved/rejected/escalated/continued events. Artifacts discovers cycle files on disk (current cycle dir, linked OpenSpec change, documents.json for this cycle, cycle-tagged docs) and merges store metadata. Status/Approvals/Artifacts/Costs/Events clip and scroll; switching those screens refreshes.

---

## 2026-08-13 — Release v1.0.7

**Problem**: Chat `/` opened the full-screen palette during live `/hero-start` sessions; Events showed UTC; Costs showed raw ms with broken column alignment.

**Decision / Outcome**: Inline Chat slash overlay; Events local timestamps; Costs aligned table with `mm:ss` duration. Tagged `v1.0.7`.

---

## 2026-08-13 — TUI Events/Costs display polish

**Problem**: Events screen showed UTC timestamps (`truncateTS` on RFC3339). Costs screen showed duration in raw milliseconds with misaligned columns (long model names broke layout).

**Decision / Outcome**: `formatEventTimeLocal` parses stored UTC RFC3339 and renders local `15:04:05`. Costs table uses dynamic column widths, header row, right-aligned numeric columns, and `formatDurationMMSS` (e.g. `540000ms` → `09:00`, no rounding).

---

## 2026-08-13 — Chat `/` overlay (Cursor-style)

**Problem**: Typing `/hero-approve` in Chat opened the full-screen palette. Enter ran TUI Execute, which failed with `No stage pending approval` while the orchestrator was still asking in Chat (SQLite not yet `PendingApproval`). Typing `hero-approve` without `/` worked as a follow-up.

**Decision / Outcome**: In Chat, `/` stays in the composer with a filtered `/hero-*` overlay (Enter inserts; next Enter sends). Other screens keep the palette. Live `/hero-start` session: `/hero-approve` and other control slashes are follow-ups, not TUI Execute.

---

## 2026-08-13 — Release v1.0.6

**Problem**: TUI Execute used YAML orchestrator model slugs rejected by Cursor Agent CLI; long errors clipped; `resource_exhausted` failed follow-ups after `/hero-start`.

**Decision / Outcome**: TUI Execute uses `/hero-model` default only; wrapped errors in response pane; Cursor adapter retries `resource_exhausted`. Tagged `v1.0.6`.

---

## 2026-08-13 — TUI default model, error wrap, resource_exhausted retry

**Problem**: `/hero-sync` and `/hero-status` passed YAML `agents.orchestration_agent` as CLI `--model` (`gpt-5.3-codex-medium`), which Cursor Agent CLI rejected. Long execute errors were a single unwrapped line (clipped). Follow-ups after `/hero-start` failed with `RetriableError: [resource_exhausted]` while the same prompt worked in Cursor chat.

**Decision / Outcome**: TUI Execute always uses `/hero-model` (`hero.json`); errors if unset. YAML agent models stay Runtime/Task-only. Execute errors wrap inside the scrollable response pane. Cursor adapter retries `resource_exhausted` / `RetriableError` (auth/trust still fail fast).

---

## 2026-08-13 — Release v1.0.5

**Problem**: TUI stage flow broke multi-stage cycles (session id lost, Tasks not returning, approval gates skipped); legacy `d` dispatch and spinner placement were confusing.

**Decision / Outcome**: Persist harness session across stages; orchestrator waits for Task completion; parser emits Task events; palette Go to Chat; Agent-line spinner. Tagged `v1.0.5`.

---

## 2026-08-13 — TUI: drop `d` dispatch, Go to Chat, Agent wait spinner

**Problem**: `d` still ran legacy harness Dispatch from Status/Approvals; palette had no Go to Chat; wait spinner sat beside “Waiting for harness” in the response body.

**Decision / Outcome**: Removed the `d` dispatch shortcut and footer hints. Palette includes `Go to - Chat`. Braille spinner lives on the Agent status line until Execute completes.

---

## 2026-08-13 — TUI stage flow: Task return + session id + approvals

**Problem**: In `task_manager` TUI, Research asked to start Planning instead of auto-advancing (`require_human_approval: false`). Planning (`true`) skipped `/hero-approve` and asked a yes/no about Implementation. Implementation/QA ran via Task but did not appear in Chat or return to the orchestrator. Header showed `Cycle C1 — qa · iter 0/2` with no session id.

**Cause**: (1) `syncConversationContext` replaced the live orchestrator session with the next stage's empty `harness_session_id`; `/hero-start` cleared `conversationStage` so Execute never persisted the id. (2) Prompts asked permission to start the *next* stage and allowed `run_in_background: true`, so Tasks did not return and nested work never streamed. (3) Stream parser skipped Task completed events.

**Decision / Outcome**: Keep the live TUI session across stages and persist it on `/hero-start` follow-ups. Orchestrator/stage assets: wait for Task (`run_in_background: false`), close the finished stage, slash CTAs only when approval is required. Parser emits Task start/complete.

---

## 2026-08-13 — TUI `/hero-start` Shell rejection + workspace leak

**Problem**: In `task_manager`, TUI `/hero-start` froze. Cursor Agent CLI `--print` without `--force` auto-rejected Shell (no TTY for Auto-review). The orchestrator then grepped parent/sibling paths and read Hero framework source. Ctrl+C did not abort because TUI skipped `Cancel` when `harnessSessionID` was still empty.

**Decision / Outcome**: Adapter Execute now passes `--force` and `--workspace <projectDir>`. `Cancel("")` kills the pending in-flight process; TUI Ctrl+C always calls it. Runtime prompts (orchestration_agent, hero-start, workflow-hero skill, TUI start preamble) constrain work to the consumer project root.

---

## 2026-08-13 — Release v1.0.4

**Problem**: TUI `/hero-start` froze when Cursor Agent CLI rejected Shell without `--force`; orchestrator leaked into parent Hero source tree; Ctrl+C could not abort before session id.

**Decision / Outcome**: Harness passes `--force --workspace`; cancel pending execute; runtime workspace guards. Tagged `v1.0.4`.

---

## 2026-08-12 — TUI control commands parity (`/hero-cancel`, `/hero-finish`, `/hero-continue`, `/hero-back`)

**Problem**: TUI used CLI direct (`Cancel`/`Finish`/`Continue`) or Dispatch (`/hero-back`) — no orchestrator markdown, no git rollback, no metrics/context updates, fixed +1 continue, no stage resume.

**Decision / Outcome**: All four use Runtime Execute with `orchestration_agent.md` + command markdown + TUI preambles; model from `agents.orchestration_agent`; Go gates (active cycle; Escalated for continue; Judge PendingApproval for back); inline `/hero-cancel <reason>` and `/hero-continue N` in Chat. Removed `cancelCmd`/`finishCmd`/`continueCmd`; `/hero-back` no longer uses Dispatch. Tests + `comparation.md` + `hero-control-alignment-plan.md` updated.

---

## 2026-08-12 — TUI `/hero-reject` parity

**Problem**: TUI `/hero-reject` called `svc.Reject("")` directly — empty reason, no stage re-run, no orchestrator markdown.

**Decision / Outcome**: Two-step Chat flow collects rejection feedback; then Runtime Execute with `orchestration_agent.md` + `hero-reject.md` + TUI preamble embedding user reason; model from `agents.orchestration_agent`; gates for active cycle + `PendingApproval`; inline `/hero-reject <reason>` supported in Chat. Tests + `comparation.md` + alignment plan updated.

---

## 2026-08-12 — TUI `/hero-approve` parity

**Problem**: TUI `/hero-approve` called `svc.Approve("", "")` directly — no metrics, no summary, no orchestrator markdown.

**Decision / Outcome**: Runtime Execute on Chat screen with `orchestration_agent.md` + `hero-approve.md` + TUI preamble; model from `agents.orchestration_agent`; gates for active cycle + `PendingApproval`; agent runs Metrics Procedure and `hero approve --metrics-json`. Tests + `comparation.md` + alignment plan updated.

---

## 2026-08-14 — TUI: remove Approvals, `/new-chat`

**Problem**: Approvals screen duplicated Chat slash workflow; users needed a way to reset the chat session without `/hero-new`.

**Decision / Outcome**: Removed Approvals screen and `Go to - Approvals` (tabs/shortcuts now Chat, Status, Artifacts, Costs, Events — `alt+1`–`alt+5`). Added `/new-chat` to palette + Chat slash overlay: clears transcript, harness session, orchestrator state; uses default model; blocked while agent is streaming (message prompts wait or ctrl+c). Chat newline (shift+enter) attempted and **reverted** — caused TUI input freeze; Enter submits only.

---

_Older 2026-08-12 / 2026-08-10 notes (cycle prepare/sync, slash parity, Chat panes) are in git history._
