# AGENTS.md — {{project.name}}

> AI agent instructions for the **{{project.name}}** project.
>
> Stable project instructions. Keep this document concise. Target maximum size: 700 words.

## Project Summary

{{project.summary}}

## Technology Stack

- **Stack**: {{project.stack}}
- **Backend**: {{project.backend}}
- **Languages**: {{project.languages}}

## Documentation Map

Always read project documentation **before** writing code or making architectural decisions.

| Document | Path | Purpose |
|----------|------|---------|
| PRD | docs/product/PRD.md | Product requirements |
| UI Spec | docs/product/UI.md | Design specification |
| ADR | docs/architecture/ADR.md | Architecture decisions |
| DEPLOY | docs/deployment/DEPLOY.md | Deployment guide |
| TESTING | docs/testing/TESTING.md | Testing strategy and commands |

Expand this table as new documents are created (see `documents.json`). Link numbered cycle documents when they exist (e.g. `PRD-C04-001-slug.md`).

**Context compression files** must be **kept up to date after every code-affecting interaction**:

| File | Path | Lifetime | Purpose |
|------|------|----------|---------|
| Current State | context/current-state.md | Long-lived | Source of truth: name, goal, stack, implemented/pending features, architecture, constraints |
| Context Log | context/context-log.md | Short/medium-lived | Operational memory: timestamp, problem, investigation, decision, outcome, refactor, rationale |

After finishing any task that changes code or decisions: (1) update `context/current-state.md` to reflect the new state, and (2) append an entry to `context/context-log.md`.

## Development Workflow

This project uses **AI Workflow Hero** for structured development cycles.

- Development cycles follow: Research → Planning → Implementation → QA → Judge → QA End-to-End
- All cycle artifacts are in `.workflow-hero/cycles/`
- Project state: `context/current-state.md`
- Decision log: `context/context-log.md`

## Reference Lookup Order

When an agent needs external or internal reference material, follow this order strictly:

1. **Project documents** — this file, PRD, UI spec, ADRs, DEPLOY.md, TESTING.md, and the context compression files above.
2. **Context7 MCP** — for library/framework documentation.
3. **Web search** — only when project docs and Context7 do not answer the question.

Do not guess or invent requirements. If project docs are silent on a topic, ask the user.

## Ambiguity and Missing Information

**Any ambiguous requirement or missing information must be questioned to the user before proceeding.** Do not assume defaults that contradict documented decisions, and do not silently fill gaps in the PRD, UI spec, ADRs, or context files.

1. Check project docs and context files first (Reference Lookup Order above).
2. If still unclear, ask the user a specific question with a recommended option — never silently pick one and proceed.
3. Record the answer in `context/context-log.md` and update the affected doc(s) if it changes scope.

## Testing

Follow the project's testing strategy in `docs/testing/TESTING.md` when present.

**After any code modification**, follow this loop without exception:

1. Run the project's automated test command (recorded in TESTING.md or inferred from the stack, e.g. `npm test`, `go test ./...`).
2. If tests fail: analyze the failure, fix the issue, re-run the test command.
3. Only stop when all tests pass.

**Never leave the project in a failing state.**

## Constraints

- Do not change architecture without an approved ADR.
- All documents are written in English.
- Tests must pass before marking a task complete.

_To be maintained by agents._
