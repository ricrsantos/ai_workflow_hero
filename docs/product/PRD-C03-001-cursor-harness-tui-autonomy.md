# PRD-C03-001 — Cursor Harness Autonomy & TUI Orchestration

> Cycle C3 product requirements. Corrects V1 implementation distortions: Cursor HarnessAdapter must execute via Cursor Agent CLI; Hero TUI must be fully autonomous from Cursor chat. Index: [PRD.md](PRD.md).

## 1. Overview

Hero V1 shipped a **minimal** `HarnessAdapter` (`Dispatch` only) with `Pusher == nil`, so TUI and `hero run` always fall back to “continue in Cursor chat.” That contradicts the V1 architecture in `docs/idea/archive/v1/06_harness_adapter.md` and `07_cursor_adapter.md`: the Runtime orchestrates workflow state; the harness executes AI via **Cursor Agent CLI** (`cursor agent`).

This cycle delivers:

1. **Full HarnessAdapter contract** with a working **Cursor Adapter** (CLI discovery, sessions, execute, resume, streaming, metrics).
2. **Autonomous TUI** — full development cycle (including interactive Research / grill-me) without depending on an open Cursor chat panel.
3. **Slash vocabulary correction** — canonical user-facing form is **`/hero-<name>`** (hyphen), amending ADR-020.
4. **New Runtime commands** — `/hero-cycles`, `/hero-todos`; extended `/hero-sync`.
5. **Dual-entry preserved** — TUI is the primary autonomous path; Cursor chat remains a supported entry with capability parity.

## 2. Goals

- TUI + Cursor harness can run the entire Hero cycle, including multi-**interação** conversational stages (Research grilling).
- Go Workflow Engine owns cycle state (SQLite); harness owns AI execution sessions; no LLM calls from Go.
- User sees real-time agent output in TUI (`stream-json`).
- Fix C2 naming mistake (`/hero:*` in TUI labels) to match actual Cursor command files (`hero-start.md` → `/hero-start`).
- Surface cycle history and project backlog without opening chat.

## 3. Terminology

| Term | Definition |
|---|---|
| **Etapa** | Workflow stage: Research, Planning, Implementation, QA, Judge, … |
| **Interação** | One user ↔ agent conversational round within an etapa (e.g. one grill-me question and answer). Not the same as etapa. |

## 4. In Scope (C3)

### 4.1 HarnessAdapter (full contract)

Implement the interface described in `06_harness_adapter.md` (Go package `internal/harness`):

| Method | Responsibility |
|---|---|
| `IsAvailable` | CLI on PATH, executable, minimum version, auth/login state |
| `CreateSession` | Start harness execution session |
| `ResumeSession` | Continue existing harness session (`--resume`) |
| `Execute` | Run prompt; support `--print`, `--output-format json` and `stream-json` |
| `Cancel` | Abort in-flight execution |
| `Status` | Report session/execution status |

`Dispatch` may remain as a thin compatibility wrapper over `Execute` for existing call sites.

### 4.2 Cursor Adapter

Per `07_cursor_adapter.md`:

- Resolve binary: `cursor-agent` on PATH and/or `cursor agent` subcommand.
- Map `ExecutionResult`: session ID, summary, output, usage, duration.
- Parse JSON / stream-json from CLI stdout.
- Translate errors (not logged in, CLI missing, version too old) into actionable messages (e.g. suggest `cursor agent login`).
- **No** IDE chat injection.

### 4.3 TUI autonomous orchestration

- **Conversation screen** (or integrated panel): shows agent output with **streaming**; user input for interações during interactive etapas.
- Each **interação** dispatches to harness (`Execute` / `ResumeSession`); etapa state transitions remain in Go (`cycle.Service` / engine).
- TUI exposes **all** Hero slash commands (`hero-*.md`) via palette and/or slash input — not only approve/reject/finish subset.
- Non-Hero imported commands continue via markdown expansion → harness `Execute`.
- Batch etapas (e.g. doc generation after grilling) use harness execution per agent prompt (Runtime markdown as prompt source).

### 4.4 TUI harness selection (boot)

- On launch, if `hero.json` → `cli.tools` has no harness: prompt user to select harness.
- Auto-detect markers; if none, show supported list (**V1: Cursor only**).
- On selection: validate harness (`IsAvailable`); on failure show CLI error + guidance; **abort TUI** (do not enter main loop).
- Persist selection to `cli.tools`.
- TUI does **not** depend on `hero doctor` (doctor keeps independent checks).

### 4.5 Slash vocabulary (`/hero-<name>`)

- Amend ADR-020: canonical user-facing commands use **hyphen** (`/hero-start`, `/hero-sync`, …).
- Update Runtime assets (`assets/cursor/commands/hero-*.md` headers), TUI labels, README, workflow-help, orchestration skill copy.
- Internal CLI verbs unchanged (`hero approve`, `hero stage start`, …).

### 4.6 `/hero-cycles` (new)

- List all cycles: SQLite (`hero.db`) **and** legacy folders under `.workflow-hero/cycles/archive/` without DB rows.
- Per cycle: number, title, status, dates.
- Per etapa: tokens, duration, cost (from SQLite metrics; parse `metrics.md` for archive-only legacy).
- Show subtotals and grand totals.

### 4.7 `/hero-todos` (new)

- Read and display pending items from `context/current-state.md` only (e.g. `## Pending Features` and related sections defined in SDD).
- Always show notice: if product/architecture docs may have changed, run `/hero-sync` then `/hero-todos` again.
- Available in chat Runtime and TUI.

### 4.8 `/hero-sync` (extended)

- Keep existing behavior (context_agent scan, AGENTS.md, current-state, context-log, project.json, doctor harness warnings).
- **Additionally**: scan `docs/product/` and `docs/architecture/` for documented pending / deferred / out-of-scope-not-yet-implemented items; merge into `context/current-state.md` pending sections per SDD rules.
- Does not auto-run when user invokes `/hero-todos` — user triggers sync manually.

### 4.9 Dual-entry

- TUI: primary autonomous orchestration path.
- Cursor chat: same slash commands and capability parity; chat path does not require TUI.

### 4.10 `hero doctor` (complementary)

- May add Cursor CLI / login checks for diagnostics.
- Not a prerequisite for TUI boot (TUI validates independently).

## 5. Out of Scope

- Non-Cursor harness implementations (D1) — only list Cursor in boot selector.
- Windows CLI target.
- IDE chat injection / pushing prompts into open Cursor panel.
- Replacing Runtime markdown orchestration logic with Go LLM reasoning (ADR-003 preserved).
- Daemon / background agent process (ADR-014).

## 6. Non-Functional Requirements

- Feature Based + Vertical Slice: extend `internal/harness`, `internal/adapters/cursor`, `internal/tui`, `internal/cycle`, Runtime assets.
- Tests: adapter CLI parsing (injectable runner), TUI harness boot validation, slash label golden tests, `hero-cycles` / `hero-todos` behavior, sync doc scan fixtures.
- Chat language: `workflow_config.user_preferred_language`; cycle docs English.
- Layer separation: Workflow Engine never shells out to Cursor directly — only adapters.

## 7. Success Criteria

- With Cursor Agent CLI installed and logged in, user completes Research grilling entirely in TUI with streamed responses.
- `hero` (TUI) without `cli.tools` prompts harness selection, validates, persists, or aborts with clear login guidance.
- TUI palette lists `/hero-start`, `/hero-new`, … (hyphen), not `/hero:start`.
- `/hero-cycles` shows C1–C3 metrics from SQLite and archived cycles.
- `/hero-todos` reflects `current-state.md`; notice mentions `/hero-sync`.
- `/hero-sync` updates pending items from PRD/ADR index docs into `current-state.md`.
- Cursor chat path still works for `/hero-start` and new commands.

## 8. References

- [PRD-C01-001-hero-1-0.md](PRD-C01-001-hero-1-0.md)
- [PRD-C02-001-slash-parity-tui-harness.md](PRD-C02-001-slash-parity-tui-harness.md)
- [ADR-C03-001-cursor-harness-tui-autonomy.md](../architecture/ADR-C03-001-cursor-harness-tui-autonomy.md)
- [UI-C03-001-tui-harness-autonomy.md](UI-C03-001-tui-harness-autonomy.md)
- `docs/idea/archive/v1/06_harness_adapter.md`, `07_cursor_adapter.md`
