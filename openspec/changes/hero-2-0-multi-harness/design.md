## Context

Hero C1–C3 delivered a full Cursor `HarnessAdapter`, TUI conversation orchestration, and harness boot via `cli.tools`. C4 extends to **two harnesses** (Cursor + OpenCode) orchestrated exclusively from the Hero TUI, while Cursor IDE remains a Cursor-only compatibility path (PRD-C04-001; ADR-C04-001).

Scope: **native only** → `generic_agent`. Terminology: **etapa** = workflow stage; **interação** = one conversational round within an etapa; **enabled** = user selected harness in `hero.json`; **available** = adapter `IsAvailable` true.

## Goals / Non-Goals

**Goals:**
- `HarnessAdapter` registry with Cursor (existing) + OpenCode (new).
- YAML `harness` + native `model` per agent; no Hero canonical translation.
- Hero-managed `opencode serve` + HTTP API; project SQLite serve registry.
- `/hero-harness`, `/hero-model` pair, Chat harness labels.
- Interactive install (≥1 harness); `--tools` removed.
- Dedicated `assets/models/*.yml` update for OpenCode-native ids.
- 1.x upgrade: Cursor stays enabled; no Cursor adapter regression.

**Non-Goals:**
- Third+ harnesses; `hero serve`; foreign serve attach; concurrent TUIs; Windows.

## Decisions

### D1 — TUI-only multi-harness (ADR-031)

Cursor IDE ignores `agents.*.harness` and passes `model` through as today. Only Hero TUI reads `harness` and dispatches to the matching adapter. Do not change Cursor Runtime orchestration to route OpenCode.

### D2 — Native model ids (ADR-032)

```yaml
agents:
  planning_agent:
    harness: cursor
    model: composer-2.5
  qa_agent:
    harness: opencode
    model: anthropic/claude-sonnet-4
fallback_model:
  harness: cursor
  model: composer-2.5
```

Missing `harness` at Execute → error. `/hero-new` injects from **enabled** harnesses: one → that id; both → `cursor`. Never inject disabled harness.

### D3 — hero.json harness state (ADR-034)

```json
{
  "cli": { "version": "2.0.0", "tools": ["cursor"] },
  "harnesses": {
    "cursor": { "enabled": true, "model": "composer-2.5", "enable_fast_model": false },
    "opencode": { "enabled": false, "model": "", "enable_fast_model": false }
  },
  "freechat_default": { "harness": "cursor", "model": "composer-2.5" }
}
```

- `cli.tools` retained read-only for upgrade migration; new installs write `harnesses.*.enabled`.
- Upgrade 1.x: if `harnesses` absent, set `cursor.enabled=true` from `cli.tools`; OpenCode `enabled=false`.

### D4 — Install harness picker (ADR-034; UI-C04-001 §2)

Replace `--tools` with huh multi-select after git/name prompts. List **supported** harnesses (`cursor`, `opencode`), not PATH-filtered. Zero selected → validation error. `--tools` on install/upgrade → exit 1 with suggestion text from UI spec.

### D5 — HarnessAdapter registry

```go
type Registry interface {
    Adapter(id string) (HarnessAdapter, error)
    SupportedIDs() []string
    EnabledIDs(heroJSON HeroJSON) []string
}
```

TUI resolves adapter by agent YAML `harness`. Freechat uses `freechat_default` pair (or `/hero-model` selection). Orchestrator Execute uses `agents.orchestration_agent.harness` + `.model` (then fallback pair, then freechat default).

### D6 — OpenCode serve lifecycle (ADR-035)

1. First OpenCode Execute in TUI session → `opencode serve` child on localhost ephemeral port.
2. Persist row in `hero.db` (`harness_serve_registry`: pid, port, url, harness, created_at).
3. Execute/stream/cancel/session via HTTP API to **that** child only.
4. TUI boot → reap orphans in DB whose PID is still `opencode serve` (crash recovery).
5. TUI quit or `/hero-harness` disable OpenCode → stop child; recreate on next Execute.
6. Injectable `ProcessRunner` + `HTTPClient` for tests; no live LLM in unit tests.

### D7 — OpenCode projection (ADR-036)

- Source: `assets/opencode/agents|commands|skills` (generated/mirrored from same Hero agent content as Cursor assets where applicable).
- Enable → write `.opencode/` immediately; track in `checksums.json`.
- Disable → `enabled=false` only; files remain.
- Root `AGENTS.md` not copied. Minimal `opencode.json` only if adapter requires it.

### D8 — Fallback chain (ADR-033)

```
1. Try agent.harness + agent.model
2. If unavailable → warn → try fallback_model.harness + fallback_model.model
3. If still fails → stop, explain, wait for /hero-continue
```

Never pick a third harness. Session IDs are harness-scoped; never resume Cursor session as OpenCode.

### D9 — `/hero-harness` (ADR-037; UI-C04-001 §3)

Palette + Chat `/` item. Show Enabled + Available per harness. Enable → provision + `enabled=true`. Disable → `enabled=false` (files kept). Cannot disable last enabled harness.

### D10 — `/hero-model` pair (ADR-037; UI-C04-001 §4)

Picker columns: Model, Harness. Aggregate models from **enabled** harness adapters' `ListModels`. On select: write `freechat_default` + `harnesses.<harness>.model`. Do not edit cycle agent YAML.

### D11 — Chat chrome (UI-C04-001 §5)

Speaker: `[ORCH - cursor-grok-4.6 · cursor]`, `[QA - anthropic/claude-sonnet-4 · opencode]`, `[HARN - composer-2.5 · cursor]`. Input status: `Build · {model} · {harness}`.

### D12 — Model catalog (PRD-C04-001 §4.10)

Add OpenCode-native keys to existing provider YAML files (e.g. `anthropic/claude-sonnet-4` in `anthropic.yml`) using **known provider rates** from sibling entries. Keep all `cursor.yml` slugs. Unknown id at metrics time: warn, cost zero/unset, no panic.

### D13 — SQLite schema v4

```sql
CREATE TABLE harness_serve_registry (
  id INTEGER PRIMARY KEY,
  harness TEXT NOT NULL,
  pid INTEGER NOT NULL,
  port INTEGER NOT NULL,
  url TEXT NOT NULL,
  created_at TEXT NOT NULL
);
-- stages.harness_id TEXT NOT NULL DEFAULT ''  (or equivalent session binding)
```

### D14 — Cursor adapter boundary

Changes to `internal/adapters/cursor` limited to: registry integration, optional `ListModels` for pair picker, harness id constant. **No** rewrite through OpenCode. Existing cursor adapter tests must pass unchanged.

## Risks / Trade-offs

- **OpenCode API drift:** injectable HTTP client; record fixtures during implementation.
- **Concurrent TUI:** out of scope; second TUI may reap first's serve (document in workflow-help).
- **Breaking `--tools`:** accepted SemVer 2.0.0; clear error message mitigates.
- **Model catalog maintenance:** OpenCode ids added incrementally; unknown ids degrade gracefully.

## Migration Plan

1. hero.json schema + upgrade migration (Cursor enabled).
2. Workflow config `harness` field + template.
3. Install picker + `--tools` removal.
4. OpenCode adapter + serve registry.
5. TUI routing, slashes, chat chrome.
6. Model catalog update (parallel track).
7. Integration tests + docs.

## Open Questions

- Exact OpenCode serve HTTP API paths — validate against installed `opencode` CLI during task 4.2.
- Minimum `opencode.json` schema — confirm with adapter spike in task 5.3.
