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
6. Ask the user to review and edit `workflow-config.yml` before proceeding.
7. Write `.workflow-hero/cycles/current/.lock` to prevent concurrent sessions.
8. Initialize `workflow.md` with all stages in `Waiting` status.
9. Initialize `metrics.md` with empty stage rows.
10. Tell the user: "Cycle initialized. Edit workflow-config.yml and run /hero:start when ready."

## Approval and Control Loop

- When `require_human_approval: false`: stage auto-completes and advances automatically.
- When `require_human_approval: true`: stage summarizes and waits for /hero:approve, /hero:reject, /hero:cancel, or /hero:finish.
- Every stage closes with: (a) summary + approval request, (b) update workflow.md, (c) update metrics.md via the **Metrics Procedure** in `orchestration_agent` and show metrics summary in chat (tokens + duration + cost), (d) advance to next configured stage.

## Fallback / Model Resolution

When later stages invoke subagents (after `/hero:start`), follow **Model Resolution** in `orchestration_agent`: always pass Task `model` from `workflow-config.yml` (`enable_fast_model` → `[fast=...]`; never omit). If the configured model is unavailable, fall back to `generic_model` and warn the user explicitly. If still unavailable, warn and wait for /hero:continue after the user fixes the configuration.

## Output Format

```
→ Initializing cycle C<N>...
✓ Cycle C<N> initialized. Edit .workflow-hero/cycles/current/workflow-config.yml and run /hero:start when ready.
```
