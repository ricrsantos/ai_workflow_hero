# /hero:reject — Reject Current Stage

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

1. Check the current stage in `.workflow-hero/cycles/current/workflow.md`.
2. Set the current stage `Human Approval` to `Rejected`.
3. Re-run the current stage, passing the rejection reason to the responsible agent.
4. Each retry consumes one iteration from `max_iterations`.
5. If `max_iterations` is exhausted, set `Human Approval` to `Escalated` and wait for /hero:continue.
6. Update `workflow.md` after every rejection.

## Approval and Control Loop

Rejection re-triggers the current stage agent with the user's feedback. The stage returns for another approval cycle.

## Iteration and Timeout Handling

On exhaustion of max_iterations, escalate to the user (Human Approval = Escalated) and wait for /hero:continue specifying extra iterations.

## Output Format

```
⚠ <Stage> rejected. Re-running with feedback...
→ Iteration <N>/<max>
```
