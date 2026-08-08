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
| **Phase** | Hero **1.0.0** + **C2 complete**; **C3 implementation complete** (2026-08-08). OpenSpec `cursor-harness-tui-autonomy` tasks 1–3 all checked; QA fixes for cycles TUI + hyphen slash copy. |

## Technology Stack

| Concern | Choice |
|---|---|
| Language | Go |
| Module path | `github.com/ricrsantos/ai_workflow_hero` |
| CLI framework | [Cobra](https://github.com/spf13/cobra) |
| Asset embedding | Go `embed.FS` (`assets` package) |
| Interactive prompts | [charmbracelet/huh](https://github.com/charmbracelet/huh) |
| TUI | [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) + [lipgloss](https://github.com/charmbracelet/lipgloss) |
| Operational store | SQLite (`modernc.org/sqlite`) |
| YAML | `gopkg.in/yaml.v3` |
| SDD / planning framework | [OpenSpec](https://github.com/Fission-AI/OpenSpec) |
| Target IDE/harness (V1) | Cursor only |
| Target platforms (V1) | Linux and macOS, `amd64` and `arm64` |
| Versioning | SemVer (`vMAJOR.MINOR.PATCH`), injected via `-ldflags "-X main.version=..."` |

## Architecture Summary

- **Feature Based + Vertical Slice**: `cmd/hero` + `internal/<feature>/` (`install`, `upgrade`, `uninstall`, `doctor`, `status`, `variables`, `update_models`, `cycle`, `store`, `engine`, `tui`, `harness`) with `command.go` / `service.go` as needed; Cursor adapter in `internal/adapters/cursor/`; shared helpers in `internal/common/` (clierr, output, template).
- **Strict CLI vs Runtime**: CLI is deterministic only; Runtime orchestration lives in embedded markdown under `assets/cursor/`.
- **Simple templating**: `internal/common/template` supports `{{path.key}}` only (ADR-006).
- **Assets**: `assets/` embedded via `assets.FS`; install copies into `.cursor/` and `.workflow-hero/`.

## Implemented Features

- CLI commands: `install --tools cursor`, `upgrade`, `uninstall`, `doctor`, `version`, `variables`, `update-models`, `status`, `help`, plus Hero 1.0 operational API (`metrics`, `events`, `approve`, `reject`, `cancel`, `finish`, `continue`, `stage`, `cycle`, `run`, `tui`) (plus global `--verbose`/`--debug`). Cycle API includes `hero cycle openspec-change` / `--clear` and `hero cycle archive --force|--skip-openspec|--openspec-change`.
- **Bubble Tea TUI** (`hero tui`): Status, Approvals, Artifacts, Costs, Events, **Conversation (Chat)** screens; command palette with `/hero-<name>` action labels + imported non-Hero Cursor commands (markdown expansion → Dispatch); empty-state `/hero-new`; in-process `cycle.Service`; refuses launch when `NO_COLOR` or non-TTY. Conversation screen streams harness `stream-json` via `Execute`, persists `harness_session_id` per etapa, Ctrl+C → `Cancel`.
- **SQLite operational store** (schema v3: `cycles.openspec_change`, `stages.harness_session_id`) + workflow engine + CLI-as-API cycle service with OpenSpec-coupled archive.
- **HarnessAdapter (full)**: `IsAvailable`, sessions, `Execute`/`Cancel`/`Status`, `Dispatch`→Execute; Cursor adapter runs Agent CLI (`cursor-agent` / `cursor agent`) with json/stream-json parsers and injectable `CommandRunner`.
- Install: git prerequisite (`--git-init` / huh confirm), name/summary flags or prompts, asset materialization, `hero.json` / `project.json` / `documents.json`, checksum tracking, `metrics-summary.md`, soft secrets hygiene, harness-marker warn-only suggestions; end-user guide at `.workflow-hero/docs/workflow-help.md`.
- Upgrade: checksum-based non-overwrite of customized files with warnings; also ensures env hygiene files/patterns; refreshes `docs/workflow-help.md` when not customized.
- Uninstall: removes only Hero-owned paths; preserves `AGENTS.md`, `context/`, `docs/`, `openspec/`, `.env.example`, `.gitignore`.
- Doctor / status / variables: table default + `--json` (`openspec_change` in status); doctor warn-only checks for secrets hygiene, unsupported harness markers (`.claude/` / `.windsurf/` / `.codex/`), and **Cursor Agent CLI on PATH + login hint** (complementary to TUI boot; PRD-C03-001 §4.10).
- `update-models`: fetches structured upstream model YAML (HTTP client injectable for tests).
- Template renderer + inventory / Runtime-semantics asset tests.
- Embedded Runtime assets: 15 `hero-*.md` commands (incl. `hero-cycles`, `hero-todos`), **11 agents** (incl. `browser_ui_agent`; Cursor YAML frontmatter with `model: inherit`), skills (`workflow-hero`, `grilling`), templates, 7 model pricing files, bilingual end-user guide (`assets/docs/workflow-help.md`); metrics use executable Metrics Procedure + subagent `input_chars`/`output_chars` contracts; **Model Resolution** builds **kebab Task slugs** from `workflow-config.yml`; each `agents.<name>` may nest a **`subagent`** block (`same_of_agent` + model fields) for nested generic Task fan-out (named Hero agents keep their own top-level model); stage order **QA → Judge → Browser UI Validation → QA End-to-End**; **Browser UI Validation** (`stages.browser_ui_validation`, default off) — Playwright Health + optional Visual vs PNGs (`visual_validation`, default `docs/ui/visual_reference`); requires `scope.frontend`; artifacts under `.workflow-hero/cycles/current/browser-ui/`; **QA End-to-End** Playwright journeys remain via `use_playwright` (distinct); **Logging standard**; **Clean Session Handoff**.
- `scripts/release.sh` + contract test for artifact naming / platforms / checksums.
- `scripts/build_dev.sh` for local cross-compiles without a release tag (version `<latest-tag>_<short-commit>`).
- Integration tests for install/upgrade/uninstall/doctor against `t.TempDir()`.
- Bilingual project README (`README.md`, EN + PT-BR in one file, Screenshot Hero style).

## Pending Features

- Archive OpenSpec change `cursor-harness-tui-autonomy` after Judge/finish when ready; archive `hero-1-0` when convenient.
- Tag/publish GitHub Release `v1.0.0` / C3 when ready.
- Post-1.0 deferred D1–D13 (multi-harness adapters, integrations, daemon/RPC, etc.).
- Note: intermediate tags `v0.6.0`–`v0.7.0` never published on GitHub; `v0.8.0` / `v0.9.0` published.

## Recent Decisions

- Cycle C3 Judge fix (2026-08-08): TUI `/hero-todos` uses `todos.ReadProject`/`Format` (no duplicate parser); `RequiredCommandFiles` includes `hero-cycles.md` + `hero-todos.md` (15 total).
- Cycle C3 QA fix (2026-08-08): TUI `/hero-cycles` uses `cycle.Service.Cycles()` + `FormatCycles()` (archive-only cycles); user-facing `/hero:start`/`/hero:new` → hyphen in CLI copy; all OpenSpec tasks marked done.
- Cycle C3 implementation (2026-08-08): harness + Cursor CLI runner; TUI boot/conversation; hyphen slash palette; `/hero-cycles`/`/hero-todos`; sync pending-doc scan; doctor Cursor CLI checks (ADR-024–029).
- Cycle C2 complete (2026-08-08): slash-first Runtime/TUI; Cursor command import; harness detect warn-only; OpenSpec-coupled archive (ADR-020–023).
- Cycle C1 complete (2026-08-07): Hero 1.0 — SQLite, AI Loop, CLI-as-API, TUI, Cursor adapter.
- ADRs 012–029; prior 0.9.x Runtime conventions.

## Known Technical Debt

- Runtime asset prompts remain concise; fuller narrative prompts from `docs/idea/ai_workflow_hero.md` can be deepened later without changing CLI APIs. Metrics still agent-estimated, not API usage.
- Cursor may still override Task/`frontmatter` models on some plans (known IDE limits).
- `update-models` upstream URL assumes `main` branch raw assets on this GitHub repo.
- Global `--verbose`/`--debug` are registered but not yet wired into panic/stack-trace printing paths.

## Next Steps

1. Judge / finish C3; archive OpenSpec `cursor-harness-tui-autonomy` (and leftover `hero-1-0` if needed).
2. Tag/publish GitHub Release `v1.0.0` when ready.
3. Post-1.0: deferred D1–D13.

---

_To be maintained by Hero agents. Update after every cycle; keep this consolidated, not a changelog._
