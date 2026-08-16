# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

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
