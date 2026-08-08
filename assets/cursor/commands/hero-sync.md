# /hero:sync — Sync Hero into an Existing Project

## Role

You are the **orchestration agent** for AI Workflow Hero. This command activates Hero in an existing project by scanning the codebase and generating Hero artifacts.

**There is no `hero sync` CLI verb** — sync runs only via this Runtime slash command (ADR-003).

## Responsibilities

1. Invoke `context_agent` via the Task tool (fresh isolated session) to scan the existing codebase.
   - Apply **Model Resolution** from `orchestration_agent`: pass Task `model` as a kebab slug from `workflow-config.yml` → `agents.context_agent` (`enable_fast_model` → `<id>-fast`; never omit `model`; never use brackets).
   - Pass file pointers: project root path, any existing AGENTS.md, docs/, context/.
   - context_agent is read-only: it never implements or decides architecture.
2. Based on context_agent output, generate:
   - `AGENTS.md` — from `.workflow-hero/templates/AGENTS.md` (all sections: doc map, context compression files, workflow, reference lookup order, ambiguity policy, testing, constraints, secrets).
   - `context/current-state.md` — source-of-truth snapshot of the current project state.
   - `context/context-log.md` — empty log (first entry written here).
   - Soft secrets hygiene (create only if missing; never overwrite): project-root `.env.example` and ensure `.gitignore` ignores `.env` (use `.workflow-hero/templates/env.example` and `gitignore-secrets` as references).
3. Update `.workflow-hero/config/project.json` with inferred project metadata.
4. Run **`hero doctor`** in the project shell and relay any harness warnings (e.g. detected `.claude/` / `.windsurf/` / `.codex/` markers vs `hero.json` → `cli.tools`). Warn-only — do not install unsupported harness assets.
5. Notify the user: sync is complete and the generated files should be reviewed. Remind them to keep real secrets in local `.env` only.
6. Point them to **`/hero:new`** to start the first cycle when ready — operational cycle state is managed via the `hero` CLI and SQLite, not `workflow.md` / `metrics.md`. Agents still run `hero cycle new` during `/hero:new`; the user-facing next step is the slash command.

## Scope Routing

context_agent receives scope from the codebase analysis. The orchestrator routes work based on the `scope` fields in the generated project metadata.

## Fallback

Fall back to `fallback_model` if configured model is unavailable; warn the user explicitly (ADR-008 / Model Resolution).

## Output Format

```
→ Scanning codebase for Hero sync...
→ Generating AGENTS.md, current-state.md, context-log.md...
→ Running hero doctor for harness warnings...
✓ Hero sync complete. Review the generated files before starting a development cycle.
→ When ready, run /hero:new to start your first cycle.
```
