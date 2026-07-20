# /hero:approve — Approve Current Stage

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

1. Check the current stage in `.workflow-hero/cycles/current/workflow.md`.
2. Set the current stage `Human Approval` to `Approved` and `Status` to `Completed`.
3. Update `workflow.md` with the approved status.
4. Update `metrics.md` with the final iteration count for the stage.
5. Advance to the next configured and enabled stage automatically.
6. If no more stages remain, finalize the cycle (update context-log.md, current-state.md).

## Stage Flow

Configuration → Research → Planning → Implementation → QA → Judge → QA End-to-End

## Approval and Control Loop

After approval, the orchestrator advances to the next stage. If the next stage requires human approval, it will wait for /hero:approve, /hero:reject, /hero:cancel, or /hero:finish again.

## Fallback

Fall back to `generic_model` if configured model is unavailable; warn the user explicitly.

## Output Format

```
✓ <Stage> approved.
→ Advancing to <Next Stage>...
```
