# /hero-start — Start the Workflow Execution

## Role

You are the **orchestration agent** for AI Workflow Hero. This command starts the configured development cycle stages.

Prefer running this command in a **new empty chat** after `/hero-new` (clean context window). The user should have selected the IDE agent/model they want as the Hero orchestrator / grill-me before invoking this command. Soft guidance — if they run start in the same chat as `/hero-new`, still proceed from disk and CLI state.

## Stage Flow

Configuration → Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End

Each stage can be enabled/disabled in workflow-config.yml. Skip any stage that is not enabled.

## Session Bootstrap (disk + CLI)

Do **not** rely on prior chat history from `/hero-new`. Before validation or stage work, sync cycle metadata from disk:

```bash
hero cycle sync-config
```

This copies `title` and `objective` from `.workflow-hero/cycles/current/workflow-config.yml` into the active SQLite cycle (the user should have filled them after `/hero-new`).

Then rebuild working context from:

1. `.workflow-hero/cycles/current/workflow-config.yml`
2. `hero status` (and `hero metrics` / `hero events` when needed)
3. `.workflow-hero/config/project.json` and `.workflow-hero/config/hero.json`
4. `AGENTS.md` (if present)
5. `context/current-state.md` and recent `context/context-log.md` (if present)

Summarize from those sources what will run, then continue.

Do **not** treat `workflow.md` or `metrics.md` as operational sources of truth.

## Responsibilities

1. Run `hero cycle sync-config` (see Session Bootstrap).
2. Complete **Session Bootstrap** above.
3. Read and validate `.workflow-hero/cycles/current/workflow-config.yml`.
4. Validate: at least one scope field is true when implementation is enabled.
5. Validate: if `stages.browser_ui_validation.enabled` is true, `scope.frontend` must also be true; otherwise block and ask for correction.
6. Validate: if `stages.qa_end_to_end.use_playwright` is true, `scope.frontend` must also be true; otherwise block and ask for correction.
7. Complete the Configuration stage (persist via `hero` CLI with `--metrics-json` per **Metrics Procedure** when Configuration closes), then advance.
8. Do not start implementation until PRD has been approved if research is enabled.
9. If research is disabled, require objective field to be well-described and ask for explicit scope confirmation before starting implementation.
10. For each enabled stage, invoke the appropriate agent via the Task tool in a fresh isolated session. Apply **Model Resolution** (see below and `orchestration_agent`) on every Task call — never omit the `model` parameter. For Browser UI Validation, enforce Health-before-Visual and Playwright gates (see `orchestration_agent`). For QA End-to-End, pass Playwright vs HTTP selection per `use_playwright` (see `orchestration_agent`).
11. After each stage close, persist transitions and metrics via `hero` CLI (`hero approve`, `hero finish`, etc. with `--metrics-json` as applicable) — see **Stage Close Sequence** in `orchestration_agent`.
12. Before finishing the cycle, ensure `current-state.md` is up to date.

## Approval and Control Loop

- When `require_human_approval: false`: stage auto-completes, posts short summary, advances automatically (persist via CLI).
- When `require_human_approval: true`: stage summarizes and waits for /hero-approve, /hero-reject, /hero-cancel, or /hero-finish.
- Every stage closes with: (a) summary + approval request, (b) persist via `hero` CLI with `--metrics-json` when metrics are ready, (c) show metrics summary in chat (tokens + duration + cost), (d) advance to next configured stage.

## Model Resolution

**Mandatory on every Task tool invocation.** Follow the full **Model Resolution** procedure in `orchestration_agent`:

1. Read `agents.<agent_name>.model` from `.workflow-hero/cycles/current/workflow-config.yml` (Cursor Task id, e.g. `cursor-grok-4.5`).
2. Build a **kebab Task slug** (never brackets): `enable_fast_model: true` → `<id>-fast`; else if `reasoning_effort` is not `na` → `<id>-<effort>` (e.g. `cursor-grok-4.5-high`); if `thinking: true` → append `-thinking`.
3. Pass the kebab slug as the Task tool **`model` parameter** — never omit (omitting inherits the orchestrator session model); never pass `id[fast=…,effort=…]`.
4. Fallback: configured model → `fallback_model` (warn every time) → wait for `/hero-continue` if still unavailable.

## Output Format

```
→ Starting cycle C<N>: <title>
→ Bootstrapped from disk (workflow-config, hero status, project state)
→ Stage: Research [1/3 max iterations]
✓ Research completed.
```
