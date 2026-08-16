# PRD-C04-001 — Multi-Harness (Cursor + OpenCode) via Hero TUI

> Cycle C4 product requirements. Ships **Hero 2.0.0**: TUI-orchestrated multi-harness (Cursor and OpenCode) while preserving Cursor IDE Runtime and existing Cursor TUI behavior. Index: [PRD.md](PRD.md). Idea notes (non-normative): [docs/idea/v2_multi_harness/multi_harness.md](../idea/v2_multi_harness/multi_harness.md).

## 1. Overview

Hero 1.x executes AI only through the **Cursor** adapter. Users cannot assign different harnesses to different agents, and `hero install --tools cursor` is the only install-time harness selector.

This cycle delivers **Hero 2.0.0**:

1. **Multi-harness in Hero TUI only** — each agent is `harness + model`; the TUI executes that pair.
2. **Cursor IDE unchanged as a Cursor-only compatibility mode** — it ignores `harness` and uses the YAML `model` string as today.
3. **OpenCode** as the first additional harness (`OpenCodeAdapter`), using a Hero-managed `opencode serve` and its **HTTP API**.
4. **No new orchestration layer** — reuse `HarnessAdapter`, `engine.Engine`, SQLite, and TUI Execute.
5. **Breaking CLI/config** — remove `--tools`; require `harness` on every agent in `workflow-config.yml`.

**Compatibility mandate:** do not break working 1.x behavior except the explicit 2.0 breaks listed in §8. Cursor Adapter CLI Execute, Cursor IDE slash Runtime, checksum upgrade, dual-entry, and the deterministic engine stay.

## 2. Goals

- One cycle can run different stages/agents on Cursor and/or OpenCode from the TUI.
- User always sees **agent + model + harness**.
- Harness/model switches are never silent.
- Adding a future harness means a new adapter + projection, not engine/stage changes.
- Existing Cursor-only projects keep working after `hero upgrade` (Cursor stays enabled; OpenCode is opt-in).

## 3. Terminology

| Term | Definition |
|---|---|
| **Enabled** | User selected this harness for the project (`hero.json`). Independent of PATH. |
| **Available** | Adapter `IsAvailable` is true (CLI/server can run). |
| **Native model id** | The model string the harness itself uses (Cursor Task slug vs OpenCode `provider/model`). |
| **Projection** | Harness-specific files generated from Hero `assets/` (`.cursor/` vs `.opencode/`). |
| **Freechat default pair** | `(harness, model)` stored for TUI freechat and `/hero-new` (not stage agents). |

## 4. In Scope (C4 / Hero 2.0.0)

### 4.1 Compatibility (do not regress)

- Cursor Adapter `Execute` / stream-json / cancel / sessions keep working.
- Cursor IDE `/hero-*` Runtime keeps working without learning OpenCode.
- With only Cursor enabled, TUI behavior matches 1.x aside from new fields/commands.
- `hero upgrade` does not overwrite customized assets (existing checksum rules).
- Engine remains deterministic; CLI still performs no LLM reasoning (ADR-003).
- **No `hero serve` daemon** (ADR-014). OpenCode’s process is owned by `OpenCodeAdapter`, not a Hero RPC service.

### 4.2 HarnessAdapter + OpenCodeAdapter

- Keep `internal/harness.HarnessAdapter` as the only execution boundary.
- Add `internal/adapters/opencode` implementing the same contract (`IsAvailable`, sessions, Execute, Cancel, Status, model list).
- Cursor stays `internal/adapters/cursor` — **do not rewrite it** to go through OpenCode or a new manager.
- OpenCode-specific protocol (serve lifecycle, HTTP API, native model ids) stays inside the OpenCode adapter.

### 4.3 `workflow-config.yml`

Each named agent and `fallback_model` MUST declare:

```yaml
agents:
  planning_agent:
    harness: cursor          # required
    model: composer-2.5      # native to that harness
    # existing: reasoning_effort, enable_fast_model, thinking, subagent
fallback_model:
  harness: cursor
  model: composer-2.5
```

- **No Hero canonical model id.** No translation table.
- Missing `harness` at `/hero-start` / Execute → **error** (do not infer).
- Invalid harness+model for that adapter → treat as model/harness unavailable → fallback chain (§4.4).
- Template default `harness` is **not** hardcoded `cursor`. `/hero-new` **injects** `harness` from **enabled** project harnesses: one enabled → that id; Cursor and OpenCode enabled → `cursor`. Never inject a disabled harness. Previous-cycle import copies explicit `harness` values; only missing fields are injected.

### 4.4 Fallback (amends ADR-008)

1. Try agent `harness` + `model`.
2. If that pair cannot run (harness disabled/unavailable **or** model missing/invalid), try `fallback_model` (may be another harness if YAML says so). **Warn every time.**
3. If fallback also fails → **stop**, explain the problem, ask the user to fix config/install, wait for `/hero-continue`. **Do not** invent a third harness.

### 4.5 Install, upgrade, `--tools`

- `hero install` (no `--tools`): show **supported** harnesses (Cursor, OpenCode); user selects **at least one**. Selection does **not** depend on PATH.
- `--tools` is **removed**. Passing it on `install` or `upgrade` is an **explicit error** with a pointer to interactive selection / `/hero-harness`.
- Core installs once; projections only for selected harnesses.
- `hero upgrade` from 1.x: **Cursor remains enabled**; OpenCode is **not** auto-enabled; no `.opencode/` until enable.
- Non-TTY install without a replacement list flag is **not** PATH auto-detect (rejected). Follow the idea doc: interactive selection of compatible harnesses.

### 4.6 `/hero-harness` and projections

- New TUI slash **`/hero-harness`**: list supported harnesses; **enable / disable**.
- Cannot disable the last enabled harness.
- **Enable** = mark enabled **and provision projection immediately** from `assets/` (`.opencode/` for OpenCode, existing `.cursor/` for Cursor).
- **Disable** = mark disabled in `hero.json` only. **Do not delete** projection files. Uninstall/checksum rules unchanged.
- Single source of truth: `assets/`. Do not maintain a second handwritten OpenCode copy of agents/skills/commands.
- Root `AGENTS.md` is not duplicated into `.opencode/`.
- Minimal `opencode.json` only if the adapter requires it.

### 4.7 `/hero-model` (amends ADR-030)

- Lists models **with harness** (no ambiguous names).
- Selecting a row stores the **pair** `(harness, model)` as the default for **freechat and `/hero-new`**, and updates `harnesses.<harness>.model`.
- Stage agents **always** use `workflow-config.yml`. `/hero-model` must not rewrite agent blocks.

### 4.8 OpenCode serve lifecycle

- Hero **manages** `opencode serve` (not `opencode run` per prompt).
- Execute / stream / cancel / session via the serve **HTTP API**.
- Lazy start on first OpenCode Execute in the TUI session; **localhost**; **ephemeral port**.
- Attach **only** to the serve **this Hero process created**. Never attach to a foreign localhost server.
- Persist registry in **project `hero.db`** (pid, port, url, harness, created_at).
- **Normal:** TUI quit or `/hero-harness` disable OpenCode **stops** the process Hero started.
- **Crash:** on `hero tui` boot, reap **orphans recorded in this project’s DB** whose PID is still `opencode serve`; then recreate on first OpenCode Execute.
- No global registry. Concurrent second TUI in the same project is **out of scope** for 2.0.

### 4.9 TUI Chat chrome

- Green pane / speaker label: **agent + model + harness** (today’s agent+model plus harness).
- Agents box and status line must not imply a second speaker named only `HARN` when a Hero agent is bound; freechat still uses the harness label **and** shows which harness.

### 4.10 Pricing catalog — **mandatory implementation task**

**Must implement as its own testable activity:** update `assets/models/*.yml` (installed as `.workflow-hero/models/*.yml`) with **OpenCode-native model ids** (typically `provider/model`) and `input` / `output` / cache / `context_window` so metrics and the Chat context bar resolve when `model` is an OpenCode id.

- Scope: Cursor and OpenCode only.
- Do not invent prices; reuse known provider rates when the same model already exists in the catalog.
- Cursor slugs in `cursor.yml` (and current lookup) **must keep working**.
- Metrics for an unknown id: do not crash; cost may be unset/zero with a clear warning (same spirit as missing `context_window`).

### 4.11 Persistence / `hero.json`

- Per harness: `enabled`, `model`, existing flags (`enable_fast_model` as applicable).
- Freechat default pair persisted (harness + model).
- SQLite schema bump as needed for OpenCode serve registry and session↔harness binding (a Cursor session id must never be resumed as OpenCode).

### 4.12 Docs / Runtime assets

- Template `workflow-config.yml` includes `harness` on every agent + `fallback_model`.
- `workflow-help.md`, README (EN + PT-BR), TUI copy: install without `--tools`, `/hero-harness`, `/hero-model` pair, OpenCode serve notes.
- Cursor Runtime assets remain; OpenCode projection generated from the same source.

## 5. Out of Scope

- Claude Code, Codex, VS Code, or other harnesses beyond Cursor + OpenCode.
- Canonical Hero model ids / cross-harness name translation.
- `hero serve` / daemon / RPC (D7).
- Attaching to user-started or foreign `opencode serve`.
- Two concurrent `hero tui` processes in one project.
- Simultaneous multi-harness inside **one** agent Execute.
- Changing Cursor IDE Runtime into a multi-harness orchestrator.
- Replacing `engine.Engine` or adding ExecutionManager/AgentOrchestrator.
- Windows CLI; CI/CD-automated releases; GPG signing.
- Auto-enabling OpenCode on upgrade.
- Deleting projections on disable.

## 6. Non-Functional Requirements

- Feature Based + Vertical Slice: `internal/adapters/opencode`, install/upgrade/TUI/store/harness; **minimize Cursor adapter diffs**.
- Tests: install picker (no `--tools`; `--tools` errors); upgrade 1.x → Cursor-only enabled; projection checksums; OpenCode adapter with injectable HTTP/process (no live LLM); orphan reap; YAML `harness` validation; model catalog lookup for an OpenCode id.
- Chat language: `workflow_config.user_preferred_language`; cycle docs English.

## 7. Success Criteria

1. Install with Cursor and/or OpenCode (OpenCode-only allowed); `--tools` errors clearly.
2. TUI detects enabled vs available; `/hero-harness` enable provisions `.opencode/`.
3. `/hero-model` shows harness + model; freechat uses the stored pair.
4. YAML `harness` + native `model` per agent; mixed Cursor/OpenCode stages in one TUI cycle.
5. Streaming, cancel, session persist **per harness**; Chat shows agent, model, and harness.
6. Metrics work for OpenCode ids after the `models/*.yml` update.
7. Unavailable pair → fallback with warning → stop + user fix + `/hero-continue`.
8. 1.x upgrade: Cursor-only still runs; Cursor IDE slash flow still runs; Cursor TUI Execute still runs.
9. New harness later = new adapter + projection, no engine rewrite.
10. `go test ./...` green; no regression in existing Cursor adapter tests.

## 8. Breaking changes (2.0.0)

| Break | Replacement |
|---|---|
| `hero install --tools …` / `hero upgrade --tools …` | Interactive harness selection; `/hero-harness` later |
| Agent YAML without `harness` | Error until filled; `/hero-new` injects from enabled set |
| `cli.tools` as the only harness list | `harnesses.<id>.enabled` (migrate Cursor enabled on upgrade) |

## 9. References

- [PRD.md](PRD.md) §2.3 (V2 additional harnesses — this cycle implements Cursor + OpenCode only)
- [PRD-C01-001](PRD-C01-001-hero-1-0.md) D1 (deferred multi-harness — now in scope for two adapters)
- [PRD-C03-001](PRD-C03-001-cursor-harness-tui-autonomy.md)
- [ADR-C04-001](../architecture/ADR-C04-001-multi-harness.md)
- [UI-C04-001](UI-C04-001-tui-multi-harness.md)
- [docs/idea/v2_multi_harness/multi_harness.md](../idea/v2_multi_harness/multi_harness.md)
