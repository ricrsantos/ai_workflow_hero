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
| **Phase** | Hero **2.6.0** released (tag `v2.6.0` on GitHub). TUI redesign + mid-cycle config update; C6 Codex shipped; QA/Judge next for cycle archive. |

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
| Target IDE/harness (V1) | Cursor (IDE Runtime + CLI adapter) |
| Target harnesses (2.0 TUI) | Cursor + OpenCode + **Codex** (C6 complete → Hero 2.5.0) |
| Target platforms (V1) | Linux and macOS, `amd64` and `arm64` |
| Versioning | SemVer via `-ldflags "-X main.version=..."` |

## Scope (implementation routing)

| Field | Value |
|---|---|
| `backend` | `true` — Go CLI and `internal/*` packages |
| `frontend` | `false` — no browser/web UI |
| `native` | `true` — Go CLI, TUI, harness adapters |
| `script` | `true` — `scripts/release.sh`, `scripts/build_dev.sh` |
| `infrastructure` | `true` — embed.FS distribution, cross-compile, DEPLOY |

## Architecture Summary

- **Feature Based + Vertical Slice**: `cmd/hero` + `internal/<feature>/` (`install`, `upgrade`, `uninstall`, `doctor`, `status`, `variables`, `update_models`, `cycle`, `store`, `engine`, `tui`, `harness`, `harnessmgr`, `todos`, `workflowconfig`) + `internal/adapters/cursor/` + `internal/adapters/opencode/` + `internal/adapters/codex/` + `internal/common/` (includes `assetconflict` for upgrade conflict backup/replace).
- **Strict CLI vs Runtime**: CLI is deterministic; orchestration lives in embedded `assets/cursor/`.
- **Simple templating**: `internal/common/template` — `{{path.key}}` only (ADR-006).
- **Assets**: embedded via `assets.FS`; install copies into `.cursor/` (when Cursor enabled), `.opencode/` (when OpenCode enabled), `.codex/` (when Codex enabled — full agents/commands/skills mirroring OpenCode; no AGENTS.md / no config.toml), and `.workflow-hero/`. Codex `SKILL.md` files include required YAML frontmatter (`name`, `description`) for Codex skill discovery.
- **Multi-harness (C4/C5)**: `hero.json` → `harnesses.<id>.enabled`, `freechat_default {harness, model}`, and C5 `model_properties`; `workflow-config.yml` requires `harness` on every agent + `fallback_model`; `internal/harnessmgr` registry + fallback chain (ADR-033); SQLite schema **v6** (`harness_serve_registry` + `project_path`, `stages.harness_id`, project model-list/capability cache).

## Implemented Features

- CLI commands: `hero install` (interactive harness multi-select; **`--tools` removed** — explicit error on install/upgrade), `upgrade`, `uninstall`, `doctor`, `version`, `variables`, `update-models`, `status`, `help`, plus Hero operational API (`metrics`, `events`, `approve`, `reject`, `cancel`, `finish`, `continue`, `stage`, `cycle`, `run`, `tui`, **`chat`**) (plus global `--verbose`/`--debug`). Cycle API includes `hero cycle openspec-change` / `--clear` and `hero cycle archive --force|--skip-openspec|--openspec-change`.
- **`hero chat`**: free-chat-only TUI (no project install/git). Config in `~/.workflow-hero/`; Execute cwd; Chat nav only; palette filter excludes `/hero*` and Go to. Global TUI palette: `/model`, `/harness`, `/hero-refresh` (before Go to).
- **Bubble Tea TUI** (`hero tui`): Left nav sidebar (SESSIONS-style) with fixed **AI Hero** title, dim `─` separators, live **agents** block (2 rows: count + labels), then Chat/Status/Artifacts/Costs/Events (`alt+1`–`alt+5`; boot Chat; hidden below ~80 cols — agents box falls back into chat header when sidebar hidden); Bonito dark palette (`#0B0E1A` / violet-blue accents). Palette `/hero-*` + `/new-chat` + `/model` + `/harness` + imported Cursor commands; `/hero-config-update` reloads `hero.json` freechat + active `workflow-config.yml` agent pair into Chat input labels (no harness Execute); `/harness-reset` stops managed OpenCode serve / Codex app-server / Cursor cancel. Orchestrator Execute uses `agents.orchestration_agent` (then `fallback_model`, then `/model`); freechat/`/hero-new` use `/model`. `/hero-start` validates/syncs YAML and prompt assets asynchronously, supports cancellation during preflight, batches stream events, and caches wrapped transcript lines; when OpenCode/Codex agents are used, syncs `.opencode`/`.codex` agents, resets serve/app-server (2s), probes — failure stops start. Research uses dedicated `discover_agent` session; control slashes stay on orchestrator. Chat: **linear borderless transcript** (full session; thin `│` accent per actor — user violet / agent green) with `You` and `[LABEL - model · harness]` labels, plus bordered composer (Build/Plan Tab); auto-follow scroll while streaming; agents box; context bar (harness `result.usage` preferred, else chars÷4 estimate); when a cycle stage is active, each TUI Execute accumulates tokens into SQLite Costs; Alt+y/r/i copy; Ctrl+C Cancel; streaming nav + destructive `[y/N]` confirm. The footer is fixed to `tab mode · / commands · enter send · alt+enter newline · alt+y/r/i copy · ↑↓ scroll · ctrl+q quit`, wrapping by hint groups, reserving its rows in the frame, and retaining all footer rows on short terminals. Status sits directly under the chat panes (no top rule); one `colorBorder` rule separates status from the footer.
- **SQLite operational store** (schema **v4**: `harness_serve_registry`, `stages.harness_id`, plus v3 fields) + workflow engine + CLI-as-API cycle service (`hero cycle new` = prepare active cycle with deferred title/objective; `hero cycle sync-config` syncs meta **and still-open stage budgets** from YAML before `/hero-start`) with OpenSpec-coupled archive (`openspec` resolved via PATH plus nvm/fnm/volta/user bins; skip OpenSpec CLI when the linked change dir is already archived). Stage close/approve merge prefers TUI-accumulated harness tokens over agent `--metrics-json` estimates.
- **HarnessAdapter (full)**: Shared `StreamDelta` + TUI warn-only watchdog (Cursor 5m / OpenCode+Codex **3m**). **Cursor** NDJSON (incl. `result.usage`); **OpenCode** SSE + serve lifecycle + `PrepareHeroStart` + `info.tokens` → `Usage`; **Codex** (`internal/adapters/codex`) Hero-managed `codex app-server` stdio JSON-RPC — lazy start, SQLite registry (no URL), stream/permission/auth/`ListModels`/C5 map, orphan reap, `ResetAppServer` + `PrepareHeroStart`. Codex notify delivery is queued off the stdout `readLoop` (avoids stdio pipe deadlock under TUI backpressure); transcript-critical methods (`item/agentMessage/delta`, `item/completed`, `turn/completed`, `thread/tokenUsage/updated`, …) use ordered blocking enqueue; Chat stream channel buffer 512 with lossless text/thinking delivery (tool/activity may drop under lag). Agent text: raw deltas + authoritative `item/completed` / optional `turn/completed` `lastAgentMessage` repair; `executeDone` reconciles parent transcript from `result.Output`; reasoning → thinking on completed only; noisy activities / unrecognized warnings only with `hero --debug`. Green response pane shows `↑/↓ more` when scrolled and `…` when a row is hard-clipped. `harnessmgr` resolves cursor+opencode+codex; fallback chain ADR-033; no cross-harness session resume.
- Model catalogs: `assets/models/*.yml` pricing + C5 `properties`; OpenCode 27 models; Cursor includes `auto`; Codex-native ids without invented ChatGPT USD rates.
- Install: interactive harness picker (≥1 required; Cursor / OpenCode / **Codex**); conditional projections; 2.4.x → 2.5.0 never auto-provisions `.codex/` (ADR-048).
- Doctor / status / variables: table + `--json`; warn-only Cursor/OpenCode/Codex CLI checks when enabled; `.codex/` supported marker (C6).
- Upgrade: checksum conflict backup/replace; env hygiene; refreshes `docs/workflow-help.md` when not customized.
- Uninstall: Hero-owned `.codex/` / `.opencode/` trees only; preserves user `config.toml`, `AGENTS.md`, `context/`, `docs/`, `openspec/`.
- `update-models`: upstream YAML fetch + conflict backup; updates `checksums.json`.
- **`/hero-harness`** / **`/hero-model`**: Codex enable→`.codex/` projection; model step lists native ids (Codex may start app-server); C5 property submenu; Chat `[LABEL - model · harness]` / `Build · model · harness` follow the **active execute pair** (`runtimeHarnessID` + `runtimeModelSlug` from agent YAML / `ResolveExecutePair`), not a cross-mix with freechat; UI-C06-001 §6 goldens.
- Embedded Runtime: Cursor + `assets/opencode/` + `assets/codex/` (no AGENTS.md / no Codex config template).
- C5 model properties: `internal/harness` + `internal/modelprops`; catalogs carry `properties` for Cursor base + OpenCode 27 + Codex ids.
- `scripts/release.sh` + `build_dev.sh` + contract tests; latest release **2.6.0**; integration tests include C6 Codex path.
- Test strategy in [docs/testing/TESTING.md](docs/testing/TESTING.md); bilingual README.

## Pending Features

- **Windows CLI** — out of scope for Hero 2.0; planned for a future major (PRD §7; DEPLOY.md).
- **CI/CD release automation and GPG-signed artifacts** — no GitHub Actions / GoReleaser pipeline in 2.0; manual `scripts/release.sh` only (ADR-010; PRD §7).
- **Additional harness adapters** — Claude Code, VS Code, and other IDEs remain deferred (Codex shipped in C6 / 2.5.0).
- **C6 QA/Judge** — next after Implementation close; Browser UI / E2E skipped by scope.
- **Post-1.0 deferred D2–D13** — e.g. external integrations, notification manager, daemon/RPC `hero serve`, full event bus (PRD-C01-001 §4).

## Recent Decisions

- **2026-08-21 — `/hero-start` responsiveness**: Removed synchronous status/config/filesystem bootstrap from the Bubble Tea `Update` loop. Preflight and OpenCode/Codex preparation now run as cancellable commands; harness deltas are coalesced in short batches and transcript wrapping/style output is cached per message.
- **C6 Implementation complete (2026-08-21)**: Codex TUI adapter native (§1–§9); SemVer **2.5.0**; OpenSpec `codex-adapter` bound on C6. Upgrade 2.4.x never auto-enables Codex (ADR-048).
- **C5 archived (2026-08-18)**: `model-properties-tui`; Judge user-approved without formal SDD verify (`judge_agent` empty in opencode harness).
- **C4 closed (2026-08-15)**: Hero 2.0.0 multi-harness (Cursor+OpenCode TUI).
- ADR-043–048 (C6 Codex); ADR-038–042 (C5 properties); ADR-033–037 (C4 multi-harness); ADR-030 amended for orchestrator Execute YAML model.

## Known Technical Debt

- No GitHub Actions / CI/CD release automation in V1 (ADR-010; deferred to V2 GoReleaser or equivalent).
- GPG-signed release artifacts deferred to V2 (PRD §7; DEPLOY.md).
- Upstream Cursor CLI gaps accepted as limitations: plugin skills, nested skill dirs (ADR-C02); nested Task assistant text is not documented in `stream-json` (TUI attributes best-effort while a Task is open, else prints `result.content`).
- Task-dispatched IDE subagents still lack harness usage in Hero; Metrics Procedure (chars÷4) remains required for those stages. TUI-direct Executes prefer adapter usage.
- Global `--verbose`/`--debug` registered but not yet wired into panic/stack-trace printing paths.
- `update-models` upstream URL assumes `main` branch raw assets on this GitHub repo.
- `.workflow-hero/config/documents.json` omits living PRD/UI index docs (`docs/product/PRD.md`, `docs/product/UI.md`).
- Cursor may still override Task/frontmatter models on some plans (known IDE limits).

## Next Steps

1. C6 QA → Judge (Browser UI / E2E skipped by scope); **v2.6.0** already published on GitHub Releases.
2. Archive C6 with `/hero-archive` when ready (OpenSpec `codex-adapter`).

---

_To be maintained by Hero agents. Update after every cycle; keep this consolidated, not a changelog._
