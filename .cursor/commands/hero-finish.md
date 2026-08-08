# /hero-finish — Finish and Close the Current Cycle

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

1. Run `hero status` and validate that all required stages are completed or disabled.
2. Apply the **Metrics Procedure** in `orchestration_agent` for any open stage metrics.
3. Persist cycle completion via the CLI (do **not** write `workflow.md` or `metrics.md`):

   ```bash
   hero finish --metrics-json '<JSON>'
   ```

   The store records `completed_at` (used by `hero cycle archive` for the archive folder date).
4. Optionally update `metrics-summary.md` (project-wide) with aggregated totals from `hero metrics`.
5. Update `context-log.md` with a summary of decisions and outcomes for the cycle.
6. Update `current-state.md` to reflect the new project state after this cycle.
7. Notify the user that the cycle is complete; show completion via `hero status` and remind them to run **`/hero-archive`** when ready (OpenSpec archive runs first when linked; folder date from store `completed_at`).

## Metrics

Compute metrics per **Metrics Procedure** in `orchestration_agent` (chars÷4, model rates from `.workflow-hero/models/*.yml`). Pass the payload via `--metrics-json`; query totals with `hero metrics`.

## Output Format

```
✓ Cycle C<N> completed successfully.
→ Completion recorded in SQLite (hero finish).
→ Metrics summary: run `hero metrics`
→ Project totals: [.workflow-hero/metrics-summary.md](.workflow-hero/metrics-summary.md) (if maintained)
→ Archive with /hero-archive when ready (folder date from store completed_at)
```
