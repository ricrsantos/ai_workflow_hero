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
| **Phase** | V1 complete; OpenSpec change `browser-ui-validation` implemented (ready to archive). Default CLI version `0.6.1`. |

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
- Install: git prerequisite (`--git-init` / huh confirm), name/summary flags or prompts, asset materialization, `hero.json` / `project.json` / `documents.json`, checksum tracking, `metrics-summary.md`, soft secrets hygiene (`.env.example` + `.gitignore` patterns), end-user guide at `.workflow-hero/docs/workflow-help.md` (path printed after successful install).
- Upgrade: checksum-based non-overwrite of customized files with warnings; also ensures env hygiene files/patterns; refreshes `docs/workflow-help.md` when not customized.
- Uninstall: removes only Hero-owned paths; preserves `AGENTS.md`, `context/`, `docs/`, `openspec/`, `.env.example`, `.gitignore`.
- Doctor / status / variables: table default + `--json`; doctor warn-only checks for secrets hygiene (tracked `.env`, missing `.env.example` / `.env` ignore).
- `update-models`: fetches structured upstream model YAML (HTTP client injectable for tests).
- Template renderer + inventory / Runtime-semantics asset tests.
- Embedded Runtime assets: 13 `hero-*.md` commands, **11 agents** (incl. `browser_ui_agent`; Cursor YAML frontmatter with `model: inherit`), skills (`workflow-hero`, `grilling`), templates, 7 model pricing files, bilingual end-user guide (`assets/docs/workflow-help.md`); metrics use executable Metrics Procedure + subagent `input_chars`/`output_chars` contracts; **Model Resolution** builds **kebab Task slugs** from `workflow-config.yml`; stage order **QA → Judge → Browser UI Validation → QA End-to-End**; **Browser UI Validation** (`stages.browser_ui_validation`, default off) — Playwright Health + optional Visual vs PNGs (`visual_validation`, default `docs/ui/visual_reference`); requires `scope.frontend`; artifacts under `.workflow-hero/cycles/current/browser-ui/`; **QA End-to-End** Playwright journeys remain via `use_playwright` (distinct); **Logging standard**; **Clean Session Handoff**.
- `scripts/release.sh` + contract test for artifact naming / platforms / checksums.
- `scripts/build_dev.sh` for local cross-compiles without a release tag (version `<latest-tag>_<short-commit>`).
- Integration tests for install/upgrade/uninstall/doctor against `t.TempDir()`.
- Bilingual project README (`README.md`, EN + PT-BR in one file, Screenshot Hero style).

## Pending Features

- Archive OpenSpec change `browser-ui-validation` when ready.
- Create GitHub Release for `v0.6.1` and upload `dist/` artifacts (binaries + checksums).
- Optional further enrichment of Runtime narrative prompts.
- Other post-V1 / V2 priorities not yet selected (see PRD §2.3).

## Recent Decisions

- Model pricing catalog (2026-07-28): `moonshot.yml` now has `kimi-k2.7-code`, `kimi-k3`, `kimi-k3-max`; `zhipu.yml` has `glm-5.2`, `glm-5.2-high` (Cursor docs rates; Task effort variants included for metrics lookup). Patch bump to `0.6.1`.
- Browser UI Validation (2026-07-28): new stage after Judge; Health always-on when enabled; Visual optional (agent vision); no `base_url`/`screens.yml`; failure routing front/back; SemVer `0.6.0`.
- Clickable chat links (2026-07-28): init review and metrics summaries must use markdown `[path](path)` so Cursor opens the file on click.
- Archive folder date (2026-07-28): `C<N>-YYYY-MM-DD-<slug>` uses `workflow.md` **Completed** (set on `/hero:finish` via `date +%Y-%m-%d`), not a guessed “today”.
- Task Model Resolution (2026-07-28): Cursor Task rejects bracket slugs; Hero builds kebab variants (`cursor-grok-4.5-high`).
- Clean Session Handoff (2026-07-28): after `/hero:init`, soft guidance to open a new empty chat, then `/hero:start`.
- Go module path: `github.com/ricrsantos/ai_workflow_hero` (from git remote).
- Subagent models: agent frontmatter stays `inherit`; effective model is Task `model` from per-cycle `workflow-config.yml` (ADR-005 / ADR-008).
- Soft secrets hygiene: commit `.env.example` only; doctor warns, does not block.
- CLI default version `0.6.1`.

## Known Technical Debt

- Runtime asset prompts remain concise; fuller narrative prompts from `docs/idea/ai_workflow_hero.md` can be deepened later without changing CLI APIs. Metrics still agent-estimated, not API usage.
- Cursor may still override Task/`frontmatter` models on some plans (known IDE limits).
- `update-models` upstream URL assumes `main` branch raw assets on this GitHub repo.
- Global `--verbose`/`--debug` are registered but not yet wired into panic/stack-trace printing paths.

## Next Steps

1. Archive OpenSpec change `browser-ui-validation` (`/opsx:archive`).
2. Create GitHub Release for `v0.6.1` and upload `dist/` (4 binaries + `checksums.txt`).
3. Optionally deepen other Runtime prompt content.

---

_To be maintained by Hero agents. Update after every cycle; keep this consolidated, not a changelog._
