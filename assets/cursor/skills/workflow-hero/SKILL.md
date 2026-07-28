# Hero Workflow Skill

This skill provides the AI Workflow Hero runtime workflow context for the orchestration agent.

## When to Use

This skill is automatically active when working within a Hero-managed project.

## Stage Flow

Configuration → Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End

## Clean Session Handoff

After `/hero:init` (configuration ready): ask the user to open a **new empty chat**, select the IDE agent/model they want as the Hero **orchestrator / grill-me**, then run `/hero:start`. Soft guidance only. `/hero:start` must bootstrap from disk files, not from the init chat history.

## Stage Close Sequence

Every stage closes with the same sequence (PRD §5.3):

1. Summary + approval request (respect `require_human_approval`)
2. Update `workflow.md`
3. Update `metrics.md` via the **Metrics Procedure** in `orchestration_agent` (chars ÷ 4 × `models/*.yml` rates; never leave `—` for a stage that ran)
4. Show the stage metrics summary in chat (tokens input/output/total, duration, cost) — required every stage — and include a clickable link to `[.workflow-hero/cycles/current/metrics.md](.workflow-hero/cycles/current/metrics.md)` noting that full details are in that file
5. Advance to the next configured stage

## Key References

- `.workflow-hero/cycles/current/workflow-config.yml` — cycle configuration (`scope`, stages, agents, `stages.browser_ui_validation`, `stages.qa_end_to_end.use_playwright`)
- `.workflow-hero/cycles/current/workflow.md` — current stage status
- `.workflow-hero/cycles/current/metrics.md` — per-cycle token/cost estimates
- `.workflow-hero/cycles/current/browser-ui/` — Browser UI Validation artifacts (when that stage runs)
- `.workflow-hero/metrics-summary.md` — project-wide aggregated metrics
- `.workflow-hero/models/*.yml` — model pricing (`unit: per_1m_tokens`)
- `.workflow-hero/config/project.json` — project identity
- `.workflow-hero/config/hero.json` — Hero installation metadata
- `.workflow-hero/docs/workflow-help.md` — full end-user guide (philosophy, install, configure, commands)
- `context/current-state.md` — current project state
- `context/context-log.md` — decision log

## Approval Commands

| Command | Meaning |
|---------|---------|
| /hero:approve | Approve current stage, advance |
| /hero:reject | Reject and re-run current stage |
| /hero:cancel | Cancel and rollback via git |
| /hero:finish | Finish and close the cycle |
| /hero:continue | Grant extra iterations after escalation |
| /hero:back | Reopen Planning (SDD ambiguity) |

## Fallback

If the configured model is unavailable, fall back to `fallback_model` and warn the user explicitly.
