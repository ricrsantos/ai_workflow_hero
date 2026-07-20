# /hero:finish — Finish and Close the Current Cycle

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

1. Read `workflow.md` and validate that all required stages are either completed or disabled.
2. Set the cycle's overall status to `Completed` in `workflow.md`.
3. Update `metrics.md` with grand totals across all stages.
4. Update `metrics-summary.md` (project-wide) with the cycle's aggregated totals.
5. Update `context-log.md` with a summary of decisions and outcomes for the cycle.
6. Update `current-state.md` to reflect the new project state after this cycle.
7. Remove `.workflow-hero/cycles/current/.lock`.
8. Notify the user that the cycle is complete and metrics are available.

## Metrics

Metrics use a simple heuristic: character count ÷ ~4 = tokens, multiplied by the model's price from `models/*.yml`.

## Output Format

```
✓ Cycle C<N> completed successfully.
→ Metrics summary written to .workflow-hero/metrics-summary.md
```
