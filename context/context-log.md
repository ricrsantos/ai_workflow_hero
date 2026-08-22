# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

---

## 2026-08-21 — TUI footer anchoring on short terminals

**Problem**: The navigation footer text was fixed, but `renderFrame` enforced a minimum three-row content area. On short terminals that made the frame taller than the viewport, hiding the final footer hint group (`ctrl+q quit`).

**Fix**: Content height now may reach zero when fixed chrome consumes the available rows; the footer remains anchored and the rendered frame stays at the terminal height. Added a regression test covering an 80×10 Chat frame and the complete fixed footer.

---

## 2026-08-21 — TUI fixed navigation footer

**Problem**: The footer hints changed by screen/streaming state and the long line could wrap without reserving its rows, causing incomplete or overwritten navigation instructions.

**Outcome**: `internal/tui/screens.go` now renders one fixed footer string for every TUI state, wraps only between hint groups, and includes the wrapped footer in frame-height calculations. Chat response sizing and the idle agents box were tightened so the header/composer remain visible. Added coverage for fixed content, narrow wrapping, and footer anchoring; UI-C03 and context state now document the fixed footer.

---

## 2026-08-21 — Codex stdio deadlock + stall 3m

**Problem**: Live Hero↔Codex hang — Codex blocked on `anon_pipe_write` (~75KB JSONL), Hero stdout pipe full (~64KB), no TCP; sync `onNotify` + TUI `ch <-` backpressure stopped `readLoop` draining.

**Fix**: Codex `rpcConn` serial `notifyQ` (never block stdout reader); TUI stream chan 512 + timed backpressure drop; OpenCode+Codex `StallTimeout` **3m** (was 6m).

---

## 2026-08-21 — GitHub release v2.5.0

**Outcome**: `go test ./...` green → annotated tag `v2.5.0` on `main` (`6c898e7`) → pushed → `scripts/release.sh` → GitHub Release with 4 binaries + `checksums.txt`. URL: https://github.com/ricrsantos/ai_workflow_hero/releases/tag/v2.5.0

---

## 2026-08-21 — Codex stream UX: debug noise + agentMessage dedupe

**Problem**: Chat showed garbled then clean duplicate of the same agent answer (`stringField` TrimSpace on deltas + full `item/completed` re-emit). Noisy activities (`userMessage`, `agent message`, tokens, rate limits) and unrecognized-event warnings always appeared.

**Decision / Outcome**: Mirror OpenCode: `stringFieldRaw` for text/tool deltas; authoritative `item/completed` for agentMessage (suffix-only / skip); reasoning live deltas suppressed, thinking from completed snapshot only; activities + unrecognized warnings gated on `ExecuteRequest.Debug` (`hero --debug`). UI-C06 §5 updated. Tests in `events_test.go` + adapter/integration mocks.

---

## 2026-08-21 — OpenAI + Codex model catalog refresh

**Change**: Added `gpt-5.6-terra`, `gpt-5.6-luna`, `gpt-5.5`, `gpt-5.4-mini` (plus `openai/` OpenCode ids) to `assets/models/openai.yml` and `.workflow-hero/models/openai.yml`. Same native ids in `assets/models/codex.yml` / `.workflow-hero/models/codex.yml` with ChatGPT rates at 0.00, `th` (summary) available, `fs` na. OpenAI pricing from API docs; GPT-5.6 `cache_write` at 1.25× input on openai.yml only.

---

## 2026-08-21 — C6 §9: Integration + close (native complete)

**Problem**: Close Implementation: integration lock for upgrade→enable→Execute→orphan reap, SemVer 2.5.0, context landing, openspec-change persistence.

**Decision / Outcome**: Added `internal/integration/codex_c6_test.go` (§9.1: 2.4→2.5 upgrade leaves Codex off; `EnableHarnessWithProjection` projects `.codex/`; mock stdio Execute streams text+thinking; Codex orphan reap clears dead registry PID). Default `cmd/hero` version + `scripts/release.sh` tag example → **2.5.0** (ldflags injection unchanged). `hero status` shows OpenSpec change `codex-adapter` on C6. Tasks 9.1–9.4 checked; `go test ./...` green. Phase: native C6 complete → QA/Judge next.

---

## 2026-08-21 — C6 §8 parallel tracks (summary)

- **8A/8B**: `/hero-harness` Codex enable/disable + projection; `/hero-model` Codex harness step + `ListModels` + C5 properties; stage YAML untouched.
- **8C**: Chat `[LABEL - model · harness]` / `Build · model · harness`; Codex auth/CLI/app-server goldens; unknown events → yellow warning.
- **8D**: Codex-native ids in `assets/models/*.yml` (no invented ChatGPT USD rates); metrics warn on unknown id.
- **8E**: `/harness-reset` includes Codex; `PrepareHeroStart` sync/reset/probe tests.
- **8F**: Doctor warn-only `codex-cli` when enabled and missing from PATH.
- **8G**: workflow-help + README EN/PT-BR + DEPLOY 2.5.0; Cursor Runtime assets unchanged.
- **8H**: Cursor/OpenCode Execute untouched; mixed three-harness integration + OpenCode Prepare when Codex unused.

---

## 2026-08-21 — C6 §1–§7 (summary)

**Outcome**: hero.json + SQLite Codex registry; `/hero-new` three-harness injection; install/upgrade Codex opt-in (2.4→2.5 never auto-enables); `internal/adapters/codex` stdio JSON-RPC + stream/permission/auth/C5/usage; `assets/codex/` projection; harnessmgr + TUI boot/orphan/health; fallback + Execute routing + Prepare-on-`/hero-start`.

---

## 2026-08-20 — Releases / pre-C6

- **v2.4.1**: TUI UX patch (bordered prompt, clipboard, context bar).
- **v2.4.0**: OpenCode serve lifecycle + orphan reap.
- **C5 archived** (`model-properties-tui`); Judge approved without formal SDD verify (judge_agent empty in opencode harness).

---
