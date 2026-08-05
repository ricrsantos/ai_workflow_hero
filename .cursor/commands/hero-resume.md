# /hero:resume — Resume an Archived Cycle

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Usage

```
/hero:resume [cycle]
```

Where `[cycle]` is the cycle identifier (e.g. `C04` or `C04-2026-07-20-checkout-flow`).

## Responsibilities

1. Locate the archived cycle directory matching the provided identifier in `.workflow-hero/cycles/`.
2. If `.workflow-hero/cycles/current/` has active content (a .lock file or non-empty workflow.md), warn and abort — archive the current cycle first with /hero:archive.
3. Move the archived cycle directory back to `.workflow-hero/cycles/current/`.
4. Recreate `.workflow-hero/cycles/current/.lock`.
5. Read the restored `workflow.md` to determine which stage was paused.
6. Notify the user: cycle resumed; the paused stage is shown and the user can run /hero:start or the appropriate command to continue.

## Output Format

```
→ Resuming cycle C<N>...
✓ Cycle C<N> restored to current. Paused at stage: <stage>.
  Run /hero:start to continue, or /hero:approve / /hero:reject as appropriate.
```
