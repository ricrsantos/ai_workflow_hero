# AGENTS.md

Guidance for AI agents working on the **AI Workflow Hero** (Hero) repository.

> Stable project instructions. Keep this document concise. Target maximum size: 700 words.

## Project Summary

Hero is an open-source framework for AI-augmented software development. It does not replace the coding agent — it coordinates specialized subagents, organizes project artifacts, compresses context, and makes AI-driven development cycles reproducible and less dependent on any single LLM provider.

Hero ships as a single Go CLI binary (`hero`) that bootstraps a project with commands, skills, prompts, and templates for a supported IDE/harness (Cursor in V1). Once installed, the reasoning-driven workflow (Research → Planning → Implementation → QA → Judge → QA End-to-End) runs entirely in the **Runtime** (the IDE's chat, via slash commands), never in the CLI binary.

This repository is Hero's own source code — not a project that consumes Hero. Agents working here build the tool, not use it.

## Documentation Map

Always read project documentation **before** writing code or making architectural decisions.

| Document | Path | Purpose |
|---|---|---|
| Product Requirements | [docs/product/PRD.md](docs/product/PRD.md) | Goals, scope, functional/non-functional requirements, V1/V2 boundaries |
| Terminal UX Spec | [docs/product/UI.md](docs/product/UI.md) | CLI visual style, output formats, prompts, error conventions |
| Architecture Decision Records | [docs/architecture/ADR.md](docs/architecture/ADR.md) | All architectural decisions, rationale, and full data-file schemas |
| Deployment Guide | [docs/deployment/DEPLOY.md](docs/deployment/DEPLOY.md) | Target platforms, build/release process, versioning, checksums |
| Design Notes (source) | [docs/idea/ai_workflow_hero.md](docs/idea/ai_workflow_hero.md) | Full design discussion and grilling session decisions log |

**Context compression files** (this repo's own state — see below) must be **kept up to date after every code-affecting interaction**:

| File | Path | Lifetime | Purpose |
|---|---|---|---|
| Current State | [context/current-state.md](context/current-state.md) | Long-lived | Source of truth: name, goal, stack, implemented/pending features, architecture, constraints |
| Context Log | [context/context-log.md](context/context-log.md) | Short/medium-lived | Operational memory: timestamp, problem, investigation, decision, outcome, refactor, rationale |

After finishing any task that changes code or decisions: (1) update `context/current-state.md` to reflect the new state, and (2) append an entry to `context/context-log.md`.

## Reference Lookup Order

When an agent needs external or internal reference material, follow this order strictly:

1. **Project documents** — this file, PRD, UI spec, ADRs, DEPLOY.md, and the context compression files above.
2. **Context7 MCP** — for library/framework documentation (Cobra, survey/huh, Go stdlib).
3. **Web search** — only when project docs and Context7 do not answer the question.

Do not guess or invent requirements. If project docs are silent on a topic, ask the user.

## Ambiguity and Missing Information

**Any ambiguous requirement or missing information must be questioned to the user before proceeding.** Do not assume defaults that contradict documented decisions, and do not silently fill gaps in the PRD, UI spec, ADRs, or context files.

1. Check project docs and context files first (Reference Lookup Order above).
2. If still unclear, ask the user a specific question with a recommended option — never silently pick one and proceed.
3. Record the answer in `context/context-log.md` and update the affected doc(s) if it changes scope.

## Testing

Principles for all Go code here: clarity over cleverness; test **behavior**, not implementation details; favor **real dependencies** over mocking (real `t.TempDir()` filesystem, real `embed.FS`); keep tests deterministic and fast; avoid over-engineered test frameworks.

Test files are colocated with the code they test, same package (e.g. `internal/install/service_test.go` next to `service.go`): **unit tests** per `internal/<feature>/` package, **golden-file tests** for template rendering, **lightweight integration tests** running the compiled binary against a temp directory for `install`/`upgrade`/`uninstall`/`doctor`.

**After any code modification**, follow this loop without exception:

1. Run `go test ./...`.
2. If tests fail: (2.1) analyze the failure, (2.2) fix the issue, (2.3) re-run `go test ./...`.
3. Only stop when all tests pass.

**Never leave the repository in a failing state.**

## Project Constraints

Mandatory, must not be violated:

- **Language**: Go, using the Cobra library for the CLI.
- **Architecture**: Feature Based + Vertical Slice (see [ADR-002](docs/architecture/ADR.md#adr-002-repository-architecture-feature-based--vertical-slice)).
- **CLI vs. Runtime separation**: the CLI is deterministic and never performs agent reasoning; anything requiring reasoning belongs exclusively to the Runtime (see [ADR-003](docs/architecture/ADR.md#adr-003-cli-vs-runtime-separation)).
- **Target platforms (V1)**: Linux and macOS, `amd64` and `arm64`.
- **License**: BSD-2-Clause.

For complete details, see the documents listed above.

_To be maintained by agents._
