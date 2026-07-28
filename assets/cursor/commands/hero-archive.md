# /hero:archive — Archive the Current Cycle

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Archive folder date (mandatory)

The archive path is `.workflow-hero/cycles/C<N>-<YYYY-MM-DD>-<slug>/`.

**`<YYYY-MM-DD>` must be the cycle completion date**, not “today” guessed from chat context and not a future date.

Resolution order:

1. Read `.workflow-hero/cycles/current/workflow.md` → header field **`Completed`** (alias **`Completed At`** if present). Use that `YYYY-MM-DD` when it is non-empty.
2. If **`Completed`** is empty and the cycle is already done (`Status` is `Completed` or `Finished by User`), treat that as a bug/legacy cycle: run `date +%Y-%m-%d` in the project shell once, write the result into **`Completed`**, then use it. Warn the user that Completed was missing and was backfilled with the local calendar date of this archive action.
3. If archiving **mid-progress** (`Paused` / in-progress stages): there is no completion date yet — run `date +%Y-%m-%d` in the project shell and use that local calendar date for the folder name only (do not invent a date; do not use IDE/chat “Today's date” text).

**Never** invent or hallucinate the calendar date. Prefer the shell command `date +%Y-%m-%d` (machine local timezone) whenever a date must be obtained at runtime. Never use a date from model memory or injected chat metadata alone.

**Slug**: derive from `workflow-config.yml` → `title` (lowercase, hyphenated).

## Responsibilities

1. Read `workflow.md` to get the current cycle number, status, and **`Completed`** date.
2. If a stage is in progress, set it to `Status=Paused`.
3. Resolve `<YYYY-MM-DD>` using **Archive folder date** above.
4. Move the entire `.workflow-hero/cycles/current/` directory to `.workflow-hero/cycles/C<N>-<YYYY-MM-DD>-<slug>/`.
5. Clear `.workflow-hero/cycles/current/` (create empty directory or minimal skeleton).
6. Remove `.workflow-hero/cycles/current/.lock` if present.
7. Update `metrics-summary.md` with any partial metrics from the archived cycle.
8. Notify the user: cycle is archived (show the exact folder path) and can be restored with /hero:resume C<N>.

## Output Format

```
→ Archiving cycle C<N>...
→ Archive date source: Completed=<YYYY-MM-DD> (from workflow.md | shell date)
✓ Cycle C<N> archived to .workflow-hero/cycles/C<N>-<YYYY-MM-DD>-<slug>/
  Resume with /hero:resume C<N>
```
