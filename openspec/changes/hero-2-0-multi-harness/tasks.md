# Tasks — hero-2-0-multi-harness

Scope: **native** → all implementation via `generic_agent`.  
Nested generic Task fan-out: `composer-2.5` (`agents.generic_agent.subagent`).  
Terminology: **etapa** = workflow stage; **interação** = conversational round within an etapa.  
**Compatibility:** do not regress Cursor adapter Execute, Cursor IDE Runtime, checksum upgrade, dual-entry, or deterministic engine. Minimize `internal/adapters/cursor` diffs.

**Parallelism legend:** **PARALLEL** = concurrent Task subagents after deps met; **SERIES** = ordered.

PRD traceability: [PRD-C04-001](../../docs/product/PRD-C04-001-multi-harness.md); ADR: [ADR-C04-001](../../docs/architecture/ADR-C04-001-multi-harness.md); UI: [UI-C04-001](../../docs/product/UI-C04-001-tui-multi-harness.md).

---

## 1. hero.json harness state + SQLite serve registry — SERIES

- [x] 1.1 Extend `HeroJSON`: `harnesses.<id>.enabled`, `freechat_default {harness, model}`; supported IDs `cursor`, `opencode`; colocated tests (spec `hero-json-harness-state`; design D3)
- [x] 1.2 Upgrade migration: read legacy `cli.tools` → set `cursor.enabled=true`; OpenCode `enabled=false`; do not auto-provision `.opencode/` (ADR-034)
- [x] 1.3 SQLite schema v4: `harness_serve_registry` table + `stages.harness_id` (or session harness binding); migration + store helpers; tests (spec `opencode-serve-registry`; design D13)
- [x] 1.4 Harness helpers: `ListEnabledHarnesses`, `IsHarnessEnabled`, `GetFreechatDefault`, `SetHarnessEnabled`, `SetFreechatDefault`; tests

## 2. Workflow config `harness` field — SERIES (after 1.1)

- [x] 2.1 Extend `internal/workflowconfig`: parse `harness` on each agent + `fallback_model`; missing `harness` → validation error (spec `workflow-config-harness`; ADR-032)
- [x] 2.2 Update `assets/templates/workflow-config.yml`: `harness` on all agents + `fallback_model` (default not hardcoded `cursor` in template comments only)
- [x] 2.3 `/hero-new` harness injection: single enabled → that id; both enabled → `cursor`; never disabled; preserve explicit imports (ADR-032)
- [x] 2.4 Colocated workflowconfig tests: valid/invalid YAML, injection rules

## 3. Install + upgrade breaking changes — SERIES (after 1.1)

- [x] 3.1 Replace `--tools` with interactive multi-select harness picker (huh); ≥1 required; success line lists enabled harnesses (spec `interactive-harness-install`; UI-C04-001 §2; design D4)
- [x] 3.2 `--tools` on `hero install` or `hero upgrade` → explicit error + suggestion; exit 1; golden test for message text
- [x] 3.3 Install/upgrade: project core once; project Cursor assets when cursor enabled; OpenCode projection only when opencode enabled; integration tests
- [x] 3.4 Update `cmd/hero/main_test.go` and install tests that assumed `--tools` required

## 4. OpenCode adapter — SERIES (after 1.3)

- [x] 4.1 Create `internal/adapters/opencode`: `HarnessAdapter` skeleton, `Name()=="opencode"`, `IsAvailable` (opencode CLI on PATH); injectable deps (spec `opencode-adapter`)
- [x] 4.2 Serve lifecycle: lazy `opencode serve`, localhost ephemeral port, registry write; injectable `ProcessRunner` (design D6)
- [x] 4.3 HTTP API: Execute with stream, Cancel, session create/resume; **never** attach to foreign serve; mock HTTP tests
- [x] 4.4 `ListModels` via serve API; adapter contract test `var _ harness.HarnessAdapter`

## 5. OpenCode projection assets — PARALLEL (after 1.1; can run with §4)

- [x] 5.1 Add `assets/opencode/` layout (agents, commands, skills) sourced from Hero `assets/cursor/` equivalents; projection writer (spec `opencode-projection`; design D7)
- [x] 5.2 Enable path: write `.opencode/` + checksums; disable: `enabled=false` only; uninstall owns opencode paths; tests
- [x] 5.3 Minimal `opencode.json` template if adapter requires it; golden test

## 6. Harness registry + TUI boot — SERIES (after 1.4, 4.1)

- [x] 6.1 `internal/harness` registry: resolve adapter by id; `SupportedIDs()` returns cursor + opencode (spec `harness-adapter` MODIFIED; design D5)
- [x] 6.2 TUI boot: replace `cli.tools`-only boot with enabled harness list; enabled ≠ available; warn when none available; corrupt zero-enabled → install-like prompt (UI-C04-001 §7)
- [x] 6.3 Orphan reap on TUI start from `harness_serve_registry`; stop serve on TUI quit and OpenCode disable (design D6)
- [x] 6.4 Update `harness.SupportedToolIDs` / doctor messages for two supported harnesses

## 7. Fallback chain + Execute routing — SERIES (after 2.1, 6.1)

- [x] 7.1 Model resolution: agent `(harness, model)` → fallback `(harness, model)` → hard stop + `/hero-continue`; warn on every fallback (spec `harness-fallback-chain`; ADR-033; UI-C04-001 §6)
- [x] 7.2 TUI Execute/Dispatch: route to registry adapter by harness; bind `stages.harness_id` + session per harness; never cross-resume (spec `tui-multi-harness-execution`)
- [x] 7.3 Orchestrator Execute uses YAML `agents.orchestration_agent.harness` + model (then fallback, then freechat default)
- [x] 7.4 Colocated tests: unavailable pair → fallback message; double failure → stop text

## 8. PARALLEL tracks after §7 — TUI commands, chat chrome, model catalog, docs, Cursor guard

### 8A. `/hero-harness` — PARALLEL

- [x] 8A.1 Palette + Chat `/` overlay: list harnesses with Enabled/Available; enable provisions projection; disable keeps files; last-harness guard (spec `hero-harness-command`; UI-C04-001 §3)
- [x] 8A.2 Colocated TUI tests: enable success line, disable success line, last-harness error text

### 8B. `/hero-model` pair picker — PARALLEL

- [x] 8B.1 Two-column picker (Model, Harness) aggregating `ListModels` from enabled adapters (spec `hero-model-pair`; UI-C04-001 §4; design D10)
- [x] 8B.2 Persist `freechat_default` + `harnesses.<harness>.model`; freechat + `/hero-new` use pair; stage YAML untouched
- [x] 8B.3 Input status `Build · {model} · {harness}`; colocated tests

### 8C. Chat speaker labels — PARALLEL

- [x] 8C.1 Green pane / speaker: `[LABEL - model · harness]` for all agent codes + HARN (spec `tui-multi-harness-execution`; UI-C04-001 §5; design D11)
- [x] 8C.2 Golden tests for speaker format; context bar still uses native model id for `models/*.yml` lookup

### 8D. OpenCode model pricing catalog — PARALLEL (mandatory standalone task)

- [x] 8D.1 Add OpenCode-native model ids (`provider/model`) to `assets/models/*.yml` with `input`/`output`/cache/`context_window` from **known provider rates** — do not invent prices (spec `opencode-model-catalog`; PRD-C04-001 §4.10; design D12)
- [x] 8D.2 Verify all existing `cursor.yml` slugs still resolve; add lookup tests for representative Cursor slug + OpenCode id
- [x] 8D.3 Metrics/context bar: unknown id warns, cost unset/zero, no panic; colocated tests in models package

### 8E. Runtime assets + user docs — PARALLEL

- [x] 8E.1 Update `assets/docs/workflow-help.md`, README EN/PT-BR, DEPLOY.md: no `--tools`; `/hero-harness`, `/hero-model` pair, OpenCode serve notes (PRD-C04-001 §4.12)
- [x] 8E.2 Template comments in `workflow-config.yml` documenting harness + native model ids side by side
- [x] 8E.3 `runtime_assets_test.go`: require `harness` in template agents; no Cursor IDE asset changes for OpenCode routing

### 8F. Cursor adapter regression guard — PARALLEL

- [x] 8F.1 Audit `internal/adapters/cursor` diffs — limit to registry id + any `ListModels` wiring; **no** behavioral change to Execute/stream/cancel
- [x] 8F.2 Run existing cursor adapter test package unchanged; fix only if registry wiring breaks compile
- [x] 8F.3 Doctor: optional OpenCode CLI check when opencode enabled; warn-only (complements TUI)

## 9. Integration + close — SERIES

- [x] 9.1 Integration: upgrade 1.x fixture → Cursor-only enabled; mixed harness workflow-config Execute with mock adapters; orphan reap fixture
- [x] 9.2 `go test ./...` green; SemVer 2.0.0 version bump in release scripts if applicable
- [x] 9.3 Update `context/current-state.md` and `context/context-log.md` when implementation lands
- [x] 9.4 Verify `hero cycle openspec-change hero-2-0-multi-harness` persisted on active cycle

---

## Parallel groups (orchestrator fan-out)

After §7 complete:

| Group | Tasks | Agent |
|---|---|---|
| A | 8A.1–8A.2 | `generic_agent` |
| B | 8B.1–8B.3 | `generic_agent` |
| C | 8C.1–8C.2 | `generic_agent` |
| D | 8D.1–8D.3 | `generic_agent` |
| E | 8E.1–8E.3 | `generic_agent` |
| F | 8F.1–8F.3 | `generic_agent` |

§5 can start after §1.1 in parallel with §4.

Hard series spine: **1 → 2 ‖ 3 → 4 + 5 → 6 → 7 → (8A ‖ 8B ‖ 8C ‖ 8D ‖ 8E ‖ 8F) → 9**.

**Task count:** 47 checklist items across §1–§9.
