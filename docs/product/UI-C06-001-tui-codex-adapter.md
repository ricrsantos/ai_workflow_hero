# UI-C06-001 — TUI Codex Adapter (Hero 2.5.0)

> Cycle C6 terminal/TUI UX for Hero **2.5.0**. Extends [UI-C04-001](UI-C04-001-tui-multi-harness.md) and [UI-C05-001](UI-C05-001-tui-model-properties.md). Product: [PRD-C06-001](PRD-C06-001-codex-adapter.md).

## 1. Scope

- **In**: Codex as a third supported harness in install and `/hero-harness`; `/hero-model` pair when Codex is enabled; Chat speaker harness id `codex`; auth / CLI / app-server error copy; doctor warn when Codex enabled but CLI missing; `/harness-reset` lists Codex.
- **Out**: Cursor IDE chat chrome (unchanged); login browser inside TUI; dump-all Codex events on the green pane; extra TUI screens; visual redesign beyond adding the Codex row/label.

## 2. `hero install` harness selection

Same prompt as UI-C04-001 §2, with a third checkbox:

```text
Hero Installation

Select the AI Harnesses you want to use (at least one):

  [x] Cursor
  [ ] OpenCode
  [ ] Codex

        Continue
```

- List is **supported** harnesses (Cursor, OpenCode, Codex), **not** filtered by PATH.
- Zero selected → inline validation, cannot continue.
- Codex-only is valid (same as OpenCode-only).
- Success line uses ✓ (UI.md §2.1) and names which harnesses were enabled.

`--tools` error text is unchanged (Hero 2.0).

## 3. `/hero-harness`

Same picker mechanics as UI-C04-001 §3. Add Codex:

```text
Harnesses
  space toggle · enter apply · esc cancel

  [x] Cursor (available)
  [ ] OpenCode (unavailable)
  [ ] Codex (unavailable)
```

- Enable Codex → ✓ `Codex enabled (projected .codex/)`.
- Disable → ✓ `Codex disabled (files kept)`.
- Last-harness guard unchanged.
- Unavailable + enabled is allowed; failure happens at Execute.

## 4. `/hero-model`

Two-step picker unchanged. When Codex is enabled, step 1 includes it:

```text
/hero-model · select harness
  Cursor
  OpenCode
  Codex
```

Step 2 lists **native** Codex ids from the adapter (may start app-server, analog to OpenCode serve). Esc returns to the harness list. Stage agents are not modified.

C5 property picker still runs after model select when properties are selectable.

## 5. Chat green pane (speaker)

Harness id is lowercase `codex`:

```text
[ORCH - gpt-5.4 · codex]
[PLAN - composer-2.5 · cursor]
[QA - anthropic/claude-sonnet-4 · opencode]
[HARN - gpt-5.4 · codex]
```

- Same 4-letter agent codes as the agents box.
- Input status (Build/Plan) shows harness next to the model.
- Context bar keys off the **native** Codex id in `models/*.yml` after the catalog task. Missing window → hide bar (existing rule).

Unknown App Server events: yellow warning in the existing status area **only when Hero runs with `hero --debug`** (OpenCode unknown-event analog). Do **not** print raw JSON-RPC dumps as the default assistant transcript. Without `--debug`, unrecognized methods are logged at debug level only.

## 6. Auth, availability, and process errors

Reuse UI.md icons. Do not prompt for an API key.

Not authenticated:

```text
✗ Codex is not authenticated.

  Suggestion: run `codex login` in a terminal, then retry.
```

CLI missing:

```text
✗ Cannot run planning_agent: harness codex is not available
  codex CLI not found on PATH.

  Suggestion: install the Codex CLI, enable it with /hero-harness,
  then run /hero-continue.
```

App-server failed to start / died / handshake incompatible:

```text
✗ Codex app-server failed to start (incompatible or not installed).

  Suggestion: verify `codex` on PATH (`codex --version`) and retry.
  Hero does not pin a Codex CLI version.
```

Fallback and hard-stop templates stay UI-C04-001 §6 (name harness `codex` when that pair fails).

`/hero-start` probe failure (Prepare analog): same pattern as OpenCode — tell the user to exit the TUI, run `hero` again, and retry `/hero-start`.

## 7. TUI boot and `/harness-reset`

- Codex app-server is **not** started at boot. Enabled ≠ available. Warn if none of the enabled harnesses are available (existing boot rule).
- `/harness-reset` picker includes Codex when it is enabled. Stop Hero-managed app-server; if not started, yellow warn (not error), same as OpenCode.

## 8. Doctor

When `harnesses.codex.enabled` is true and `codex` is not on PATH, warn-only (OpenCode analog):

```text
⚠ opencode-cli … (unchanged)
⚠ codex-cli  Codex CLI not on PATH — Codex harness will be unavailable until installed
```

No extra doctor failure when Codex is disabled.

## 9. Slash table

No new slash. Existing `/hero-harness`, `/hero-model`, `/harness-reset` gain Codex rows/behavior.

## 10. Testing

Golden/unit tests for: install checkbox includes Codex; enable provisions `.codex/`; disable keeps files; speaker label `· codex`; unauthenticated error copy; unknown event warning; last-harness still cannot be disabled. `go test ./...` must pass.
