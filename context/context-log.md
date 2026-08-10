# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

---

## 2026-08-10 — `/hero-sync` (orchestration agent, refresh)

**Problem**: `/hero-sync` requested to refresh Hero artifacts and merge pending items from product/architecture docs (ADR-029).

**Investigation**: Fresh `context_agent` scan (model `composer-2.5`). Pending-doc scan of `docs/product/` and `docs/architecture/` — v1.0.0 release, D1–D13, V2 scope, and `hero upgrade` already captured in `current-state.md`; merged GPG signing deferral and upstream Cursor CLI gaps (plugin skills, nested skill dirs) into technical debt. No `.claude/` / `.windsurf/` / `.codex/` harness markers; `.cursor/` present (matches `hero.json` → `cli.tools: cursor`). `hero doctor` not runnable in agent shell — run `go run ./cmd/hero doctor` locally (expect `version-match` warn until `hero upgrade`).

**Decision / Outcome**: Refreshed `current-state.md` technical debt; verified `AGENTS.md`, `project.json`, secrets hygiene (`.env.example`, `.gitignore` Hero block).

**Rationale**: ADR-029 — keep compression files current after sync.

---

## 2026-08-10 — Empty Artifacts/Costs/Events without active cycle

**Problem**: TUI showed `No * for cycle C0` when no cycle was active.

**Decision / Outcome**: Empty views now say `No active cycle. Run /hero-new to start.` when cycle number is 0; keep `No * for cycle CN` only for a real active cycle with empty data.

---

## 2026-08-10 — TUI status bar + `/hero-sync` UX fixes

**Problem**: `/hero-sync` from the palette looked broken — menu stayed open, no running indicator, truncated/broken return text, parallel agent storms, and Dispatch sometimes resumed a prior session (`--resume`).

**Decision / Outcome**: Fixed footer status bar (running/ok/error with wrap); close palette on select + busy-guard; `Execute` only resumes when `SessionID` is set; `defaultPush` no longer truncates at 240 chars. Tests green (`go test ./...`).

---

## 2026-08-10 — `/hero-sync` (orchestration agent)

**Problem**: `/hero-sync` requested to refresh Hero artifacts and merge pending items from product/architecture docs (ADR-029).

**Investigation**: Fresh `context_agent` scan (model `composer-2.5`). Pending-doc scan of `docs/product/` and `docs/architecture/` — v1.0.0 release, D1–D13, V2 scope, and `hero upgrade` already captured in `current-state.md`; added CI/CD deferral to technical debt. Harness markers: no `.claude/` / `.windsurf/` / `.codex/`; `.cursor/` present (matches `hero.json` → `cli.tools: cursor`). `hero doctor` blocked in agent shell — run `go run ./cmd/hero doctor` locally.

**Decision / Outcome**: Verified `AGENTS.md`, `current-state.md`, `project.json`, `docs/testing/TESTING.md`, secrets hygiene (`.env.example`, `.gitignore` Hero block). Registered `docs/testing/TESTING.md` in `documents.json`.

**Rationale**: ADR-029 — keep compression files and doc registry current after sync.
