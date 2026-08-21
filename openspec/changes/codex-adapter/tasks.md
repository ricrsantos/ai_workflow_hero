# Tasks — codex-adapter

Scope: **native** → all implementation via `generic_agent`.  
Nested generic Task fan-out: `composer-2.5` (`agents.generic_agent.subagent`).  
Terminology: **etapa** = workflow stage; **interação** = conversational round within an etapa.  
**Compatibility:** do not regress Cursor adapter Execute, Cursor IDE Runtime, OpenCode serve/HTTP, C5 model properties, checksum upgrade, dual-entry, or deterministic engine. Minimize Cursor/OpenCode adapter diffs.

**Parallelism legend:** **PARALLEL** = concurrent Task subagents after deps met; **SERIES** = ordered.

PRD traceability: [PRD-C06-001](../../docs/product/PRD-C06-001-codex-adapter.md); ADR: [ADR-C06-001](../../docs/architecture/ADR-C06-001-codex-adapter.md); UI: [UI-C06-001](../../docs/product/UI-C06-001-tui-codex-adapter.md).

---

## 1. hero.json + SQLite app-server registry — SERIES

- [ ] 1.1 Extend `HeroJSON` / install helpers: supported ID `codex`; `harnesses.codex.enabled`, `model`, C5 `model_properties`; colocated tests (spec `hero-json-harness-state`; design D4; PRD-C06-001 §4.10)
- [ ] 1.2 Upgrade migration 2.4.x → 2.5.0: add `harnesses.codex.enabled=false`; do not auto-provision `.codex/` (ADR-048; PRD-C06-001 §4.6)
- [ ] 1.3 SQLite schema bump: Codex app-server registry rows in `harness_serve_registry` (pid, harness=`codex`, project_path, identity fields; no HTTP URL); migration + store helpers; tests (spec `codex-app-server-registry`; design D13; ADR-044)
- [ ] 1.4 Session harness binding: Codex thread id never resumes as Cursor/OpenCode; colocated store tests (PRD-C06-001 §4.3; ADR-044)

## 2. Workflow config + `/hero-new` — SERIES (after 1.1)

- [ ] 2.1 Extend `/hero-new` harness injection for three harnesses: one enabled → that id; multiple including Cursor → `cursor`; never disabled (PRD-C06-001 §4.5; design D4)
- [ ] 2.2 Update `assets/templates/workflow-config.yml` comments/examples with `harness: codex` + native model id (PRD-C06-001 §4.11)
- [ ] 2.3 Colocated workflowconfig tests: valid `harness: codex`; injection rules unchanged for Cursor/OpenCode

## 3. Install + upgrade Codex opt-in — SERIES (after 1.1)

- [ ] 3.1 Install harness picker: third checkbox **Codex**; ≥1 harness; PATH-independent; success line lists enabled harnesses (spec `interactive-harness-install`; UI-C06-001 §2; design D5)
- [ ] 3.2 Enable Codex on install → provision `.codex/` immediately from embedded assets (ADR-046)
- [ ] 3.3 Upgrade integration: 2.4.x fixture → Codex remains disabled; no `.codex/` until user enables (ADR-048)
- [ ] 3.4 Colocated install/upgrade tests: Codex-only selection valid; last-harness rules unchanged

## 4. Codex adapter core — SERIES (after 1.3)

- [ ] 4.1 Create `internal/adapters/codex`: `HarnessAdapter` skeleton, `Name()=="codex"`, injectable deps; contract test `var _ harness.HarnessAdapter` (spec `codex-adapter`; ADR-044)
- [ ] 4.2 App-server lifecycle: lazy `codex app-server` stdio child; registry write; graceful stop/force kill; injectable `ProcessRunner` (design D3; ADR-044)
- [ ] 4.3 JSON-RPC: Execute with stream, Cancel, session create/resume via threads/turns; **never** attach to foreign app-server; mock stdio tests (PRD-C06-001 §4.3–4.4)
- [ ] 4.4 `ListModels` via app-server; return native Codex ids only (PRD-C06-001 §4.8)
- [ ] 4.5 Stream mapping: text/thinking/tool/warning/permission/activity/session; unknown events → `StreamKindWarning` (ADR-045; PRD-C06-001 §4.3)
- [ ] 4.6 Permission prompts: `OnPermissionRequest` → TUI Allow? [y/N]; no `--yolo` (PRD-C06-001 §4.3; UI-C06-001)
- [ ] 4.7 Auth detection: unauthenticated → explicit `codex login` error; no API key prompt (spec `codex-adapter`; ADR-047; UI-C06-001 §6)
- [ ] 4.8 C5 property mapping: normalized `fs`/`th`/`ef` → native payload; unsupported `na`; explicit rejection (PRD-C06-001 §4.3; ADR-045)
- [ ] 4.9 Usage mapping when app-server provides quota/usage; missing USD rates → unset/zero + warning (PRD-C06-001 §4.4)

## 5. Codex projection assets — PARALLEL (after 1.1; can run with §4)

- [ ] 5.1 Add `assets/codex/` layout (agents, commands, skills) mirroring OpenCode families; embed in binary (spec `codex-projection`; design D6; ADR-046)
- [ ] 5.2 Projection writer: enable → `.codex/` + checksums; disable → `enabled=false` only; uninstall owns codex paths; tests (PRD-C06-001 §4.7)
- [ ] 5.3 Minimal Codex config template only if adapter requires; golden test; root `AGENTS.md` not copied

## 6. Harness registry + TUI boot — SERIES (after 1.4, 4.1)

- [ ] 6.1 `internal/harnessmgr` registry: `SupportedIDs()` returns cursor + opencode + codex; resolve `CodexAdapter` (spec `harness-adapter` MODIFIED; design D1)
- [ ] 6.2 TUI boot: enabled harness list includes Codex; enabled ≠ available; corrupt zero-enabled unchanged (UI-C06-001 §7)
- [ ] 6.3 Orphan reap on TUI start from registry for Codex app-server; stop on TUI quit and Codex disable (design D3)
- [ ] 6.4 Watchdog/health hooks for Codex during TUI Execute (OpenCode analog); stall prompt uses `/harness-reset` logic

## 7. Fallback chain + Execute routing — SERIES (after 2.1, 6.1)

- [ ] 7.1 Model resolution: Codex `(harness, model)` in existing two-step fallback; warn on fallback; hard stop unchanged (spec `harness-fallback-chain`; ADR-033)
- [ ] 7.2 TUI Execute/Dispatch: route to Codex adapter by YAML `harness`; bind session per harness; never cross-resume (spec `tui-multi-harness-execution`; PRD-C06-001 §4.3)
- [ ] 7.3 Prepare-on-`/hero-start`: when agents use `harness: codex`, sync `.codex/agents`, reset app-server, probe; failure stops start (spec `runtime-workflow-execution`; design D9; UI-C06-001 §6)
- [ ] 7.4 Colocated tests: unavailable Codex pair → fallback message; cross-harness session resume rejected

## 8. PARALLEL tracks after §7 — TUI commands, chat chrome, catalog, docs, regression

### 8A. `/hero-harness` Codex row — PARALLEL

- [ ] 8A.1 Extend harness picker: Codex checkbox with `(available)`/`(unavailable)`; enable provisions `.codex/`; disable keeps files; last-harness guard (spec `hero-harness-command`; UI-C06-001 §3)
- [ ] 8A.2 Colocated TUI tests: enable `✓ Codex enabled (projected .codex/)`; disable `✓ Codex disabled (files kept)`

### 8B. `/hero-model` Codex harness step — PARALLEL

- [ ] 8B.1 Step 1 harness list includes Codex when enabled; step 2 lists native ids from `ListModels` (may start app-server) (spec `hero-model-pair`; UI-C06-001 §4)
- [ ] 8B.2 C5 property submenu unchanged after Codex model select; persist `freechat_default` + `harnesses.codex.model`
- [ ] 8B.3 Colocated tests: Esc returns to harness list; stage YAML untouched

### 8C. Chat speaker + errors — PARALLEL

- [ ] 8C.1 Green pane / speaker: `[LABEL - model · codex]`; input status `Build · {model} · codex` (spec `hero-tui` MODIFIED; UI-C06-001 §5; design D11)
- [ ] 8C.2 Golden tests for auth missing, CLI missing, app-server start failure copy (UI-C06-001 §6)
- [ ] 8C.3 Unknown app-server event → yellow warning in status area, not raw JSON dump

### 8D. Codex model catalog — PARALLEL (mandatory standalone)

- [ ] 8D.1 Add Codex-native model ids to `assets/models/*.yml` with `context_window` and C5 `properties` when known — do not invent ChatGPT USD rates (spec `codex-model-catalog`; PRD-C06-001 §4.8; design D12)
- [ ] 8D.2 Update `hero update-models` / docs to mention Codex ids alongside Cursor and OpenCode
- [ ] 8D.3 Metrics/context bar: unknown Codex id warns, cost unset/zero, no panic; colocated catalog tests

### 8E. `/harness-reset` + Prepare — PARALLEL

- [ ] 8E.1 `/harness-reset` picker includes Codex when enabled; stop Hero-managed app-server; yellow warn if not started (UI-C06-001 §7)
- [ ] 8E.2 `PrepareHeroStart` Codex path colocated tests: sync, reset delay, probe success/failure messages

### 8F. Doctor + CLI suite — PARALLEL

- [ ] 8F.1 Doctor: warn-only `codex-cli` when Codex enabled and CLI not on PATH (spec `cli-deterministic-command-suite`; UI-C06-001 §8)
- [ ] 8F.2 No doctor failure when Codex disabled; OpenCode/Cursor doctor lines unchanged

### 8G. Runtime assets + user docs — PARALLEL

- [ ] 8G.1 Update `assets/docs/workflow-help.md`, README EN/PT-BR: Codex optional harness, `codex login`, `.codex/` projection (PRD-C06-001 §4.11)
- [ ] 8G.2 DEPLOY.md: Hero 2.5.0 minor; no auto-enable Codex on upgrade
- [ ] 8G.3 `runtime_assets_test.go`: Cursor IDE assets unchanged; no Codex routing in Cursor Runtime

### 8H. Cursor + OpenCode regression guard — PARALLEL

- [ ] 8H.1 Audit `internal/adapters/cursor` and `internal/adapters/opencode` diffs — registry wiring only; no Execute/stream/cancel behavior change
- [ ] 8H.2 Run existing cursor/opencode adapter test packages unchanged; fix only compile/registry breaks
- [ ] 8H.3 Integration: mixed Cursor+OpenCode+Codex workflow-config with mock adapters; OpenCode Prepare unchanged when no Codex agents

## 9. Integration + close — SERIES

- [ ] 9.1 Integration: upgrade 2.4 fixture; enable Codex via `/hero-harness`; mock stdio Execute streams deltas; orphan reap fixture
- [ ] 9.2 `go test ./...` green; SemVer 2.5.0 version bump in release scripts (`scripts/release.sh`)
- [ ] 9.3 Update `context/current-state.md` and `context/context-log.md` when implementation lands
- [ ] 9.4 Verify `hero cycle openspec-change codex-adapter` persisted on active cycle

---

## Parallel groups (orchestrator fan-out)

After §7 complete:

| Group | Tasks | Agent |
|---|---|---|
| A | 8A.1–8A.2 | `generic_agent` |
| B | 8B.1–8B.3 | `generic_agent` |
| C | 8C.1–8C.3 | `generic_agent` |
| D | 8D.1–8D.3 | `generic_agent` |
| E | 8E.1–8E.2 | `generic_agent` |
| F | 8F.1–8F.2 | `generic_agent` |
| G | 8G.1–8G.3 | `generic_agent` |
| H | 8H.1–8H.3 | `generic_agent` |

§5 can start after §1.1 in parallel with §4.

Hard series spine: **1 → 2 ‖ 3 → 4 + 5 → 6 → 7 → (8A ‖ 8B ‖ 8C ‖ 8D ‖ 8E ‖ 8F ‖ 8G ‖ 8H) → 9**.

**Task count:** 52 checklist items across §1–§9.
