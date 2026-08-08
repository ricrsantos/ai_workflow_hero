# PRD-C01-001 — Hero 1.0: Harness Orchestrator, AI Loop, TUI & SQLite

> Cycle C1 product requirements. Supersedes prior V1 “chat-only orchestration” product framing where they conflict. Historical 0.9.x behavior remains documented in [PRD.md](PRD.md). Idea drafts archived at `docs/idea/archive/v1/` are inspiration only.

## 1. Overview

**Hero 1.0** transforms Hero from a Cursor-chat Runtime coordinator into a **harness orchestrator with an AI Loop**:

- A **Go state machine** owns cycle/stage transitions, gates, events, and cost tracking (still **no LLM reasoning** in the CLI).
- **SQLite** is the sole store for Hero-exclusive operational data (token-efficient; no cycle markdown projections such as `workflow.md` / `metrics.md`).
- Users choose an **entry UI with parity**: **Cursor chat** (same experience as today) **or** **Hero TUI**; both drive the full cycle over the same core via the **`hero` CLI as API**.
- **Cursor-only** concrete harness in 1.0, behind a stable **`HarnessAdapter`** interface.
- **Project vs Hero separation** remains mandatory: do not kidnap the project — `context/*.md`, `docs/`, `openspec/`, and code stay first-class files for agents outside Hero.

## 2. Goals

- Ship Hero **1.0** as a usable orchestrator (core + TUI), not a docs-only leap.
- Keep Cursor-chat workflow experience; add equivalent TUI entry.
- Cut token waste from agents reading Hero operational markdown.
- Establish adapter boundary for future harnesses without implementing them in 1.0.
- Provide a documented **breaking major** upgrade from 0.9.x (`hero upgrade` → SQLite migration).

## 3. In Scope (1.0)

| Area | Requirement |
|---|---|
| AI Loop / Workflow Engine | Deterministic Go state machine: stage advance, approval gates, iteration/timeout accounting, lock/concurrency. |
| State Store | SQLite under Hero-owned paths (e.g. `.workflow-hero/`). Cycle/stage status, events, metrics/costs, operational conversation/context history, artifact **metadata**. |
| CLI as API | Subcommands for status, advance, approve/reject/cancel/finish equivalents, metrics, events, cycle lifecycle. Slash-command Runtime and TUI invoke these — no daemon in 1.0. |
| Dual UI parity | Chat **or** TUI; both can run the full cycle. TUI is not monitor-only. |
| Cursor adapter | Interface + Cursor implementation; chat-driven path preserved; TUI/`hero run` can dispatch via adapter when available. |
| Conversation (minimal) | Approvals and clarifying questions surfaced in chat and TUI. |
| Cost tracker | Evolve current metrics into SQLite-backed estimates; query via CLI/TUI. |
| TUI | Bubble Tea (+ huh where appropriate). Screens: cycle status, approvals, artifacts, costs/metrics, recent events, basic command palette. UX **inspired by** Claude Code TUI (patterns, not a clone). |
| Config | User-editable config files (e.g. `workflow-config.yml`) remain; imported into store as needed. |
| Upgrade | SemVer **major** 1.0.0; `hero upgrade` migrates; legacy cycle markdown ceases to be canonical. |
| Idea archive | Move `docs/idea/v1/` → `docs/idea/archive/v1/`; cycle docs supersede on conflict. |

## 4. Out of Scope (deferred — post-1.0)

Canonical deferred list (also in cycle research checkpoint):

| ID | Item |
|---|---|
| D1 | Concrete multi-harness (OpenCode, Claude Code, …) |
| D2 | External integrations (Slack, GitHub Issues, cloud, …) |
| D3 | Rich multi-channel Notification Manager |
| D4 | Expanding Browser Visual Validation beyond current Runtime |
| D5 | TUI as sole primary UI (removing slash-command parity) |
| D6 | “Almost everything” from archived idea roadmap Fases 1–3 |
| D7 | Daemon + local RPC (`hero serve`) |
| D8 | Full event bus with plugins/external subscribers |
| D9 | Human-readable cycle markdown projections / dual-write |
| D10 | Full rich TUI as in archived `12_hero_tui.md` |
| D11 | Soft dual-mode compatibility with 0.9.x markdown cycles |
| D12 | Treating rewritten idea docs as living product spec |
| D13 | Chat-only or push-only without dual-UI parity |

## 5. Functional Requirements (delta)

### 5.1 Entry points

- **Chat**: existing `/hero:*` experience, updated to call CLI API for operational state instead of editing cycle markdown.
- **TUI**: `hero tui` (name TBD in SDD) launches Bubble Tea app; can drive cycle including adapter dispatch.
- User picks one entry; both share SQLite state.

### 5.2 Persistence

- **Write** Hero-exclusive ops **only** to SQLite.
- **Do not** generate `workflow.md` / `metrics.md` projections (token economy).
- **Keep** project `context/*.md` (and other project docs) as real files; Hero may update them after meaningful cycle outcomes.
- Query: `hero status`, `hero metrics`, `hero events` (exact verbs in SDD).

### 5.3 Events

- Append-only event log in SQLite (`stage_started`, `approved`, `harness_invoked`, …).
- No external subscribers in 1.0.

### 5.4 Stage close sequence (1.0)

Replace “update workflow.md / metrics.md” with:

1. Summary + approval request (respect `require_human_approval`).
2. Persist stage transition + metrics to SQLite via CLI API.
3. Show metrics summary in chat/TUI (tokens, duration, cost) with pointer to `hero metrics` (and project-wide summary command/file as designed in SDD).
4. Advance to next configured stage.

### 5.5 CLI vs reasoning (amended ADR-003)

- CLI/Go: install, upgrade, state machine, store, adapter **invocation plumbing**, TUI rendering — **never** LLM reasoning.
- Stage **reasoning** remains in harness agents (Cursor chat / Task), same philosophy as 0.9.x.

## 6. Non-Functional Requirements

- Platforms: Linux/macOS, amd64/arm64 (unchanged).
- Deterministic CLI; testable state machine without LLMs.
- Token efficiency: agents must not need to read Hero ops markdown from the tree.
- Upgrade path documented; checksum-safe asset upgrade preserved.
- Soft secrets hygiene unchanged.

## 7. Success Criteria

- User can complete a cycle via **chat** with UX parity to 0.9.x (commands/flow), backed by SQLite.
- User can complete a cycle via **TUI** with the same stage/approval/metrics semantics.
- `hero upgrade` from 0.9.x produces a working SQLite-backed install; legacy cycle markdown is not required at runtime.
- `go test ./...` green; doctor/status/metrics commands work against the store.
- Deferred items D1–D13 are not required to ship 1.0.

## 8. References

- Research checkpoint: `.workflow-hero/cycles/current/research-checkpoint.md`
- Architecture: [ADR-C01-001-hero-1-0.md](../architecture/ADR-C01-001-hero-1-0.md), [ADR.md](../architecture/ADR.md)
- UI: [UI-C01-001-hero-tui.md](UI-C01-001-hero-tui.md), [UI.md](UI.md)
- Deploy: [DEPLOY.md](../deployment/DEPLOY.md)
- Archived idea (inspiration): [docs/idea/archive/v1/](../idea/archive/v1/)
