# UI-C04-001 — TUI Multi-Harness (Cursor + OpenCode)

> Cycle C4 terminal/TUI UX for Hero **2.0.0**. Extends [UI-C01-001](UI-C01-001-hero-tui.md), [UI-C02-001](UI-C02-001-tui-slash-command-parity.md), [UI-C03-001](UI-C03-001-tui-harness-autonomy.md). Product: [PRD-C04-001](PRD-C04-001-multi-harness.md).

## 1. Scope

- **In**: Install harness picker (no `--tools`); `/hero-harness`; `/hero-model` as harness+model pair; Chat labels with harness; fallback and hard-stop copy; `--tools` error; OpenCode serve/orphan is silent except on failure.
- **Out**: Cursor IDE chat chrome (unchanged); second concurrent TUI; extra TUI screens beyond Chat/palette; visual redesign of existing panes except the speaker/harness label.

## 2. `hero install` harness selection

Replace `--tools`. After git/name prompts:

```text
Hero Installation

Select the AI Harnesses you want to use (at least one):

  [x] Cursor
  [ ] OpenCode

        Continue
```

- List is **supported** harnesses (Cursor, OpenCode), **not** filtered by PATH.
- Zero selected → inline validation, cannot continue.
- OpenCode-only is valid.
- Success line still uses ✓ (UI.md §2.1). Mention which harnesses were enabled.

**`--tools` (install or upgrade):**

```text
✗ Flag --tools is not supported in Hero 2.0.

  Suggestion: run `hero install` and select harnesses interactively,
  or enable them later in the TUI with /hero-harness.

(exit code: 1)
```

Non-interactive name/summary/`--yes` flags remain for **project** fields. Harness choice is interactive in 2.0 (no replacement list flag).

## 3. `/hero-harness`

Palette + Chat `/` overlay item **`/hero-harness`** (execute immediately, like other non-approval slashes).

Show each supported harness as a **checkbox**. Enabled = checked. Availability is in parentheses (PATH/CLI), independent of the checkbox.

```text
Harnesses
  space toggle · enter apply · esc cancel

  [x] Cursor (available)
  [ ] OpenCode (unavailable)
```

- Space toggles the highlighted checkbox (does not save yet).
- Enter applies the selection: enable newly checked (provision projection), disable newly unchecked (files kept).
- Esc cancels without saving.
- Zero checked → inline validation, picker stays open: `Select at least one harness.`
- Enable OpenCode → ✓ `OpenCode enabled (projected .opencode/)`.
- Disable → ✓ `OpenCode disabled (files kept)`.

Unavailable + enabled is allowed; failure happens at Execute (then fallback copy in §6).

## 4. `/hero-model`

Two-step picker. Do **not** pre-select a default model (including `composer-2.5`). The user must choose.

**Step 1 — harness** (only when more than one harness is enabled; instant, no CLI availability checks):

```text
/hero-model · select harness
  Cursor
  OpenCode
```

**Step 2 — models** (live list from the adapter; OpenCode may start serve here):

```text
/hero-model · OpenCode
  anthropic/claude-sonnet-4
  xai/grok-4
```

Esc returns to the harness list. On select: persist pair; Chat input status shows `Build · {model} · {harness}`. Stage agents are not modified.

## 5. Chat green pane (speaker)

Amend UI-C03 speaker line. Include harness:

```text
[ORCH - cursor-grok-4.6 · cursor]
[PLAN - composer-2.5 · cursor]
[QA - anthropic/claude-sonnet-4 · opencode]
[HARN - composer-2.5 · cursor]
```

- Same 4-letter agent codes as the agents box.
- Harness id is lowercase (`cursor`, `opencode`).
- Input status (Build/Plan) also shows harness next to the model.
- Context bar still keys off the **native** model id in `models/*.yml` (OpenCode ids after catalog update).

## 6. Fallback and stop

Reuse UI.md icons.

```text
⚠ Fallback: planning_agent cursor/composer-2.5 unavailable
→ Using fallback_model opencode/anthropic/claude-sonnet-4
```

Hard stop:

```text
✗ Cannot run qa_agent: harness opencode is not available
  Fallback cursor/composer-2.5 also failed.

  Suggestion: install/enable the harness or fix workflow-config.yml,
  then run /hero-continue.
```

Do not ask informal yes/no to pick another harness.

## 7. TUI boot

- If at least one harness is enabled, do **not** require it to be available to enter the TUI (enabled ≠ available). Warn if none of the enabled harnesses are available.
- If none enabled (corrupt config): prompt like install (pick ≥1) or exit with the same error structure.
- OpenCode serve is **not** started at boot. No extra boot chrome for serve. On serve start failure at first Execute, show the adapter error + suggestion.

## 8. Slash table addition

| Action | TUI label |
|---|---|
| Manage harnesses | `/hero-harness` |

Existing `/hero-model` label unchanged; behavior is the pair picker (§4).

## 9. Testing

Golden/unit tests for: `--tools` error text; install “at least one” validation; last-harness disable error; speaker label with harness; `/hero-harness` / `/hero-model` palette presence. `go test ./...` must pass.
