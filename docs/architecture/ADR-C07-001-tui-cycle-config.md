# ADR-C07-001 — TUI Cycle Configuration and Failed-Stage Retry

> Cycle C07 architecture decisions for the Hero 2.8 configuration-screen idea. Status: Proposed; requires Planning and human approval before implementation.

| # | Title | Status |
|---|---|---|
| [ADR-049](#adr-049-config-is-a-tui-editor-over-the-active-yaml-document) | Config is a TUI editor over the active YAML document | Proposed |
| [ADR-050](#adr-050-managed-node-merge-preserves-unmanaged-yaml-content) | Managed-node merge preserves unmanaged YAML content | Proposed |
| [ADR-051](#adr-051-cycle-sync-remains-the-boundary-between-yaml-and-sqlite) | Cycle sync remains the YAML/SQLite boundary | Proposed |
| [ADR-052](#adr-052-failed-stage-retry-is-an-explicit-engine-transition) | Failed-stage retry is an explicit engine transition | Proposed |
| [ADR-053](#adr-053-tui-elm-architecture-keeps-file-and-harness-work-off-update) | TUI Elm Architecture keeps I/O off `Update` | Proposed |

**Related:** [PRD-C07-001](../product/PRD-C07-001-tui-cycle-config.md), [UI-C07-001](../product/UI-C07-001-tui-cycle-config.md), [ADR-C05-001](ADR-C05-001-model-properties-tui.md).

## ADR-049: Config is a TUI editor over the active YAML document

**Context:** Users need a guided way to configure the active cycle, but Hero already defines `.workflow-hero/cycles/current/workflow-config.yml` as the cycle configuration and supports direct editing. Creating a second form-owned configuration would create drift and would make Cursor and TUI behavior diverge.

**Decision:** Add a Config screen only to the project TUI when an active cycle exists. The screen loads and edits the active YAML document, and never stores cycle configuration in `hero.json` or a TUI-only state file. The Cursor IDE workflow remains unchanged and continues to consume the YAML directly.

**Consequences:**

- TUI and Cursor share one source of truth.
- Config is unavailable in free-chat mode and when the active file is missing or invalid.
- The TUI must represent loading, read-only/busy, validation, and save states explicitly.
- The navigation rail gains a conditional second item, shifting the project screen shortcuts to Chat, Config, Status, Artifacts, Costs, Events.

## ADR-050: Managed-node merge preserves unmanaged YAML content

**Context:** `workflow-config.yml` can contain comments, ordering, `workflow_rules`, and keys introduced by future versions. A full struct unmarshal/marshal would destroy content outside the current form. The user also decided that a TUI draft wins when edits happen in parallel, but only for fields administered by Config.

**Decision:** Keep a round-trip-safe YAML node document and a typed managed projection in `internal/workflowconfig`. On Save, load the latest valid file, mutate only managed paths from the TUI draft, validate, and atomically replace the file via a same-directory temporary file and rename. Do not block on a revision mismatch and do not overwrite unmanaged content from the original screen snapshot.

**Consequences:**

- The latest file supplies comments, ordering, unknown keys, and non-managed values.
- The TUI draft supplies managed values, even after a parallel edit.
- Missing or syntactically invalid files fail closed and remain untouched.
- Managed-path definitions and mutation tests become a compatibility surface for future YAML keys.

## ADR-051: Cycle sync remains the YAML/SQLite boundary

**Context:** SQLite is the operational source of truth for cycle and stage state, while the YAML is the user-editable configuration source. Duplicating synchronization in the Config screen would create a second lifecycle implementation.

**Decision:** After a successful Save, the TUI calls the existing cycle synchronization service. Synchronization updates cycle title, objective, config snapshot, and still-open stage budgets/flags. Completed stages are protected. Failed stages remain terminal until the explicit retry transition in ADR-052.

**Consequences:**

- TUI does not write SQLite directly for ordinary configuration changes.
- Save and `/hero-start` share the same synchronization semantics.
- The sync service remains deterministic and testable without harness execution.
- Changes to terminal stages are retained in YAML where the form permits them, but do not retroactively rewrite completed stage state.

## ADR-052: Failed-stage retry is an explicit engine transition

**Context:** A failed stage currently cannot be started again. Users need to correct its configuration and retry only that stage, without rewriting completed work or losing prior evidence.

**Decision:** Add a deterministic, stage-specific retry transition exposed by the Config screen after a successful configuration change. Retry returns the selected failed stage to `Waiting`, resets the next attempt's iteration and timeout counters, and preserves prior events and metrics. It does not modify completed stages, other stages, or unrelated configuration. Retry is disabled until the failed stage has a changed and successfully saved managed configuration.

**Consequences:**

- `internal/engine` owns the state transition and invariant checks; `internal/tui` only requests it.
- The event log records the retry transition, while existing metrics remain available for Costs/history.
- Save and start can run the requeued stage through the existing start/preflight path.
- Planning must define the exact service/CLI boundary and tests before implementation; no new free-form CLI command is implied by this ADR.

## ADR-053: TUI Elm Architecture keeps I/O off `Update`

**Context:** Loading YAML, validating capabilities, writing files, synchronizing SQLite, and starting harness preflight can block or perform external work. Blocking Bubble Tea's event loop would make the Config screen unresponsive.

**Decision:** Implement Config as an Elm Architecture screen. Window size, theme, focus, dirty state, validation messages, and transitions are handled as messages. File I/O, catalog refresh, atomic writes, SQLite synchronization, retry calls, and `/hero-start` preflight run in `tea.Cmd` workers and return typed messages. Key bindings use centralized `key.Binding` values and `key.Matches`.

**Consequences:**

- `Update` stays non-blocking and deterministic.
- View functions remain pure, use `strings.Builder`, and calculate widths with Lip Gloss/ANSI-aware helpers.
- Responsive rendering must hide lower-priority controls or show an intentional too-small warning.
- TUI tests can exercise state transitions with temporary files and typed messages without live harnesses.

