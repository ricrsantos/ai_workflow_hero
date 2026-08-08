# /hero:approve — Approve Current Stage

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

1. Run `hero status` (or `hero status --json`) to confirm the current stage is pending approval.
2. Apply the **Metrics Procedure** in `orchestration_agent` (model, input/output tokens, cost USD, duration) — never leave token/cost unset for an approved stage.
3. Persist approval and metrics via the CLI (do **not** write `workflow.md` or `metrics.md`):

   ```bash
   hero approve --metrics-json '<JSON>'
   ```

   Optional: `--summary '<short approval note>'`.

   JSON shape: object or array with `stage_name`, `agent`, `model`, `input_tokens`, `output_tokens`, `cost_usd`, `duration_ms` (see **Metrics Procedure** in `orchestration_agent`).
4. The engine advances to the next configured and enabled stage automatically.
5. If no more stages remain, remind the user to run `/hero:finish` (or finish is triggered per stage-close rules in `orchestration_agent`).
6. When the cycle closes, update `context-log.md` and `current-state.md` as needed.

## Stage Flow

Configuration → Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End

## Approval and Control Loop

After approval, the orchestrator advances to the next stage. If the next stage requires human approval, it will wait for /hero:approve, /hero:reject, /hero:cancel, or /hero:finish again.

## Fallback

Fall back to `fallback_model` if configured model is unavailable; warn the user explicitly.

## Output Format

```
✓ <Stage> approved.
→ Metrics: <stage>
  Model: <model_id>
  Input: <input_tokens> tokens | Output: <output_tokens> tokens | Total: <total_tokens> tokens
  Duration: <duration>
  Cost: ~$<cost_usd>
→ Full details: run `hero metrics` (or `hero metrics --json`)
→ Advancing to <Next Stage>...
```
