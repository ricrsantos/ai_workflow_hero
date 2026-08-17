# ADR-C05-001 — Dynamic Model Properties, Capability Discovery, and Cache

Cycle C5 architecture decisions for model-property selection in the Hero TUI. These decisions extend C4's multi-harness model picker without changing the Cursor IDE Runtime boundary.

## Decision index

- ADR-038: Optional harness capability discovery and normalization — Accepted
- ADR-039: API-first metadata with project cache and local fallback — Accepted
- ADR-040: Per-harness/model property persistence in hero.json — Accepted
- ADR-041: Adapter-owned property transport and explicit rejection — Accepted
- ADR-042: TUI property picker and effective-property projection — Accepted

## ADR-038: Optional capability discovery and normalization

Context: C4 adapters can enumerate model IDs, but model properties differ by harness and may not be available from every API. Making capability metadata mandatory would break adapters that can list models but cannot describe their options.

Decision:

1. Keep `ListModels` compatible with its current model-ID contract.
2. Add an optional capability-discovery contract for adapters that can provide model metadata.
3. Normalize native responses into a Hero-owned capability representation containing property key, accepted values, default, availability, and adapter mapping data.
4. Keep API response parsing and native key mapping inside each adapter.
5. Treat missing capability support as a normal fallback condition, not as an adapter failure.

Consequences:

- Cursor can use local catalog data until its harness exposes richer metadata.
- OpenCode can evolve independently from the TUI's normalized representation.
- Future harnesses can implement capability discovery incrementally.
- Planning must define the concrete interface and normalized Go types without coupling the TUI to provider schemas.

## ADR-039: API-first metadata with project cache and local fallback

Context: Live harness metadata is the most accurate source, but OpenCode startup and API access can introduce a visible delay or fail when a CLI is unavailable. The project already has a SQLite operational store and versioned model catalog assets.

Decision:

1. Resolve model IDs and capabilities using this precedence:
   - live harness API;
   - project-scoped cache in `hero.db`;
   - installed local `assets/models/*.yml` catalog;
   - unknown values represented by `na`.
2. Persist model metadata and a refresh timestamp in a dedicated operational cache structure in `hero.db`.
3. Use persisted cache regardless of age when live refresh fails, while reporting staleness when relevant.
4. Treat a successful API response as authoritative and replace the cached accepted-value arrays.
5. Start background refresh for every enabled harness when `/hero-model` opens, not at TUI boot.
6. Apply completed refresh results on the next selector opening so the active list never moves beneath the user's cursor.
7. Do not start OpenCode at TUI boot solely for metadata preloading.

Consequences:

- The first `/hero-model` interaction can start OpenCode, but ordinary TUI boot remains lazy.
- A user can select a model immediately from cache or catalog while refresh runs.
- SQLite schema/version migration is required for the cache.
- Cache data is project-specific and must not be treated as a global provider catalog.

## ADR-040: Per-harness/model property persistence in hero.json

Context: A single global property set would carry invalid or surprising values between models. C5 requires model-specific restoration while preserving C4's existing `harnesses.<harness>.model` and `freechat_default` fields.

Decision:

1. Keep `harnesses.<harness>.model` and `freechat_default` as the selected model-pair fields.
2. Add an extensible `model_properties` map keyed by harness and native model ID.
3. Store selected values as strings so boolean, enum, and future harness values use one persistence shape.
4. Restore values only when they remain accepted by current metadata.
5. Convert removed or invalid values to `na` and warn the user.
6. Do not write these selections into `agents.*` or `fallback_model` blocks in a cycle's `workflow-config.yml`.

Consequences:

- Existing hero.json files remain valid without `model_properties`.
- `/hero-model` can restore independent choices for each model.
- Stage execution remains controlled by cycle YAML and is not silently changed by a freechat preference.

## ADR-041: Adapter-owned property transport and explicit rejection

Context: A normalized UI key such as `ef` cannot be assumed to be the same request key or value format in every harness. Silent conversion or retry would hide a mismatch between the user's choice and the model execution.

Decision:

1. Pass normalized selected properties through the execution request to the adapter.
2. Each adapter maps `fs`, `th`, and `ef` to its native request representation.
3. Cursor retains its established native slug composition where that is the supported mechanism.
4. OpenCode keeps native HTTP payload construction inside the OpenCode adapter.
5. If the harness rejects a selected property, return an explicit execution error naming the property; do not strip it or retry silently.

Consequences:

- The TUI does not contain provider-specific protocol logic.
- Adapter tests must cover request serialization and rejection behavior.
- A future harness can support a subset of properties without changing the shared TUI contract.

## ADR-042: TUI picker and effective-property projection

Context: Users need to understand both what a model supports and what is currently applied. The picker must handle booleans and dynamic value arrays without partial saves, and the Chat status line must remain useful during freechat and workflow execution.

Decision:

1. Open a property picker after model selection only when at least one property is selectable.
2. Show unsupported properties as visible, gray, disabled rows.
3. Use Space for boolean checkboxes and Enter to open/confirm multi-value lists.
4. Show `ENTER to save` in the footer; Enter commits the complete selection and Escape cancels it.
5. Render `fs`, `th`, and `ef` values below the response pane beside the scroll/context line.
6. Render configured supported values green; render `na`, unavailable, and unvalidated values gray.
7. Show freechat properties for ordinary Chat and `/hero-new`; show the active workflow agent's configured values during workflow execution.
8. Use the existing yellow status warning convention for missing, stale, or invalid metadata, clearing the warning on the next user action.

Consequences:

- The property line is visible before the first Chat message when a model is selected.
- Responsive wrapping is required for narrow terminals.
- Golden and interaction tests must assert colors semantically where possible and exact labels in non-color output.
