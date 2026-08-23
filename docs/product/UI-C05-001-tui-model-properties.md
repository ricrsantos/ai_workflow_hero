# UI-C05-001 — TUI Model Property Selection and Status

Cycle C5 terminal UX for dynamic model properties in `/hero-model` and Chat.

## 1. Scope

This specification extends the C4 harness/model picker. It covers the property picker, metadata loading states, warnings, the active-property line, and responsive behavior. Existing Hero colors, icons, Chat panes, harness selection, and model list conventions remain unchanged.

The first visible property keys are:

- `fs`: fast mode
- `th`: thinking
- `ef`: reasoning effort

## 2. Model picker flow

The existing flow remains:

1. `/hero-model` opens the harness selector when multiple harnesses are enabled.
2. The selected harness shows its model list.
3. Model rows may come immediately from memory, `hero.db`, or the local catalog.
4. Background refresh begins for all enabled harnesses when the picker opens.
5. Selecting a model opens the property picker when at least one property is available.

If no property is selectable, the model is saved immediately. If metadata is still loading, the picker uses the best available cache/catalog values and does not block.

## 3. Property picker

Header:

```text
/hero-model · OpenCode · properties
```

Rows use friendly names and current values:

```text
Fast model: true
Thinking: max
Reasoning effort: high
```

Interaction guidance:

```text
↑↓ navigate · space toggle · enter select · ENTER to save · esc cancel
```

Behavior:

- Boolean properties use Space to toggle.
- Properties with multiple accepted values use Enter to open a secondary list.
- The secondary list uses ↑↓ and Enter to choose a value.
- Unsupported properties remain visible in gray and cannot be edited.
- Enter on the main property picker saves the model and all properties together.
- Esc cancels the complete model/property selection and restores the prior pair.

The selector MUST NOT persist a partial selection.

## 4. Active-property line

When a model is selected, the line below the linear transcript appears beside the context bar. Example:

```text
↑↓ scroll response    [fs-true] [th-max] [ef-high]    ████░░ 12k/200k
```

Rules:

- `fs`, `th`, and `ef` are rendered as prefix-value labels.
- A supported and configured value is green.
- `na`, unavailable properties, and unvalidated workflow values are gray.
- The line is visible in an empty Chat as soon as a model is selected.
- Narrow terminals wrap the line instead of hiding labels or the context bar.
- In normal Chat and `/hero-new`, the line uses the selected `freechat_default` pair.
- During workflow execution, the line uses the active agent's `workflow-config.yml` values.

Examples:

```text
[fs-true] [th-max] [ef-high]
[fs-na] [th-na] [ef-na]
[fs-false] [th-na] [ef-medium]
```

## 5. Warnings and loading states

Warnings use the existing yellow `⚠` semantic style and the same status area used for execution errors. They are not fatal unless the harness rejects a selected property during execution.

Recommended messages:

```text
⚠ No catalog is available for the selected model. Model properties will use their default values.
```

```text
⚠ Using stale model properties because the harness API is unavailable.
```

```text
⚠ The selected value is no longer supported by this model and was reset to na.
```

Warnings clear on the next user action. A refresh completing in background does not reorder the currently open model/property list; the refreshed data is used the next time the selector opens.

## 6. Source behavior visible to the user

The normal interface stays compact. The source of capability data is not displayed as permanent chrome. When the source is not a live API, the status warning explains that cache or local catalog data is being used when that information matters.

The user may still select a model from a local catalog when API and cache are unavailable. The TUI must not fail silently or block the selection solely because capabilities could not be verified.

## 7. Accessibility and terminal behavior

- Meaning MUST not depend only on color; labels and `na` values remain textual.
- `NO_COLOR` and non-TTY behavior continue to use the existing semantic text fallback.
- Labels must wrap at rune-safe boundaries and must not panic with multi-byte icons or model names.
- The property picker must remain navigable with the same arrow, Space, Enter, and Escape conventions used by existing TUI pickers.

## 8. UI verification

Tests and golden fixtures must cover:

- Complete, partial, and absent property metadata.
- Gray disabled rows and green configured labels.
- `ENTER to save` and Escape cancellation.
- Per-model restoration after switching models.
- Warning messages for missing catalog, stale cache, and invalidated values.
- Responsive wrapping beside the scroll/context line.
- Non-color output and `NO_COLOR` behavior.
