# /hero:status — Show Current Cycle Status

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

1. Run `hero status` in the project shell (add `--json` when structured output is needed).
2. Display the CLI table (or format the JSON) showing cycle number, title, cycle status, and per-stage rows: name, status, iteration, human approval.
3. If the CLI reports no active cycle, tell the user: "No active cycle. Run /hero:new to start."

Do **not** read `.workflow-hero/cycles/current/workflow.md` for operational status — SQLite (via `hero status`) is the source of truth.

## Allowed Field Values

Status and human-approval values come from the engine/CLI output (e.g. Waiting, Running, PendingApproval, Completed, Escalated, Failed, Cancelled).

## Output Format

Relay the `hero status` table in chat. When using `--json`, summarize the same fields in a readable table for the user.
