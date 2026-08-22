---
name: workflow-hero
description: >-
  AI Workflow Hero runtime workflow context for the orchestration agent. Use when
  working in a Hero-managed project, running /hero-* slash commands, advancing
  stages, or applying Hero stage-close and workspace rules.
---

# Hero Workflow Skill

This skill provides the AI Workflow Hero runtime workflow context for the orchestration agent.

## When to Use

This skill is automatically active when working within a Hero-managed project.

## Project workspace

Work only inside the **current project root** (the directory that contains `.workflow-hero/`). Do not read, grep, glob, or search parent directories, sibling folders, or Hero framework/source trees. Run `hero` from PATH via Shell. If Shell is rejected, stop and tell the user — do not hunt for binaries or inspect other repositories.

## Slash-first user vocabulary (ADR-024)

Tell users to run **`/hero-<name>` slash commands** as the primary CTA (`/hero-new`, `/hero-start`, `/hero-approve`, `/hero-reject`, `/hero-cancel`, `/hero-finish`, `/hero-archive`, `/hero-resume`, `/hero-sync`, `/hero-status`, `/hero-continue`, `/hero-back`, `/hero-cycles`, `/hero-todos`, `/hero-help`). Use `hero …` CLI verbs only as implementation detail for agents. After `/hero-new`, the primary handoff is **`/hero-start`** in a new empty chat.

## Stage Flow

Configuration → Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End

## Clean Session Handoff

On `/hero-new`, if prior cycles exist, import previous `workflow_config` + `fallback_model` + `stages` + `agents` into the new `workflow-config.yml`; reset `title` / `objective` / `scope` to template defaults (see `hero-new.md` — **Previous Cycle Config Import**).

Chat with the user in `workflow_config.user_preferred_language` (default `EN`) unless they explicitly ask for a different chat language. Cycle artifacts stay English.

After `/hero-new` (configuration ready): ask the user to open a **new empty chat**, select the IDE agent/model they want as the Hero **orchestrator / grill-me**, then run `/hero-start`. Soft guidance only. `/hero-start` must bootstrap from disk files, not from the `/hero-new` chat history.

## Stage Close Sequence

Every stage closes with the same sequence (PRD-C01-001 §5.4). The **orchestrator** does this after the stage agent returns — stage agents must not ask to start the next stage.

1. Summary + approval request **only when** this stage's `require_human_approval` is true (list `/hero-approve` `/hero-reject` `/hero-cancel` `/hero-finish` and stop). If false, do not ask yes/no — auto-advance in the same turn.
2. Persist stage transition + metrics to SQLite via the `hero` CLI (`hero stage start` before work; `hero approve|reject|finish|continue` with `--metrics-json` as needed) — do **not** write `workflow.md` / `metrics.md`
3. Show the stage metrics summary in chat (tokens input/output/total, duration, cost) — required every stage — and point the user to `hero metrics` for full details
4. Advance to the next configured stage only after the current Task has returned and the engine advances on approve / auto-complete. Wait for each Task (`run_in_background: false`); post the agent's Output Format in chat because nested Task work does not stream.

## Key References

- `.workflow-hero/hero.db` — SQLite operational store (cycle/stage status, events, metrics)
- `.workflow-hero/cycles/current/workflow-config.yml` — cycle configuration (`workflow_config.user_preferred_language`, `scope`, stages, agents, `fallback_model`, `stages.browser_ui_validation`, `stages.qa_end_to_end.use_playwright`)
- `hero status` / `hero metrics` / `hero events` — query operational state (table or `--json`)
- `.workflow-hero/cycles/current/browser-ui/` — Browser UI Validation artifacts (when that stage runs)
- `.workflow-hero/metrics-summary.md` — project-wide aggregated metrics (optional summary file)
- `.workflow-hero/models/*.yml` — model pricing (`unit: per_1m_tokens`)
- `.workflow-hero/config/project.json` — project identity
- `.workflow-hero/config/hero.json` — Hero installation metadata
- `.workflow-hero/docs/workflow-help.md` — full end-user guide (philosophy, install, configure, commands)
- `context/current-state.md` — current project state
- `context/context-log.md` — decision log

## Approval Commands

| Command | Meaning |
|---------|---------|
| /hero-approve | Approve current stage via `hero approve`, advance |
| /hero-reject | Reject via `hero reject` and re-run current stage |
| /hero-cancel | Cancel via `hero cancel` and rollback via git |
| /hero-finish | Finish via `hero finish` and close the cycle |
| /hero-continue | Grant extra iterations via `hero continue` after escalation |
| /hero-back | Reopen Planning (SDD ambiguity) |
| /hero-cycles | List cycles with per-etapa metrics |
| /hero-todos | Show pending items from `current-state.md` |
| /hero-sync | Activate Hero in an existing project (Runtime only — no `hero sync` CLI) |
| /hero-help | Show Runtime command reference |

## Archive + OpenSpec

`/hero-archive` runs OpenSpec archive first when linked (`openspec archive <name> -y`), then `hero cycle archive`. On OpenSpec failure: offer retry, `hero cycle archive --force` (alias `--skip-openspec`), and manual `openspec archive <name> -y`.

## Sync + doctor

`/hero-sync` completes with **`hero doctor`** so harness marker warnings surface. There is no `hero sync` CLI verb. When product/architecture docs change, users should run `/hero-sync` then `/hero-todos` to refresh pending items in `current-state.md` (ADR-029).

## Fallback

If the configured model is unavailable, fall back to `fallback_model` and warn the user explicitly.
