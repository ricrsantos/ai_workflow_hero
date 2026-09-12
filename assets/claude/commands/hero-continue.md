# /hero-continue — Grant Extra Iterations

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Usage

```
/hero-continue [N]
```

Where `[N]` is the number of extra iterations to grant (e.g. `/hero-continue 2`). Defaults to 1 if not specified.

## Responsibilities

1. Run `hero status` and confirm the current stage shows **Escalated** (max iterations exhausted).
2. Grant extra iterations via the CLI (do **not** edit `workflow.md`):

   ```bash
   hero continue --extra N
   ```

   Omit `N` to use the default (`--extra 1`).
3. Resume execution of the current stage with the additional iterations.
4. On each subsequent stage close, pass metrics via `--metrics-json` on the appropriate mutating command per **Metrics Procedure** in `orchestration_agent`.

## Iteration and Timeout Handling

Extra iterations are granted per /hero-continue invocation and recorded in SQLite by the engine. The base `max_iterations` in `workflow-config.yml` is never modified.


## Escalation triage (PRD-C15-001 §8)

After granting iterations with `/hero-continue`, remind the user they may also run `/hero-add-todo` to defer selected findings to project ToDos. Partial deferral keeps the loop Escalated until remaining blockers are resolved or every blocker is deferred (terminal `completed_with_deferred_todos` closure).

## Output Format

```
→ Granting +<N> extra iteration(s) to <Stage>...
✓ Granted +<N> extra iteration(s). (hero continue)
→ Continuing <Stage>: iteration <current>/<max+extra> (hero status)
```
