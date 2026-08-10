# AGENTS.md

Guidance for AI agents working on the **AI Workflow Hero** (Hero) repository.

> Stable project instructions. Keep this document concise. Target maximum size: 700 words.

## Project Summary

Hero is an open-source framework for AI-augmented software development. It does not replace the coding agent — it coordinates specialized subagents, organizes project artifacts, compresses context, and makes AI-driven development cycles reproducible and less dependent on any single LLM provider.

Hero ships as a single Go CLI binary (`hero`) that bootstraps a project with commands, skills, prompts, and templates for a supported IDE/harness (Cursor in V1). Once installed, the reasoning-driven workflow (Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End) runs entirely in the **Runtime** (the IDE's chat, via slash commands), never in the CLI binary.

**This repository is Hero's own source code — not a project that consumes Hero.** Agents working here build the tool, not use it.

## Technology Stack

- **Stack**: Go CLI (Cobra) + `embed.FS` + Bubble Tea TUI + SQLite + embedded Cursor Runtime assets
- **Backend**: Go single-binary CLI (`hero`) with feature-based vertical slices under `internal/`
- **Languages**: Go

## Documentation Map

Always read project documentation **before** writing code or making architectural decisions.

| Document | Path | Purpose |
|---|---|---|
| Product Requirements (index) | [docs/product/PRD.md](docs/product/PRD.md) | Goals, scope, V1/V2 boundaries |
| PRD C1 — Hero 1.0 | [docs/product/PRD-C01-001-hero-1-0.md](docs/product/PRD-C01-001-hero-1-0.md) | AI Loop, SQLite, TUI, Cursor adapter |
| PRD C2 — Slash parity | [docs/product/PRD-C02-001-slash-parity-tui-harness.md](docs/product/PRD-C02-001-slash-parity-tui-harness.md) | Slash-first UX, TUI command import, archive coupling |
| PRD C3 — Harness autonomy | [docs/product/PRD-C03-001-cursor-harness-tui-autonomy.md](docs/product/PRD-C03-001-cursor-harness-tui-autonomy.md) | Cursor CLI harness, TUI conversation, hero-sync scan |
| Terminal UX Spec (index) | [docs/product/UI.md](docs/product/UI.md) | CLI visual style, prompts, error conventions |
| UI C1 / C2 / C3 | [docs/product/UI-C01-001-hero-tui.md](docs/product/UI-C01-001-hero-tui.md), [UI-C02-001](docs/product/UI-C02-001-tui-slash-command-parity.md), [UI-C03-001](docs/product/UI-C03-001-tui-harness-autonomy.md) | Cycle-specific terminal UX deltas |
| Architecture (index) | [docs/architecture/ADR.md](docs/architecture/ADR.md) | ADR-001–030, schemas, rationale |
| ADR C1 / C2 / C3 | [ADR-C01-001](docs/architecture/ADR-C01-001-hero-1-0.md), [ADR-C02-001](docs/architecture/ADR-C02-001-slash-parity-harness-archive.md), [ADR-C03-001](docs/architecture/ADR-C03-001-cursor-harness-tui-autonomy.md) | Cycle architecture decisions |
| Deployment Guide | [docs/deployment/DEPLOY.md](docs/deployment/DEPLOY.md) | Build, release, versioning, checksums |
| Testing | [docs/testing/TESTING.md](docs/testing/TESTING.md) | Test strategy and commands (`go test ./...`) |
| Design Notes (source) | [docs/idea/v0/ai_workflow_hero.md](docs/idea/v0/ai_workflow_hero.md) | Pre-1.0 design discussion |
| Idea archive (non-normative) | [docs/idea/archive/v1/](docs/idea/archive/v1/) | Archived 1.0 idea drafts; superseded by cycle PRD/ADR/UI |

See also `.workflow-hero/config/documents.json` for the machine-readable doc registry.

**Context compression files** (this repo's own state) must be **kept up to date after every code-affecting interaction**:

| File | Path | Lifetime | Purpose |
|---|---|---|---|
| Current State | [context/current-state.md](context/current-state.md) | Long-lived | Source of truth: stack, features, architecture, constraints |
| Context Log | [context/context-log.md](context/context-log.md) | Short/medium-lived | Operational memory: decisions, investigations, outcomes |

After finishing any task that changes code or decisions: (1) update `context/current-state.md`, and (2) append an entry to `context/context-log.md`.

## Development Workflow

This repo **builds** Hero and dogfoods Hero cycles on itself.

- Cycles: Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End
- Cycle artifacts: `.workflow-hero/cycles/` (archived C1–C3; no active cycle)
- OpenSpec SDD: `openspec/` (living specs + archived changes)
- Project state: `context/current-state.md`; decision log: `context/context-log.md`

## Reference Lookup Order

1. **Project documents** — this file, PRD, UI spec, ADRs, DEPLOY.md, TESTING.md, context files.
2. **Context7 MCP** — Cobra, charmbracelet/huh/bubbletea, Go stdlib.
3. **Web search** — only when project docs and Context7 do not answer the question.

Do not guess or invent requirements. If project docs are silent, ask the user.

## Ambiguity and Missing Information

Any ambiguous requirement must be questioned before proceeding.

1. Check project docs and context files first.
2. If still unclear, ask the user with a recommended option.
3. Record the answer in `context/context-log.md` and update affected docs if scope changes.

## Testing

Principles: test **behavior**, not implementation; favor **real dependencies** (`t.TempDir()`, real `embed.FS`); keep tests fast and deterministic.

Colocated `*_test.go` in each `internal/<feature>/` package; golden tests for templates; integration tests for install/upgrade/uninstall/doctor.

**After any code modification**:

1. Run `go test ./...`.
2. If tests fail: analyze, fix, re-run.
3. Only stop when all tests pass.

**Never leave the repository in a failing state.**

## Project Constraints

- **Language**: Go, Cobra CLI.
- **Architecture**: Feature Based + Vertical Slice ([ADR-002](docs/architecture/ADR.md#adr-002-repository-architecture-feature-based--vertical-slice)).
- **CLI vs Runtime**: CLI is deterministic only; reasoning belongs in Runtime ([ADR-003](docs/architecture/ADR.md#adr-003-cli-vs-runtime-separation)).
- **Platforms (V1)**: Linux and macOS, `amd64` and `arm64`.
- **License**: BSD-2-Clause.
- Do not change architecture without an approved ADR.
- Cycle artifacts and docs are English; chat language follows `workflow_config.user_preferred_language` (default `EN`).

## Secrets and Environment Variables

- Never commit secrets, API keys, tokens, private keys, or credential files.
- Commit only `.env.example` (placeholders). Keep real values in local `.env` (gitignored).
- Do not stage `.env`, `.env.*` (except `.env.example`), `*.pem`, `credentials.json`, or `secrets.json`.
- If a secret was committed, stop and tell the user to rotate and untrack it.

_To be maintained by agents._
