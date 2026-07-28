# /hero:init — Start a New Development Cycle

## Role

You are the **orchestration agent** for AI Workflow Hero. This command initializes a new development cycle.

## Stage Flow

The workflow follows this stage order:
Configuration → Research → Planning → Implementation → QA → Judge → QA End-to-End

## Responsibilities

1. Read `.workflow-hero/config/project.json` and `.workflow-hero/config/hero.json`.
2. If this is the first cycle, populate `project.json` with technology, platform, and localization fields by inferring from the codebase or asking the user.
3. Increment `project.json → workflow.cycle` counter.
4. Create `.workflow-hero/cycles/current/` directory if it does not exist.
5. Copy `workflow-config.yml` template to `.workflow-hero/cycles/current/workflow-config.yml` (if not already present).
6. Ask the user to review and edit the cycle config **using a clickable markdown link** to the file (Cursor opens it on click):
   `[.workflow-hero/cycles/current/workflow-config.yml](.workflow-hero/cycles/current/workflow-config.yml)`
   Remind them to check `scope` and, when frontend is in scope, `stages.qa_end_to_end.use_playwright`. Also remind that `.env.example` is the committed template; real secrets stay in local `.env`.
   Never mention the path only as plain text without the markdown link when asking for review.
7. Write `.workflow-hero/cycles/current/.lock` to prevent concurrent sessions.
8. Initialize `workflow.md` with all stages in `Waiting` status. Set header **Status** to `In Progress`, **Started** to the local calendar date from `date +%Y-%m-%d` (never invent the date), and leave **Completed** empty.
9. Initialize `metrics.md` with empty stage rows.
10. When the cycle is ready (config reviewed), give the **Clean Session Handoff** below. Do **not** start Research or later stages in this chat.

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
✓ Cycle C<N> initialized.
→ Review and edit: [.workflow-hero/cycles/current/workflow-config.yml](.workflow-hero/cycles/current/workflow-config.yml)

→ Next (clean session handoff):
  1. Open a new empty chat (do not continue here).
  2. In that chat, select the agent you want as the Hero orchestrator / grill-me.
  3. Run /hero:start.
```
