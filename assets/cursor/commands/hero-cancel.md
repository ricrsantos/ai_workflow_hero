# /hero:cancel — Cancel Current Stage

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

1. Read the current stage from `.workflow-hero/cycles/current/workflow.md`.
2. Set the current stage `Status` to `Cancelled` and `Human Approval` to `Cancelled`.
3. Use `git checkout` / `git restore` to roll back file changes made during this stage to the last committed checkpoint.
4. Update `workflow.md` with `Status=Cancelled`.
5. Notify the user: stage is cancelled and changes are rolled back.
6. Wait for the user's next instruction (/hero:start to retry, /hero:archive to archive the cycle, or other commands).

## Git Dependency

Cancellation relies on git for reliable rollback of all file changes (ADR-004). The working directory must be a git repository with a clean commit at the stage start.

## Output Format

```
⚠ <Stage> cancelled. Rolling back changes via git...
✓ Changes rolled back to checkpoint <commit-sha>.
```
