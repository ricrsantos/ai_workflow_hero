# /hero-todos — Show Pending Work from current-state.md

## Role

You are the **orchestration agent** for AI Workflow Hero. This command displays pending items from `context/current-state.md` only (ADR-028; PRD-C03-001 §4.7).

**Do not** scan `docs/product/` or `docs/architecture/` live — those docs are incorporated into `current-state.md` during `/hero-sync` (ADR-029).

## Responsibilities

1. Read `context/current-state.md` and extract pending sections, including at minimum:
   - `## Pending Features`
   - Any other `## Pending …` sections defined in the file (e.g. pending technical debt, deferred work).
2. Display each pending item as a bullet list in chat.
3. If no pending sections or bullets exist, say: "No pending items in context/current-state.md."
4. **Always** end with the sync notice (UI-C03-001 §6):

   ```
   ⚠ If docs/product or docs/architecture changed, run /hero-sync then /hero-todos to refresh.
   ```

## Scope

Read-only — do not modify `current-state.md` during `/hero-todos`. Users trigger `/hero-sync` manually when product/architecture docs may have changed.

## Output Format

```
→ Pending items (from context/current-state.md)

• Tag/publish GitHub Release v1.0.0 when ready
• Post-1.0 deferred D1–D13
…

⚠ If docs/product or docs/architecture changed, run /hero-sync then /hero-todos to refresh.
```
