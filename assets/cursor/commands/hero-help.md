# /hero:help — Show Hero Runtime Commands

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

Display a summary of all available Hero Runtime commands.

## Command Reference

| Command | Description |
|---------|-------------|
| /hero:init | Initialize a new development cycle (then open a new chat for start) |
| /hero:start | Start the configured workflow stages (prefer new empty chat; select orchestrator / grill-me first) |
| /hero:approve | Approve the current stage and advance |
| /hero:reject | Reject the current stage and re-run |
| /hero:cancel | Cancel the current stage and rollback via git |
| /hero:finish | Finish and close the current cycle (writes Completed date for archive naming) |
| /hero:archive | Archive current cycle; folder date from workflow.md Completed field |
| /hero:resume [cycle] | Restore an archived cycle |
| /hero:sync | Activate Hero in an existing project |
| /hero:status | Show current cycle stage status |
| /hero:continue | Grant extra iterations after exhaustion |
| /hero:back | Reopen Planning stage (when Judge finds SDD ambiguity) |
| /hero:help | Show this help |

## Full user guide

For philosophy, install/uninstall, configuration, CLI commands, agents, architecture docs, and logging standards, open:

`.workflow-hero/docs/workflow-help.md`

## Stage Flow

Configuration → Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End

## Output Format

Display the table above clearly in the chat.
