# AGENTS.md

Guidance for AI agents working on the **AI Workflow Hero** (Hero) repository.

> Stable project instructions. Keep this document concise. Target maximum size: 700 words.

## Project Summary

Hero is an open-source framework for AI-augmented software development. It does not replace the coding agent — it coordinates multiple specialized subagents, organizes project artifacts, compresses context, and makes AI-driven development cycles reproducible and less dependent on any single LLM provider.

Hero ships as a single Go CLI binary (`hero`) that bootstraps a project with commands, skills, prompts, and templates for a supported IDE/harness (Cursor in V1). Once installed, the actual reasoning-driven workflow (Research → Planning → Implementation → QA → Judge → QA End-to-End) runs entirely in the **Runtime** (the IDE's chat, via slash commands), never inside the CLI binary itself.

This repository is the Hero framework's own source code — not a project that consumes Hero. Agents working here are building the tool, not using it.

## Documentation Map

Always read project documentation **before** writing code or making architectural decisions.

| Document | Path | Purpose |
|---|---|---|
| Product Requirements | [docs/product/PRD.md](docs/product/PRD.md) | Goals, scope, functional/non-functional requirements, V1/V2 boundaries |
| Terminal UX Spec | [docs/product/UI.md](docs/product/UI.md) | CLI visual style, output formats, prompts, error conventions |
| Architecture Decision Records | [docs/architecture/ADR.md](docs/architecture/ADR.md) | All architectural decisions and their rationale |
| Deployment Guide | [docs/deployment/DEPLOY.md](docs/deployment/DEPLOY.md) | Target platforms, build/release process, versioning, checksums |
| Design Notes (source) | [docs/idea/ai_workflow_hero.md](docs/idea/ai_workflow_hero.md) | Full design discussion and grilling session decisions log |

## Testing

Follow these principles for all Go code in this repository:

- Prefer **clarity over cleverness**.
- Test **behavior**, not implementation details.
- Favor **real dependencies** over excessive mocking — use `t.TempDir()` and the real filesystem, and the real `embed.FS` for asset-related tests, rather than mocking the filesystem.
- Keep tests **deterministic and fast**.
- Avoid over-engineered test frameworks.

Test files are colocated with the code they test, in the same package (e.g. `internal/install/service_test.go` next to `service.go`). Use:

- **Unit tests** for the business logic of each `internal/<feature>/` package.
- **Golden-file tests** for template rendering, to guarantee `{{placeholder}}` substitution is correct.
- **Lightweight integration tests** that run the compiled binary against a temporary directory for `install`/`upgrade`/`uninstall`/`doctor`.

Always run `go test ./...` before completing a task. Never leave the repository in a failing state.

## Reference Lookup Order

When an agent needs external or internal reference material, follow this order:

1. **Project documents** — PRD, UI spec, ADRs, DEPLOY.md, and this file.
2. **Context7 MCP** — for library/framework documentation (Cobra, survey/huh, Go stdlib).
3. **Web search** — only when project docs and Context7 do not answer the question.

Do not guess or invent requirements. If project docs are silent on a topic, ask the user.

## Ambiguity and Missing Information

**Any ambiguous requirement or missing information must be questioned to the user before proceeding.**

Do not assume defaults that contradict documented decisions. Do not silently fill gaps in the PRD, UI spec, or ADRs. When in doubt:

1. Check project docs first.
2. If still unclear, ask the user a specific question with a recommended option.
3. Record the user's answer and update affected docs if the decision changes scope.

## Project Constraints

The following constraints are mandatory and must not be violated:

- **Language**: Go, using the Cobra library for the CLI.
- **Architecture**: Feature Based + Vertical Slice (see [ADR-002](docs/architecture/ADR.md#adr-002-repository-architecture-feature-based--vertical-slice)).
- **CLI vs. Runtime separation**: the CLI is deterministic and never performs agent reasoning; anything requiring reasoning belongs exclusively to the Runtime (see [ADR-003](docs/architecture/ADR.md#adr-003-cli-vs-runtime-separation)).
- **Target platforms (V1)**: Linux and macOS, `amd64` and `arm64`.
- **License**: BSD-2-Clause.

For complete details, see the documents listed above.

_To be maintained by agents._
