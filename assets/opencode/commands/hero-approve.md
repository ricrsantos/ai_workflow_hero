# /hero-approve — Approve Current Stage

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

1. Run `hero status` (or `hero status --json`) to confirm the current stage is pending approval.
2. Persist the approval via the CLI (do **not** write `workflow.md` or `metrics.md`):

   ```bash
   hero approve
   ```

   Optional: `--summary '<short approval note>'`.

3. The engine advances to the next configured and enabled stage automatically.
4. If no more stages remain, remind the user to run `/hero-finish` (or finish is triggered per stage-close rules in `orchestration_agent`).
5. When the cycle closes, update `context-log.md` and `current-state.md` as needed.

## Stage Flow

Configuration → Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End

## Approval and Control Loop

After approval, the orchestrator advances to the next stage. If the next stage requires human approval, it will wait for /hero-approve, /hero-reject, /hero-cancel, or /hero-finish again.

## Fallback

Fall back to `fallback_model` if configured model is unavailable; warn the user explicitly.

## Output Format

```
✓ <Stage> approved.→ Advancing to <Next Stage>...
```
