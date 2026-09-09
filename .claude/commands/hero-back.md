# /hero-back — Reopen Planning Stage

## Role

You are the **orchestration agent** for AI Workflow Hero. This command reopens Planning when the Judge identifies ambiguity in the SDD itself.

## When to Use

Only invokable during the Judge stage when the judge_agent identifies that the failure is due to ambiguity in the SDD (not an implementation gap). The orchestrator will have presented the user with a choice between /hero-back and /hero-approve (accept as-is).

## Responsibilities

1. Run `hero status` to confirm Judge stage context.
2. Orchestrate a Planning reopen (no dedicated `hero back` CLI verb): reset Planning to in progress and Implementation / QA / Judge to waiting in the engine by driving stage execution — use `hero reject` with an explicit SDD-ambiguity reason when the current stage is pending approval, or advance via the normal stage-close path after Judge, then re-dispatch Planning.
3. Invoke `planning_agent` via the Task tool (fresh isolated session) with the ambiguity report from `judge_agent`.
   - Apply **Model Resolution** from `orchestration_agent`: pass Task `model` as a kebab slug from `workflow-config.yml` → `agents.planning_agent` (`enable_fast_model` → `<id>-fast`; never omit `model`; never use brackets).
4. `planning_agent` edits the existing OpenSpec proposal in place (preserving change history — no archive/recreate).
5. After the planning_agent completes, re-run Implementation → QA → Judge from scratch (every Task call still applies Model Resolution for the target agent). Persist each stage close via `hero` CLI with `--metrics-json` per **Metrics Procedure**.
6. Record the back-step decision in `context-log.md`.

Do **not** update `workflow.md` — operational state lives in SQLite (`hero status`).

## Fallback

Fall back to `fallback_model` if configured model is unavailable; warn the user explicitly (ADR-008 / Model Resolution).

## Output Format

```
⚠ Judge identified SDD ambiguity. Reopening Planning...
→ Invoking planning_agent to resolve ambiguity...
✓ Planning updated. Re-running: Implementation → QA → Judge.
```
