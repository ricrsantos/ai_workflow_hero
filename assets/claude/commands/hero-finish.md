# /hero-finish — Finish and Close the Current Cycle

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

1. Run `hero status` and validate that all required stages are completed or disabled.
2. Persist cycle completion via the CLI (do **not** write `workflow.md` or `metrics.md`):

   ```bash
   hero finish
   ```

   The store records `completed_at` (used by `hero cycle archive` for the archive folder date).
3. Optionally update `metrics-summary.md` (project-wide) with aggregated totals from `hero metrics`.
4. Update `context-log.md` with a summary of decisions and outcomes for the cycle.
5. Update `current-state.md` to reflect the new project state after this cycle.
6. Notify the user that the cycle is complete; show completion via `hero status` and remind them to run **`/hero-archive`** when ready (OpenSpec archive runs first when linked; folder date from store `completed_at`).

## Emergency finish vs deferred closure

`/hero-finish` remains available for emergency cycle termination. When open or reopened findings still exist, require strong confirmation that those findings will **not** become deferred ToDos. This is distinct from `/hero-add-todo` deferring every blocker, which closes with disposition `completed_with_deferred_todos` without treating the cycle as a normal validated completion.

## Metrics

The TUI records stage metrics from harness usage and prices them from `.workflow-hero/models/*.yml`. Do not compute or pass them; query totals with `hero metrics`.

## Output Format

```
✓ Cycle C<N> completed successfully.
→ Completion recorded in SQLite (hero finish).
→ Metrics summary: run `hero metrics`
→ Project totals: [.workflow-hero/metrics-summary.md](.workflow-hero/metrics-summary.md) (if maintained)
→ Archive with /hero-archive when ready (folder date from store completed_at)
```
