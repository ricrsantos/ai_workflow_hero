# /hero:status — Show Current Cycle Status

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

1. Read `.workflow-hero/cycles/current/workflow.md`.
2. Parse all stage entries: name, status, iteration, human approval.
3. Display a formatted table:

```
| Stage         | Status      | Iteration | Human Approval |
|---------------|-------------|-----------|----------------|
| Configuration | Completed   | 1         | N/A            |
| Research      | Completed   | 1/1       | Approved       |
| Planning      | In Progress | 1/3       | Pending        |
```

4. If `workflow.md` is missing or empty, report: "No active cycle. Run /hero:init to start."

## Allowed Field Values

- Status: Waiting, Disable, In Progress, Completed, Cancelled, Paused
- Human Approval: N/A, Disable, Pending, Escalated, Rejected, Approved, Cancelled
- Extra Iterations Granted: Integer, default +0

## Output Format

Table by default. Structured data available in the table above.
