## Context

Hero today (0.9.x) is a deterministic Go CLI for install/upgrade/doctor plus a Cursor Runtime that orchestrates stages by editing `.workflow-hero/cycles/current/workflow.md` and `metrics.md`. Cycle C1 Research approved PRD-C01-001 / ADR-012–019 / UI-C01-001: leap to **1.0** harness orchestrator with Go-owned AI Loop, SQLite ops store, CLI-as-API, Cursor HarnessAdapter, and dual UI (chat + Bubble Tea TUI). Idea drafts live under `docs/idea/archive/v1/` (non-normative). Scope: **native** → implementation via `generic_agent`.

Constraints: ADR-002 vertical slices; ADR-003 amended (CLI never reasons; CLI owns state transitions); no daemon (D7); no cycle markdown projections (D9); Cursor-only concrete harness (D1 deferred).

## Goals / Non-Goals

**Goals:**

- Ship a testable Go state machine + SQLite store as the single source of truth for Hero ops.
- Expose that engine through CLI verbs used by Runtime slash commands and TUI (parity).
- Minimal Bubble Tea TUI that can drive the full cycle (including adapter dispatch when available).
- Breaking `hero upgrade` from 0.9.x → 1.0 SQLite + refreshed Runtime assets (DEPLOY §3.1).
- Preserve project files (`context/*.md`, `docs/`, `openspec/`, code) as first-class artifacts.

**Non-Goals:**

- Deferred D1–D13 (multi-harness, daemon, event bus, markdown projections, rich TUI, soft dual-mode, etc.).
- Replacing slash commands with TUI-only UX (D5).
- Expanding Browser Visual Validation beyond current Runtime (D4).

## Decisions

### D1 — Package layout (ADR-002)

| Package | Responsibility |
|---|---|
| `internal/store` | SQLite open/migrate/schema; repositories for cycle, stage, event, metrics, artifact metadata |
| `internal/engine` | Deterministic state machine (advance, approve/reject/cancel/finish/continue, lock, iteration/timeout rules) |
| `internal/cycle` | Cobra commands that are pure CLI-as-API over store+engine (metrics, events, approve, …) |
| `internal/harness` | `HarnessAdapter` interface; Cursor impl may live in `internal/adapters/cursor` extending existing paths package |
| `internal/tui` | Bubble Tea app; calls engine/store via same service APIs as CLI (not shelling out to self in-process) |
| Existing `install` / `upgrade` / `doctor` / `status` | Extended for SQLite bootstrap, migration, integrity |

**Alternatives considered:** Single `internal/workflow` mega-package — rejected (hurts vertical-slice clarity). Daemon with RPC — deferred (D7).

### D2 — SQLite location and schema (ADR-013)

- **Path:** `.workflow-hero/hero.db` (Hero-owned; not a project artifact).
- **Driver:** pure-Go SQLite (`modernc.org/sqlite`) to keep cross-compile simple without CGO on release targets.
- **Schema (v1):**
  - `schema_migrations` (version)
  - `cycles` (id, number, title, objective, status, started_at, completed_at, config_snapshot_json, lock_holder, lock_at)
  - `stages` (cycle_id, name, status, iteration, max_iterations, extra_iterations, require_human_approval, started_at, completed_at, summary)
  - `events` (id, cycle_id, ts, type, payload_json) — append-only
  - `metrics` (cycle_id, stage_name, model, input_tokens, output_tokens, cost_usd, duration_ms, …)
  - `artifacts` (cycle_id, path, kind, label, created_at) — metadata only
  - `conversation` (id, cycle_id, ts, role, kind, body) — minimal approvals/questions
- **Import:** On cycle start / upgrade, read `workflow-config.yml` (and related config) into store snapshot; files remain editable source for users.
- **No** writes of `workflow.md` / `metrics.md` in 1.0.

**Alternatives:** Keep dual-write markdown — deferred (D9). Embed DB outside `.workflow-hero/` — rejected (violates Hero-owned boundary).

### D3 — CLI verb set (ADR-014; UI-C01-001 §4)

Final verbs for 1.0:

| Command | Kind | Behavior |
|---|---|---|
| `hero status` | read | Stage machine view from SQLite (extend existing) |
| `hero metrics` | read | Per-stage + totals from SQLite |
| `hero events` | read | Recent/filtered event log (`--limit`, optional `--type`) |
| `hero approve` / `reject` / `cancel` / `finish` / `continue` | mutate | Map to engine transitions (flags for non-interactive) |
| `hero cycle new` | mutate | Create cycle row + stages from config; replace markdown init |
| `hero cycle archive` | mutate | Archive current cycle in store + filesystem archive folder naming from `completed_at` |
| `hero cycle resume` | mutate | Resume paused cycle semantics |
| `hero tui` | UI | Launch Bubble Tea |
| `hero run` | dispatch | Optional Cursor adapter push when driving from TUI |

All reads: table default + `--json`. Errors: `✗ Error: <message>.` (UI.md §5). Mutating commands take `--project`/`cwd` as today (repo root discovery unchanged).

**In-process rule:** TUI and CLI commands share Go service APIs; Runtime agents shell out to `hero` binary (CLI-as-API).

### D4 — State machine rules (ADR-012)

- Stage order and enablement from imported `workflow-config.yml` snapshot (same stage list as 0.9 Runtime, including Browser UI Validation when enabled).
- Transitions: Waiting → Running → (PendingApproval | Completed | Escalated | Failed) per existing semantics.
- Approvals honor `require_human_approval`; auto-complete when false after successful stage close payload.
- Iteration/timeout escalation → Escalated; `hero continue` grants extra iterations.
- File or DB lock: reject concurrent writers with clear error (UI-C01-001 §6). Prefer SQLite transaction + `cycles.lock_holder` for cross-process.

### D5 — HarnessAdapter (ADR-016)

```go
type HarnessAdapter interface {
    Name() string
    // EnsureChatPath documents/supports chat-driven execution (Cursor slash commands).
    SupportsChat() bool
    // Dispatch attempts push execution for a stage from TUI/CLI when the harness allows it.
    Dispatch(ctx context.Context, req DispatchRequest) (DispatchResult, error)
}
```

Cursor impl: chat path is always the reliable baseline; `Dispatch` best-effort (may return “use chat” guidance if IDE APIs unavailable). No other harnesses in 1.0.

### D6 — TUI (ADR-017; UI-C01-001)

- Stack: Bubble Tea + huh for discrete forms; colors/icons from UI.md.
- Screens: Status, Approvals, Artifacts, Costs, Events, Palette.
- Actions call the same engine services as CLI.
- Non-TTY / `NO_COLOR`: refuse interactive TUI with actionable error; CLI still works.
- Inspiration: Claude Code keyboard-first / low-chrome patterns — not a clone.

### D7 — Runtime asset updates (ADR-003/012)

- Orchestration, skills, and `/hero:*` commands: stop reading/writing `workflow.md`/`metrics.md`; call `hero status|metrics|events|approve|…`.
- Stage close sequence per PRD-C01-001 §5.4: summary → CLI persist → show metrics + pointer to `hero metrics` → advance.
- Keep Metrics Procedure math (chars÷4 × models) but **persist via CLI** (`hero metrics record` internal flag or approve payload carrying metrics) — prefer: mutating approve/finish accept `--metrics-json` from agents so estimation stays in Runtime, storage in Go.

### D8 — Upgrade migration (ADR-018; DEPLOY §3.1)

- Bump default version `1.0.0`.
- `hero upgrade`: create `hero.db` if missing; run migrations; if legacy `workflow.md` exists for current cycle, **one-shot import** into SQLite then leave files in place but **non-canonical** (doctor may warn “legacy markdown present, ignored”); do **not** maintain soft dual-mode (D11).
- Refresh Runtime assets with checksum-safe behavior unchanged.
- Document in `workflow-help.md` / DEPLOY (already partially updated).

### D9 — Idea archive (ADR-019)

Research already moved `docs/idea/v1/` → `docs/idea/archive/v1/`. Implementation task: verify tree + AGENTS/README pointers; docs-only fixes if stale links remain. No product behavior from idea folder.

## Risks / Trade-offs

- **[Risk] Cursor Dispatch limited by IDE APIs** → Mitigation: chat path remains first-class; `hero run` returns clear fallback (ADR-016 consequences).
- **[Risk] Process-per-CLI-call overhead** → Mitigation: acceptable for 1.0; daemon deferred (D7).
- **[Risk] In-flight 0.9.x cycles lose soft dual-mode** → Mitigation: one-shot import on upgrade; document breaking major; D11 explicit.
- **[Risk] Agents still write markdown out of habit** → Mitigation: Runtime prompt updates + contract tests asserting no “update workflow.md” instructions; doctor optional warn on new markdown writes not required for 1.0.
- **[Risk] Schema evolution** → Mitigation: versioned migrations table from day one.

## Migration Plan

1. Implement store + engine + CLI verbs behind tests.
2. Update Runtime assets to CLI API; keep install embedding new assets.
3. Extend upgrade path; add integration test: temp 0.9-like tree → upgrade → `hero status` works from DB.
4. Ship TUI last against stable services (or parallel after engine API freeze).
5. Version `1.0.0`; update help/README/context files.
6. Rollback: users stay on 0.9.x binary; no automatic downgrade of DB (document).

## Open Questions

None blocking Planning — verb names and DB path are decided above. If Cursor Task/dispatch APIs change before Implementation, `Dispatch` remains best-effort without blocking chat parity.
