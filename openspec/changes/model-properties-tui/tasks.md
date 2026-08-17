# Tasks — model-properties-tui

Scope: **native only** → every implementation task is assigned to `generic_agent`. Browser UI Validation and QA End-to-End are disabled for this cycle.

References: [PRD-C05-001](../../docs/product/PRD-C05-001-model-properties-tui.md), [UI-C05-001](../../docs/product/UI-C05-001-tui-model-properties.md), [ADR-C05-001](../../docs/architecture/ADR-C05-001-model-properties-tui.md). Existing C4 Cursor/OpenCode behavior, session binding, lazy OpenCode boot, deterministic engine, and Cursor IDE Runtime must not regress.

**Parallelism legend:** **PARALLEL** tasks may be dispatched concurrently after their listed dependencies are complete. **SERIES** tasks must wait for their listed dependencies. Every code task adds colocated tests and runs the relevant package tests; the repository must remain compatible with `go test ./...`.

## 1. Shared contract — SERIES

- [ ] **task-1-normalized-property-contract** — **SERIES** — `generic_agent`: Extend `internal/harness` with normalized C5 property keys/types, optional capability discovery, and `ExecuteRequest.Properties` while keeping `ModelLister` and existing request callers source-compatible. Define safe map-copy/normalization and a property-aware rejection error contract. Add colocated contract tests for optional discovery, dynamic string values, unknown keys, and request immutability (PRD §4.2–4.3; ADR-038/041).

## 2. Independent native tracks after the contract — PARALLEL

- [ ] **task-2-model-catalog-parser** — **PARALLEL after task-1** — `generic_agent`: Add the optional per-model `properties` YAML schema to a native catalog loader (planned `internal/modelprops`), reading embedded `assets.FS` plus installed `.workflow-hero/models`. Preserve pricing-only files, model IDs, dynamic value order, defaults, availability, and unknown keys. Test with real embedded assets and `t.TempDir()` fixtures (PRD §4.2.8/12, §4.3.1–2; ADR-039).

- [ ] **task-3-sqlite-capability-cache** — **PARALLEL after task-1** — `generic_agent`: Add schema migration v5 and store helpers for project-scoped model lists/capabilities, normalized JSON, and refresh timestamps. Cover v4→v5 migration, round trips, API replacement semantics, stale reads, and cross-project isolation with real SQLite temp files (PRD §4.2.8–10/§5; ADR-039).

- [ ] **task-4-hero-json-property-state** — **PARALLEL after task-1** — `generic_agent`: Extend `install.HeroJSON` with nested `model_properties` keyed by harness/native model ID, preserving old files and legacy fast behavior. Add clone/read helpers and backward-compatibility tests for absent maps, independent pairs, future keys, and existing C4 fields (PRD §4.1.5–8; ADR-040).

- [ ] **task-5-cursor-property-transport** — **PARALLEL after task-1** — `generic_agent`: Keep Cursor's existing Execute/stream/cancel/session behavior while mapping normalized C5 properties through its established native model/slug composition. Ensure workflow-composed slugs are not double-suffixed. Add focused adapter tests and run the existing Cursor adapter package unchanged (PRD §4.5.2; ADR-041).

- [ ] **task-6-opencode-property-discovery-transport** — **PARALLEL after task-1** — `generic_agent`: Implement OpenCode's optional capability discovery and normalized response parsing inside `internal/adapters/opencode`, and extend native HTTP payload construction for normalized `fs`, `th`, and `ef`. Use injectable HTTP fixtures, including `opencode-go/deepseek-v4-pro`; do not require a live LLM (PRD §4.2.1–3, §4.5.3–4; ADR-038/041).

- [ ] **task-7-workflow-property-projection** — **PARALLEL after task-1** — `generic_agent`: Add workflowconfig/TUI helpers that derive `fs` from `enable_fast_model`, `th` from `thinking`, and `ef` from `reasoning_effort`, retaining an unvalidated marker for display. Test that workflow/fallback YAML remains authoritative and freechat JSON is not read or mutated for stage execution (PRD §4.5.6/§4.6.7; ADR-040/042).

## 3. Resolution and persistence composition — SERIES/PARALLEL

- [ ] **task-8-capability-resolver-reconciliation** — **SERIES after tasks 2–4** — `generic_agent`: Compose API/cache/catalog/`na` precedence, API-authoritative replacement, default precedence, timestamp/stale warnings, valid-choice preservation, invalid-choice→`na`, and missing-catalog warning in the model-property service. Cover complete, partial, absent, dynamic, stale, and invalidated fixtures (PRD §4.2–4.3; UI §5–6; ADR-039/040).

- [ ] **task-9-background-refresh-coordinator** — **SERIES after task-8 and task-6** — `generic_agent`: Implement refresh orchestration invoked only when `/hero-model` opens, fan out across every enabled harness, retain immediate local/cache snapshots, and persist results with a generation/pending marker. Add tests proving OpenCode is not started at TUI boot and completed refreshes do not reorder an open model/property selector (PRD §4.2.4–7; §4.4.8; UI §2/§5; ADR-039).

- [ ] **task-10-atomic-model-selection-commit** — **PARALLEL after task-4** — `generic_agent`: Add one install-layer helper that commits `freechat_default`, `harnesses.<harness>.model`, and the complete property draft atomically (temporary-file/rename or equivalent). Add tests proving no writes during row edits, complete Enter commit, full Escape cancellation, immediate save when no property is selectable, and unchanged workflow YAML (PRD §4.1; UI §3; ADR-040/042).

- [ ] **task-17-catalog-fixtures-and-asset-contract** — **PARALLEL after task-2** — `generic_agent`: Add capability-only/synthetic complete, partial, and absent catalog fixtures plus the primary OpenCode `opencode-go/deepseek-v4-pro` fixture without inventing provider pricing. Update embedded-asset/catalog contract tests so optional property metadata survives install projection and existing context-window/pricing entries remain valid (PRD §4.2.12, §6–7; UI §8).

## 4. TUI state integration — SERIES

- [ ] **task-11-tui-model-picker-integration** — **SERIES after tasks 8–10 and 17** — `generic_agent`: Integrate the model-property service into `internal/tui` state and `/hero-model`: load the immediate snapshot, launch background refresh for enabled harnesses, retain current pair/draft, and transition from C4 model selection to the property screen or atomic skip-save. Add tests for enabled-harness flow, persisted selection loading, missing metadata continuation, and next-opening refresh application (PRD §4.1–4.2; UI §§2–3).

## 5. Independent UI tracks after TUI state hook — PARALLEL

- [ ] **task-12-property-picker-interactions** — **PARALLEL after task-11** — `generic_agent`: Implement the main/secondary property picker interactions, friendly labels, boolean Space toggle, dynamic value lists, disabled gray rows, `ENTER to save` footer, complete Enter commit, and Escape cancellation. Add Bubble Tea interaction and golden tests, including per-model restoration (PRD §4.4; UI §3/§8; ADR-042).

- [ ] **task-13-chat-property-status-line** — **PARALLEL after task-11** — `generic_agent`: Extend Chat response/status rendering with stable `[fs-…] [th-…] [ef-…]` labels beside the scroll hint/context bar, empty-chat visibility, green/gray semantics including fast-off behavior, freechat/workflow projection, and rune-safe responsive wrapping that never hides the context bar. Add golden/render tests (PRD §4.6; UI §4/§7; ADR-042).

- [ ] **task-14-warning-and-terminal-accessibility** — **PARALLEL after task-11** — `generic_agent`: Add yellow warning status handling and next-user-action clearing without changing red execution errors. Cover missing-catalog, stale-cache, invalidated-value, and rejection messages, no-color textual semantics, non-TTY behavior, and multi-byte narrow wrapping without panics (PRD §4.6.8–10; UI §§5/7–8; ADR-042).

## 6. Execution integration — SERIES

- [ ] **task-15-execution-property-routing** — **SERIES after tasks 5–7 and 12–13** — `generic_agent`: Update TUI execution resolution so ordinary Chat and `/hero-new` attach the selected freechat property map, while active workflow/runtime commands attach the YAML-derived map. Preserve C4 pair fallback, session/harness binding, Cursor slug behavior, and no stage-YAML mutation. Add injected-adapter tests asserting exact normalized requests for freechat, `/hero-new`, orchestration, and Research (PRD §4.1.4/§4.5–4.6; ADR-041/042).

- [ ] **task-16-explicit-property-rejection** — **PARALLEL after task-15** — `generic_agent`: Thread adapter property-aware errors through the TUI execution path so a rejected property is identified in the red status/transcript error, leaves persisted choices intact, and cannot trigger property stripping, silent retry, or unrelated harness fallback. Add Cursor and OpenCode rejection tests (PRD §4.5.5; UI §5; ADR-041).

## 7. Integration, compatibility, and documentation — PARALLEL after execution

- [ ] **task-18-end-to-end-tui-contract-tests** — **SERIES after tasks 12–16** — `generic_agent`: Build deterministic temp-project TUI integration coverage for full/partial/absent metadata, model switching/restoration, Enter/Escape, background refresh responsiveness, active status-line projection, warnings, narrow/no-color output, Chat execution, and `/hero-new` execution. Use injected adapters and real SQLite/JSON/catalog dependencies; no live LLM (PRD §6–7; UI §8).

- [ ] **task-19-c4-regression-and-migration-tests** — **PARALLEL after task-15** — `generic_agent`: Run and extend existing C4 tests for Cursor/OpenCode model pair selection, lazy OpenCode serve, harness-scoped sessions, two-level fallback, legacy hero.json, schema v4 migration, context bar, and Cursor Execute/stream/cancel. Fix only C5 integration regressions and keep Cursor adapter diffs minimal (PRD §2/§5; ADR-038–042 plus C4 compatibility constraints).

- [ ] **task-20-runtime-help-and-asset-documentation** — **PARALLEL after task-15** — `generic_agent`: Update the installed workflow help/asset contract documentation to describe `/hero-model` properties, `[fs-…]`/`[th-…]`/`[ef-…]` labels, cache/catalog warnings, and the separation from workflow YAML. Keep all cycle artifacts English and add asset golden/inventory assertions (PRD §5.4–5; UI §§2–6).

## 8. Close — SERIES

- [ ] **task-21-full-suite-and-review-handoff** — **SERIES after tasks 18–20** — `generic_agent`: Run `go test ./...`, review the complete diff against the approved PRD/UI/ADR, verify no browser/E2E task was introduced, and resolve all failures. Confirm the implementation is limited to `native`/`generic_agent` routing.

- [ ] **task-22-cycle-context-and-openspec-link** — **SERIES after task-21** — `generic_agent`: Update `context/current-state.md` and append the implementation outcome to `context/context-log.md`; verify the active cycle stores `model-properties-tui` as its OpenSpec change and that all C5 artifacts remain in English.

## Parallel Groups

| Group | Tasks | Dependency boundary |
|---|---|---|
| A — shared native tracks | `task-2-model-catalog-parser`, `task-3-sqlite-capability-cache`, `task-4-hero-json-property-state`, `task-5-cursor-property-transport`, `task-6-opencode-property-discovery-transport`, `task-7-workflow-property-projection` | After `task-1-normalized-property-contract` |
| B — composition helpers | `task-8-capability-resolver-reconciliation`, `task-10-atomic-model-selection-commit`, `task-17-catalog-fixtures-and-asset-contract` | Each waits only for its listed catalog/cache/JSON dependency; may run concurrently |
| C — TUI surfaces | `task-12-property-picker-interactions`, `task-13-chat-property-status-line`, `task-14-warning-and-terminal-accessibility` | After `task-11-tui-model-picker-integration` |
| D — post-routing work | `task-16-explicit-property-rejection`, `task-19-c4-regression-and-migration-tests`, `task-20-runtime-help-and-asset-documentation` | After `task-15-execution-property-routing` |

Hard series spine: `1 → (2‖3‖4‖5‖6‖7) → (8‖10‖17) → 9 → 11 → (12‖13‖14) → 15 → (16‖19‖20) → 18 → 21 → 22`.

**Task count: 22 checklist items.**
