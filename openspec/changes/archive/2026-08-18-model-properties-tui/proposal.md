## Why

Hero 2.0 already lets a TUI user choose a harness/model pair, but the selected model's fast mode, thinking mode, and reasoning effort are implicit. The user cannot configure those values in `/hero-model`, cannot see the effective values in Chat, and cannot tell whether a value is unavailable, stale, or rejected. Cycle C5 adds model-property selection without changing the Cursor IDE Runtime boundary or the C4 pair-selection contract (PRD-C05-001 §1; ADR-038–042; UI-C05-001 §1).

The authoritative C5 display format is the documented prefix/value form: `[fs-true] [th-max] [ef-high]`. `fs`, `th`, and `ef` are the visible C5 keys; property values remain dynamic strings supplied by a harness or catalog. The fast label is green only when fast is enabled and validated; it is gray when disabled or unavailable.

## What Changes

- Add an optional, adapter-owned capability-discovery contract alongside the unchanged `ListModels` contract.
- Normalize harness metadata into Hero-owned property capabilities with dynamic accepted values, defaults, availability, and opaque adapter transport state.
- Resolve model IDs and capabilities API-first, then from the project-scoped SQLite cache, then from embedded/installed `assets/models/*.yml`, and finally as `na` with a yellow warning.
- Start refresh for every enabled harness when `/hero-model` opens, while keeping TUI boot and OpenCode boot lazy. A completed refresh is applied on the next selector opening.
- Add a project SQLite model-list/capability cache with refresh timestamps and a migration from schema v4.
- Add an extensible `model_properties` map to `hero.json`, keyed by harness and native model ID, while preserving `harnesses.<harness>.model` and `freechat_default`.
- Make the C4 model picker open a property picker when at least one C5 property is selectable; support boolean checkboxes, dynamic multi-value submenus, disabled unsupported rows, atomic Enter-to-save, and Escape cancellation.
- Pass normalized properties to real freechat Chat and `/hero-new` executions. Keep Cursor slug composition and OpenCode request serialization inside their adapters, and surface property rejection explicitly without stripping or retrying.
- Render the effective `fs`, `th`, and `ef` labels below the response pane beside the scroll/context line, including empty-chat, workflow-agent, narrow-terminal, and no-color behavior.
- Keep workflow stage values authoritative in `workflow-config.yml`; freechat `model_properties` never edits `agents.*` or `fallback_model`.

## In Scope

- Native Go code under `internal/`, Bubble Tea TUI behavior, Cursor/OpenCode adapter contracts, SQLite migration/helpers, project `hero.json` persistence, embedded/local model catalog parsing, adapter fixtures, and colocated tests.
- The C5 keys `fs`, `th`, and `ef`. Future keys may be cached and persisted but are ignored by the C5 TUI and execution projection.
- Cursor and OpenCode only, through the existing C4 harness registry.
- Synthetic complete/partial/absent capability fixtures plus `opencode-go/deepseek-v4-pro` as the primary OpenCode adapter fixture (PRD-C05-001 §7).

## Out of Scope

- Editing `agents.*` or `fallback_model` in `workflow-config.yml` through `/hero-model`, or adding model properties to Cursor IDE Runtime prompts.
- New harnesses, cross-harness canonical model IDs, provider-specific logic in the TUI, or a global metadata cache.
- Starting OpenCode at TUI boot solely to preload metadata.
- Browser UI Validation, QA End-to-End, web UI work, or any task routed to `backend_agent`/`frontend_agent`; C5 scope is `native: true` and all implementation tasks target `generic_agent`.

## CLI vs Runtime Classification

Per ADR-003, this is a deterministic native/TUI feature. Capability parsing, cache resolution, JSON persistence, adapter transport, and rendering belong to the Go CLI/TUI. Cursor IDE Runtime assets remain Cursor-only and continue to use cycle YAML model fields; they do not read `model_properties`. No LLM reasoning is added to the binary.

## Capabilities

### New Capabilities

- `model-property-discovery`: optional adapter discovery, normalized capabilities, source precedence, and background refresh.
- `model-property-catalog`: dynamic property metadata in embedded/installed model catalogs and user-choice reconciliation.
- `model-property-persistence`: backward-compatible per-harness/model choices in `hero.json` and atomic selection commits.

### Modified Capabilities

- `hero-tui`: C4 `/hero-model` gains the property submenu and Chat gains the effective-property status line/warnings.
- `harness-adapter`: `ExecuteRequest` carries normalized properties; adapters own native mapping and explicit rejection behavior.
- `sqlite-operational-store`: schema v5 stores project-scoped model lists/capabilities and refresh timestamps.
- `runtime-workflow-execution`: stage execution projects YAML values, never freechat values, while preserving C4 routing and lazy OpenCode behavior.

## Requirement Traceability

Every approved C5 source section is represented by at least one delta requirement and one implementation task. Requirement IDs refer to the `specs/` files in this change.

| Approved source | Delta requirements | Implementing tasks |
|---|---|---|
| PRD-C05-001 §§1–3 | discovery R1–R3; persistence R1/R3; runtime R1 | 1–4, 7–11, 15 |
| PRD-C05-001 §4.1 | persistence R1–R3; hero-tui R1/R2 | 4, 10–12, 18 |
| PRD-C05-001 §4.2 | discovery R1–R3; catalog R1; SQLite R1/R2 | 1–3, 6–9, 17 |
| PRD-C05-001 §4.3 | discovery R1/R2; catalog R1/R2 | 1–3, 8, 17 |
| PRD-C05-001 §4.4 | hero-tui R1/R2/R4 | 11–14, 18 |
| PRD-C05-001 §4.5 | harness R1–R3; runtime R1 | 5–7, 15–16, 19 |
| PRD-C05-001 §4.6 | hero-tui R3/R4; runtime R1 | 7, 13–14, 18 |
| PRD-C05-001 §§5–7 | SQLite R1/R2; persistence R1/R3; runtime R1; all test scenarios | 3–4, 7, 17–22 |
| UI-C05-001 §§1–3 | hero-tui R1/R2 | 11–12, 18 |
| UI-C05-001 §§4–6 | hero-tui R3/R4; runtime R1 | 7, 13–15, 18 |
| UI-C05-001 §§7–8 | hero-tui R4; test obligations in tasks 14 and 18 | 14, 18, 21 |
| ADR-038 | discovery R1; harness R1 | 1, 5–6 |
| ADR-039 | discovery R2/R3; SQLite R1/R2; catalog R1 | 2–3, 6–9, 17 |
| ADR-040 | persistence R1–R3 | 4, 10, 18 |
| ADR-041 | harness R1–R3 | 1, 5–6, 15–16, 19 |
| ADR-042 | hero-tui R1–R4 | 11–14, 18 |

## Impact

Expected implementation areas are `internal/harness`, `internal/modelprops` (new vertical slice), `internal/store`, `internal/install`, `internal/workflowconfig`, `internal/adapters/cursor`, `internal/adapters/opencode`, and `internal/tui`, plus optional `properties` fields in `assets/models/*.yml` and adapter test fixtures. The implementation must minimize Cursor adapter changes and keep existing C4 tests green.

## Success Criteria

The implementation is complete when a full-capability, partial-capability, and no-metadata model can be selected; choices restore independently; Enter saves and Escape cancels atomically; background refresh does not block or reorder an open picker; Chat and `/hero-new` send the selected values; workflow execution uses YAML values; colors/text/wrapping match UI-C05-001; rejected properties fail explicitly; and `go test ./...` passes.
