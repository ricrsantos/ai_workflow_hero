# Tasks — tui-cycle-config

Scope: **native** → all implementation via `generic_agent`.  
Nested generic Task fan-out: `auto` (`agents.generic_agent.subagent`, `same_of_agent: true`).  
Terminology: **etapa** = workflow stage; **interação** = conversational round within an etapa; **managed field** = YAML path owned by Config (PRD-C07-001 §4.2).  
**Compatibility:** do not regress Cursor IDE Runtime, C4/C5/C6 harness Execute, C5 `/hero-model` properties, checksum upgrade, dual-entry, or deterministic engine. Follow golang-tui Elm Architecture on existing `github.com/charmbracelet/bubbletea` (no v2 migration).  
**Divergence:** idea-file reload/merge/cancel dialog is out of scope (ADR-050).

**Parallelism legend:** **PARALLEL** = concurrent Task subagents after deps met; **SERIES** = ordered.

PRD traceability: [PRD-C07-001](../../docs/product/PRD-C07-001-tui-cycle-config.md); ADR: [ADR-C07-001](../../docs/architecture/ADR-C07-001-tui-cycle-config.md); UI: [UI-C07-001](../../docs/product/UI-C07-001-tui-cycle-config.md).

---

## 1. workflowconfig managed document — SERIES (foundation)

Align existing `internal/workflowconfig` Document scaffolding to ADR-050. Do not add a second YAML writer.

- [x] 1.1 Make `Document.Save` (or equivalent) load the latest valid file, apply the TUI draft only to managed paths, validate, and atomically `rename` a same-directory temp file. Do **not** fail Save on revision mismatch. Remove user-facing `ErrExternalChange` / conflict-dialog API. Colocated test: parallel unmanaged edit is kept and managed title from the draft wins (spec `workflow-config-document`; design D2; PRD-C07-001 §4.5)
- [x] 1.2 Golden/round-trip tests: comments, mapping order, `workflow_rules`, unknown keys, and disabled-stage/hidden-agent values survive a managed Save (PRD-C07-001 §4.5 AC7)
- [x] 1.3 `ManagedConfig.Validate` covers every PRD-C07-001 §4.6 gate (empty title/objective/language, enabled-stage budgets `> 0`, implementation scope, frontend gates, required agents, enabled-harness membership, same-harness subagent model). Field-path errors. Capability-unknown and PATH-unavailable are **not** hard failures. Colocated tests (spec `workflow-config-document` R3)
- [x] 1.4 Missing file, unreadable file, and syntactically invalid YAML fail closed: no write, no template create. Colocated tests (PRD-C07-001 §4.5 AC9)
- [x] 1.5 Export `ManagedDiff(before, after)` listing changed managed paths so TUI can enable per-stage Retry. Test: title-only change does not include `stages.qa`; QA budget change does (design D5; PRD-C07-001 §4.8)
- [x] 1.6 Atomic write: validation failure or encode failure leaves the original file bytes unchanged; crash-safe temp+rename. Colocated test with real `t.TempDir()` files

## 2. Engine retry + sync invariants — PARALLEL with §3 after §1

- [x] 2.1 Add `Engine.RetryFailedStage(cycleID, stageName)`: Failed → Waiting only; reject other statuses. Colocated engine tests (spec `failed-stage-retry`; ADR-052)
- [x] 2.2 On retry, reset `Iteration=0`, clear `StartedAt`/`CompletedAt`, and apply current YAML `max_iterations` / `timeout_minutes` / `require_human_approval` / enabled flag for **that** stage. Do not change other stages. Test: Failed QA with stale SQLite budget picks up the just-saved YAML budget (design D5)
- [x] 2.3 Append `store.EventStageRetried` (`stage_retried`); preserve prior events and metrics rows. Test: existing `stage_completed` Failed event and metric row still readable (spec `sqlite-operational-store`; ADR-052)
- [x] 2.4 Confirm `SyncCycleConfigFromWorkflow` still skips Completed and Failed; still updates Waiting/Skipped budgets. Extend existing engine tests (spec `runtime-workflow-execution` R1; ADR-051)
- [x] 2.5 Expose `cycle.Service.RetryFailedStage(name)` wrapping the engine. **No new Cobra command.** Colocated cycle tests (ADR-052)

## 3. TUI navigation — PARALLEL with §2 after §1.1

- [ ] 3.1 Insert Config as second project nav item when an active cycle exists: `Chat | Config | Status | Artifacts | Costs | Events`. Hide Config when no active cycle. Update `nav_sidebar` tests (spec `hero-tui` R1; UI-C07-001 §2)
- [ ] 3.2 Remap shortcuts: Chat `alt+1`, Config `alt+2`, Status `alt+3`, Artifacts `alt+4`, Costs `alt+5`. Events remains palette-reachable. Update existing `alt+2` Status tests. Footer hints stay complete (UI-C07-001 §2)
- [ ] 3.3 `hero chat` / `freeChatMode`: Chat only; Config shortcut is a no-op. Colocated freechat test (PRD-C07-001 AC1)
- [ ] 3.4 Missing/invalid active YAML: Config shows red error with path, parse/read reason, and manual-correction suggestion; does not render the form; does not write a template. Colocated test (UI-C07-001 §2)

## 4. Config screen form — SERIES after §1 and §3

- [ ] 4.1 Add `screenConfig` child model: Init/Update/View, `tea.WindowSizeMsg`, centralized `key.Binding`/`key.Matches`, scrollable viewport, package-level styles, `strings.Builder`, Lip Gloss/ANSI widths. I/O only via `tea.Cmd`. Too-small terminal → centered “window too small”. Colocated render tests (spec `tui-cycle-config` R6; ADR-053; golang-tui skill)
- [ ] 4.2 Identity (title, objective, chat language as non-empty free text), five scope toggles (Space), per-stage enable/purpose/iterations/timeout/approval, Shared/Advanced for orchestration/context/fallback. Tab/Shift+Tab focus. Colocated edit tests (PRD-C07-001 §4.2; UI-C07-001 §§3–4)
- [ ] 4.3 Progressive disclosure: disabled stage shows only toggle + muted “configuration retained”; implementation agents follow scopes; Browser UI visual validation and Playwright require frontend; subagent only when `same_of_agent=false`; stage agents only when their stage is enabled. Hidden values remain in the draft/YAML. Colocated tests (PRD-C07-001 §4.3 AC3–4)
- [x] 4.4 Completed stages render read-only with “completed stage is protected”. Failed stages remain editable. Fixture tests using StatusView/stage list (UI-C07-001 §9)
- [ ] 4.5 Read-only busy during Execute stream, `actionBusy`, and `/hero-start` preflight/bootstrap: muted controls, Save actions disabled, copy “Editing is available when execution/preflight finishes.” Colocated test (PRD-C07-001 AC2)
- [x] 4.6 Visual states: Loading spinner, Ready, Dirty marker, Saving (“Saving configuration…”), Saved (`✓` + cycle number + sync confirmation), validation summary + field errors (no write), Save error (draft kept). Golden/render coverage (UI-C07-001 §7/§12)

## 5. Harness / model / properties — PARALLEL after 4.1 (can overlap remaining §4)

Reuse `internal/modelprops` and `internal/harnessmgr`. Do not persist to `hero.json.model_properties`.

- [ ] 5.1 Per visible agent/fallback: harness picker lists project-enabled harnesses only. Unavailable enabled harness shows yellow `⚠` and is not silently replaced. Colocated tests (PRD-C07-001 §4.4 AC5–6)
- [ ] 5.2 Model picker filters by selected harness using existing catalog/cache; background capability refresh does not block editing. Missing catalog warns and keeps the YAML model. Colocated tests (UI-C07-001 §6)
- [ ] 5.3 Show `fs`/`th`/`ef` only when capability data is known and supported; explicit form values win over later catalog defaults; write `enable_fast_model`/`thinking`/`reasoning_effort` into the YAML agent block. Colocated tests (PRD-C07-001 §4.4)
- [x] 5.4 Nested subagent block only when `same_of_agent=false`; model list restricted to parent harness; validation error if model is known-not-present. Colocated tests (PRD-C07-001 §4.3/§4.6)
- [ ] 5.5 Missing-capability warning copy uses `⚠` and does not block Save. Colocated status test (UI-C07-001 §6/§11)

## 6. Persistence actions — SERIES after §2, §4, and §5

- [ ] 6.1 Save: validate → managed merge write → `SyncCycleConfig` → remain on Config with success copy. Open-stage budgets update; completed stages unchanged. Integration test with temp project + SQLite (spec `runtime-workflow-execution` R1; PRD-C07-001 AC10)
- [x] 6.2 Dirty navigation/quit dialog: Save / Discard / Cancel. Cancel stays on Config with draft. Colocated interaction test (UI-C07-001 §7)
- [ ] 6.3 Save and start: enabled only for valid editable form; performs Save then existing `/hero-start` preflight/Execute path (no duplicated Prepare logic). Busy/read-only disables the action with reason in the action row. Colocated test with fake harness (PRD-C07-001 AC12; UI-C07-001 §8)
- [x] 6.4 After Save, show per-failed-stage Retry only when `ManagedDiff` includes that stage. Confirming Retry calls `RetryFailedStage`, shows `✓` queued copy, and leaves other stages alone. Hidden/disabled until qualifying Save. Colocated TUI + engine test (PRD-C07-001 AC11; UI-C07-001 §9)

## 7. Docs, regression, close — PARALLEL tracks then SERIES lock

### 7A. Architecture overview — PARALLEL

- [ ] 7A.1 Update `docs/architecture/architecture-overview.md`: Config nav, YAML node merge, sync/retry data flow, package map note for workflowconfig Document API (AGENTS.md architecture-overview rule)

### 7B. Cursor IDE + existing TUI regression — PARALLEL

- [ ] 7B.1 `runtime_assets_test.go`: Cursor IDE `/hero-start` and YAML-direct editing unchanged; no Config-screen Runtime asset
- [ ] 7B.2 Existing Chat, Status, Artifacts, Costs, Events, free-chat, `/hero-model`, Cursor/OpenCode/Codex Execute tests stay green; fix only compile/nav-shortcut breaks (UI-C07-001 §12)

### 7C. Context landing — PARALLEL

- [ ] 7C.1 Update `context/current-state.md` and append `context/context-log.md` when implementation lands (pending → implemented Config screen)

### 7D. Integration lock — SERIES after 7A–7C and §6

- [ ] 7D.1 End-to-end temp-dir: load YAML with comments/unknown keys → edit managed fields → external unmanaged edit → Save merge → sync → failed-stage retry → `go test ./...` green
- [ ] 7D.2 Verify `hero cycle openspec-change tui-cycle-config` is persisted on the active cycle (ADR-023)

---

## Parallel groups (orchestrator fan-out)

| Group | When | Tasks | Agent |
|---|---|---|---|
| Foundation | start | §1 SERIES | `generic_agent` |
| A | after 1.1 | 2.1–2.5 | `generic_agent` |
| B | after 1.1 | 3.1–3.4 | `generic_agent` |
| C | after 4.1 | 5.1–5.5 | `generic_agent` |
| D | after §6 | 7A.1, 7B.1–7B.2, 7C.1 | `generic_agent` |

Hard series spine: **1 → (2 ‖ 3) → 4.1 → (4.2–4.6 ‖ 5) → 6 → (7A ‖ 7B ‖ 7C) → 7D**.

§5 may start as soon as 4.1 exists (picker host), in parallel with remaining form work.

**Task count:** 36 checklist items across §1–§7.
