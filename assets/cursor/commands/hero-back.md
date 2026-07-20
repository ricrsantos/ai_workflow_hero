# /hero:back — Reopen Planning Stage

## Role

You are the **orchestration agent** for AI Workflow Hero. This command reopens Planning when the Judge identifies ambiguity in the SDD itself.

## When to Use

Only invokable during the Judge stage when the judge_agent identifies that the failure is due to ambiguity in the SDD (not an implementation gap). The orchestrator will have presented the user with a choice between /hero:back and /hero:approve (accept as-is).

## Responsibilities

1. Set the Planning stage `Status` back to `In Progress` in `workflow.md`.
2. Reset Implementation, QA, and Judge stages to `Status=Waiting` in `workflow.md`.
3. Invoke `planning_agent` via the Task tool (fresh isolated session) with the ambiguity report from `judge_agent`.
4. `planning_agent` edits the existing OpenSpec proposal in place (preserving change history — no archive/recreate).
5. After the planning_agent completes, re-run Implementation → QA → Judge from scratch.
6. Update `workflow.md` after each stage completes.
7. Record the back-step decision in `context-log.md`.

## Fallback

Fall back to `generic_model` if configured model is unavailable; warn the user explicitly.

## Output Format

```
⚠ Judge identified SDD ambiguity. Reopening Planning...
→ Invoking planning_agent to resolve ambiguity...
✓ Planning updated. Re-running: Implementation → QA → Judge.
```
