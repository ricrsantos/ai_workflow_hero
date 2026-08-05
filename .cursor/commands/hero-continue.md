# /hero:continue — Grant Extra Iterations

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Usage

```
/hero:continue [N]
```

Where `[N]` is the number of extra iterations to grant (e.g. `/hero:continue 2`). Defaults to 1 if not specified.

## Responsibilities

1. Read `.workflow-hero/cycles/current/workflow.md`.
2. Find the current stage that has `Human Approval = Escalated`.
3. Increment `Extra Iterations Granted` for that stage by N (without altering workflow-config.yml).
4. Update `workflow.md` to reflect the new iteration budget.
5. Resume execution of the current stage with the additional iterations.
6. Update metrics.md after each additional iteration via the **Metrics Procedure** in `orchestration_agent`.

## Iteration and Timeout Handling

Extra iterations are granted per /hero:continue invocation and recorded in workflow.md as `Extra Iterations Granted: +<N>`. The base max_iterations in workflow-config.yml is never modified.

## Output Format

```
→ Granting +<N> extra iteration(s) to <Stage>...
→ Continuing <Stage>: iteration <current>/<max+extra>
```
