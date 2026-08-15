# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

---

## 2026-08-15 — TUI Chat context bar

**Problem**: Chat had no view of how full the running agent's context window was. Cursor CLI only reports `usage` on the terminal `result` event; Hero parsed it into `ExecutionResult.Usage` but the TUI ignored it. Model YAML had pricing only.

**Decision / Outcome**: Added `context_window` to `assets/models/*.yml`. TUI loads embed then overlays `.workflow-hero/models`. Lookup is exact slug, then one effort suffix (`-high`/`-fast`/`-medium`/`-low`/`-max`). The scroll-hint line under the green pane shows a right-aligned bar (`█`/`░`, green/yellow/red) plus `used/max`. Used = last Execute input+output tokens; `/new-chat` resets. Bar hidden when the slug has no window. Not a live mid-stream meter (CLI limitation). `go test ./...` passes.

---

## 2026-08-14 — TUI Chat: iterations, orch/discover models, HARN

**Problem**: Header `iter x/x` missed YAML `max_iterations`; Execute used `/hero-model` instead of `agents.orchestration_agent`; Research could not pick a distinct model; agents box showed extra `HARN` for nested generic Tasks.

**Decision / Outcome**: Normalize stage keys in the header; `SyncCycleConfig` updates still-open stage budgets. Orchestrator Execute uses `AgentModelSlug(orchestration_agent)` → fallback_model → `/hero-model`. Added `agents.discover_agent`; TUI Research is a dedicated session with that YAML slug; control slashes stay on ORCH; Research close resumes ORCH. Task parse prefers named Hero agents; `HARN` only for the parent with no Hero agent. Amends ADR-030 §4.

---

## 2026-08-14 — Chat wrap panic, newline, speaker labels, streaming nav

**Problem**: `wrapOutputLine` panicked on multi-byte glyphs (`✔`); Shift/Ctrl+Enter could not insert a newline; green pane said `Agent` instead of 4-letter labels; navigation keys were swallowed while streaming.

**Decision / Outcome**: Wrap on rune spaces (`lastSpaceRune`). **Alt+Enter** inserts a newline. Transcript/status use `[LABEL - model]`. Stream messages always process off Chat; `ctrl/alt+1–6` navigate while streaming; destructive actions show `[y/N]` confirm.

---

_Older 2026-08-13 / 2026-08-12 notes (releases v1.0.4–v1.2.1, slash overlay, archive PATH, Approvals/`/new-chat`) are in git history._
