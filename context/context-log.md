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

---

## 2026-08-10 — TUI title rename

**Decision / Outcome**: TUI header title changed from "Hero TUI" to "AI Hero" (`internal/tui/screens.go`, loading message in `app.go`). Tests green (`go test ./internal/tui/...`).

---

## 2026-08-10 — TUI Chat OpenCode UX + `/hero-model`

**Decision / Outcome**: Chat input redesigned (boxed accent bar, Build/Plan via Tab → Cursor `--mode plan`); boot lists models via `agent models` (`harness.ModelLister`); `/hero-model` picker persists slug to `hero.json` harnesses; `ExecuteRequest.Mode` + `SaveHarnessModel`. Docs: UI-C03-001 §3; asset `hero-model.md`.

---

## 2026-08-10 — Chat input polish

**Decision / Outcome**: Homogeneous `chatBg` inside the input box; blank line between text and Build/model status; removed `type here` and blink/`|` caret; focus-based filled vs hollow caret; ←→ (and Home/End) navigate `inputCursor` for insert/backspace.

---

## 2026-08-10 — Chat accent bar + white caret + no Esc clear

**Decision / Outcome**: Accent bar flush left on every row (solid bg cell, no left padding); white filled caret; Esc no longer clears chat input (hints updated).

---

## 2026-08-10 — Screen/quit shortcuts use modifiers

**Decision / Outcome**: Bare `q` / `1–6` removed (typed into chat). Quit is `ctrl+q`. Screen nav matches `ctrl+N` and `alt+N` (Bubble Tea v1 cannot distinguish Ctrl+digit on most terminals — Alt+digit is the reliable binding; hints show `alt+1-6`). Removed chat empty-input special-case for bare digits/`q`.

---

## 2026-08-10 — Live harness streaming in TUI chat

**Decision / Outcome**: Stream executes no longer wait for full buffered stdout. Cursor `Execute` with `Stream: true` passes `--stream-partial-output` and uses `StreamingCommandRunner.RunStreaming` → `io.Pipe` → `ParseStreamJSON` so `OnStreamDelta` fires while the agent runs. TUI `executeDoneMsg` overwrites agent transcript with canonical `result.Output`. Tests cover flag + mid-run deltas.

---

## 2026-08-10 — Stream thinking + tool activity in TUI chat

**Decision / Outcome**: `OnStreamDelta` now carries `StreamDelta{Kind, Text}` (`text` / `thinking` / `tool`). Parser forwards Cursor `thinking` deltas and `tool_call` started events (plus thinking content parts on assistant messages). TUI shows muted italic `Thinking:` and `→ Read path` above the agent answer; completion still replaces only the agent bubble with `result.Output`. Note: Cursor docs say thinking is suppressed in `--print`, but stream-json examples from Cursor staff include `thinking` events — parse them when present.
