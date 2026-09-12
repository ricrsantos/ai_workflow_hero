# /hero-add-todo — Defer Findings to Project ToDos (Escalated)

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Usage

```
/hero-add-todo [find-id ...]
```

In Cursor Runtime, supply one or more open or reopened finding IDs (for example `/hero-add-todo find-qa-1 find-judge-2`). The TUI may open a checklist when IDs are omitted.

## Responsibilities

1. Run `hero status` and confirm the relevant stage loop is **Escalated** with at least one open or reopened finding.
2. If preconditions fail, explain precisely (for example: not Escalated, no open findings, unknown ID, already deferred) and **stop** without mutating state.
3. When valid, invoke the deterministic CLI (exact flags evolve with C15 engine work — prefer `hero add-todo` with finding IDs). The service:
   - moves selected findings to `deferred_todo` and creates/links pending project ToDos;
   - updates the Pending projection in `context/current-state.md` idempotently;
   - leaves unselected open findings and unchecked OpenSpec tasks as blockers when only a subset is deferred.
4. After **partial** deferral: tell the user the loop stays Escalated and they must run `/hero-continue [N]` before more productive work.
5. When **every** blocker is deferred (no unchecked tasks, no open/reopened findings): the cycle closes with disposition `completed_with_deferred_todos` — no empty Implementation wave, enabled downstream stages are skipped. State this explicitly in chat.
6. Do not edit OpenSpec checkboxes or call `hero stage loop-back` from this command.

## Output Format

```
→ Escalation triage via /hero-add-todo …
✓ Added <N> finding(s) to project ToDos: find-…
⚠ … (Escalated remainder or completed_with_deferred_todos summary)
```
