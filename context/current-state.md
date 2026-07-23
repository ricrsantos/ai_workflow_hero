# Current State

> Long-lived document. Single source of truth for **this repository** (the Hero CLI + Runtime assets themselves — not a project that uses Hero).
>
> Must be updated after every implementation cycle. Keep this document under 2,000 words by consolidating information and removing obsolete content. Do not keep a full history here — that belongs in `context/context-log.md` and git.

---

## Project Identity

| Field | Value |
|---|---|
| **Name** | AI Workflow Hero (Hero) |
| **Repository** | `github.com/ricrsantos/ai_workflow_hero` |
| **Goal** | Open-source framework that coordinates specialized AI subagents, organizes project artifacts, compresses context, and makes AI-driven development cycles reproducible and less dependent on any single LLM provider. |
| **License** | BSD-2-Clause |
| **Phase** | V1 implementation complete for OpenSpec change `v1-ai-workflow-hero` (`go test ./...` green). Ready to archive the change. |

## Technology Stack

| Concern | Choice |
|---|---|
| Language | Go |
| Module path | `github.com/ricrsantos/ai_workflow_hero` |
| CLI framework | [Cobra](https://github.com/spf13/cobra) |
| Asset embedding | Go `embed.FS` (`assets` package) |
| Interactive prompts | [charmbracelet/huh](https://github.com/charmbracelet/huh) |
| YAML | `gopkg.in/yaml.v3` |
| SDD / planning framework | [OpenSpec](https://github.com/Fission-AI/OpenSpec) |
| Target IDE/harness (V1) | Cursor only |
| Target platforms (V1) | Linux and macOS, `amd64` and `arm64` |
| Versioning | SemVer (`vMAJOR.MINOR.PATCH`), injected via `-ldflags "-X main.version=..."` |

## Architecture Summary

- **Feature Based + Vertical Slice**: `cmd/hero` + `internal/<feature>/` (`install`, `upgrade`, `uninstall`, `doctor`, `status`, `variables`, `update_models`) with `command.go` / `service.go` / `validator.go` as needed; Cursor paths in `internal/adapters/cursor/`; shared helpers in `internal/common/` (clierr, output, template).
- **Strict CLI vs Runtime**: CLI is deterministic only; Runtime orchestration lives in embedded markdown under `assets/cursor/`.
- **Simple templating**: `internal/common/template` supports `{{path.key}}` only (ADR-006).
- **Assets**: `assets/` embedded via `assets.FS`; install copies into `.cursor/` and `.workflow-hero/`.

## Implemented Features

- CLI commands: `install --tools cursor`, `upgrade`, `uninstall`, `doctor`, `version`, `variables`, `update-models`, `status`, `help` (plus global `--verbose`/`--debug`).
- Install: git prerequisite (`--git-init` / huh confirm), name/summary flags or prompts, asset materialization, `hero.json` / `project.json` / `documents.json`, checksum tracking, `metrics-summary.md`.
- Upgrade: checksum-based non-overwrite of customized files with warnings.
- Uninstall: removes only Hero-owned paths; preserves `AGENTS.md`, `context/`, `docs/`, `openspec/`.
- Doctor / status / variables: table default + `--json`.
- `update-models`: fetches structured upstream model YAML (HTTP client injectable for tests).
- Template renderer + inventory / Runtime-semantics asset tests.
- Embedded Runtime assets: 13 `hero-*.md` commands, 10 agents (Cursor YAML frontmatter with `model: inherit`), skills (`workflow-hero`, `grilling`), templates, 7 model pricing files; metrics use executable Metrics Procedure + subagent `input_chars`/`output_chars` contracts; **Model Resolution** requires Task `model` from `workflow-config.yml` on every subagent call.
- `scripts/release.sh` + contract test for artifact naming / platforms / checksums.
- Integration tests for install/upgrade/uninstall/doctor against `t.TempDir()`.
- Bilingual project README (`README.md`, EN + PT-BR in one file, Screenshot Hero style).

## Pending Features

- Archive OpenSpec change `v1-ai-workflow-hero` (`/opsx:archive`).
- First tagged release via `scripts/release.sh` + GitHub Release upload.
- Optional further enrichment of Runtime narrative prompts (stage flow, approval, metrics procedure, and Task isolation are encoded; metrics estimation remains agent-driven).

## Recent Decisions

- Go module path: `github.com/ricrsantos/ai_workflow_hero` (from git remote).
- Interactive prompts: `charmbracelet/huh` (not survey).
- OpenSpec change `v1-ai-workflow-hero` implemented; all 42 tasks marked complete; `go test ./...` green.
- Subagent models: agent frontmatter stays `inherit`; effective model is Task `model` from per-cycle `workflow-config.yml` (ADR-005 / ADR-008). UI may still show Inherit; execution must pass Task `model`.

## Known Technical Debt

- Runtime asset prompts remain concise; fuller narrative prompts from `docs/idea/ai_workflow_hero.md` can be deepened later without changing CLI APIs. Metrics now have an executable Metrics Procedure + subagent `input_chars`/`output_chars` contract (still agent-estimated, not API usage).
- Cursor may still override Task/`frontmatter` models on some plans (known IDE limits); Hero cannot bypass that from Runtime prompts alone.
- `update-models` upstream URL assumes `main` branch raw assets on this GitHub repo; first publish must keep that layout stable.
- Global `--verbose`/`--debug` are registered but not yet wired into panic/stack-trace printing paths.

## Next Steps

1. Archive `v1-ai-workflow-hero` with `/opsx:archive`.
2. Tag `v0.1.0` (or `v1.0.0`) and run `./scripts/release.sh`.
3. Optionally deepen Runtime prompt content.

---

_To be maintained by Hero agents. Update after every cycle; keep this consolidated, not a changelog._
