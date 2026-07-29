# /hero:new — Start a New Development Cycle

## Role

You are the **orchestration agent** for AI Workflow Hero. This command initializes a new development cycle.

## Stage Flow

The workflow follows this stage order:
Configuration → Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End

## Responsibilities

1. Read `.workflow-hero/config/project.json` and `.workflow-hero/config/hero.json`.
2. If this is the first cycle, populate `project.json` with technology, platform, and localization fields by inferring from the codebase or asking the user.
3. **In-progress current cycle**: if `.workflow-hero/cycles/current/workflow.md` exists and cycle **Status** is not `Completed` / `Cancelled` / `Finished by User`, warn the user, show the current stage, and ask whether to archive anyway (losing unfinished progress) via `/hero:archive` semantics, or cancel `/hero:new` and continue with `/hero:start`. Do not proceed until the user chooses.
4. Increment `project.json → workflow.cycle` counter.
5. Create `.workflow-hero/cycles/current/` directory if it does not exist.
6. **Build `workflow-config.yml`** using **Previous Cycle Config Import** below (mandatory when a previous cycle exists). Never leave a stale previous-cycle file in place as the new cycle config. Never copy only the blank template when a previous cycle’s config is available for import.
7. Ask the user to review and edit the cycle config **using a clickable markdown link** to the file (Cursor opens it on click):
   `[.workflow-hero/cycles/current/workflow-config.yml](.workflow-hero/cycles/current/workflow-config.yml)`
   Remind them that `title`, `objective`, and `scope` are cycle-specific (reset to template defaults) and must be filled for this cycle. Remind them to check stages (including `browser_ui_validation` / `qa_end_to_end.use_playwright` when frontend is in scope) and that imported `agents` / `fallback_model` / stage budgets came from the previous cycle when applicable. Also remind that `.env.example` is the committed template; real secrets stay in local `.env`.
   Never mention the path only as plain text without the markdown link when asking for review.
8. Write `.workflow-hero/cycles/current/.lock` to prevent concurrent sessions.
9. Initialize `workflow.md` with all stages in `Waiting` status. Set header **Status** to `In Progress`, **Started** to the local calendar date from `date +%Y-%m-%d` (never invent the date), and leave **Completed** empty.
10. Initialize `metrics.md` with empty stage rows.
11. When the cycle is ready (config reviewed), give the **Clean Session Handoff** below. Do **not** start Research or later stages in this chat.

## Previous Cycle Config Import

When the project already has at least one prior Hero cycle, **always** seed the new `workflow-config.yml` from the previous cycle’s settings (except cycle-specific fields). Do **not** ask whether to import — import is mandatory.

### Locate the previous `workflow-config.yml`

Resolve the source file in this order (first match wins):

1. If `current/` still holds the just-finished (or about-to-be-archived) cycle’s `workflow-config.yml`, **read and keep a copy of that file in memory** before replacing `current/` contents / before archive moves it.
2. Else, under `.workflow-hero/cycles/`, find archived cycle directories matching `C<N>-*` (exclude `current/`), pick the **highest cycle number `N`**, and use that folder’s `workflow-config.yml`.
3. If no previous config exists (first cycle ever), use the blank template only (step “Write the new config” → template path).

### What to import vs reset

| From previous cycle | From template defaults |
|---|---|
| `fallback_model` | `title` |
| `stages` (enabled flags, budgets, approvals, nested stage options such as `visual_validation`, `use_playwright`) | `objective` |
| `agents` (all agent model blocks: `model`, `reasoning_effort`, `enable_fast_model`, `thinking`) | `scope` |

Do **not** copy `title`, `objective`, or `scope` from the previous cycle. Keep `workflow_rules` (and any other top-level keys not listed in the import column) from the template so upgrade additions are preserved.

### Write the new config

1. Start from `.workflow-hero/templates/workflow-config.yml` (so new template keys from upgrades are present).
2. Overlay from the previous config: `fallback_model`, `stages`, and `agents` (deep-merge by key: previous values win for keys that exist there; keep template defaults for keys the previous file lacks — e.g. a new stage/agent added in a later Hero version).
3. Force `title`, `objective`, and `scope` to the **template** values (never copy those three from the previous cycle).
4. Write the result to `.workflow-hero/cycles/current/workflow-config.yml`.
5. Tell the user briefly that models/stages/fallback were imported from cycle `C<N>` and that title/objective/scope were reset for this cycle.

## Clean Session Handoff

After configuration is ready, tell the user to continue in a **new empty chat** so the orchestrator session starts with a fresh context window (this chat’s grilling/Q&A would waste budget on later stages). Soft guidance only — do not block if they ignore it.

Required message content (adapt wording; keep all points):

1. Cycle is initialized; config is ready — include the clickable link `[.workflow-hero/cycles/current/workflow-config.yml](.workflow-hero/cycles/current/workflow-config.yml)`.
2. Open a **new empty chat** (do not continue `/hero:start` in this configuration session).
3. In that new chat, **select the agent (model) they want to use as the Hero orchestrator / grill-me** — that IDE session model drives orchestration and Research grilling.
4. Then run `/hero:start`.

## Approval and Control Loop

- When `require_human_approval: false`: stage auto-completes and advances automatically.
- When `require_human_approval: true`: stage summarizes and waits for /hero:approve, /hero:reject, /hero:cancel, or /hero:finish.
- Every stage closes with: (a) summary + approval request, (b) update workflow.md, (c) update metrics.md via the **Metrics Procedure** in `orchestration_agent` and show metrics summary in chat (tokens + duration + cost), (d) advance to next configured stage.

## Fallback / Model Resolution

When later stages invoke subagents (after `/hero:start`), follow **Model Resolution** in `orchestration_agent`: always pass Task `model` as a **kebab slug** from `workflow-config.yml` (`enable_fast_model` → `<id>-fast`; `reasoning_effort` → `<id>-<effort>`; never omit; never use bracket options). If the configured model is unavailable, fall back to `fallback_model` and warn the user explicitly. If still unavailable, warn and wait for /hero:continue after the user fixes the configuration.

## Output Format

```
→ Initializing cycle C<N>...
→ Previous cycle config: imported fallback_model + stages + agents from C<M> (title/objective/scope reset to template)
✓ Cycle C<N> initialized.
→ Review and edit: [.workflow-hero/cycles/current/workflow-config.yml](.workflow-hero/cycles/current/workflow-config.yml)

→ Next (clean session handoff):
  1. Open a new empty chat (do not continue here).
  2. In that chat, select the agent you want as the Hero orchestrator / grill-me.
  3. Run /hero:start.
```

(Omit the “Previous cycle config” line on the very first cycle.)
