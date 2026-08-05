# /hero:finish — Finish and Close the Current Cycle

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

1. Read `workflow.md` and validate that all required stages are either completed or disabled.
2. Set the cycle's overall **Status** to `Completed` in `workflow.md`.
3. Set **`Completed`** (header field) to today's **local calendar date** by running `date +%Y-%m-%d` in the project shell. Write that exact `YYYY-MM-DD` into `workflow.md`. Do **not** invent the date from chat context or model memory — this value is what `/hero:archive` must use in the archive folder name.
4. Update `metrics.md` with grand totals across all stages (follow **Metrics Procedure** in `orchestration_agent`).
5. Update `metrics-summary.md` (project-wide) with the cycle's aggregated totals.
6. Update `context-log.md` with a summary of decisions and outcomes for the cycle.
7. Update `current-state.md` to reflect the new project state after this cycle.
8. Remove `.workflow-hero/cycles/current/.lock`.
9. Notify the user that the cycle is complete, show the `Completed` date written, and remind them that `/hero:archive` (or a later `/hero:new`) will name the folder with that completion date.

## Metrics

Before writing grand totals, ensure every completed stage row has numeric Input/Output/Cost (not `—`). If any completed stage still has placeholders, re-apply the Metrics Procedure:

1. Use each stage's recorded model and any available `input_chars` / `output_chars` (or re-estimate from cycle artifacts and chat).
2. `tokens = round(chars / 4)`.
3. Look up rates in `.workflow-hero/models/<provider>.yml` (`unit: per_1m_tokens`).
4. `cost_usd = (input_tokens * input_rate + output_tokens * output_rate) / 1_000_000`.
5. Sum all stage costs into Grand Total; append one cycle row to `.workflow-hero/metrics-summary.md`.

## Output Format

```
✓ Cycle C<N> completed successfully.
→ Completed date: <YYYY-MM-DD> (from date +%Y-%m-%d)
→ Metrics summary: [.workflow-hero/metrics-summary.md](.workflow-hero/metrics-summary.md)
→ Cycle details: [.workflow-hero/cycles/current/metrics.md](.workflow-hero/cycles/current/metrics.md)
→ Archive with /hero:archive when ready (folder date = Completed above)
```
