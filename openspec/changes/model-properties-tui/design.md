## Context

Cycle C4 leaves the TUI with a persisted `(harness, native model)` freechat pair and a harness registry. `internal/tui/model_picker.go` currently stops after the model row is selected, `internal/harness` has `ModelLister` but no capability contract, and `internal/store` is at schema v4 with only the OpenCode serve registry as its C4 cache-like structure. `hero.json` contains harness defaults and `freechat_default`, but no per-model properties. `contextbar.go` renders one right-aligned scroll/context line below the response pane.

The approved sources are authoritative: [PRD-C05-001](../../docs/product/PRD-C05-001-model-properties-tui.md), [UI-C05-001](../../docs/product/UI-C05-001-tui-model-properties.md), and [ADR-C05-001](../../docs/architecture/ADR-C05-001-model-properties-tui.md). Scope is native only, so implementation is assigned to `generic_agent`; Browser UI Validation and QA End-to-End are disabled.

## Goals and Non-Goals

### Goals

- Add optional capability discovery without breaking `ListModels` or adapters that cannot expose metadata.
- Make API/cache/catalog resolution deterministic and project-scoped.
- Persist independent, string-valued choices per harness/native model pair.
- Give `/hero-model` a non-blocking, cancelable property-selection flow.
- Send normalized properties through real Chat and `/hero-new` executions while keeping provider protocol code in adapters.
- Project effective properties into a compact, color-semantic, responsive Chat status line.

### Non-Goals

- No fixed enum of thinking or effort values; only the C5 keys are fixed.
- No changes to Cursor IDE Runtime prompts, cycle YAML authoring, additional harnesses, or a Hero daemon.
- No automatic retry, stripping, or mutation after an adapter rejects a property.

## Design

### 1. Normalized harness contract

Extend `internal/harness` without changing the existing `ModelLister` interface:

```go
const (
    PropertyFast   = "fs"
    PropertyThink  = "th"
    PropertyEffort = "ef"
)

type PropertyCapability struct {
    Key            string
    AcceptedValues []string
    DefaultValue   string
    Available      bool
}

type ModelCapabilities struct {
    HarnessID string
    ModelID   string
    Properties []PropertyCapability
    RetrievedAt time.Time
}

type ModelPropertyDiscoverer interface {
    DiscoverModelProperties(ctx context.Context, modelID string) (ModelCapabilities, error)
}
```

The actual implementation may use project naming conventions, but the contract must preserve these semantics. `ListModels` remains independent and optional capability support is detected through a type assertion. Adapter-native response parsing and mapping references stay inside each adapter; the TUI and cache consume only normalized fields. Unknown property keys may be retained in normalized/cache/JSON structures but are filtered out of C5 rendering and execution.

Add `Properties map[string]string` to `harness.ExecuteRequest`. The map is normalized (`fs`, `th`, `ef`) and copied at the request boundary so adapters cannot mutate TUI state. `na` is an effective-display sentinel and is omitted from freechat transport; workflow values are marked unvalidated for display but remain available to the adapter when configured in YAML.

### 2. Model-property service and source resolution

Add a small native vertical slice, planned as `internal/modelprops`, that owns:

- normalized snapshots for one harness/model;
- embedded and installed catalog loading;
- SQLite cache reads/writes;
- selection validation/defaulting and invalid-choice reconciliation;
- background refresh orchestration and warning/source metadata.

The service exposes a synchronous snapshot for immediate UI use and a background refresh command. Snapshot precedence is strict:

1. successful live harness model/capability API;
2. project cache in `.workflow-hero/hero.db`;
3. embedded `assets/models/*.yml`, overlaid by installed `.workflow-hero/models/*.yml`;
4. an unknown snapshot with the visible C5 keys represented as `na`.

Live API data is authoritative: a successful response replaces the cached accepted-value arrays and default for the affected model/property. A cache is usable at any age after API failure, and its persisted timestamp is retained. The resolver reports that it is using stale data when a failed live refresh falls back to cache; it does not invent an age threshold. If every source is absent, the selected model remains usable and the service emits the exact missing-catalog warning required by UI-C05-001 §5.

When `/hero-model` opens, the service immediately returns the best local/cache snapshot and launches refresh work for every enabled harness. The refresh includes model IDs where the adapter can list them and capabilities where it implements `ModelPropertyDiscoverer`. Cursor's missing capability API is a normal fallback. OpenCode is not touched during `newModel` or TUI boot; its managed serve process may start only as part of the explicit picker refresh. Refresh completion is stored and delivered with a generation/pending marker, but the current `paletteItems` and property draft are not replaced. The next `/hero-model` opening reloads the snapshot.

### 3. Catalog schema and cache

The existing pricing YAML remains backward compatible. A model entry may add an optional property block:

```yaml
models:
  example/model:
    input: 1.0
    output: 2.0
    properties:
      fs:
        available: true
        values: ["true", "false"]
        default: "false"
      th:
        available: true
        values: ["off", "max"]
        default: "off"
      ef:
        available: true
        values: ["medium", "high"]
        default: "medium"
```

`values` and `default` are strings; the parser does not define a fixed value enum. Model map keys are valid local model IDs, so a catalog can provide model rows when live listing and cache are unavailable. The C5 catalog loader accepts absent pricing fields for capability-only fixtures and ignores malformed/unknown property entries without panicking.

Migration v5 adds project-scoped cache tables (names may follow package conventions) for normalized model lists and per-harness/model capabilities. Each capability record stores normalized JSON and an RFC3339 refresh timestamp. No native provider response or global path is stored. Store helpers provide upsert/read/list operations and make API replacement atomic at the row level. Existing v4 databases migrate on open; existing projects without cache rows continue normally.

### 4. `hero.json` persistence and selection transaction

Extend `internal/install.HeroJSON` with an optional, extensible shape:

```json
{
  "model_properties": {
    "opencode": {
      "opencode-go/deepseek-v4-pro": {
        "fs": "true",
        "th": "max",
        "ef": "high"
      }
    }
  }
}
```

The Go type is a nested map keyed by harness and native model ID with string values. Missing `model_properties` unmarshals as empty and never causes a migration failure. Existing `harnesses.<harness>.model`, `freechat_default`, `cli.tools`, and legacy `enable_fast_model` fields remain readable. For backward compatibility, a legacy enabled fast flag may seed the effective `fs` value when no new pair entry exists; new commits use the model-property map and do not write stage configuration.

The picker keeps a complete in-memory draft containing the old pair and all property values. It performs no persistence while rows are toggled or a secondary value list is open. Final Enter calls one install-layer selection helper that writes the pair and the complete property map together (using a temporary file plus rename or equivalent atomic write). Escape discards the draft and restores the prior in-memory pair without touching disk. A model with no selectable property commits only the model pair through the same final-save path.

On draft construction, saved values are restored only when accepted by the current capability snapshot. A removed value is represented as `na`, produces a yellow warning, and is never sent as a freechat property. Defaults resolve live API → local catalog → `na`; a valid user value wins over a refreshed default. Switching between models changes only the draft and restores each pair's independent saved values.

### 5. TUI picker state machine

Retain C4's harness submenu and model submenu. After a model row is selected:

1. Build a property snapshot from memory/cache/catalog without waiting for refresh.
2. If no C5 property has `Available == true` and an editable value list, atomically save the pair and return to the prior Chat screen.
3. Otherwise enter a property screen with header `/hero-model · <Harness> · properties`, stable friendly rows `Fast model`, `Thinking`, and `Reasoning effort`, and current values.
4. Use Space for an available boolean (`fs`) and Enter to open a multi-value list for `th`/`ef` (or any future multi-value key exposed by the normalized contract). The secondary list uses the existing arrow/Enter conventions.
5. Keep unavailable rows visible, disabled, and gray. Show `ENTER to save` in the main footer. Main Enter commits the complete draft; Escape from either level cancels the complete model/property selection.

Refresh messages never move the active rows or replace the draft. The user can finish with local/cache data while refresh is pending; the next picker opening sees any completed refresh.

### 6. Effective-property projection and warnings

Add TUI state for the selected freechat property map, current capability metadata, workflow projection, and a yellow warning separate from red execution errors. `renderConversationResponse` will use a status-line renderer that can return multiple rune-safe lines rather than allowing `renderScrollHintLine` to hide the right side when the terminal is narrow.

The stable output order is `fs`, `th`, `ef`, with labels `[fs-<value>]`, `[th-<value>]`, and `[ef-<value>]`. A validated configured `th`/`ef` value is green; `na`, unavailable, and unvalidated values are gray. `fs` is green only for validated `true`, and gray for `false`, `na`, unavailable, or unvalidated. Textual values remain present in no-color output so meaning never depends only on ANSI color.

The status line is below the green response box beside `↑↓ scroll response` and the context bar, including when Chat is empty after a model is selected. At narrow widths it wraps the scroll hint, all property labels, and the context bar onto rune-safe lines instead of hiding a component. The context bar keeps its existing model-window lookup behavior.

Missing catalog, stale-cache fallback, and invalidated-value messages use the existing yellow `⚠` semantic style in the status area. Use the documented wording, including `No catalog is available for the selected model. Model properties will use their default values.` A background completion does not reorder an open selector. The warning is cleared by the next user action; a property rejection remains a red execution error and is not cleared or retried silently.

### 7. Adapter transport and workflow authority

`resolveExecuteResolution` gains a normalized property projection:

- ordinary Chat and `/hero-new`: read the selected `freechat_default` pair and its `model_properties` entry;
- active workflow/runtime command: derive `fs` from `enable_fast_model`, `th` from `thinking`, and `ef` from `reasoning_effort` in the active agent block/fallback resolution; do not read or write the freechat map;
- C4 harness/model/session fallback and session-binding rules remain unchanged.

Cursor receives the normalized values and continues its existing native model/slug composition, with composition kept in `internal/adapters/cursor`. OpenCode converts the normalized map to the native HTTP option keys/value formats in `internal/adapters/opencode`; the TUI never constructs a provider payload. Unknown future keys are ignored by the C5 execution projection.

If an adapter rejects a selected property, it returns an explicit property-aware error (for example, `property "ef" rejected by opencode for model "..."`). The TUI displays that error in the existing execution-error path. It must not remove the choice, silently retry without it, or turn a property rejection into an unrelated harness fallback.

### 8. Verification strategy

- Unit tests use real `t.TempDir()` files, real embedded `assets.FS` where relevant, and injectable HTTP/process/discovery dependencies already used by adapters.
- Store tests cover v4→v5 migration, cache timestamps, API replacement, stale fallback, and project isolation.
- Catalog tests cover complete, partial, absent, capability-only, dynamic-value, and unknown-key fixtures.
- TUI golden/interaction tests cover picker headers/footer, checkbox/secondary list, disabled rows, save/cancel, warnings, per-model restoration, color semantics, narrow wrapping, multi-byte content, and no-color output.
- Adapter contract tests cover `opencode-go/deepseek-v4-pro`, Cursor slug preservation, native OpenCode serialization, rejection, and unchanged C4 behavior.
- Final verification runs `go test ./...`; no Browser UI or browser end-to-end task is created.

## Dependency and Parallelism Rationale

The normalized contract is the only hard foundation. Catalog parsing, SQLite migration, hero.json types, Cursor transport, OpenCode transport, and workflow projection can then proceed in parallel because each has an explicit boundary. Resolution and refresh compose the catalog/cache/JSON tracks. TUI integration consumes the resulting snapshot/commit APIs; picker, status rendering, and warning styling are independent once the TUI state hook exists. Execution routing and integration tests are serialized after the full picker/adapter contract so they test the actual end-to-end behavior rather than mocks of an unfinished API.
