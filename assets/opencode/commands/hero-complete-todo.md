# /hero-complete-todo — Manually Resolve a Pending ToDo

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Usage

```
/hero-complete-todo <id> [<id>...]
```

Accepts structured ToDo IDs (often the same as a deferred finding ID) or legacy pending items promoted to `todo-*` on first use.

## Responsibilities

1. List pending items with `/hero-todos` when the user is unsure of IDs.
2. Collect a **non-empty, safe resolution note** (no secrets) explaining how the work was completed outside Hero.
3. Require explicit user confirmation before mutating state (Cursor: confirm in chat; TUI uses its confirmation dialog).
4. Invoke the deterministic CLI (`hero complete-todo` or equivalent) which:
   - accepts only **pending** items (rejects items **adopted** by the active cycle);
   - sets state to `resolved` with timestamp and note;
   - removes the item from the Pending projection and default `/hero-todos` output;
   - retains history in SQLite/events (idempotent retry for the same resolved ID).
5. `/hero-todos` remains read-only — this command performs the mutation.

## Output Format

```
→ Manual ToDo completion: <id> …
✓ Resolved <id> with note recorded.
→ Run /hero-todos to verify Pending projection.
```
