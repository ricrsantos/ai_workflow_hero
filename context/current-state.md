# Current State

> Long-lived document. Single source of truth for **this repository** (the Hero CLI + Runtime assets themselves — not a project that uses Hero).
>
> Must be updated after every implementation cycle. Keep this document under 2,000 words by consolidating information and removing obsolete content. Do not keep a full history here — that belongs in `context/context-log.md` and git.

---

## Project Identity

| Field | Value |
|---|---|
| **Name** | AI Workflow Hero (Hero) |
| **Repository** | `ai_workflow_hero` (local; no remote published yet) |
| **Goal** | Open-source framework that coordinates specialized AI subagents, organizes project artifacts, compresses context, and makes AI-driven development cycles reproducible and less dependent on any single LLM provider. |
| **License** | BSD-2-Clause |
| **Phase** | Documentation / design complete for V1. No Go code written yet. |

## Technology Stack

| Concern | Choice |
|---|---|
| Language | Go |
| CLI framework | [Cobra](https://github.com/spf13/cobra) |
| Asset embedding | Go `embed.FS` (commands, skills, prompts, templates ship inside the binary) |
| Interactive prompts | Go survey-style library (`AlecAivazis/survey` or `charmbracelet/huh`) |
| SDD / planning framework | [OpenSpec](https://github.com/Fission-AI/OpenSpec) |
| E2E testing (Runtime side) | Playwright (frontend) / direct HTTP calls (backend-only) |
| Target IDE/harness (V1) | Cursor only |
| Target platforms (V1) | Linux and macOS, `amd64` and `arm64` |
| Versioning | SemVer (`vMAJOR.MINOR.PATCH`), injected via `-ldflags` from git tag |

## Architecture Summary

- **Feature Based + Vertical Slice** repository structure: each CLI capability lives in its own `internal/<feature>/` package (`internal/install/`, `internal/upgrade/`, `internal/uninstall/`, `internal/doctor/`, etc.), each with `command.go` (Cobra wiring), `service.go` (business logic), `validator.go` (validation). IDE-specific logic lives in `internal/adapters/cursor/`. Shared low-level concerns live in `internal/common/`. See [ADR-002](../docs/architecture/ADR.md#adr-002-repository-architecture-feature-based--vertical-slice).
- **Strict CLI vs. Runtime separation**: the Go binary is 100% deterministic (install/upgrade/uninstall/doctor/version/variables/update-models/status/help) and never calls an LLM. All reasoning-driven workflow execution (Research → Planning → Implementation → QA → Judge → QA End-to-End) happens exclusively in the Runtime (IDE chat, via `/hero:*` slash commands). See [ADR-003](../docs/architecture/ADR.md#adr-003-cli-vs-runtime-separation).
- **Clean subagent sessions**: every subagent (`backend_agent`, `frontend_agent`, `generic_agent`, `qa_agent`, `judge_agent`, `end2end_qa_agent`, `context_agent`) runs via the Cursor Task tool in a fresh, isolated session, receiving file pointers instead of pasted content. See [ADR-005](../docs/architecture/ADR.md#adr-005-subagent-invocation-via-task-tool-with-clean-sessions).
- **Simple placeholder templating** (`{{path.key}}`), no loop/conditional engine. See [ADR-006](../docs/architecture/ADR.md#adr-006-simple-placeholder-templating-no-loop-engine).
- **Three-level model fallback**: agent's configured model → `generic_model` (with explicit warning) → user must fix config and run `/hero:continue`. See [ADR-008](../docs/architecture/ADR.md#adr-008-three-level-model-fallback-chain).
- Full architectural rationale: [docs/architecture/ADR.md](../docs/architecture/ADR.md) (10 ADRs, all Accepted).

## Implemented Features

- None yet. This repository currently contains only documentation and design artifacts (no Go source code).

## Pending Features (V1 scope)

- Go module scaffold (`cmd/hero`, `internal/...`) per [ADR-002](../docs/architecture/ADR.md#adr-002-repository-architecture-feature-based--vertical-slice).
- CLI commands: `hero install --tools cursor`, `hero upgrade`, `hero uninstall`, `hero doctor`, `hero version`, `hero variables`, `hero update-models`, `hero status`, `hero help`.
- Embedded assets: `.cursor/agents/*.md` (10 agent prompts), `.cursor/commands/hero-*.md` (13 Runtime command files), `.cursor/skills/workflow-hero/` and `.cursor/skills/grilling/`, `.workflow-hero/templates/` (docs templates, `workflow-config.yml`, `workflow.md`, `metrics.md`, etc.), `models/*.yml` (7 provider pricing files).
- Runtime-side prompt files implementing the full stage flow (Configuration → Research → Planning → Implementation → QA → Judge → QA End-to-End) and all `/hero:*` commands.
- `scripts/release.sh` for manual cross-compiled releases (4 platform/arch combinations + `checksums.txt`).
- `README.md` / `README_PT_BR.md` (bilingual, Hero's own tool documentation).
- Test suite: unit tests per feature package, golden-file tests for template rendering, lightweight integration tests for `install`/`upgrade`/`uninstall`/`doctor`.

## Recent Decisions

- All V1 design decisions (70+) were resolved in a grilling session on 2026-07-20; see [docs/idea/ai_workflow_hero.md](../docs/idea/ai_workflow_hero.md) for the full log.
- Core documents created: `AGENTS.md`, `docs/product/PRD.md`, `docs/product/UI.md`, `docs/architecture/ADR.md`, `docs/deployment/DEPLOY.md`.
- `/hero:sync` (basic activation of Hero in existing projects) promoted from V2 to V1.

## Known Technical Debt

- None yet (no code exists). Track here once implementation starts.

## Next Steps

1. Scaffold the Go module and `internal/` package layout per ADR-002.
2. Implement `hero install --tools cursor` first (all other CLI commands depend on the installed layout it creates).
3. Author the embedded asset files (agent prompts, Runtime commands, templates) referenced throughout the PRD/ADR/DEPLOY docs.
4. Set up `go test ./...` in CI-equivalent local workflow per the Testing section of `AGENTS.md`.

---

_To be maintained by Hero agents. Update after every cycle; keep this consolidated, not a changelog._
