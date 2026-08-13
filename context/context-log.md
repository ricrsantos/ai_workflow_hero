# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

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

## 2026-08-12 — Cycle prepare/sync lifecycle (`/hero-new` → `/hero-start`)

**Problem**: Users had to run `hero cycle new` manually after `/hero-new`; title/objective were read at cycle creation instead of at start.

**Decision / Outcome**: `hero cycle new` prepares active cycle with **empty** title/objective (`DeferMeta`); TUI calls `PrepareCycle()` after `/hero-new` stream; `/hero-start` runs `SyncCycleConfig()` (CLI `hero cycle sync-config` in chat) before orchestration. Tests + command markdown + `comparation.md` updated.

---

## 2026-08-12 — TUI `/hero-start` parity

**Problem**: TUI `/hero-start` used `RunWith` + generic Dispatch prompt; did not read `hero-start.md` or `orchestration_agent.md`; required `/hero-model` instead of workflow-config orchestrator model.

**Decision / Outcome**: Runtime Execute on Chat screen with `orchestration_agent.md` + `hero-start.md` + TUI preamble; model from `agents.orchestration_agent` in workflow-config (TUI-only block added to template); requires active SQLite cycle (error + `/hero-new` if missing, no auto `NewCycle`); `internal/workflowconfig` resolves kebab slugs. `/hero-start` no longer gated by `/hero-model`. Tests + `comparation.md` updated.

---

## 2026-08-12 — Release v1.0.1

**Outcome**: Patch release — TUI `/hero-new` parity, required `/hero-model`, Chat formatting, `hero uninstall` interactive confirm. Tagged `v1.0.1`.

---

## 2026-08-12 — hero uninstall interactive confirm

**Decision / Outcome**: `hero uninstall` prompts with huh confirm (`.workflow-hero/` removal warning) instead of exiting with `--yes` only; `--yes` skips prompt; non-interactive still requires `--yes`.

---

## 2026-08-12 — TUI Chat output formatting

**Problem**: Chat agent pane showed broken markdown tables, raw link syntax, and Cursor-only `/hero-start` handoff text from `/hero-new`.

**Decision / Outcome**: `formatChatAgentText` flattens markdown for terminal; TUI `/hero-new` skips `hero cycle new`/confirmation prompts and ends with `/hero-start` closing line; header `Free chat` → `Chat`. Tests green.

---

## 2026-08-12 — Required TUI default model selection

**Problem**: Fresh installs pre-filled `harnesses.cursor.model` with `composer-2.5`; users never explicitly chose a default.

**Decision / Outcome**: Install/upgrade seed empty `model`; `HarnessModelSlugForProject` returns `""` until `/hero-model`. TUI gates Chat submit, `/hero-new`, `/hero-sync`, `/hero-back`, dispatch (`d`), and imported harness commands — opens model picker with status hint when unset. `/hero-start` uses workflow-config orchestrator model (not `/hero-model`). Chat UI shows `not set` until configured. Tests + README/`hero-model.md` updated.

---

## 2026-08-12 — Release v1.0.3

**Problem**: `hero upgrade` falsely reported "customized locally" when `checksums.json` was stale but disk already matched embedded assets (common in dogfooding repos where `.cursor/` is updated via git).

**Decision / Outcome**: Upgrade reconciles checksums silently when `existingHash == newHash` (embedded asset) even if `originalHash` differs. Synced lagging installed copies (`hero-help.md`, `hero-model.md`, `workflow-config.yml`) and refreshed `checksums.json`. Tagged `v1.0.3`.

---

## 2026-08-12 — Release v1.0.2

**Outcome**: Patch release — TUI slash parity with Cursor chat for `/hero-start`, `/hero-approve`, `/hero-reject`, `/hero-cancel`, `/hero-finish`, `/hero-continue`, `/hero-back`, `/hero-sync`, `/hero-status`, `/hero-archive`, `/hero-resume` (Runtime Execute + orchestration agent). Includes `/hero-new`/`/hero-start` cycle-state fix and Grok 4.6 model asset. Tagged `v1.0.2`.

---

## 2026-08-12 — TUI `/hero-sync`, `/hero-status`, `/hero-archive`, `/hero-resume` parity

**Problem**: TUI used Dispatch (`/hero-sync`) or direct CLI (`statusCmd`, `archiveCmd`, `resumeCmd`) — one-line status, no cycle number on resume, no Chat stream parity with Cursor chat orchestrator.

**Decision / Outcome**: All four commands → Runtime Execute with `orchestration_agent.md` + command markdown + TUI preambles. Sync/status/resume resolve model from `agents.orchestration_agent` or fallback `/hero-model`; archive requires active cycle + orchestrator model. Resume supports `/hero-resume [N]` in Chat input. Removed `heroAssetCmd`, `statusCmd`, `archiveCmd`, `resumeCmd`. Plan: `docs/idea/commands_alignments/hero-sync-status-archive-resume-alignment-plan.md`. Tests green.

---

## 2026-08-12 — TUI `/hero-new` parity + default model

**Problem**: TUI `/hero-new` called `hero cycle new` directly (no `hero-new.md` orchestration); `/hero-model` labeled “chat model” only; Dispatch/`hero run` ignored the user-selected default model.

**Decision / Outcome**: `/hero-new` → Chat screen + stream `hero-new.md` with default model (fresh session, multi-turn resume). `DispatchRequest` gains `Model`/`Mode`; TUI passes default model to sync/back/start/imported dispatches. Palette hint `/hero-model` → “select default model”. README + `hero-model.md` updated. Cursor adapter `Execute` passes `--trust` for non-interactive workspace trust. Tests green.

---

## 2026-08-10 — Chat panes: response box + scroll + wait animation

**Problem**: Conversation response area was plain text; user wanted OpenCode-style response pane matching the send box (green accent), scroll, wait animation; later black gutters and dead vertical gaps.

**Decision / Outcome**: Dual OpenCode panes. Black “scrollbar” blocks were nested Background / Width underfill — in-box text is fg-only; box uses `Background`+`Width`; accent rows pad to exact inner width. Sticky frame: footer chrome fixed; response line count = leftover height (probe `buildConversation(0)`). Etapa hint under `ready`; session inline with `│`. Tests green.

---

## 2026-08-10 — `/hero-sync` (orchestration agent, refresh)

**Problem**: `/hero-sync` requested to refresh Hero artifacts and merge pending items from product/architecture docs (ADR-029).

**Decision / Outcome**: Refreshed `current-state.md` technical debt; verified `AGENTS.md`, `project.json`, secrets hygiene. Pending-doc scan already covered v1.0.0 / D1–D13 / V2; merged GPG + upstream Cursor CLI gaps into debt.

---

## 2026-08-10 — Empty Artifacts/Costs/Events without active cycle

**Decision / Outcome**: Empty views say `No active cycle. Run /hero-new to start.` when cycle number is 0.

---

## 2026-08-10 — TUI status bar + `/hero-sync` UX fixes

**Decision / Outcome**: Fixed footer status bar (running/ok/error with wrap); close palette on select + busy-guard; `Execute` only resumes when `SessionID` is set; `defaultPush` no longer truncates at 240 chars.

---

## 2026-08-10 — TUI Chat OpenCode UX (consolidated)

**Decision / Outcome**: Chat input boxed accent bar (Build/Plan via Tab → `--mode plan`); `/hero-model` picker; homogeneous `chatBg`; focus caret; accent flush left; screen nav `ctrl/alt+N`, quit `ctrl+q`. Live stream via `--stream-partial-output` + thinking/tool deltas in transcript. Title: "AI Hero".
