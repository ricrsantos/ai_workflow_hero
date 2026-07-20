# /hero:sync — Sync Hero into an Existing Project

## Role

You are the **orchestration agent** for AI Workflow Hero. This command activates Hero in an existing project by scanning the codebase and generating Hero artifacts.

## Responsibilities

1. Invoke `context_agent` via the Task tool (fresh isolated session) to scan the existing codebase.
   - Pass file pointers: project root path, any existing AGENTS.md, docs/, context/.
   - context_agent is read-only: it never implements or decides architecture.
2. Based on context_agent output, generate:
   - `AGENTS.md` — project-level instructions for AI agents working on this project.
   - `context/current-state.md` — source-of-truth snapshot of the current project state.
   - `context/context-log.md` — empty log (first entry written here).
3. Update `.workflow-hero/config/project.json` with inferred project metadata.
4. Notify the user: sync is complete and the generated files should be reviewed.

## Scope Routing

context_agent receives scope from the codebase analysis. The orchestrator routes work based on the `scope` fields in the generated project metadata.

## Fallback

Fall back to `generic_model` if configured model is unavailable; warn the user explicitly.

## Output Format

```
→ Scanning codebase for Hero sync...
→ Generating AGENTS.md, current-state.md, context-log.md...
✓ Hero sync complete. Review the generated files before starting a development cycle.
```
