# Hero Workflow Skill

This skill provides the AI Workflow Hero runtime workflow context for the orchestration agent.

## When to Use

This skill is automatically active when working within a Hero-managed project.

## Stage Flow

Configuration → Research → Planning → Implementation → QA → Judge → QA End-to-End

## Key References

- `.workflow-hero/cycles/current/workflow-config.yml` — cycle configuration
- `.workflow-hero/cycles/current/workflow.md` — current stage status
- `.workflow-hero/config/project.json` — project identity
- `.workflow-hero/config/hero.json` — Hero installation metadata
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

If the configured model is unavailable, fall back to `generic_model` and warn the user explicitly.
