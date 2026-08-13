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
| **Phase** | Hero **1.0.2** patch release; **C3 archived** (2026-08-09). No active cycle. |

## Technology Stack

| Concern | Choice |
|---|---|
| Language | Go 1.25+ |
| Module path | `github.com/ricrsantos/ai_workflow_hero` |
| CLI framework | [Cobra](https://github.com/spf13/cobra) |
| Asset embedding | Go `embed.FS` (`assets` package) |
| Interactive prompts | [charmbracelet/huh](https://github.com/charmbracelet/huh) |
| TUI | [bubbletea](https://github.com/charmbracelet/bubbletea) + [lipgloss](https://github.com/charmbracelet/lipgloss) |
| Operational store | SQLite (`modernc.org/sqlite`) |
| YAML | `gopkg.in/yaml.v3` |
| SDD / planning | [OpenSpec](https://github.com/Fission-AI/OpenSpec) |
| Target IDE/harness (V1) | Cursor only |
| Target platforms (V1) | Linux and macOS, `amd64` and `arm64` |
| Versioning | SemVer via `-ldflags "-X main.version=..."` |

## Scope (implementation routing)

| Field | Value |
|---|---|
| `backend` | `true` — Go CLI and `internal/*` packages |
| `frontend` | `false` — no browser/web UI |
| `native` | `false` |
| `script` | `true` — `scripts/release.sh`, `scripts/build_dev.sh` |
| `infrastructure` | `true` — embed.FS distribution, cross-compile, DEPLOY |

## Architecture Summary

- **Feature Based + Vertical Slice**: `cmd/hero` + `internal/<feature>/` (`install`, `upgrade`, `uninstall`, `doctor`, `status`, `variables`, `update_models`, `cycle`, `store`, `engine`, `tui`, `harness`, `todos`, `workflowconfig`) + `internal/adapters/cursor/` + `internal/common/`.
- **Strict CLI vs Runtime**: CLI is deterministic; orchestration lives in embedded `assets/cursor/`.
- **Simple templating**: `internal/common/template` — `{{path.key}}` only (ADR-006).
- **Assets**: embedded via `assets.FS`; install copies into `.cursor/` and `.workflow-hero/`.

## Implemented Features

- CLI commands: `install --tools cursor`, `upgrade`, `uninstall`, `doctor`, `version`, `variables`, `update-models`, `status`, `help`, plus Hero 1.0 operational API (`metrics`, `events`, `approve`, `reject`, `cancel`, `finish`, `continue`, `stage`, `cycle`, `run`, `tui`) (plus global `--verbose`/`--debug`). Cycle API includes `hero cycle openspec-change` / `--clear` and `hero cycle archive --force|--skip-openspec|--openspec-change`.
- **Bubble Tea TUI** (`hero tui`): Status, Approvals, Artifacts, Costs, Events, **Conversation (Chat)** screens; command palette with `/hero-<name>` action labels (`Go to - *` for screen jumps) + imported non-Hero Cursor commands (markdown expansion → Dispatch); palette closes on select with **busy-guard**; fixed **footer status bar** (running/ok/error, **2** wrapped lines); empty-state `/hero-new`; in-process `cycle.Service`; refuses launch when `NO_COLOR` or non-TTY. **`/hero-new`** opens Chat and streams `hero-new.md` via harness with the **default model**; on success TUI calls **`PrepareCycle()`** (active SQLite cycle, empty title/objective until `/hero-start`). **`/hero-start`** requires active cycle; calls **`SyncCycleConfig()`** then streams `orchestration_agent.md` + `hero-start.md` with model from `workflow-config.yml` → `agents.orchestration_agent` (TUI-only; no `/hero-model` gate). **`/hero-approve`** requires active cycle + stage `PendingApproval`; streams `orchestration_agent.md` + `hero-approve.md` with the same orchestrator model; agent persists via `hero approve --metrics-json`. **`/hero-reject`** requires active cycle + stage `PendingApproval`; collects rejection reason in Chat (or inline `/hero-reject <reason>`), then streams `orchestration_agent.md` + `hero-reject.md` with the same orchestrator model; agent calls `hero reject --reason` and re-dispatches the stage agent. **`/hero-cancel`** requires active cycle; streams `orchestration_agent.md` + `hero-cancel.md`; agent runs `hero cancel` + git rollback (optional inline `/hero-cancel <reason>`). **`/hero-finish`** requires active cycle; streams orquestrator + `hero-finish.md`; agent validates stages, `hero finish --metrics-json`, updates `context/*`. **`/hero-continue`** requires Escalated stage; `/hero-continue [N]` (default 1); streams orquestrator + `hero-continue.md`; agent grants extra iterations and resumes stage. **`/hero-back`** requires Judge `PendingApproval`; streams orquestrator + `hero-back.md`; agent reopens Planning via Task and re-runs pipeline. **`/hero-sync`** streams orquestrator + `hero-sync.md` (model YAML or fallback `/hero-model`); agent runs context sync via Task `context_agent`, writes artifacts, `hero doctor`. **`/hero-status`** streams orquestrator + `hero-status.md`; agent relays full `hero status` table. **`/hero-archive`** requires active cycle; streams orquestrator + `hero-archive.md`; agent runs `hero cycle archive` (+ optional `metrics-summary.md`). **`/hero-resume`** streams orquestrator + `hero-resume.md`; `/hero-resume [N]` inline; agent runs `hero cycle resume` + post `hero status`. **`/hero-model`** selects the default harness model (Chat + non-agent dispatches); persisted to `hero.json` → `harnesses.<tool>.model`. Conversation uses dual OpenCode-style panes: **input** (Build/Plan via **Tab**; Plan → `--mode plan`; scrollable) and **response** (green accent; height = leftover terminal space after fixed chrome; fg-only in-box text to avoid black gutters) with braille wait animation in content while harness streams; boot lists harness models (`agent models`); Chat `Execute` and `Dispatch` pass `--model` + optional `--mode`; freechat without active etapa (in-memory session); with etapa persists `harness_session_id`; Ctrl+C → `Cancel`. Palette Dispatch starts a **fresh** agent session (no leaked `--resume`).
- **SQLite operational store** (schema v3: `cycles.openspec_change`, `stages.harness_session_id`) + workflow engine + CLI-as-API cycle service (`hero cycle new` = prepare active cycle with deferred title/objective; `hero cycle sync-config` syncs meta from YAML before `/hero-start`) with OpenSpec-coupled archive.
- **HarnessAdapter (full)**: `IsAvailable`, sessions, `Execute`/`Cancel`/`Status`, `Dispatch`→Execute; Cursor adapter runs Agent CLI (`cursor-agent` / `cursor agent`) with json/stream-json parsers; stream mode uses `--stream-partial-output` + `StreamingCommandRunner` (live stdout pipe → `ParseStreamJSON`); `OnStreamDelta` emits text, thinking, and tool activity; injectable `CommandRunner`.
- Install: git prerequisite (`--git-init` / huh confirm), name/summary flags or prompts, asset materialization, `hero.json` / `project.json` / `documents.json`, checksum tracking, `metrics-summary.md`, soft secrets hygiene, harness-marker warn-only suggestions; end-user guide at `.workflow-hero/docs/workflow-help.md`.
- Upgrade: checksum-based non-overwrite of customized files with warnings; also ensures env hygiene files/patterns; refreshes `docs/workflow-help.md` when not customized.
- Uninstall: removes only Hero-owned paths; preserves `AGENTS.md`, `context/`, `docs/`, `openspec/`, `.env.example`, `.gitignore`.
- Doctor / status / variables: table default + `--json` (`openspec_change` in status); doctor warn-only checks for secrets hygiene, unsupported harness markers (`.claude/` / `.windsurf/` / `.codex/`), and **Cursor Agent CLI on PATH + login hint** (complementary to TUI boot; PRD-C03-001 §4.10).
- `update-models`: fetches structured upstream model YAML (HTTP client injectable for tests).
- Template renderer + inventory / Runtime-semantics asset tests.
- Embedded Runtime assets: 15 `hero-*.md` commands (incl. `hero-cycles`, `hero-todos`), **11 agents** (incl. `browser_ui_agent`; Cursor YAML frontmatter with `model: inherit`), skills (`workflow-hero`, `grilling`), templates (`workflow-config.yml` includes `agents.orchestration_agent` for TUI `/hero-start` model), 7 model pricing files, bilingual end-user guide (`assets/docs/workflow-help.md`); metrics use executable Metrics Procedure + subagent `input_chars`/`output_chars` contracts; **Model Resolution** builds **kebab Task slugs** from `workflow-config.yml`; each `agents.<name>` may nest a **`subagent`** block (`same_of_agent` + model fields) for nested generic Task fan-out (named Hero agents keep their own top-level model); stage order **QA → Judge → Browser UI Validation → QA End-to-End**; **Browser UI Validation** (`stages.browser_ui_validation`, default off) — Playwright Health + optional Visual vs PNGs (`visual_validation`, default `docs/ui/visual_reference`); requires `scope.frontend`; artifacts under `.workflow-hero/cycles/current/browser-ui/`; **QA End-to-End** Playwright journeys remain via `use_playwright` (distinct); **Logging standard**; **Clean Session Handoff**.
- `scripts/release.sh` + contract test for artifact naming / platforms / checksums.
- `scripts/build_dev.sh` for local cross-compiles without a release tag (version `<latest-tag>_<short-commit>`).
- Integration tests for install/upgrade/uninstall/doctor against `t.TempDir()`.
- Test strategy documented in [docs/testing/TESTING.md](docs/testing/TESTING.md) (`go test ./...`, golden tests, integration layout).
- Bilingual project README (`README.md`, EN + PT-BR in one file, Screenshot Hero style).

## Pending Features

- Publish GitHub Release **`v1.0.2`** (binaries + `checksums.txt`).
- `.workflow-hero/config/hero.json` still records `cli.version` / `assets.version` **0.9.0** while CLI default is **1.0.2** — run `hero upgrade` to align.
- Post-1.0 deferred **D1–D13** (multi-harness adapters, integrations, notification manager, rich TUI, daemon/RPC, event bus, markdown projections, etc. — see PRD-C01 §4).
- V2 scope per PRD §2.3 / §7: Windows CLI, CI/CD-automated releases, GPG-signed artifacts, non-interactive-only CLI, additional harnesses (OpenCode, Claude Code, Codex, VS Code), advanced sync/drift detection, optional stages (UX, observability, security review), AI hooks, richer project memory (RAG/DB).
- Note: intermediate tags `v0.6.0`–`v0.7.0` never published on GitHub; `v0.8.0` / `v0.9.0` published.

## Recent Decisions

- C3 archived 2026-08-09 (Cursor harness autonomy + TUI conversation).
- ADR-030 (2026-08-08): `hero.json` → `harnesses.<tool>` default model (user picks via TUI `/hero-model` on first use; no install-time slug); TUI freechat without active etapa (in-memory session); Execute passes `--model` kebab slug.
- ADR-024–029 (C3): hyphen slashes, full harness contract, TUI orchestration, hero-cycles/todos, hero-sync pending-doc scan; TUI `/hero-sync`, `/hero-status`, `/hero-archive`, `/hero-resume` aligned to Runtime Execute (2026-08-12).
- ADR-020–023 (C2): slash parity, imported commands, harness warnings, OpenSpec archive coupling.
- ADR-012–019 (C1): SQLite, AI Loop, CLI-as-API, dual UI, 0.9→1.0 breaking upgrade.

## Known Technical Debt

- `.workflow-hero/config/hero.json` still records `cli`/`assets` version **0.9.0** while source default is **1.0.2** — run `hero upgrade` to align.
- No GitHub Actions / CI/CD release automation in V1 (ADR-010; deferred to V2 GoReleaser or equivalent).
- GPG-signed release artifacts deferred to V2 (PRD §7; DEPLOY.md).
- Upstream Cursor CLI gaps accepted as limitations: plugin skills, nested skill dirs (ADR-C02).
- Runtime asset prompts remain concise; metrics are agent-estimated, not API usage.
- Global `--verbose`/`--debug` registered but not yet wired into panic/stack-trace printing paths.
- `update-models` upstream URL assumes `main` branch raw assets on this GitHub repo.
- `docs/product/UI.md` cycle index table omits C03 UI spec (`UI-C03-001-tui-harness-autonomy.md`).
- `.workflow-hero/config/documents.json` omits living PRD/UI index docs (`docs/product/PRD.md`, `docs/product/UI.md`).
- Cursor may still override Task/frontmatter models on some plans (known IDE limits).

## Next Steps

1. Tag and publish **`v1.0.2`** release.
2. Run `hero upgrade` to sync local install metadata to `1.0.2`.
3. Start next cycle via `/hero-new` or review pending work via `/hero-todos`.

---

_To be maintained by Hero agents. Update after every cycle; keep this consolidated, not a changelog._
