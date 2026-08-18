# Implementation Progress — model-properties-tui

Implementation resumed from checkpoint `0ab1310` (the prior session had
already landed the main C5 vertical slice and its colocated tests). This
iteration reviewed the partial work, fixed the workflow-property routing
regression, completed local/cache refresh composition and Runtime guidance,
and verified the full native suite. Browser UI Validation and QA End-to-End
are intentionally out of scope for this native-only change.

## Task status

| Task | Status | Evidence |
|---|---|---|
| task-1-normalized-property-contract | done | `internal/harness/properties.go`; `TestModelPropertyDiscovererIsOptionalContract`, `TestNormalizeExecuteRequestCopiesAndFiltersProperties`, rejection contract tests |
| task-2-model-catalog-parser | done | `internal/modelprops/catalog.go`; embedded/installed parser tests, dynamic order/default/unknown-key fixtures, provider-scoped model rows |
| task-3-sqlite-capability-cache | done | schema migration v5 and `model_props_cache.go`; migration, round-trip, replacement, stale-read, generation, and project-isolation tests |
| task-4-hero-json-property-state | done | `HeroJSON.ModelProperties` and clone/read helpers; backward-compatibility, legacy-fast, future-key, and C4-field tests |
| task-5-cursor-property-transport | done | Cursor normalized slug composition and rejection handling; no-double-suffix and unchanged execute/stream/cancel tests |
| task-6-opencode-property-discovery-transport | done | OpenCode normalized discovery, native option serialization, aliases, injectable HTTP, DeepSeek fixture, and rejection tests |
| task-7-workflow-property-projection | done | `workflowconfig/properties.go`; YAML-authority, fallback, unvalidated display, and freechat-isolation tests |
| task-8-capability-resolver-reconciliation | done | API/cache/catalog/unknown precedence, partial merge, defaults, stale warning, valid restoration, and invalidation tests |
| task-9-background-refresh-coordinator | done | `modelprops.Service.Refresh` fan-out, cache generations/pending state, local-first snapshots, stale refresh state, and open-selector stability tests |
| task-10-atomic-model-selection-commit | done | temp-file/rename install commit; no-partial-write, Enter, Escape, skip-picker, and unchanged workflow-YAML tests |
| task-11-tui-model-picker-integration | done | `/hero-model` harness/model/property flow, cache/catalog model rows, persisted restoration, warning continuation, and next-opening refresh behavior |
| task-12-property-picker-interactions | done | Bubble Tea picker interaction tests for Space, secondary dynamic lists, disabled rows, footer, Enter commit, Escape, and per-model restoration |
| task-13-chat-property-status-line | done | status-line render tests for `[fs-*] [th-*] [ef-*]`, empty Chat, green/gray semantics, workflow projection, context bar, and narrow wrapping |
| task-14-warning-and-terminal-accessibility | done | missing/stale/invalidated warning paths, warning clearing, red rejection preservation, no-color textual output, and multi-byte narrow rendering tests |
| task-15-execution-property-routing | done | injected adapter assertions for freechat Chat, `/hero-new`, orchestration/workflow YAML projection, and no YAML mutation |
| task-16-explicit-property-rejection | done | Cursor/OpenCode adapter rejection tests plus TUI red-error/no-strip/no-retry coverage |
| task-17-catalog-fixtures-and-asset-contract | done | `testdata/{complete,partial,absent}.yml`, embedded `opencode-go/deepseek-v4-pro`, install projection and pricing/context-window inventory assertions |
| task-18-end-to-end-tui-contract-tests | done | deterministic temp-project TUI coverage across full/partial/absent metadata, switching, save/cancel, status/warnings, Chat and `/hero-new`; no live LLM |
| task-19-c4-regression-and-migration-tests | done | existing C4 harness/session/lazy-serve/context/adapter suites plus v4→v5 migration; Cursor diff kept focused |
| task-20-runtime-help-and-asset-documentation | done | Cursor/OpenCode `/hero-model`/`/hero-help`, installed workflow-help EN/PT-BR guidance, and embedded help/model asset assertions |
| task-21-full-suite-and-review-handoff | done | `go test ./...` green; `go vet ./...` green; targeted `go test -race ./internal/modelprops ./internal/tui` green; no browser/web task introduced |
| task-22-cycle-context-and-openspec-link | done | updated `context/current-state.md`, appended `context/context-log.md`, retained active change slug `model-properties-tui`, and kept C5 artifacts English |

## Deviations and notes

- The prior checkpoint contained a failing direct workflow projection test; the
  TUI follow-up path now recognizes an active workflow agent without replacing
  injected C4 test harness model behavior.
- A live capability response may be partial. The resolver replaces covered
  properties while retaining unaffected cached keys for the next local
  snapshot, preserving API-first precedence without discarding usable data.
- The repository's existing C4 logging call sites still contain historical
  `slog.Warn` usages; new/changed C5 paths use only `error`, `info`, and
  `debug` levels per the implementation logging standard.
- No `.workflow-hero/cycles/current/workflow-config.yml`, Hero state SQLite,
  or `opencode.json` files were modified by this resumed implementation.
