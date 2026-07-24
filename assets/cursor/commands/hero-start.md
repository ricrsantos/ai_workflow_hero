# /hero:start — Start the Workflow Execution

## Role

You are the **orchestration agent** for AI Workflow Hero. This command starts the configured development cycle stages.

## Stage Flow

Configuration → Research → Planning → Implementation → QA → Judge → QA End-to-End

Each stage can be enabled/disabled in workflow-config.yml. Skip any stage that is not enabled.

## Responsibilities

1. Read `.workflow-hero/cycles/current/workflow-config.yml`.
2. Validate: at least one scope field is true when implementation is enabled.
3. Validate: if `stages.qa_end_to_end.use_playwright` is true, `scope.frontend` must also be true; otherwise block and ask for correction.
4. Do not start implementation until PRD has been approved if research is enabled.
5. If research is disabled, require objective field to be well-described and ask for explicit scope confirmation before starting implementation.
6. For each enabled stage, invoke the appropriate agent via the Task tool in a fresh isolated session. Apply **Model Resolution** (see below and `orchestration_agent`) on every Task call — never omit the `model` parameter. For QA End-to-End, pass Playwright vs HTTP selection per `use_playwright` (see `orchestration_agent`).
7. Update workflow.md after completing each stage.
8. Before finishing, ensure current-state.md is up to date.

## Approval and Control Loop

- When `require_human_approval: false`: stage auto-completes, posts short summary, advances automatically.
- When `require_human_approval: true`: stage summarizes and waits for /hero:approve, /hero:reject, /hero:cancel, or /hero:finish.
- Every stage closes with: (a) summary + approval request, (b) update workflow.md, (c) update metrics.md via the **Metrics Procedure** in `orchestration_agent` and show metrics summary in chat (tokens + duration + cost), (d) advance to next configured stage.

## Model Resolution

**Mandatory on every Task tool invocation.** Follow the full **Model Resolution** procedure in `orchestration_agent`:

1. Read `agents.<agent_name>.model` from `.workflow-hero/cycles/current/workflow-config.yml`.
2. Apply `enable_fast_model` as `[fast=true]` or `[fast=false]`; append `effort=<value>` when `reasoning_effort` is not `na`; append `thinking=<value>` when `thinking` is not `na`.
3. Pass the result as the Task tool **`model` parameter** — never omit (omitting inherits the orchestrator session model).
4. Fallback: configured model → `fallback_model` (warn every time) → wait for `/hero:continue` if still unavailable.

## Output Format

```
→ Starting cycle C<N>: <title>
→ Stage: Research [1/3 max iterations]
✓ Research completed.
```
