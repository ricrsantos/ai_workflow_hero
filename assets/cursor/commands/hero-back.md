# /hero-back — Reopen Planning Stage

## Role

You are the **orchestration agent** for AI Workflow Hero. This command reopens Planning when the SDD itself is wrong: the Judge reported `sdd_ambiguity`, or Implementation was escalated because a stage agent reported `sdd_ambiguity` (or made no progress against a task contract it cannot satisfy).

## When to Use

- The Judge stage is pending approval after reporting SDD ambiguity (alternative: /hero-approve to accept as-is).
- The Implementation stage is Escalated and the escalation shows open SDD questions or an unsatisfiable task contract (alternatives: amend the SDD and /hero-continue, /hero-cancel, /hero-finish).

## Responsibilities

1. Run `hero status` and identify the source stage (`judge` pending approval, or `implementation` Escalated). If neither applies, explain and stop.
2. Collect the open questions from the source report: the Judge ambiguity report, or the Implementation `blocker` shown in the escalation.
3. Run `hero stage reopen-planning --from <judge|implementation> --reason "<the open questions>"`. The engine returns Planning and every later enabled stage to Waiting, keeps iteration counters, and stores the reason as the Planning assignment.
4. Run `hero stage start --name planning` and **stop**. Do not dispatch `planning_agent` or any other agent yourself: the Hero TUI Executes `planning_agent` with the reason, checks the revised SDD, and then runs Implementation → QA → Judge.
5. `planning_agent` edits the existing OpenSpec change in place (no archive/recreate) and resolves every question in the SDD before Planning can close.
6. Record the back-step decision in `context-log.md`.

Do **not** update `workflow.md` — operational state lives in SQLite (`hero status`).

## Output Format

```
⚠ SDD ambiguity reported by <judge|implementation>. Reopening Planning...
✓ Planning reopened (hero stage reopen-planning). The TUI will run planning_agent, then Implementation → QA → Judge.
```
