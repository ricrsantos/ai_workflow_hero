# /hero-sync — Sync Hero into an Existing Project

## Role

You are the **orchestration agent** for AI Workflow Hero. This command activates Hero in an existing project by scanning the codebase and generating Hero artifacts.

**There is no `hero sync` CLI verb** — sync runs only via this Runtime slash command (ADR-003).

## Responsibilities

1. Invoke `context_agent` via the Task tool (fresh isolated session) to scan the existing codebase.
   - Apply **Model Resolution** from `orchestration_agent`: pass Task `model` as a kebab slug from `workflow-config.yml` → `agents.context_agent` (`enable_fast_model` → `<id>-fast`; never omit `model`; never use brackets).
   - Pass file pointers: project root path, any existing AGENTS.md, docs/, context/.
   - context_agent is read-only: it never implements or decides architecture.
2. Based on context_agent output, generate:
   - `AGENTS.md` — from `.workflow-hero/templates/AGENTS.md` (all sections: doc map, context compression files, architecture overview policy, workflow, reference lookup order, ambiguity policy, testing, constraints, secrets).
   - `context/current-state.md` — source-of-truth snapshot of the current project state.
   - `context/context-log.md` — empty log (first entry written here).
   - `docs/architecture/architecture-overview.md` — when missing (or refresh when context_agent flags it stale): synthetic high-level diagrams and package map; non-normative; register in `documents.json` when created.
   - Soft secrets hygiene (create only if missing; never overwrite): project-root `.env.example` and ensure `.gitignore` ignores `.env` (use `.workflow-hero/templates/env.example` and `gitignore-secrets` as references).
3. **Pending docs scan (ADR-029)** — after the codebase snapshot, analyze `docs/product/` and `docs/architecture/` (including cycle PRDs/ADRs such as `PRD-C*.md` and `ADR-C*.md`) for explicit **pending**, **deferred**, **not yet implemented**, or **out-of-scope-for-later** items:
   - Merge discovered items into the appropriate pending sections of `context/current-state.md` (e.g. `## Pending Features`, `## Known Technical Debt`).
   - **Dedupe**: do not add a bullet when the same pending work already exists in `current-state.md` (match by substance, not only exact wording).
   - Do not remove existing pending bullets unless they are clearly obsolete per the scanned docs.
   - Record a brief note in `context/context-log.md` when significant pending items were merged from product/architecture docs.
4. Update `.workflow-hero/config/project.json` with inferred project metadata.
5. Run **`hero doctor`** in the project shell and relay any harness warnings (e.g. detected `.claude/` / `.windsurf/` / `.codex/` markers vs `hero.json` → `cli.tools`). Warn-only — do not install unsupported harness assets.
6. Notify the user: sync is complete and the generated files should be reviewed. Remind them to keep real secrets in local `.env` only.
7. Point them to **`/hero-new`** to start the first cycle when ready — operational cycle state is managed via the `hero` CLI and SQLite, not `workflow.md` / `metrics.md`. Agents still run `hero cycle new` during `/hero-new`; the user-facing next step is the slash command.
8. Suggest **`/hero-todos`** to review merged pending items after sync when product/architecture docs were scanned.

## Scope Routing

context_agent receives scope from the codebase analysis. The orchestrator routes work based on the `scope` fields in the generated project metadata.

## Fallback

Fall back to `fallback_model` if configured model is unavailable; warn the user explicitly (ADR-008 / Model Resolution).

## Output Format

```
→ Scanning codebase for Hero sync...
→ Generating AGENTS.md, current-state.md, context-log.md, architecture-overview.md (when needed)...
→ Scanning docs/product and docs/architecture for pending items...
→ Running hero doctor for harness warnings...
✓ Hero sync complete. Review the generated files before starting a development cycle.
→ Run /hero-todos to review pending items, or /hero-new to start your first cycle.
```
