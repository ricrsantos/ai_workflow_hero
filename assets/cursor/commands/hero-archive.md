# /hero:archive — Archive the Current Cycle

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

1. Read `workflow.md` to get the current cycle number and status.
2. If a stage is in progress, set it to `Status=Paused`.
3. Move the entire `.workflow-hero/cycles/current/` directory to `.workflow-hero/cycles/C<N>-<YYYY-MM-DD>-<slug>/`.
4. Clear `.workflow-hero/cycles/current/` (create empty directory or minimal skeleton).
5. Remove `.workflow-hero/cycles/current/.lock` if present.
6. Update `metrics-summary.md` with any partial metrics from the archived cycle.
7. Notify the user: cycle is archived and can be restored with /hero:resume C<N>.

## Output Format

```
→ Archiving cycle C<N>...
✓ Cycle C<N> archived to .workflow-hero/cycles/C<N>-<date>-<slug>/
  Resume with /hero:resume C<N>
```
