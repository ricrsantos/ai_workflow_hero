# PRD-C05-001 — Dynamic Model Properties in the Hero TUI

Cycle C5 product requirements for selecting, persisting, executing, and displaying model properties through the Hero TUI `/hero-model` flow.

## 1. Overview

Hero 2.0 already lets the TUI user select a harness/model pair. The current flow does not expose model-specific properties such as fast mode, thinking modes, or reasoning effort. Users must either accept implicit defaults or edit unrelated configuration fields without seeing which values are active in Chat.

C5 adds a model-property selection step after model selection. The feature applies to the TUI freechat default and `/hero-new`; it does not edit cycle agent blocks in `workflow-config.yml`.

The first implemented property keys are:

- `fs`: fast mode
- `th`: thinking
- `ef`: reasoning effort

Property values are dynamic strings supplied by the selected harness or a local catalog. The implementation is extensible, but the TUI only renders and sends these three keys in C5.

## 2. Goals

- Let users select model-specific properties immediately after choosing a harness/model in `/hero-model`.
- Use harness APIs for model lists and capabilities when available.
- Preserve a deterministic local fallback through `assets/models/*.yml` and a persistent project cache.
- Persist choices independently for every harness/model pair.
- Apply selected properties to real Chat and `/hero-new` executions.
- Show the effective properties below the response pane using Hero's existing colors and layout conventions.
- Avoid silently ignoring missing metadata, stale cache, invalid selections, or harness rejection.
- Preserve C4 behavior for stage configuration, Cursor IDE Runtime, and existing model selection.

## 3. Scope

### 3.1 In scope

- Optional model-capability discovery in harness adapters.
- Normalization of harness-specific capability metadata for TUI consumption.
- Model and capability sources with API, SQLite cache, and local catalog fallback.
- Background refresh when `/hero-model` opens.
- Dynamic property values and per-model defaults.
- TUI property picker and confirmation/cancellation behavior.
- `hero.json` persistence for per-pair user choices.
- Adapter-specific execution mapping and explicit rejection errors.
- Effective-property status line and warning messages.
- Unit, integration, golden, and adapter contract tests.

### 3.2 Out of scope

- Editing `agents.*` or `fallback_model` blocks through `/hero-model`.
- Automatically changing stage YAML properties.
- Rendering or sending future capability keys beyond `fs`, `th`, and `ef` in C5.
- Starting OpenCode at TUI boot solely to preload metadata.
- New harnesses beyond Cursor and OpenCode.
- Cross-harness canonical model IDs or generic provider-specific API semantics.

## 4. Functional Requirements

### 4.1 Selection scope and persistence

1. `/hero-model` MUST continue to select a harness/model pair using the C4 flow.
2. After a model is selected, the TUI MUST open the property picker when at least one supported property is selectable.
3. If no property is selectable, the TUI MUST save the model immediately and skip the property picker.
4. The selected properties MUST apply only to the freechat default and `/hero-new` execution path.
5. Existing `harnesses.<harness>.model` and `freechat_default` fields MUST continue to be updated.
6. User choices MUST be persisted independently for each harness/model pair in an extensible `model_properties` structure in `hero.json`.
7. Existing projects without `model_properties` MUST continue to load without migration failure.
8. Switching models MUST restore the saved choices for the selected pair when those choices are still valid.
9. A choice MUST NOT be persisted partially. Entering the final confirmation saves all choices; Escape cancels the model and all property changes.

### 4.2 Capability and model discovery

1. A harness MAY expose a separate optional capability-discovery contract in addition to `ListModels`.
2. Each adapter MUST normalize its native API response into common metadata containing property key, accepted values, default value, availability, and any adapter-owned mapping information.
3. The TUI MUST NOT depend on Cursor or OpenCode response shapes.
4. When `/hero-model` opens, the TUI MUST immediately use available in-memory cache, persisted cache, or local catalog data.
5. The TUI MUST start background refresh for all enabled harnesses when `/hero-model` opens.
6. OpenCode MUST remain lazy at TUI boot. Opening `/hero-model` is an explicit user action that may start its managed serve process for refresh.
7. A refresh completing while the selector is open MUST be applied on the next selector opening, not by moving rows under the user's current selection.
8. The refresh source precedence MUST be:
   - live harness API;
   - persisted project cache in `hero.db`;
   - local model catalog in `assets/models/*.yml` or its installed projection;
   - unknown capability values represented by `na`.
9. API results MUST replace the cached accepted-value array for a model/property. The API is authoritative when available.
10. A cache MAY be used regardless of age when the API fails. The cache timestamp MUST be retained so stale data can be identified in warnings or diagnostics.
11. If no API, cache, or local catalog is available, the TUI MUST continue without failing silently and MUST show a yellow warning.
12. The local catalog MAY provide model IDs as well as property metadata when live model listing and cache are unavailable.

### 4.3 Property values and defaults

1. Accepted values MUST be dynamic strings. The code MUST NOT hardcode a fixed enum for model values.
2. The local catalog MUST support a per-model property structure containing accepted values and an optional default.
3. Default precedence MUST be live API default, then local catalog default, then `na`.
4. API or catalog refresh MUST preserve an existing user choice when it remains valid.
5. If a previously selected choice is no longer accepted, it MUST become `na` and the user MUST receive a yellow warning.
6. C5 MUST support `fs`, `th`, and `ef`.
7. Future properties MAY be stored in the extensible cache, but unknown keys MUST be ignored by the C5 TUI and execution path.

### 4.4 Property picker behavior

1. The picker MUST use friendly property names:
   - Fast model
   - Thinking
   - Reasoning effort
2. Boolean properties MUST use a checkbox interaction with Space to toggle.
3. Multi-value properties MUST open a secondary value list with Enter, arrow navigation, and Enter to confirm the value.
4. Unsupported properties MUST remain visible, disabled, and gray.
5. The picker MUST clearly display `ENTER to save` in its footer.
6. Enter on the property picker MUST save the complete model/property selection.
7. Escape MUST return without changing the prior harness/model pair or its saved properties.
8. A background capability refresh MUST NOT block selection. The picker uses the best available local/cache data while the refresh is pending.

### 4.5 Execution

1. Chat and `/hero-new` MUST send the selected property values to the selected harness adapter.
2. Cursor MUST continue to use its existing native model/slug composition where applicable.
3. OpenCode MUST serialize properties using the native API keys and value formats mapped by its adapter.
4. Adapter-specific API key mappings MUST remain inside the adapter; the TUI uses only normalized `fs`, `th`, and `ef` keys.
5. If a harness rejects a selected property, execution MUST fail explicitly with the rejected property identified. Hero MUST NOT remove the property or retry silently.
6. Workflow commands such as `/hero-start` MUST continue to use the active agent's `workflow-config.yml` values rather than `freechat_default` properties.

### 4.6 Chat display and warnings

1. When a model is selected, the property line MUST be visible even in an empty Chat.
2. The line MUST appear alongside the response scroll hint and context bar.
3. Narrow terminals MUST wrap the line responsively rather than hide property labels.
4. Labels MUST use the prefixes and value format:
   - `[fs-<value>]`
   - `[th-<value>]`
   - `[ef-<value>]`
5. A configured, supported property MUST be green.
6. An unavailable property, an unconfigured `na` value, or an unvalidated workflow value MUST be gray.
7. The line MUST use freechat properties for ordinary Chat and `/hero-new`, and the effective agent configuration during workflow execution.
8. Warnings MUST use the existing yellow warning style and appear in the same status area used for execution errors.
9. A warning MUST be cleared by the next user action.
10. Missing metadata MUST produce wording equivalent to: `No catalog is available for the selected model. Model properties will use their default values.`

## 5. Data and compatibility constraints

- `hero.json` remains user/project configuration; dynamic model metadata belongs in the operational SQLite store.
- The model metadata cache MUST be project-scoped; there is no global cache.
- C4's Cursor/OpenCode pair selection, session binding, and lazy OpenCode boot behavior remain intact unless explicitly amended during Planning.
- The feature remains TUI/native scope. Cursor IDE Runtime remains Cursor-only and does not consume `model_properties`.
- Cycle artifacts are written in English. Chat follows `workflow_config.user_preferred_language`.

## 6. Verification and acceptance

The implementation is accepted when:

1. A model with all three capabilities can be selected and configured.
2. A model with a partial capability set shows unavailable properties as gray `na` labels.
3. A model without live metadata or local catalog continues with `na` and a yellow warning.
4. A stale cache works when the API is unavailable and produces an appropriate warning.
5. Switching between two models restores each model's independent choices.
6. Enter saves all properties and Escape cancels the complete selection.
7. Chat and `/hero-new` send the selected values to the adapter.
8. Workflow execution displays agent-configured values without changing stage YAML.
9. A harness rejection is surfaced as an explicit execution error.
10. The TUI remains responsive while model/capability refresh runs in background.
11. Existing Cursor and OpenCode behavior remains green under the existing test suite.
12. `go test ./...` passes.

## 7. Reference fixtures

- `opencode-go/deepseek-v4-pro` is the primary real-model fixture for OpenCode capability tests.
- Synthetic fixtures MUST cover a model with all properties, a model with a partial set, and a model with no property metadata.
