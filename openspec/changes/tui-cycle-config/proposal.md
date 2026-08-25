## Why

Hero TUI users still configure an active cycle by editing `.workflow-hero/cycles/current/workflow-config.yml` by hand. That file is large, mixes always-relevant identity fields with stage/agent settings that depend on scope, and is easy to break (empty title, zero budgets, frontend gates, harness/model mismatches). Cycle C7 adds a guided Config screen that reads and writes the same YAML, so TUI and Cursor IDE continue to share one source of truth (PRD-C07-001 §1–§3; ADR-049).

The idea file `docs/idea/v2_8_config/config-screen.md` is the product origin. On divergence it yields to [PRD-C07-001](../../docs/product/PRD-C07-001-tui-cycle-config.md), [UI-C07-001](../../docs/product/UI-C07-001-tui-cycle-config.md), and [ADR-C07-001](../../docs/architecture/ADR-C07-001-tui-cycle-config.md) (ADR-049–053). In particular, the idea-file reload/merge/cancel conflict dialog is **not** in scope: Save always merges the latest valid file with TUI precedence on managed paths only (PRD-C07-001 §4.5; UI-C07-001 §7; ADR-050).

## What Changes

- Add **Config** as the second project TUI nav item (`Chat | Config | Status | Artifacts | Costs | Events`) only when an active cycle exists; never in `hero chat` (PRD-C07-001 §4.1; UI-C07-001 §2; ADR-049).
- Load the active YAML as a `yaml.v3.Node` document plus a typed managed form; mutate only managed paths on Save; preserve comments, order, `workflow_rules`, unknown keys, and future fields (PRD-C07-001 §4.2/§4.5; ADR-050).
- Progressive disclosure for disabled stages, implementation agents by scope, Browser UI / Playwright frontend gates, and nested `subagent` when `same_of_agent` is false (PRD-C07-001 §4.3; UI-C07-001 §5).
- Harness → native model → C5 properties (`fs`/`th`/`ef`) reusing `internal/modelprops` and `internal/harnessmgr`; persist into YAML agent/fallback blocks, never `hero.json.model_properties` (PRD-C07-001 §4.4; ADR-C05-001).
- Save validates, writes atomically, then calls existing `SyncCycleConfig` (title/objective/snapshot + still-open stage budgets). Completed stages stay protected (PRD-C07-001 §4.6–§4.7; ADR-051).
- Explicit **Retry failed stage** engine transition after a successful Save that changed that stage’s managed configuration (PRD-C07-001 §4.8; ADR-052).
- **Save and start** saves first, then follows the existing `/hero-start` preflight/execution path (PRD-C07-001 §4.9; UI-C07-001 §8).
- Config is read-only while any agent Execute or `/hero-start` preflight is running (PRD-C07-001 §4.1; UI-C07-001 §7).
- Align in-progress `internal/workflowconfig` Document scaffolding to ADR-050 (latest-file merge; no revision-mismatch dialog).

### In Scope

- Native Go TUI Config screen, workflowconfig managed-document API, cycle sync invocation, and failed-stage retry service (no new free-form CLI command).
- Managed fields listed in PRD-C07-001 §4.2 only: identity, chat language, five scopes, per-stage enable/purpose/budgets/approval, Browser UI visual validation, Playwright flag, agent/fallback harness/model/properties/subagent.
- Reuse of C5 catalogs, cache, capability snapshots, and enabled-harness lists.
- Colocated tests with real temp files; golang-tui Elm Architecture constraints on the existing `github.com/charmbracelet/bubbletea` stack (no Bubble Tea v2 migration).

### Out of Scope

- Free-chat configuration; Config item in `hero chat`.
- Cursor IDE Runtime, slash-command, or YAML-editing changes.
- A second configuration source in SQLite or `hero.json` agent settings.
- Reload/merge/cancel conflict dialog (idea file; superseded by ADR-050).
- Concurrent editing while an agent or preflight is running.
- New harness adapter protocol, `hero serve` daemon, Windows CLI, CI/CD, GPG signing.
- Browser UI Validation / QA End-to-End implementation work (C7 scope `native` only; those stages remain configurable in YAML/UI when the user enables them).
- Migrating the TUI to Bubble Tea v2.

### CLI vs Runtime Classification (ADR-003)

- **CLI/TUI:** `internal/tui` Config screen, `internal/workflowconfig` node merge, `internal/engine` retry, `internal/cycle` service wrapper, reuse of `internal/modelprops` / `internal/harnessmgr`.
- **Runtime:** unchanged. Cursor IDE continues to read `workflow-config.yml` directly and run `/hero-start` as today.

## Capabilities

### New Capabilities

- `workflow-config-document`: round-trip-safe YAML node document, managed-path mutation, latest-file merge, semantic validation, atomic write, managed-field diff (ADR-050; PRD-C07-001 §4.2/§4.5–§4.6).
- `tui-cycle-config`: Config screen, progressive disclosure, harness/model/property pickers, save states, dirty-exit, read-only busy, Save and start (UI-C07-001; ADR-049/053).
- `failed-stage-retry`: deterministic engine transition from Failed → Waiting with counter reset and preserved history/metrics (ADR-052; PRD-C07-001 §4.8).

### Modified Capabilities

- `hero-tui`: conditional Config nav item; shortcut order Chat/Config/Status/Artifacts/Costs; Events remains palette-reachable; free-chat unchanged (UI-C07-001 §2).
- `runtime-workflow-execution`: Save invokes existing sync; Save and start reuses `/hero-start`; retry is the only path that requeues a failed stage (ADR-051/052).
- `sqlite-operational-store`: append-only retry event type; no schema bump; metrics rows remain (ADR-052).

## Requirement Traceability

Every approved C7 source section is represented by at least one delta requirement and one implementation task.

| Approved source | Delta requirements | Implementing tasks |
|---|---|---|
| PRD-C07-001 §4.1 availability | tui-cycle-config R1; hero-tui R1 | 3.1–3.4, 4.5 |
| PRD-C07-001 §4.2 managed fields | workflow-config-document R1 | 1.1–1.3, 4.2 |
| PRD-C07-001 §4.3 progressive disclosure | tui-cycle-config R2 | 4.3, 5.4 |
| PRD-C07-001 §4.4 harness/model/properties | tui-cycle-config R3 | 5.1–5.5 |
| PRD-C07-001 §4.5 persistence/merge | workflow-config-document R2–R4 | 1.1–1.4, 6.1 |
| PRD-C07-001 §4.6 validation | workflow-config-document R3 | 1.3, 4.6 |
| PRD-C07-001 §4.7 cycle sync | runtime-workflow-execution R1 | 2.4, 6.1 |
| PRD-C07-001 §4.8 failed-stage retry | failed-stage-retry R1; tui-cycle-config R5 | 1.5, 2.1–2.5, 6.4 |
| PRD-C07-001 §4.9 save actions | tui-cycle-config R4 | 6.1–6.3 |
| PRD-C07-001 §4.10 compatibility | hero-tui R2; runtime R2 | 7.2 |
| UI-C07-001 §§2–4 nav/controls | hero-tui R1; tui-cycle-config R1 | 3.1–3.3, 4.1 |
| UI-C07-001 §§5–7 disclosure/states | tui-cycle-config R2/R4 | 4.3–4.6, 5.5 |
| UI-C07-001 §§8–9 save/retry | tui-cycle-config R4/R5 | 6.1–6.4 |
| UI-C07-001 §§10–12 responsive/testing | tui-cycle-config R6 | 4.1, 4.6, 7.4 |
| ADR-049 | tui-cycle-config R1; hero-tui R1 | 3, 4 |
| ADR-050 | workflow-config-document R2 | 1.1–1.4, 6.1 |
| ADR-051 | runtime-workflow-execution R1 | 2.4, 6.1 |
| ADR-052 | failed-stage-retry R1 | 2.1–2.5, 6.4 |
| ADR-053 | tui-cycle-config R6 | 4.1, 5, 6 |

## Impact

- Packages: `internal/workflowconfig` (complete/align Document API), `internal/tui` (new Config screen + nav), `internal/engine` + `internal/cycle` (retry), reuse `internal/modelprops` / `internal/harnessmgr` / `internal/store` (event constant). `docs/architecture/architecture-overview.md` after the TUI/data-flow lands.
- Existing WIP: `Document.Write` currently returns `ErrExternalChange` on revision mismatch; C7 Save must load latest + apply managed draft instead. Replace that test with merge-on-save coverage.
- Tests: YAML goldens, validation, TUI navigation/states, harness/model/property filtering, sync, retry, Cursor Runtime regression, `go test ./...`.
- Implementation agent: **generic_agent** (`scope.native: true`).
- OpenSpec change: `tui-cycle-config`.

## Success Criteria

1. Config appears only with an active cycle and never in free chat.
2. Save preserves comments, order, `workflow_rules`, and unknown keys; parallel edits merge with TUI precedence on managed fields only.
3. Progressive disclosure hides irrelevant controls without deleting YAML values.
4. Harness → model → properties persist to the correct YAML blocks; missing capabilities warn and do not silently rewrite values.
5. Sync updates open stages only; completed stages stay protected; failed-stage retry is explicit, stage-specific, and preserves events/metrics.
6. Save and start uses the existing `/hero-start` path; Cursor IDE workflow is unchanged.
7. `go test ./...` is green.
