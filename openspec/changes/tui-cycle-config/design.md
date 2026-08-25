## Context

Hero already treats `.workflow-hero/cycles/current/workflow-config.yml` as the cycle configuration, exposes it to Cursor IDE Runtime, and syncs title/objective/open-stage budgets into SQLite via `SyncCycleConfigFromWorkflow`. The TUI navigation is `Chat | Status | Artifacts | Costs | Events`. C5 already discovers harness/model capabilities through `internal/modelprops` and `internal/harnessmgr`. Failed stages cannot be started again (`StartStage` rejects `Failed`).

In-progress scaffolding in `internal/workflowconfig` (`Document`, `ManagedConfig`, `Write`, `Reapply`) is the starting point, not a greenfield package. It currently rejects a revision mismatch (`ErrExternalChange`). Approved research requires the opposite: **no conflict dialog**; Save loads the latest valid file and applies the TUI draft only to managed paths (ADR-050).

Authoritative sources: [PRD-C07-001](../../docs/product/PRD-C07-001-tui-cycle-config.md), [UI-C07-001](../../docs/product/UI-C07-001-tui-cycle-config.md), [ADR-C07-001](../../docs/architecture/ADR-C07-001-tui-cycle-config.md). Scope is **native only** → `generic_agent`. Follow golang-tui Elm Architecture on the existing `github.com/charmbracelet/bubbletea` import; do not migrate to Bubble Tea v2.

## Goals / Non-Goals

**Goals:**

- Guided Config editor over the active YAML document (ADR-049).
- Round-trip-safe managed-node merge (ADR-050).
- Existing cycle sync as the YAML/SQLite boundary (ADR-051).
- Explicit failed-stage retry (ADR-052).
- Non-blocking TUI I/O (ADR-053).

**Non-Goals:**

- Second config store; `hero.json` agent settings; Cursor IDE UI changes.
- Idea-file reload/merge/cancel dialog.
- New Cobra command for retry.
- Bubble Tea v2 migration; harness protocol changes.

## Decisions

### D1 — Config is a TUI editor, not a second configuration (ADR-049)

Config is a project-TUI screen available only when `GetActiveCycle` succeeds. It never appears in `hero chat`. It never writes cycle configuration to `hero.json` or a TUI-only state file. Missing or invalid YAML → error state with path + reason + “correct the file manually”; never materialize a template replacement.

Navigation after C7:

```text
Chat     alt+1
Config   alt+2   (hidden without active cycle)
Status   alt+3
Artifacts alt+4
Costs    alt+5
Events   palette (when hints cannot show a sixth shortcut)
```

### D2 — Managed-node merge is the only Save path (ADR-050)

Keep a `yaml.Node` tree plus `ManagedConfig` in `internal/workflowconfig`. Save:

1. Read the current file. If missing or not a valid mapping, fail closed (draft stays in memory; file untouched).
2. Decode latest unmanaged content from that file.
3. Mutate **only** managed paths from the TUI draft.
4. `ManagedConfig.Validate`.
5. Write a same-directory temp file and `rename` over the target.

Managed paths (PRD-C07-001 §4.2): `title`, `objective`, `workflow_config.user_preferred_language`, `scope.*`, each canonical stage’s `enabled`/`purpose`/`max_iterations`/`timeout_minutes`/`require_human_approval`, `stages.browser_ui_validation.visual_validation.{enabled,reference_dir}`, `stages.qa_end_to_end.use_playwright`, each known agent’s `harness`/`model`/`enable_fast_model`/`thinking`/`reasoning_effort`/`subagent.*`, and `fallback_model.*`.

Unmanaged: comments, key order, `workflow_rules`, unknown keys, extra agent/stage fields.

**Supersedes idea-file conflict UX.** Do not prompt reload/merge/cancel. Do not fail Save solely because the hash changed. Align `Document.Write` to this merge (today’s `Reapply` semantics become the default Save). `ErrExternalChange` is not part of the user-facing flow.

Disabled-stage and hidden-agent values remain in YAML; the form simply does not show them.

### D3 — Validation stays in workflowconfig (PRD-C07-001 §4.6)

Save fails with field-path errors and writes nothing when:

- `title` or `objective` is empty.
- `user_preferred_language` is empty.
- An enabled stage has `max_iterations <= 0` or `timeout_minutes <= 0` (document schema; do not import engine zero-timeout behavior).
- Implementation is enabled and all five scopes are false.
- Browser UI Validation is enabled without `scope.frontend`.
- `qa_end_to_end.use_playwright` is true without `scope.frontend`.
- A required visible agent/fallback lacks non-empty harness and model.
- Selected harness is not in the project-enabled list (UI cannot pick disabled harnesses).
- `same_of_agent=false` and subagent model is empty or not in the parent harness catalog when that catalog is known.

Warnings (Save still allowed): enabled harness unavailable on PATH/auth; missing catalog/capability snapshot; preserved compatible YAML property the UI cannot confirm.

Availability failures for **execution** remain `/hero-start` preflight’s job.

### D4 — Sync remains the YAML/SQLite boundary (ADR-051)

After a successful atomic write, TUI calls `cycle.Service.SyncCycleConfig()` (existing `Engine.SyncCycleConfigFromWorkflow`). Sync updates title, objective, config snapshot, and still-open stage budgets/flags (`Waiting`/`Skipped`; running/pending/escalated follow current sync rules). Completed and Failed stages are not rewritten by sync.

TUI does not issue raw SQLite stage updates for ordinary Save.

### D5 — Failed-stage retry is an engine transition (ADR-052)

Add `Engine.RetryFailedStage(cycleID, stageName)` exposed as `cycle.Service.RetryFailedStage(stageName)`. No new Cobra command.

Preconditions:

- Named stage exists and status is `Failed`.
- Caller (TUI) only enables the action after a successful Save whose managed diff includes that stage (`stages.<name>.*` and/or that stage’s required agent block(s)). Document API exposes `ManagedDiff(before, after) []path`.

Effects:

- Status → `Waiting`.
- Reset next-attempt counters: `Iteration = 0`, `StartedAt = ""`, `CompletedAt = ""` (clears timeout wall-clock).
- Apply current YAML budgets/flags for that stage (`max_iterations`, `timeout_minutes`, `require_human_approval`, enabled→Waiting vs skipped).
- Do not modify other stages, cycle meta (already synced), events, or metrics rows.
- Append event `stage_retried` with `{"stage":"<name>"}`.
- Keep `HarnessSessionID` / `HarnessID` as-is (next Execute uses existing session-binding rules).

Reject retry for Completed, Running, Waiting, Escalated, Skipped, or PendingApproval.

### D6 — Config is an Elm Architecture child screen (ADR-053 + golang-tui)

Implement as a child model (e.g. `configScreen`) routed from `internal/tui` `screenConfig`. Follow existing app value-model style **or** a pointer child that returns `tea.Cmd` only; do not mix both in one component. Constraints:

- Handle `tea.WindowSizeMsg`; store width/height; account for sidebar, borders, padding, status, footer (`GetHorizontalFrameSize` / `GetVerticalFrameSize`).
- Centralized `key.Binding` + `key.Matches` (Tab/Shift+Tab, Space toggle, Up/Down in lists, Enter activate, Esc cancel picker/dialog). No scattered `msg.String()` checks unique to Config.
- All file, catalog, SQLite, retry, and `/hero-start` work in `tea.Cmd`; `Update` never blocks.
- `View` uses `strings.Builder`; widths via Lip Gloss/ANSI helpers, never `len()` on rendered strings.
- Styles live at package level, not inside `View`.
- Scrollable viewport for the form; hide descriptions before validation/actions; if too small to operate, centered “window too small” instead of clipping Save/errors.
- Semantic copy: `✓` / `⚠` / `✗` / `→` per UI-C07-001 §11.

States: Loading, Ready, Dirty, Saving, Saved, Validation error, Read-only busy, Save error (UI-C07-001 §7). Dirty leave → dialog Save / Discard / Cancel.

Read-only busy when `actionBusy`, streaming Execute, or `/hero-start` bootstrap/prepare is active. Explain: “Editing is available when execution/preflight finishes.”

Completed stages: visible, muted “completed stage is protected”, not editable. Failed stages: editable; Retry shown only after a qualifying Save.

### D7 — Harness → model → properties reuse C5 (PRD-C07-001 §4.4)

For each visible agent/fallback:

1. Harness picker = project-enabled harnesses (`hero.json`). Unavailable enabled harness: yellow warning, never silent replace.
2. Model picker = catalog/cache/ListModels for that harness (existing modelprops snapshot + background refresh). Missing catalog: warn, keep current YAML model.
3. Property controls `fs`/`th`/`ef` from normalized capabilities. Hide unsupported controls when capability data is **known**. When unknown: warn and keep compatible YAML (`false`/`na`).
4. Explicit form choice wins over later catalog defaults.
5. Persist to the YAML agent/fallback block. Never write `hero.json.model_properties`.
6. Subagent shown only when `same_of_agent=false`; models restricted to parent harness. Subagent has no independent harness field.

Shared / Advanced: `orchestration_agent`, `context_agent`, `fallback_model`.

Implementation agents: `backend_agent` iff `scope.backend`; `frontend_agent` iff `scope.frontend`; `generic_agent` iff native|script|infrastructure.

### D8 — Save and start reuses `/hero-start` (PRD-C07-001 §4.9)

`Save` → validate → merge-write → `SyncCycleConfig` → stay on Config with `✓ Configuration saved.` (include cycle number).

`Save and start` enabled only when the form is valid and editable (not busy/read-only). Same Save, then invoke the existing TUI `/hero-start` command path (preflight, Prepare, transcript, cancellation). Do not duplicate preflight logic.

### D9 — Cursor IDE compatibility

No Cursor Runtime asset, slash, or template behavior change except comments if a template example is already allowed. `internal/common/runtime_assets_test.go` must keep Cursor IDE `/hero-start` and YAML-direct editing semantics.

## Data flow

```text
Config screen (draft ManagedConfig)
        │ tea.Cmd load / save / retry / start
        ▼
internal/workflowconfig.Document
        │ yaml.Node + managed projection
        ▼
.workflow-hero/cycles/current/workflow-config.yml
        │ SyncCycleConfig (after successful write)
        ▼
SQLite cycle + still-open stages
        │ RetryFailedStage (explicit)
        ▼
Failed stage → Waiting (counters reset; events/metrics kept)
        │ Save and start
        ▼
existing /hero-start preflight + Execute
```

## Risks / Trade-offs

- **WIP Document API vs ADR-050:** current `Write` revision lock will break approved Save. Align first; tests that expect `ErrExternalChange` must become merge tests.
- **yaml.v3 comment fidelity:** marshal may not preserve every comment style. Golden tests define the acceptable round-trip; prefer in-place node mutation over full re-encode when comments would be lost.
- **Retry vs sync:** sync skips Failed; retry must apply the just-saved YAML budgets or the requeued stage would keep stale SQLite limits.
- **Shortcut remap:** existing `alt+2` Status tests must move to `alt+3`.

## Migration / compatibility

No SQLite schema bump. No `hero.json` migration. Existing cycles with valid YAML load as-is. Invalid YAML is not overwritten.

## Open questions (none)

Research closed the conflict-dialog question (ADR-050). Retry has no CLI command (ADR-052). Timeout validation stays `> 0` at the document layer (PRD-C07-001 §4.2).
