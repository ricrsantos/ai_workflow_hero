# /hero-resume — Resume an Archived or Cancelled Cycle

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Usage

```
/hero-resume [cycle]
```

Where `[cycle]` is the cycle number (e.g. `4` for C4). Omit to resume the latest non-archived cycle.

## Responsibilities

1. If another cycle is active, run `hero status` and warn — archive or finish the current cycle first when appropriate.
2. Resume via the CLI:

   ```bash
   hero cycle resume
   ```

   Or target a specific cycle:

   ```bash
   hero cycle resume --number N
   ```

3. Run `hero status` to show the restored cycle and paused/current stage.
4. Notify the user: cycle resumed; run /hero-start or the appropriate approval command to continue.

Do **not** rely on `workflow.md` for resume state — SQLite is the source of truth.

## Output Format

```
→ Resuming cycle C<N>...
✓ Cycle resumed. (hero cycle resume)
→ Paused at stage: <stage> (hero status)
  Run /hero-start to continue, or /hero-approve / /hero-reject as appropriate.
```
