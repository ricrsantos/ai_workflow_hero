# /hero-help — Show Hero Runtime Commands

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

Display a summary of all available Hero Runtime commands.

## Command Reference

| Command | Description |
|---------|-------------|
| /hero-new | Prepare `workflow-config.yml`, then `hero cycle new` (import prior models/stages; reset title/objective/scope; clean-session handoff) |
| /hero-start | Start configured workflow stages (prefer new empty chat; bootstrap from config + `hero status`) |
| /hero-approve | Approve via `hero approve --metrics-json` and advance |
| /hero-reject | Reject via `hero reject --reason` and re-run stage |
| /hero-cancel | Cancel active cycle via `hero cancel` (git rollback when needed) |
| /hero-finish | Close cycle via `hero finish --metrics-json` |
| /hero-archive | Archive via `hero cycle archive` (OpenSpec `openspec archive <name> -y` first when linked; `--force` on OpenSpec failure) |
| /hero-resume [cycle] | Resume via `hero cycle resume [--number N]` |
| /hero-sync | Activate Hero in an existing project (Runtime only; runs `hero doctor` after sync — no `hero sync` CLI) |
| /hero-status | Show stage status via `hero status` |
| /hero-continue | Grant extra iterations via `hero continue --extra N` |
| /hero-back | Reopen Planning (SDD ambiguity; orchestrator-driven, no dedicated CLI verb) |
| /hero-cycles | List cycles with per-etapa metrics (SQLite + archive folders) |
| /hero-todos | Show pending items from `context/current-state.md` (run `/hero-sync` first when docs changed) |
| /hero-model | Select TUI chat model (Hero TUI palette; persists to `hero.json`) |
| /hero-help | Show this help |

## CLI query helpers

| CLI | Purpose |
|-----|---------|
| `hero status` | Cycle and stage state (add `--json` for machine output) |
| `hero metrics` | Per-stage token/cost estimates |
| `hero events` | Recent cycle events |

## Full user guide

For philosophy, install/uninstall, configuration, CLI commands, agents, architecture docs, and logging standards, open:

`.workflow-hero/docs/workflow-help.md`

## Stage Flow

Configuration → Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End

## Output Format

Display the table above clearly in the chat.
