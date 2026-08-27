# UI-C07-001 — TUI Cycle Configuration Screen

> Cycle C07 terminal UX for the Hero 2.8 configuration-screen idea. Status: Research complete; pending human approval and Planning. Extends UI-C05 model properties and UI-C06 multi-harness behavior.

## 1. Scope

In scope: a conditional Config screen for the active project cycle, YAML-backed form editing, progressive disclosure, harness/model/property selection, validation, Save, Save and start, dirty-exit confirmation, read-only busy state, and stage-specific failed-stage retry.

Out of scope: free-chat configuration, Cursor IDE UI changes, a global agent configuration in `hero.json`, a new harness protocol, and a second YAML/SQLite source of truth.

## 2. Navigation and availability

The project sidebar becomes:

```text
Chat
Status
Artifacts
Costs
Events
Config
```

Config is appended only when an active cycle exists. `hero chat` continues to show Chat only. Project shortcuts are Chat `alt+1`, Status `alt+2`, Artifacts `alt+3`, Costs `alt+4`, and Events `alt+5`; when Config is visible it is `alt+6`. The navbar footer shows only `alt+1-5` without Config and `alt+1-6` with Config; `alt+6` is a no-op when Config is hidden.

If the active YAML is missing or invalid, Config is not rendered as an editable form. Show a red error state with the path, parse/read reason, and a suggestion to correct the file manually. Never create a replacement file from the template.

## 3. Screen structure

The normal layout is a vertically scrollable form with a persistent header, section labels, field-level feedback, and a bottom action row:

```text
Config · Cycle C07

Identity
  Title
  Objective
  Chat language

Scope
  Backend  Frontend  Native  Script  Infrastructure

Stages
  Research       enabled  purpose  iterations  timeout  approval
    Discover agent  harness  model  properties
  Planning       ...
  Implementation ...
    Backend agent / Frontend agent / Generic agent
  QA             ...
  Judge          ...
  Browser UI validation ...
  QA end-to-end ...

Shared / Advanced
  Orchestration agent
  Context agent
  Fallback model

 [Save]  [Save and start]
```

Use the existing Hero dark palette, semantic icons, warning colors, and border/footer conventions. Form content is lower priority than status and action feedback on narrow terminals.

## 4. Controls and focus

- Text fields use the existing TUI text-input behavior and remain editable until validation.
- Boolean fields use visible toggles; Space changes a toggle.
- Numeric fields accept positive integers and show an inline field error for invalid input.
- Closed choices use keyboard-navigable pickers. Harness selection filters the model picker; model selection filters known properties.
- Tab and Shift+Tab move between focusable controls. Up/Down navigate within lists; Enter selects or activates the focused control; Escape cancels a picker or closes a dialog.
- Save and Save and start are explicit focusable actions. The action row always shows the current enabled/disabled reason when an action is unavailable.
- Help text remains visible in the fixed footer or the existing help overlay; it must include navigation, editing, save, cancel, and retry actions.

The implementation must use centralized key bindings with `key.Matches`, never raw key-string checks scattered through the screen.

## 5. Progressive disclosure

- A disabled stage shows only its toggle and a muted “configuration retained” hint.
- Enabled stages show purpose, budgets, approval, and only their applicable agent blocks.
- Implementation shows agents based on active scopes.
- Browser UI validation exposes visual validation only when enabled and requires frontend scope.
- QA End-to-End exposes Playwright only when frontend scope is active.
- Subagent settings appear only when `same_of_agent` is false and model options are restricted to the parent harness.
- Shared / Advanced contains orchestration, context, and fallback settings rather than duplicating them under stages.

## 6. Harness and model feedback

Harnesses enabled in the project are selectable regardless of PATH availability. An unavailable harness is rendered with a yellow warning. The UI never silently changes the configured harness.

Known model capabilities determine which `fs`, `th`, and `ef` controls are shown. An explicit saved UI choice is styled as configured and is not replaced by a later catalog default. Unknown capability metadata produces a warning and preserves compatible YAML values. Save remains possible; execution availability errors use the existing harness error copy.

## 7. Editing states

The screen has these user-visible states:

- Loading: spinner and “Loading cycle configuration…”.
- Ready: editable form with no dirty marker.
- Dirty: header/action hint indicates unsaved changes.
- Saving: form controls remain visible but actions are disabled; show “Saving configuration…”.
- Saved: green success message names the cycle and confirms synchronization.
- Validation error: red summary plus field-level messages; no file or SQLite write occurs.
- Read-only busy: muted form controls and “Editing is available when execution/preflight finishes.”
- Save error: red message with actionable path/rule details; keep the user's draft in memory.

When leaving a dirty form, show a dialog:

```text
Unsaved configuration changes

  Save     Discard     Cancel
```

External edits do not produce a conflict-choice dialog. Save merges the latest valid YAML and applies TUI values only to managed paths.

## 8. Save and start

Save validates, applies managed node changes, writes atomically, synchronizes the cycle, and remains on Config.

Save and start is enabled only for a valid editable form. It performs the same Save operation first and then enters the existing `/hero-start` preflight and execution flow. Harness preparation, availability checks, session routing, transcript behavior, and cancellation remain owned by the existing TUI flow.

## 9. Completed and failed stages

Completed stages are shown read-only with a muted “completed stage is protected” label. Their YAML values remain visible but cannot be changed through the form.

Failed stages remain editable. After a successful Save changes a failed stage, show:

```text
✓ Configuration saved.
  Retry failed stage: [Retry]
```

Retry is stage-specific and hidden or disabled until that stage has a changed saved configuration. Confirming Retry returns only that stage to Waiting, resets its next-attempt counters, and preserves prior events and metrics. Save and start can then execute the new attempt.

## 10. Responsive behavior

- Store dimensions from `tea.WindowSizeMsg` and account for sidebar, borders, padding, status, and footer rows.
- Use a scrollable viewport for long stage/agent forms.
- Hide or collapse lower-priority descriptions before hiding validation and action feedback.
- Use ANSI-aware width measurement and truncation; never use byte length for rendered layout.
- If the terminal is too small to operate meaningfully, show a centered “window too small” message with the minimum guidance rather than clipping Save or errors.
- All file, catalog, SQLite, and preflight work runs asynchronously through Bubble Tea commands; `Update` must never block.

## 11. Messages

Follow the shared semantic convention:

- `✓` success: saved, synchronized, retry queued.
- `⚠` warning: unavailable harness, missing capabilities, preserved unknown model property.
- `✗` error: invalid YAML, validation failure, atomic write failure, sync failure, retry failure.
- `→` progress: loading, refreshing capabilities, saving, starting preflight.

Errors identify the field/path and the violated rule. Suggestions must tell the user whether to edit the form, install/enable a harness, or correct the YAML manually.

## 12. Testing UX

- Screen navigation verifies conditional Config visibility and shortcut order.
- Form tests verify progressive disclosure, focus movement, toggle/numeric/text editing, validation feedback, and responsive clipping behavior.
- Golden/render tests cover ready, dirty, saving, error, read-only, unavailable-harness, missing-capability, completed-stage, and failed-stage states.
- Interaction tests cover Save, Save and start, dirty exit, external managed-field merge, and stage-specific Retry.
- Existing Chat, Status, Artifacts, Costs, Events, free-chat, Cursor, OpenCode, and Codex flows must retain their current behavior.
