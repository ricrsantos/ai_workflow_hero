## Context

Hero C4–C5 delivered Cursor + OpenCode multi-harness TUI orchestration, C5 model properties, and OpenCode Prepare-on-`/hero-start`. C6 adds **Codex** as a third TUI harness with OpenCodeAdapter as the behavioral template (PRD-C06-001; ADR-C06-001; UI-C06-001).

Scope: **native only** → `generic_agent`. Terminology: **etapa** = workflow stage; **interação** = one conversational round within an etapa; **enabled** = user selected harness in `hero.json`; **available** = adapter `IsAvailable` true.

Reference implementation: `internal/adapters/opencode/` (`adapter.go`, `server.go`, `events.go`, `prepare.go`, `health.go`).

## Goals / Non-Goals

**Goals:**
- `HarnessAdapter` registry with Cursor + OpenCode (existing) + Codex (new).
- Hero-managed `codex app-server` over stdio/JSON-RPC; project SQLite app-server registry.
- OpenCode-equivalent lifecycle, streaming, permissions, watchdog, reset, Prepare-on-start.
- `.codex/` projection from `assets/codex/`; install/`/hero-harness` third checkbox.
- `/hero-model` Codex harness step; Chat `· codex` labels; C5 properties via adapter mapping.
- ChatGPT auth failure → `codex login` instruction (no API key, no in-TUI login).
- Dedicated `assets/models/*.yml` entries for Codex-native ids.
- 2.4.x upgrade: Codex disabled; Cursor/OpenCode unchanged.

**Non-Goals:**
- MCP, images, web search, dump-all-events, skill pipelines, `--yolo`, Hero sandbox YAML, plugin marketplace, Windows, `hero serve`, foreign app-server attach, concurrent TUIs, CLI version pin.

## Decisions

### D1 — TUI-only Codex (ADR-043)

Cursor IDE ignores `agents.*.harness` and never starts `codex app-server`. Only Hero TUI reads `harness: codex` and dispatches to `CodexAdapter`.

### D2 — OpenCodeAdapter is the behavioral spec (ADR-045)

CodexAdapter mirrors OpenCodeAdapter surface area: managed child process, lazy start, registry, stream mapping, permissions, `/harness-reset`, Prepare-on-`/hero-start`, doctor warn, catalog task. Idea file divergences are rejected.

### D3 — App-server transport (ADR-044)

```
TUI Execute(harness=codex)
  → CodexAdapter ensures codex app-server child (stdio JSON-RPC)
  → thread/turn JSON-RPC for prompt/stream/cancel
  → StreamDelta mapping (text/thinking/tool/warning/permission/activity/session)
  → unknown Codex events → StreamKindWarning
```

- Lazy start on first Codex Execute; stop on TUI quit and `/hero-harness` disable.
- Attach **only** to Hero-started child; never user socket/foreign process.
- Registry in `hero.db`: pid, harness=`codex`, project_path, identity fields (OpenCode serve-registry analog without HTTP URL).
- Injectable `ProcessRunner` + stdio reader/writer for tests.

### D4 — hero.json harness state (ADR-048)

```json
{
  "harnesses": {
    "cursor": { "enabled": true, "model": "composer-2.5" },
    "opencode": { "enabled": false, "model": "" },
    "codex": { "enabled": false, "model": "" }
  },
  "freechat_default": { "harness": "cursor", "model": "composer-2.5" },
  "model_properties": { "cursor": { "composer-2.5": { "fs": "false", "th": "na", "ef": "na" } } }
}
```

- Upgrade 2.4.x: add `harnesses.codex.enabled=false`; do not provision `.codex/`.
- `/hero-new` injection: one enabled → that id; multiple including Cursor → `cursor`; never inject disabled harness.

### D5 — Install harness picker (UI-C06-001 §2)

Supported IDs: `cursor`, `opencode`, `codex`. Multi-select not PATH-filtered. Enable Codex → provision `.codex/` immediately.

### D6 — Codex projection (ADR-046)

- Source: `assets/codex/agents|commands|skills` (+ minimal config only if adapter requires).
- Enable → write `.codex/` + checksums; disable → `enabled=false` only.
- Root `AGENTS.md` not copied. Conflict backup `{filename}_{timestamp}.conflict` for customized Hero-managed files.
- Do not hijack global `~/.codex` auth via `CODEX_HOME` unless proven required.

### D7 — Auth and availability (ADR-047)

- `IsAvailable` / Execute detect unauthenticated Codex → explicit error naming `codex login`.
- Never prompt for API key; never embed interactive login in Bubble Tea.
- Accept PATH `codex`; missing `app-server` or failed handshake → explicit incompatible/not-installed error.

### D8 — Fallback chain (ADR-033; unchanged shape)

```
1. Try agent.harness + agent.model
2. If unavailable → warn → try fallback_model.harness + fallback_model.model
3. If still fails → stop, explain, wait for /hero-continue
```

Codex may appear in YAML pairs. Never invent a third harness. Session/thread ids are harness-scoped.

### D9 — Prepare-on-`/hero-start` (OpenCode analog)

When any `agents.*` uses `harness: codex` and Codex is enabled:
1. Sync `.codex/agents` frontmatter from workflow-config.
2. Reset managed app-server (stop → wait → restart).
3. Probe first configured Codex agent with minimal prompt.
4. Probe failure stops `/hero-start` with exit-TUI-retry copy (UI-C06-001 §6).

### D10 — C5 properties on Codex (ADR-038–042 via adapter)

- CodexAdapter maps normalized `fs`/`th`/`ef` to native app-server fields where supported.
- Unsupported → `na`; rejection explicit (no strip/retry).
- `/hero-model` property picker unchanged; refresh on open includes Codex when enabled.

### D11 — Chat chrome (UI-C06-001 §5)

Speaker: `[ORCH - gpt-5.4 · codex]`. Input status: `Build · {model} · codex`. Unknown app-server events → yellow warning, not raw JSON-RPC dump.

### D12 — Model catalog (PRD-C06-001 §4.8)

Add Codex-native keys to `assets/models/*.yml` with `context_window` and C5 `properties` when known. Do not invent ChatGPT-subsidized USD rates; unknown cost unset/zero with warning.

### D13 — SQLite schema bump

Extend `harness_serve_registry` (or parallel table) for Codex app-server rows. Ensure `stages.harness_id` / session binding prevents resuming Codex thread as Cursor/OpenCode session.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| Codex CLI protocol drift | No version pin; explicit handshake failure; unknown events → warning |
| User `.codex/` collision | Checksum + conflict backup; disable never deletes user files |
| Auth UX in TUI | Fail fast with `codex login` copy; no embedded OAuth |
| OpenCode regression | Minimal diffs; dedicated regression task group; existing test suite green |

## Migration Plan

1. Schema + hero.json: Codex disabled by default on upgrade.
2. Register adapter; extend pickers and labels.
3. Ship `assets/codex/` + catalog entries.
4. SemVer 2.5.0 via existing release scripts.

## Open Questions

None — Research and ADR-C06-001 are approved. Idea file is non-normative.
