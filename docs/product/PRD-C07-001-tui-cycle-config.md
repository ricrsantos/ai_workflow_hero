# PRD-C07-001 — TUI Cycle Configuration Screen

> Cycle C07 research specification for the Hero 2.8 configuration-screen idea. Status: Research complete; pending human approval and Planning.

## 1. Summary

Hero TUI shall provide a guided Config screen for editing the active cycle's `.workflow-hero/cycles/current/workflow-config.yml`. The YAML remains the single source of truth and remains directly editable. The screen reduces manual configuration work without creating a second cycle configuration or changing the Cursor IDE Runtime.

The feature extends the existing Bubble Tea navigation, `internal/workflowconfig` round-trip document handling, model/property discovery, cycle synchronization, and deterministic engine state transitions.

## 2. User and problem

The primary user is a developer configuring a Hero cycle in the terminal before or between agent executions. The current workflow requires editing a large YAML file manually, including harness/model pairs, model properties, stage budgets, scope gates, and agent fan-out settings. Manual edits are error-prone and make it difficult to understand which settings are relevant to an enabled stage.

## 3. Goals

- Provide a guided editor for the active cycle configuration.
- Keep `workflow-config.yml` as the only canonical configuration.
- Preserve comments, mapping order, `workflow_rules`, unknown keys, and future fields.
- Reuse existing harness/model/property catalogs and capability snapshots.
- Synchronize saved cycle metadata and still-open stage budgets with SQLite.
- Permit a deliberate retry of a failed stage after its configuration is corrected.
- Keep Cursor IDE configuration and `/hero-start` behavior unchanged.

## 4. Scope

### 4.1 Config screen availability

- Add Config as the second project TUI navigation item: `Chat | Config | Status | Artifacts | Costs | Events`.
- Show Config only when an active cycle exists.
- Never show Config in `hero chat` free-chat mode.
- Permit editing when no agent is executing and no `/hero-start` preflight is running.
- Render read-only state while any agent or preflight is active; explain that saving is available after execution finishes.

### 4.2 Managed fields

The screen owns these fields and only these fields:

- `title` and `objective`.
- `workflow_config.user_preferred_language`, as non-empty free text.
- The five `scope` booleans: `backend`, `frontend`, `native`, `script`, and `infrastructure`.
- Every stage's `enabled`, `purpose`, `max_iterations`, `timeout_minutes`, and `require_human_approval`.
- `browser_ui_validation.visual_validation.enabled` and `reference_dir`.
- `qa_end_to_end.use_playwright`.
- Agent and fallback `harness`, `model`, `enable_fast_model`, `thinking`, `reasoning_effort`, and nested `subagent` settings.

Stage budgets must be positive integers for every enabled stage. The Config screen follows the existing document validator and requires `timeout_minutes > 0`; it does not introduce the engine's lower-level zero-timeout behavior into the workflow-config schema.

### 4.3 Progressive disclosure

- A disabled stage shows only its toggle. Its stored values remain in YAML and reappear when re-enabled.
- Implementation agent controls are shown only for active implementation scopes: `backend_agent` for backend, `frontend_agent` for frontend, and `generic_agent` for native/script/infrastructure.
- Stage-specific agents appear only when their stage is enabled.
- `orchestration_agent`, `context_agent`, and `fallback_model` appear under Shared / Advanced.
- Browser UI Validation requires `scope.frontend`; when enabled it reveals visual validation and `browser_ui_agent`.
- `qa_end_to_end.use_playwright` requires `scope.frontend`.
- A nested `subagent` block is shown only when `same_of_agent` is false. Its model list is restricted to the parent agent's harness.

### 4.4 Harness, model, and property selection

Each visible agent/fallback selection follows the sequence enabled harness → native model for that harness → supported model properties.

- Harness choices are limited to project-enabled harnesses.
- Unavailable enabled harnesses are displayed with a warning and are never silently replaced.
- Model choices reuse the existing catalog/cache and background capability refresh.
- `fs`, `th`, and `ef` use normalized C5 capability data. Unsupported controls are hidden when capability data is known.
- A valid explicit UI property choice takes precedence over catalog or harness defaults.
- These cycle choices are written to the relevant YAML agent block, not to `hero.json.model_properties`.
- Missing catalog/capability data does not block selection; existing compatible YAML values are preserved and the UI warns.

### 4.5 Persistence and merge behavior

The screen loads the YAML as a `yaml.Node` document plus a managed form model. Saving:

1. Reads the current file and verifies that it is present and syntactically valid.
2. Applies the current form only to managed YAML paths.
3. Preserves comments, order, `workflow_rules`, unknown keys, and all other current-file content.
4. Validates the complete managed configuration.
5. Writes a temporary file in the same directory and atomically renames it into place.

If another process edits the file while the screen is open, there is no conflict dialog and no automatic full-file overwrite. The TUI draft prevails on managed fields; the latest file prevails for unmanaged content. A missing or invalid file makes Config unavailable and provides an actionable manual-correction error.

### 4.6 Validation

Save must fail without writing a partial file when any of these rules fail:

- `title` and `objective` are non-empty.
- Every enabled stage has positive `max_iterations` and `timeout_minutes`.
- An enabled implementation stage has at least one active scope.
- Browser UI Validation is not enabled without frontend scope.
- Playwright E2E is not enabled without frontend scope.
- Required visible agent/fallback blocks exist, with a non-empty harness and model.
- A subagent model belongs to the same harness as its parent agent.
- Known capability snapshots accept explicit properties.

Harness availability and missing metadata are warnings rather than silent substitutions. Save remains possible; Save and start follows the existing preflight and reports execution availability failures explicitly.

### 4.7 Cycle synchronization

Every successful Save calls the existing cycle configuration synchronization path. It updates the active cycle's title, objective, configuration snapshot, and still-open stage budgets/flags. Completed stages are not changed by synchronization. Failed stages remain failed until the explicit retry action is used.

### 4.8 Failed-stage retry

After a successful Save changes the configuration of a failed stage, that stage exposes an explicit `Retry failed stage` action. The action:

- Targets only the selected failed stage.
- Does not alter completed stages, other open stages, or unrelated cycle state.
- Returns the stage to `Waiting`.
- Resets the new attempt's iteration and timeout counters.
- Preserves the prior attempt's event history and metrics.

Retry is not enabled for an unchanged failed configuration. Save and start can execute the requeued stage through the normal `/hero-start` flow.

### 4.9 Save actions and navigation

- `Save` validates, writes, synchronizes, and leaves the user on Config.
- `Save and start` is available only for a valid editable configuration, saves first, then follows the same preflight and execution path as `/hero-start`.
- While read-only or busy, save actions are unavailable.
- Leaving or changing screens with unsaved changes prompts with Save, Discard, or Cancel.

### 4.10 Compatibility and non-goals

- Cursor IDE YAML editing and `/hero-start` remain unchanged.
- The screen does not create global agent settings in `hero.json`.
- Harness adapter protocol contracts do not change.
- The CLI does not gain LLM reasoning or a second configuration source.
- No concurrent editing mode is added; only managed-field precedence is defined.

## 5. Acceptance criteria

1. Config appears only with an active cycle and never in free chat.
2. Config is read-only during agent execution and preflight.
3. Disabled stages and irrelevant agents hide without losing YAML values.
4. Scope toggles reveal exactly the corresponding implementation agents.
5. Harness → model → property selection filters and persists to the correct YAML blocks.
6. Missing capability metadata produces a visible warning and does not silently change values.
7. Save preserves comments, order, `workflow_rules`, and unknown keys.
8. Parallel edits are merged with TUI precedence only for managed fields.
9. Missing or invalid YAML cannot be overwritten by the screen.
10. Save synchronizes active-cycle metadata and open-stage budgets while leaving completed stages unchanged.
11. Retry is explicit, stage-specific, requires a changed saved configuration, resets attempt counters, and preserves history/metrics.
12. Save and start uses the existing `/hero-start` preflight and execution flow.
13. Cursor IDE workflow behavior remains unchanged.
14. `go test ./...` remains green.

## 6. Proposed implementation slices

- `internal/tui`: Config screen model, navigation, form focus, progressive disclosure, status/read-only states, save dialogs, retry action, and async commands.
- `internal/workflowconfig`: managed document state, targeted node mutations, latest-file merge, validation, atomic write, and managed-field diffing.
- `internal/modelprops` and `internal/harnessmgr`: reuse catalogs, cache, capability snapshots, and enabled-harness warnings.
- `internal/cycle` / `internal/engine`: invoke synchronization after save and add explicit failed-stage requeue/retry semantics.
- `internal/architecture overview`: document the new TUI and data flow after the ADR is approved.

## 7. Testing requirements

- Round-trip golden tests for comments, order, unknown keys, `workflow_rules`, and managed-field merge.
- Validation tests for every semantic gate and field-specific error.
- TUI tests for conditional navigation, responsive rendering, read-only/busy states, dirty-exit confirmation, save errors, save success, and retry state transitions.
- Harness/model/property tests for filtering, same-harness subagents, unavailable harnesses, missing capability metadata, and explicit property precedence.
- Engine/cycle tests for synchronization, completed-stage protection, failed-stage retry, counter reset, and metrics/event preservation.
- Integration coverage with real temporary files and the existing test dependencies.

