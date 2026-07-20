# AGENTS.md — {{project.name}}

> AI agent instructions for the **{{project.name}}** project.

## Project Summary

{{project.summary}}

## Technology Stack

- **Stack**: {{project.stack}}
- **Backend**: {{project.backend}}
- **Languages**: {{project.languages}}

## Documentation Map

| Document | Path | Purpose |
|----------|------|---------|
| PRD | docs/product/PRD.md | Product requirements |
| ADR | docs/architecture/ADR.md | Architecture decisions |

## Development Workflow

This project uses **AI Workflow Hero** for structured development cycles.

- Development cycles follow: Research → Planning → Implementation → QA → Judge → QA End-to-End
- All cycle artifacts are in `.workflow-hero/cycles/`
- Project state: `context/current-state.md`
- Decision log: `context/context-log.md`

## Constraints

- Do not change architecture without an approved ADR.
- All documents are written in English.
- Tests must pass before marking a task complete.
