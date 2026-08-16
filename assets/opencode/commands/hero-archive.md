# /hero-archive — Archive the Current Cycle

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Archive folder date (mandatory)

The archive path is `.workflow-hero/cycles/archive/C<N>-<YYYY-MM-DD>-<slug>/` (exact path returned by the CLI).

**`<YYYY-MM-DD>` is the cycle completion date** from SQLite (`completed_at` set when `hero finish` runs), not “today” guessed from chat context and not a future date.

**Never** invent or hallucinate the calendar date. The `hero cycle archive` command resolves the folder date from the operational store. For mid-progress archives, the CLI may use the current local date when `completed_at` is empty — do not substitute model memory or chat metadata.

**Slug**: derived from cycle `title` in the store / `workflow-config.yml` (lowercase, hyphenated).

## OpenSpec archive (first)

When the cycle has a linked OpenSpec change, archive it **before** the Hero filesystem archive. The CLI orchestrates this; agents invoke `hero cycle archive` (do not hand-roll folder moves).

**Name resolution** (CLI applies this order):

1. Stored `openspec_change` on the cycle (set during Planning via `hero cycle openspec-change <name>`).
2. Else scan `openspec/changes/*` (exclude `archive/`): **0** → skip OpenSpec step; **1** → use that directory name; **N** → fail closed until the user sets the name (`hero cycle openspec-change <name>`) or passes `--openspec-change <name>` on archive.

**Sequence:**

1. Run `hero status` to confirm cycle number and state (include `openspec_change` when using `--json`).
2. Archive via the CLI (do **not** read or write `workflow.md` for archive semantics):

   ```bash
   hero cycle archive
   ```

   Optional override when multiple active changes exist: `hero cycle archive --openspec-change <name>`.

   The CLI runs `openspec archive <name> -y` first (merge delta specs), then performs the Hero archive on success or when OpenSpec is skipped.

3. Relay the CLI success message (includes archive directory path).
4. Optionally update `metrics-summary.md` with totals from `hero metrics` for the archived cycle.
5. Notify the user: cycle is archived (show the exact folder path) and can be restored with `/hero-resume`.

## OpenSpec failure — force path

If OpenSpec archive fails (missing `openspec` binary, non-zero exit, or unresolved change name), the CLI **does not** archive Hero unless forced. Tell the user clearly and offer:

1. **Retry** — fix the OpenSpec issue, then run `/hero-archive` again (or `hero cycle archive`).
2. **Force Hero archive** — after explicit user consent, run:

   ```bash
   hero cycle archive --force
   ```

   (`--skip-openspec` is an alias for `--force`.)

3. **Manual OpenSpec** — archive the change yourself, then force or retry:

   ```bash
   openspec archive <name> -y
   ```

**User-facing failure template** (adapt wording; keep all points):

```text
✗ OpenSpec archive failed: <reason>
→ Options: retry /hero-archive | force Hero archive (--force) and archive OpenSpec manually
  Manual: openspec archive <name> -y
```

Do **not** proceed with `--force` without the user choosing that path.

## Output Format

**Success:**

```
→ Archiving cycle C<N>...
→ OpenSpec: openspec archive <name> -y (or skipped when no active change)
✓ Cycle C<N> archived to <path from hero cycle archive>
  Resume with /hero-resume C<N>
```

**OpenSpec failure (before force):**

```
✗ OpenSpec archive failed: <reason>
→ Options: retry /hero-archive | force Hero archive (hero cycle archive --force) and archive OpenSpec manually
  Manual: openspec archive <name> -y
```
