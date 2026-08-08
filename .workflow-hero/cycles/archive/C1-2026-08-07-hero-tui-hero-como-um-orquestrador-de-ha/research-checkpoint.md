# Research Checkpoint — Cycle C1

**Status**: Research completed (docs generated; deferred list frozen for 1.0)  
**Updated**: 2026-08-07  
**Stage**: Research · completed → Planning next  
**Chat language**: PT-BR · cycle artifacts: English

## Pre-document gate

**Passed** (user: proceed). Documents generated and registered in `documents.json`.

## Objective (from workflow-config)

Create Hero **1.0**: TUI + Hero as harness orchestrator with AI Loop. Keep current Cursor-chat Runtime capabilities and add refined capabilities from `docs/idea/v1/` (those drafts were written without full knowledge of current Hero state — refine before implementing).

## Agreed decisions (grilling so far)

| # | Topic | Decision |
|---|--------|----------|
| 1 | Ambition | Leap to 1.0 as harness orchestrator + AI Loop; refine `docs/idea/v1/`, remove ambiguities, define scope. Keep Hero↔project separation. Prefer SQLite for efficient context save/recover/sync. |
| 2 | Chat vs TUI | **(A)** Two entry UIs on the **same core** — Cursor chat **or** TUI; both read/write the same cycle state. |
| 3 | SQLite vs files | **(A)** SQLite = **Hero operational store** (cycle state, stages, events, metrics/costs, operational conversation/context history, chat↔TUI sync). Project artifacts stay files. **Do not kidnap the project:** `context/*.md`, `docs/`, `openspec/`, code remain real project files for agents **outside** Hero. Hero may sync/update `context/*.md` but they stay first-class, editable project context — not optional-only exports. |
| 4 | Harnesses in 1.0 | **(A)** Cursor-only in practice + stable `HarnessAdapter` interface; other harnesses post-1.0. |
| 5 | Where the loop runs | **(C)** Hybrid: **Go owns the state machine** (advance stage, gates, metrics, events). Harness/agent executes stage work. Chat and TUI are UIs over the same Go engine. Preserves “CLI does not reason.” |
| 6 | 1.0 package | **(A)** Core + usable TUI (see In / Deferred below). |
| 7 | Chat ↔ Go bridge | **(A)** CLI as API — slash commands/agents invoke `hero` subcommands that read/write SQLite; no daemon in 1.0. |
| 8 | Event system | **(A)** Persistent append-only event log in SQLite; query via `hero`; no external subscribers in 1.0. |
| 9 | Cycle persistence / migration | **(C)+** Hero-exclusive operational state lives **only in SQLite** (no `workflow.md` / `metrics.md` projections — avoid burning tokens when agents read the tree). Users query via `hero` commands. Project files (`context/*.md`, `docs/`, etc.) stay on disk. Config may still be file-based where users edit it; import into store as needed. |
| 10 | TUI stack / screens | **(A)** Bubble Tea + huh where it fits; minimal screens: cycle status, approvals, artifacts, costs/metrics, recent events, basic command palette. **UX inspiration:** Claude Code’s TUI (popular among developers) — inspire interaction patterns, not a clone. Full `12_hero_tui.md` richness → deferred. |
| 11 | 1.0 done / upgrade | **(A)** Breaking SemVer major with migration: `hero upgrade` creates/migrates SQLite; Runtime orchestrates via CLI API; documented path from 0.9.x; legacy cycle markdown ceases to be canonical. Soft dual-mode legacy → deferred. |
| 12 | `docs/idea/v1/` treatment | **(A+C)** Idea folder = historical inspiration only. Canonical requirements come from this grilling + current ADR/PRD, then new cycle docs that **supersede** idea on conflict. Move `docs/idea/v1/` → `docs/idea/archive/` (or equivalent). Do not rewrite idea in-place as product spec. |
| 13 | Cursor stage execution / UI parity | **(C)+** User chooses entry UI: **Cursor chat** (same experience as today) **or** **Hero TUI**. Both must support running the full cycle. Chat-driven path remains; TUI/`hero run` can also drive the loop (dispatch via Cursor adapter when available). Not chat-only monitoring from TUI. Pure headless/multi-harness push without Cursor → covered by D1/D13. |

## In scope for Hero 1.0

- AI Loop / Workflow Engine (Go state machine)
- State Store (SQLite) as **sole** store for Hero-exclusive ops (cycle/stage/events/metrics); sync/update project `context/*.md` as needed; **no** cycle markdown projections
- CLI query commands for cycle/status/metrics/events (replace reading `workflow.md` / `metrics.md`)
- Harness Adapter **interface** + **Cursor** implementation
- Minimal Conversation Layer (approvals / questions)
- Cost Tracker (evolution of current metrics)
- Persistent event log in SQLite (query via CLI; no external subscribers)
- Artifact **metadata** in the store (artifacts themselves stay project files)
- Usable TUI (Bubble Tea + huh): progress, approvals, artifacts, costs, recent events, basic palette — UX inspired by Claude Code TUI (patterns, not a clone)
- Cursor chat as second UI with **parity**: same cycle experience as today; user picks chat **or** TUI
- TUI/`hero run` can drive the loop (Cursor adapter dispatch), not monitor-only
- Refine idea against current Hero; archive `docs/idea/v1/` → `docs/idea/archive/`; cycle PRD/ADR supersede idea on conflict
- Breaking major upgrade path from 0.9.x (`hero upgrade` → SQLite; documented migration)

## Deferred — out of Hero 1.0 (register for later)

> Source of truth for “continue later.” Append as grilling finds more exclusions. Do **not** implement these in the 1.0 cycle unless scope is explicitly reopened.

| ID | Item | Origin / notes | Target (tentative) |
|----|------|----------------|--------------------|
| D1 | Concrete multi-harness (OpenCode, Claude Code, etc.) | Decision 4 — only Cursor impl in 1.0; adapter interface ships | Post-1.0 |
| D2 | External integrations (Slack, GitHub Issues, cloud, monitoring, deploy hooks, …) | Decision 6 / `docs/idea/v1/13_integrations.md` | Post-1.0 |
| D3 | Rich Notification Manager (multi-channel push beyond TUI/chat prompts) | Decision 6 / Runtime §7.10 | Post-1.0 |
| D4 | Browser Visual Validation beyond what already exists in Runtime | Decision 6 — keep current behavior; no 1.0 expansion | Post-1.0 / optional |
| D5 | TUI as **sole** primary UI (replacing slash commands) | Decision 2 keeps dual UI; roadmap Fase 2 “TUI primary” deferred as exclusivity | Post-1.0 |
| D6 | Full roadmap Fase 1–3 “almost everything” | Decision 6 rejected option C | Post-1.0 |
| D7 | Daemon + local RPC (`hero serve`) | Decision 7 — CLI API only in 1.0 | Post-1.0 |
| D8 | Full event bus (in-process plugins / external subscribers) | Decision 8 — SQLite event log only in 1.0 | Post-1.0 |
| D9 | Human-readable cycle markdown projections (`workflow.md`, `metrics.md`, dual-write) | Decision 9 — SQLite-only for Hero ops to save tokens; CLI query instead | Post-1.0 (optional) |
| D10 | Full rich TUI as in archived `12_hero_tui.md` | Decision 10 — minimal Bubble Tea TUI in 1.0; Claude Code as inspiration only | Post-1.0 |
| D11 | Soft dual-mode compatibility with 0.9.x markdown cycles | Decision 11 — breaking major + migrate; no long-lived legacy mode in 1.0 | Post-1.0 (optional) |
| D12 | Treat rewritten `docs/idea/v1/` as living product spec (in-place) | Decision 12 — archive idea; cycle docs are canonical | Won't do for 1.0 |
| D13 | Chat-only stage execution (TUI monitor-only) or push-only without chat | Decision 13 — dual entry with UX parity (chat and TUI both drive the cycle) | Rejected for 1.0 |

_Add new rows (D7…) when further grilling parks a feature outside 1.0._

## Pre-document gate

**Passed** (user: proceed). Documents generated:

- `docs/product/PRD-C01-001-hero-1-0.md`
- `docs/architecture/ADR-C01-001-hero-1-0.md`
- `docs/product/UI-C01-001-hero-tui.md`
- `docs/deployment/DEPLOY.md` (living, 1.0 upgrade §)
- Idea archived: `docs/idea/archive/v1/`

Registered in `.workflow-hero/config/documents.json`.

## Still to grill

None for Research. Deferred exclusions remain in the **Deferred** table (D1–D13).

---

_Update this file as grilling continues. Deferred table is what we register for later — not a full Research pause unless the user asks to stop._
