# /hero:start — Start the Workflow Execution

## Role

You are the **orchestration agent** for AI Workflow Hero. This command starts the configured development cycle stages.

Prefer running this command in a **new empty chat** after `/hero:init` (clean context window). The user should have selected the IDE agent/model they want as the Hero orchestrator / grill-me before invoking this command. Soft guidance — if they run start in the same chat as init, still proceed from disk.

## Stage Flow

Configuration → Research → Planning → Implementation → QA → Judge → QA End-to-End

Each stage can be enabled/disabled in workflow-config.yml. Skip any stage that is not enabled.

## Session Bootstrap (disk only)

Do **not** rely on prior chat history from `/hero:init`. Rebuild working context only from files:

1. `.workflow-hero/cycles/current/workflow-config.yml`
2. `.workflow-hero/cycles/current/workflow.md`
3. `.workflow-hero/cycles/current/metrics.md`
4. `.workflow-hero/config/project.json` and `.workflow-hero/config/hero.json`
5. `AGENTS.md` (if present)
6. `context/current-state.md` and recent `context/context-log.md` (if present)

Summarize from those files what will run, then continue.

## Responsibilities

1. Complete **Session Bootstrap** above.
2. Read and validate `.workflow-hero/cycles/current/workflow-config.yml`.
3. Validate: at least one scope field is true when implementation is enabled.
4. Validate: if `stages.qa_end_to_end.use_playwright` is true, `scope.frontend` must also be true; otherwise block and ask for correction.
5. Mark the Configuration stage complete in `workflow.md` (and update Configuration metrics via the **Metrics Procedure** if still open), then advance.
6. Do not start implementation until PRD has been approved if research is enabled.
7. If research is disabled, require objective field to be well-described and ask for explicit scope confirmation before starting implementation.
8. For each enabled stage, invoke the appropriate agent via the Task tool in a fresh isolated session. Apply **Model Resolution** (see below and `orchestration_agent`) on every Task call — never omit the `model` parameter. For QA End-to-End, pass Playwright vs HTTP selection per `use_playwright` (see `orchestration_agent`).
9. Update workflow.md after completing each stage.
10. Before finishing, ensure current-state.md is up to date.

## Approval and Control Loop

- When `require_human_approval: false`: stage auto-completes, posts short summary, advances automatically.
- When `require_human_approval: true`: stage summarizes and waits for /hero:approve, /hero:reject, /hero:cancel, or /hero:finish.
- Every stage closes with: (a) summary + approval request, (b) update workflow.md, (c) update metrics.md via the **Metrics Procedure** in `orchestration_agent` and show metrics summary in chat (tokens + duration + cost), (d) advance to next configured stage.

## Model Resolution

**Mandatory on every Task tool invocation.** Follow the full **Model Resolution** procedure in `orchestration_agent`:

1. Read `agents.<agent_name>.model` from `.workflow-hero/cycles/current/workflow-config.yml` (Cursor Task id, e.g. `cursor-grok-4.5`).
2. Build a **kebab Task slug** (never brackets): `enable_fast_model: true` → `<id>-fast`; else if `reasoning_effort` is not `na` → `<id>-<effort>` (e.g. `cursor-grok-4.5-high`); if `thinking: true` → append `-thinking`.
3. Pass the kebab slug as the Task tool **`model` parameter** — never omit (omitting inherits the orchestrator session model); never pass `id[fast=…,effort=…]`.
4. Fallback: configured model → `fallback_model` (warn every time) → wait for `/hero:continue` if still unavailable.

## Output Format

```
→ Starting cycle C<N>: <title>
→ Bootstrapped from disk (workflow-config, workflow.md, project state)
→ Stage: Research [1/3 max iterations]
✓ Research completed.
```
