# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

---

## 2026-08-17 — OpenCode serve process leak (TUI)

**Problem**: Each TUI interaction with OpenCode spawned a new `opencode serve` child; dozens of processes accumulated and exhausted RAM/swap.

**Root cause**: `DefaultRegistry.Adapter()` returned a **new** `opencode.Adapter` on every call, so in-memory `baseURL`/`servePID` were always empty and `ensureServe` started another serve. `StopServe` on TUI quit used a fresh adapter (`servePID=0`), only cleared SQLite registry, and used Unix `kill` — processes survived (especially on Windows).

**Decision / Outcome**: Cache singleton adapters in `DefaultRegistry`; `ensureServe` adopts a live serve from `harness_serve_registry` via HTTP health check before spawning; `StopServe` kills all registry PIDs with `os.FindProcess` + `Kill()`; orphan reap uses URL liveness + cross-platform kill. Cursor adapter unchanged — one short-lived CLI process per Execute is by design (`--resume` for continuity).

---

## 2026-08-15 — OpenCode TUI chat hang ("Waiting for harness")

**Problem**: Chat with OpenCode harness stuck on "Waiting for harness…" — no stream deltas, no error.

**Root cause**: `OpenCodeAdapter` had three integration bugs vs real `opencode serve` 1.18.x: (1) `defaultServeURLResolver` hardcoded `:4096` instead of parsing `listening on http://…` from the child process; (2) `Execute` sent `model` as a string (`provider/model`) but the API expects `{providerID, modelID}`; (3) `/event` is SSE (`data: …`) with `message.part.updated` / `session.idle`, not raw JSON lines — decode loop never saw text or completion.

**Decision / Outcome**: Fixed adapter: stdout URL scan on `ExecRunner`, SSE reader, incremental part text deltas, HTTP error surfacing on failed `prompt_async`. Follow-up: ignore user `message.part.updated` events until assistant message id is known (OpenCode echoes user text in SSE before assistant reply). Boot: skip false `not in harness catalog` for OpenCode defaults — aggregate `ListModels` intentionally omits OpenCode at boot to avoid starting `opencode serve`. TUI palette/slash overlay order: Hero slashes first (user-specified), then Go to screens, then Refresh/Quit/imported commands.

---

**Problem**: TUI auto-selected `composer-2.5`; `/hero-model` listed only Cursor models; harness screen was unclear.

**Decision / Outcome**: Do not invent `freechat_default.model`. `/hero-model` lists only **enabled** harnesses (no availability checks on open; skip submenu when only one). Model list fetched on demand per harness. OpenCode `/config/providers` models parsed as object map. `/hero-harness` remains checkboxes with `(available)`/`(unavailable)`.

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

---

## 2026-08-17 — C5 started (model properties selection in TUI)

**Problem**: `/hero-start` for C5 ("seleção das propriedades dos modelos na TUI") in the Hero TUI.

**Decision / Outcome**: Configuration validated: scope `native` only (→ generic_agent on Implementation); Browser UI Validation and QA End-to-End disabled (gates N/A); research auto-advances (no human approval), planning requires approval. Chat language PT-BR.

**Exceptions**:
- `hero stage start --name configuration` → "not found": the SQLite store tracks only Research→E2E, no Configuration row. Configuration metrics will be folded into the Research close as a multi-entry metrics-json array (entry 1: configuration/orchestration_agent, entry 2: research/discover_agent).
- No pricing entry for `opencode-go/deepseek-v4-pro` (or any opencode-go id) in `.workflow-hero/models/`; `hero update-models` upstream ships no `opencode.yml`. Cost for orchestration-stage models is recorded as 0.00 pending catalog availability (tokens/duration still recorded).

---

## 2026-08-17 — C5 Research completed (dynamic model properties)

**Problem**: The TUI `/hero-model` flow selected only a harness/model pair, leaving model-specific fast, thinking, and reasoning-effort properties implicit and invisible in Chat.

**Decision / Outcome**: Requirements confirmed and documented in PRD-C05-001, ADR-C05-001 (ADR-038–042), and UI-C05-001. C5 will support `fs`, `th`, and `ef` with dynamic values obtained from an optional harness capability API, a project-scoped `hero.db` cache, or `assets/models/*.yml`. `/hero-model` refreshes enabled harnesses in background without OpenCode boot preloading; choices persist by harness/model in `hero.json`; adapters own API mapping and rejected properties fail explicitly; workflow agent YAML remains authoritative during stages. Planning is pending.

---

## 2026-08-17 — C5 Planning completed (model properties TUI)

**Decision / Outcome**: Created OpenSpec change `model-properties-tui` under `openspec/changes/model-properties-tui/`. The SDD defines 19 normalized/cache/persistence/TUI/adapter/workflow requirements and 22 independently testable native tasks. Catalog, SQLite, hero.json, adapter, and workflow-contract tracks can run in parallel after the shared contract; TUI picker/status tracks can run in parallel after state integration. The active cycle stores the OpenSpec slug for archive coupling.

---

## 2026-08-17 — C5 generic_agent model probe (pre-Implementation diagnostic)

**Problem**: User asked to probe the generic_agent identity before `/hero-start`. The TUI appeared frozen ("solicitou alguma permissão que não chegou").

**Decision / Outcome**: Task dispatch to generic_agent returned `completed` with answer `deepseek-v4-pro` (matches `agents.generic_agent.model`). On resume-interrogation the subagent confirmed it never executed any tool and had no pending permission request — the freeze was a TUI/harness rendering artifact, not a subagent permission stall. Probe passed: model routing resolves the configured `opencode-go/deepseek-v4-pro`.

---

## 2026-08-18 — C5 resume: sync .opencode/agents models with workflow-config.yml

**Problem**: Before restarting Implementation after `/hero-resume`, the `.opencode/agents/*.md` frontmatter models drifted from `workflow-config.yml` `agents.*` blocks (context_agent/qa_agent/end2end_qa_agent still pointed at `kimi-k2.7-code`; orchestration/generic/judge/browser at `deepseek-v4-pro`).

**Decision / Outcome**: Synced every agent frontmatter to the opencode harness IDs + reasoningEffort/thinking in the C5 workflow-config (orchestration/context/qa/judge/browser/end2end → `opencode/deepseek-v4-flash-free`; generic → `opencode-go/gpt-5.6-luna`; discover/planning/backend/frontend already matched). `reasoning_effort: na` → omit `reasoningEffort`; `thinking: na` → omit `thinking`. opencode.json untouched. Ready for `/hero-start`.

---

## 2026-08-18 — C5 Implementation completed (model properties TUI)

**Decision / Outcome**: Completed all 22 native `generic_agent` tasks in `openspec/changes/model-properties-tui/`. The implementation adds normalized `fs`/`th`/`ef` contracts, optional OpenCode discovery and Cursor-safe composition, schema-v5 project cache, embedded/installed catalog fallback, atomic per-pair `hero.json` persistence, background refresh at `/hero-model` open, the Bubble Tea property picker, responsive status labels/warnings, workflow-YAML projection, explicit rejection errors, and Runtime help/inventory assertions. Existing C4 Cursor/OpenCode/session/lazy-serve behavior remains green. `go test ./...`, `go vet ./...`, and targeted race checks passed; no browser or web work was introduced. The active cycle remains native-only with OpenSpec slug `model-properties-tui`.
