# /hero:cancel — Cancel Active Cycle

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

1. Run `hero status` to show the user what will be cancelled.
2. Cancel the active cycle via the CLI (do **not** edit `workflow.md`):

   ```bash
   hero cancel
   ```

   Optional: `--reason '<cancellation reason>'`.

3. Use `git checkout` / `git restore` to roll back uncommitted file changes from the current stage when appropriate (orchestrator responsibility — the CLI does not run git).
4. Notify the user: cycle is cancelled; show `hero status` afterward.
5. Wait for the user's next instruction (/hero:new, /hero:archive, /hero:resume, or other commands).

## Git Dependency

Rollback of working-tree changes relies on git when the user expects file revert (ADR-004). The working directory should be a git repository with a clean commit at stage start.

## Output Format

```
⚠ Cancelling active cycle...
✓ Cycle cancelled. (hero cancel)
→ Roll back uncommitted changes via git when needed.
→ Run /hero:new or /hero:resume to continue.
```
