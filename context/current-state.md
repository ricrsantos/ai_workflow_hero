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
| **Phase** | Hero **2.1.2** released 2026-08-19 (model catalog expansion); **C5 archived** 2026-08-18 (`model-properties-tui`). **C4 archived** 2026-08-16 (`hero-2-0-multi-harness`). No active cycle. |

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
| Target harnesses (2.0 TUI) | Cursor + OpenCode (TUI Execute/Dispatch; OpenCode via `opencode serve` + HTTP API) |
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

- **Feature Based + Vertical Slice**: `cmd/hero` + `internal/<feature>/` (`install`, `upgrade`, `uninstall`, `doctor`, `status`, `variables`, `update_models`, `cycle`, `store`, `engine`, `tui`, `harness`, `harnessmgr`, `todos`, `workflowconfig`) + `internal/adapters/cursor/` + `internal/adapters/opencode/` + `internal/common/`.
- **Strict CLI vs Runtime**: CLI is deterministic; orchestration lives in embedded `assets/cursor/`.
- **Simple templating**: `internal/common/template` — `{{path.key}}` only (ADR-006).
- **Assets**: embedded via `assets.FS`; install copies into `.cursor/` (when Cursor enabled), `.opencode/` (when OpenCode enabled), and `.workflow-hero/`.
- **Multi-harness (C4/C5)**: `hero.json` → `harnesses.<id>.enabled`, `freechat_default {harness, model}`, and C5 `model_properties`; `workflow-config.yml` requires `harness` on every agent + `fallback_model`; `internal/harnessmgr` registry + fallback chain (ADR-033); SQLite schema **v5** (`harness_serve_registry`, `stages.harness_id`, project model-list/capability cache).

## Implemented Features

- CLI commands: `hero install` (interactive harness multi-select; **`--tools` removed** — explicit error on install/upgrade), `upgrade`, `uninstall`, `doctor`, `version`, `variables`, `update-models`, `status`, `help`, plus Hero operational API (`metrics`, `events`, `approve`, `reject`, `cancel`, `finish`, `continue`, `stage`, `cycle`, `run`, `tui`) (plus global `--verbose`/`--debug`). Cycle API includes `hero cycle openspec-change` / `--clear` and `hero cycle archive --force|--skip-openspec|--openspec-change`.
- **Bubble Tea TUI** (`hero tui`): **Chat**, Status, Artifacts, Costs, Events screens (`alt+1` Chat … `alt+5` Events; boot opens Chat); **Artifacts** list cycle-generated files (current dir, OpenSpec change, documents.json, cycle-tagged docs) with ↑↓ scroll (also Status/Costs/Events); **Events** show local timestamps; **Costs** aligned table with duration as `mm:ss`; command palette with `/hero-<name>` action labels (`Go to - Chat/Status/Artifacts/Costs/Events` for screen jumps) + `/new-chat` (clear session, default model) + imported non-Hero Cursor commands (markdown expansion → Dispatch); no `d` dispatch key (Chat/`/hero-*` Execute is the driver); palette closes on select with **busy-guard**; fixed **footer status bar** (running/ok/error, **2** wrapped lines); empty-state `/hero-new`; in-process `cycle.Service`; refuses launch when `NO_COLOR` or non-TTY. **`/new-chat`** clears transcript + harness session (blocked while agent streaming — wait or ctrl+c interrupt); **`/hero-new`** opens Chat and streams `hero-new.md` via harness with the **default model**; on success TUI calls **`PrepareCycle()`** (active SQLite cycle, empty title/objective until `/hero-start`). **TUI Execute** for orchestrator commands (`/hero-start`, `/hero-approve`, `/hero-sync`, …) uses **`agents.orchestration_agent`** (kebab slug; then `fallback_model`, then `/hero-model`); `/hero-new` and freechat use `/hero-model`. YAML stage-agent slugs remain Runtime/Task only, except TUI Research which Execute-s `discover_agent` with `agents.discover_agent`. **`/hero-start`** requires active cycle; calls **`SyncCycleConfig()`** (title/objective **and** still-open stage `max_iterations` / timeout / approval / enabled from YAML) then streams `orchestration_agent.md` + `hero-start.md`. After Configuration, TUI hands Research to a dedicated **`discover_agent`** session using `agents.discover_agent` from workflow-config.yml (kebab slug; fallback_model then `/hero-model` if missing); Cursor IDE chat still grills in the orchestrator session and ignores that YAML block. Free-text follow-ups during Research resume the discover session; `/hero-approve` and other control slashes still go to the orchestrator session. When Research closes, TUI resumes the orchestrator session to dispatch later stages. **`/hero-approve`** requires active cycle + stage `PendingApproval`; streams `orchestration_agent.md` + `hero-approve.md` with the same orchestrator model; agent persists via `hero approve --metrics-json`. **`/hero-reject`** requires active cycle + stage `PendingApproval`; collects rejection reason in Chat (or inline `/hero-reject <reason>`), then streams `orchestration_agent.md` + `hero-reject.md` with the same orchestrator model; agent calls `hero reject --reason` and re-dispatches the stage agent. **`/hero-cancel`** requires active cycle; streams `orchestration_agent.md` + `hero-cancel.md`; agent runs `hero cancel` + git rollback (optional inline `/hero-cancel <reason>`). **`/hero-finish`** requires active cycle; streams orquestrator + `hero-finish.md`; agent validates stages, `hero finish --metrics-json`, updates `context/*`. **`/hero-continue`** requires Escalated stage; `/hero-continue [N]` (default 1); streams orquestrator + `hero-continue.md`; agent grants extra iterations and resumes stage. **`/hero-back`** requires Judge `PendingApproval`; streams orquestrator + `hero-back.md`; agent reopens Planning via Task and re-runs pipeline. **`/hero-sync`** streams orquestrator + `hero-sync.md`; agent runs context sync via Task `context_agent`, writes artifacts, `hero doctor`. **`/hero-status`** streams orquestrator + `hero-status.md`; agent relays full `hero status` table. **`/hero-archive`** requires active cycle; streams orquestrator + `hero-archive.md` with the **orchestrator YAML model** (not YAML stage agents); agent runs `hero cycle archive` (+ optional `metrics-summary.md`). **`/hero-resume`** streams orquestrator + `hero-resume.md`; `/hero-resume [N]` inline; agent runs `hero cycle resume` + post `hero status`. **`/hero-model`** selects the default harness model (freechat + `/hero-new`; fallback when YAML orch model is missing); persisted to `hero.json` → `harnesses.<tool>.model`. Conversation uses dual OpenCode-style panes: **input** (Build/Plan via **Tab** when the slash overlay is closed; Plan → `--mode plan`; scrollable) and **response** (green accent; height = leftover terminal space after fixed chrome; fg-only in-box text to avoid black gutters) with braille wait animation on the speaker status line while harness streams; scroll-hint line has a right-aligned **context bar** (last `result.usage` vs `models/*.yml` `context_window`; hidden if unknown); Chat header shows a live **agents** box (`agents: N` + 4-letter labels `ORCH`/`PLAN`/`BACK`/…; **`HARN` only for the parent session with no Hero agent** — freechat / direct harness; nested generic Tasks are omitted); Chat header iteration matches YAML `max_iterations` via normalized stage names (`qa_end_to_end` == `Qa End To End`); transcript and green-pane status use `[LABEL - model]` matching that box (`[ORCH - gpt-5.3-codex-medium]`, `[QA - composer-2.5]`, `[HARN - grok-4.6]`; blank line before/after subagent); Task `stream-json` start/complete drives the box (Hero agent names preferred over generic `subagent_type`); nested Task text is attributed when forwarded, else Task result content is printed. **Chat `/`** opens a palette autocomplete overlay (including `Go to - *`); Enter/Tab inserts only `/hero-approve` `/hero-reject` `/hero-cancel` `/hero-continue` `/hero-finish` `/hero-back` (next Enter sends); other items execute immediately like the full-screen palette on other screens. With a live `/hero-start` session, `/hero-approve` (and other stage-control slashes) are follow-ups — they do not TUI-Execute or gate on SQLite `PendingApproval`. Boot lists harness models (`agent models`); Chat `Execute` and `Dispatch` pass `--model` + optional `--mode`; freechat without active etapa (in-memory session); `/hero-start` keeps one orchestrator session across etapas except Research (dedicated discover session; live orch id is saved and restored after Research closes; follow-ups resume the active session + its model); Ctrl+C → `Cancel`. Runtime assets require `run_in_background: false`, wait for each Task to return, close the **finished** stage's `require_human_approval` (slash CTAs, no informal yes/no), then advance. Stream parser shows Task start/complete. Palette Dispatch starts a **fresh** agent session (no leaked `--resume`). Long execute errors wrap inside the scrollable response pane.
- **SQLite operational store** (schema **v4**: `harness_serve_registry`, `stages.harness_id`, plus v3 fields) + workflow engine + CLI-as-API cycle service (`hero cycle new` = prepare active cycle with deferred title/objective; `hero cycle sync-config` syncs meta **and still-open stage budgets** from YAML before `/hero-start`) with OpenSpec-coupled archive (`openspec` resolved via PATH plus nvm/fnm/volta/user bins; skip OpenSpec CLI when the linked change dir is already archived).
- **HarnessAdapter (full)**: Shared **`StreamDelta` normalization** (`internal/harness/stream.go`: text/thinking/tool/warning/permission/activity/session); **Cursor** NDJSON parser (`parse.go`: `permission_request` → `OnPermissionRequest`, denied tool warnings; CLI `--force`/`--trust`/`--approve-mcps`/`--sandbox disabled`); warns on unknown types and fails on exit-without-`result`; **OpenCode adapter** SSE map aligned with `anomalyco/opencode` `dev` `EventManifest.Definitions` (`events.go`: v1 `message.*` + v2 `session.next.*` streaming, `permission.v2.asked`, `sync` unwrap, transport events); `permission.asked`/`permission.v2.asked` → TUI chat + status bar `Allow? [y/N]` + `POST /permission/{id}/reply?directory=` (session-scoped fallback); serve URL from child stdout (ADR-035); **`harnessmgr.Registry`** per harness id; two-step fallback chain; session binding never mixes Cursor/OpenCode sessions.
- Install: interactive harness picker (≥1 required); conditional Cursor / OpenCode projection; legacy `cli.tools` → `harnesses.cursor.enabled` on upgrade; OpenCode stays disabled unless selected.
- Doctor: warn-only **OpenCode CLI** check when opencode enabled (complements Cursor CLI check).
- Upgrade: checksum-based non-overwrite of customized files with warnings; reconciles stale `checksums.json` when disk already matches embedded assets (no false-positive skip); also ensures env hygiene files/patterns; refreshes `docs/workflow-help.md` when not customized.
- Uninstall: removes only Hero-owned paths; preserves `AGENTS.md`, `context/`, `docs/`, `openspec/`, `.env.example`, `.gitignore`.
- Doctor / status / variables: table default + `--json` (`openspec_change` in status); doctor warn-only checks for secrets hygiene, unsupported harness markers (`.claude/` / `.windsurf/` / `.codex/`), and **Cursor Agent CLI on PATH + login hint** (complementary to TUI boot; PRD-C03-001 §4.10).
- `update-models`: fetches structured upstream model YAML (HTTP client injectable for tests).
- Template renderer + inventory / Runtime-semantics asset tests.
- **`/hero-harness`** checkbox picker (space toggle, enter apply) with `(available)`/`(unavailable)` per harness and OpenCode projection on enable; **`/hero-model`** two-step picker (harness submenu, then live models for that harness; no invented `composer-2.5` default); Chat labels **`[LABEL - model · harness]`**; OpenCode HTTP adapter tests; `assets/opencode/` embedded in binary.
- Embedded Runtime assets: Cursor + **OpenCode projection** (`assets/opencode/`); `models/*.yml` includes OpenCode-native `provider/model` ids for context bar lookup.
- Model catalog pricing/capabilities: `assets/models/*.yml` (mirrored in `.workflow-hero/models/`) carry researched pricing (`input`/`cache_write`/`cache_read`/`output`/`context_window`) plus C5 `properties` (`fs`/`ef`/`th`). `opencode.yml` is complete (2026-08-19): all 27 models from `opencode models` — 7 `opencode/` Zen free + 20 `opencode-go/` (pricing/context from the live opencode server `/provider` + models.dev TOML; `fs`/`th`/`ef` from models.dev `reasoning_options`/variants). Unsupported properties use `values: ["na"]`/`default: "na"`; unfindable data uses `["not found"]`. Cursor Grok context windows corrected to 500000 (grok-4.5/4.6), GLM-5.2 and Kimi K3 to 1M, fast Grok variants priced at $4/$1/$12.
- C5 Research complete: `/hero-model` model-property requirements are approved for TUI/native scope. The planned first keys are `fs` (fast), `th` (thinking), and `ef` (reasoning effort), with dynamic harness values, API-first discovery, project SQLite cache, local catalog fallback, per-harness/model persistence, adapter-owned execution mapping, and Chat status labels.
- C5 native implementation: `internal/harness` now carries normalized property capabilities/request maps and explicit rejection errors; `internal/modelprops` resolves API/cache/catalog/`na`, stores schema-v5 metadata, and refreshes enabled harnesses only when `/hero-model` opens. Cursor refresh infers capabilities from `ListModels` output (`internal/adapters/cursor/capabilities.go`) and persists them to SQLite; `SnapshotCacheOnly` skips catalog after a refresh wait. `hero.json` persists atomic per-harness/native-model drafts. Cursor/OpenCode own property transport, workflow YAML projects unvalidated stage values, and the TUI picker/status line supports dynamic values, warnings, responsive labels, and freechat/`/hero-new` routing. `/hero-model` waits with Chat-style animation when the user selects a model while refresh is in flight, then applies `SnapshotCacheOnly` (catalog fallback only on refresh failure). Variant Cursor slugs lock fixed properties in the picker (gray rows). Embedded/installed catalogs include C5 `properties` for Cursor base models (`composer-2.5`, `cursor-grok-4.5/4.6`) and all 27 OpenCode models (7 `opencode/` + 20 `opencode-go/`); variant Cursor slugs inherit via catalog base-slug lookup.
- `scripts/release.sh` + contract test for artifact naming / platforms / checksums.
- `scripts/build_dev.sh` for local cross-compiles without a release tag (version `<latest-tag>_<short-commit>`).
- Integration tests for install/upgrade/uninstall/doctor against `t.TempDir()`.
- Test strategy documented in [docs/testing/TESTING.md](docs/testing/TESTING.md) (`go test ./...`, golden tests, integration layout).
- Bilingual project README (`README.md`, EN + PT-BR in one file, Screenshot Hero style).

## Pending Features

- **Windows CLI** — out of scope for Hero 2.0; planned for a future major (PRD §7; DEPLOY.md).
- **CI/CD release automation and GPG-signed artifacts** — no GitHub Actions / GoReleaser pipeline in 2.0; manual `scripts/release.sh` only (ADR-010; PRD §7).
- **Additional harness adapters** — Claude Code, Codex, VS Code, and other IDEs remain deferred; C4 ships **Cursor + OpenCode** in the TUI only (PRD §2.3; PRD-C04-001).
- **C5 QA/Judge** — QA PASS 2026-08-18 (build/vet/tests green, race clean, ADR-038–042 + C4 constraints honored); 2 minor findings open (`t.Skip` condicional em `internal/tui/model_properties_test.go:393`; branch duplicado em `internal/tui/property_picker.go:169`). Judge completed by user `/hero-approve` without formal SDD coverage verification because `judge_agent` cannot emit output in the opencode harness (7 empty Task returns; re-sync/fix agent for next cycles).
- **Post-1.0 deferred D2–D13** not covered by C4 — e.g. external integrations (D2), notification manager (D3), daemon/RPC `hero serve` (D7), full event bus (D8), rich TUI roadmap (D10) (PRD-C01-001 §4).

## Recent Decisions

- **C4 closed (2026-08-15)**: Hero 2.0.0 multi-harness shipped in-tree (TUI Cursor+OpenCode, `--tools` gone, native model ids, OpenCode serve + HTTP API). Research→Judge complete; Browser UI / E2E skipped. Archive via `/hero-archive`.
- **C5 Research (2026-08-17)**: Requirements confirmed for dynamic model properties in `/hero-model`. API metadata is preferred; persistent `hero.db` cache and `assets/models/*.yml` are fallbacks. `/hero-model` refreshes enabled harnesses in background without starting OpenCode at boot; choices persist per harness/model in `hero.json`; stage YAML remains authoritative for workflow agents.
- **C5 Planning (2026-08-17)**: OpenSpec SDD `model-properties-tui` created with 19 delta requirements and 22 native `generic_agent` tasks. The hard spine is normalized contract → API/cache/catalog resolution → TUI picker/status → execution routing → integration; independent catalog, cache, JSON, adapter, picker, status, and regression tracks are marked parallel.
- **C5 Implementation (2026-08-18)**: All 22 native tasks are implemented in-tree. Schema v5, model-property catalog/cache resolution, atomic persistence, adapter-owned transport/rejection, TUI selection/status/warnings, workflow projection, Runtime help, and regression coverage are green under `go test ./...`; Browser UI Validation and QA End-to-End remain disabled by scope.
- TUI Chat context bar: right side of the `↑↓ scroll response` line shows used/max tokens for the speaking model. Max from `context_window` in `assets/models/*.yml`; used from Cursor `result.usage` (input+output) at end of Execute. Bar omitted when the slug has no window. (2026-08-15).
- TUI Chat: header `iter x/x` matches display stage names to slugs; `/hero-start` `SyncCycleConfig` updates still-open stage `max_iterations`/timeout/approval/enabled from YAML. Orchestrator Execute + input box use `agents.orchestration_agent` (then fallback_model, then `/hero-model`); `/hero-new` and freechat stay on `/hero-model`. Agents box `HARN` only for parent with no Hero agent; Task parse prefers named Hero agents over generic `subagent_type`. Amends ADR-030 §4 for orchestrator Execute. (2026-08-14).
- TUI Research uses `agents.discover_agent` in workflow-config.yml (dedicated session + YAML `--model`). Cursor IDE chat still grills in the orchestrator session and ignores that block (comment on the YAML). Control slashes during Research still go to the orchestrator. (2026-08-14).
- TUI `wrapOutputLine` searches spaces on **runes**, not UTF-8 byte indexes — `strings.LastIndex` on multi-byte glyphs (`✔` in Angular CLI output) panicked `slice bounds out of range [n:m]` while streaming harness output into the Chat response pane. (2026-08-14).
- Chat composer: **Enter** sends; **Alt+Enter** inserts a newline (does not conflict with Alt+1–5). Ctrl+Enter is not bound (Cursor/xterm.js does not deliver it). (2026-08-14).
- Chat is first in the TUI tab bar (`alt+1`) and is the boot screen. Approvals screen removed — approve/reject via Chat `/hero-*` only. `/new-chat` clears session (blocked while streaming). Chat `/` overlay lists Go to / palette items; only `/hero-approve` `/hero-reject` `/hero-cancel` `/hero-continue` `/hero-finish` `/hero-back` insert into the composer — other items execute immediately like the full-screen palette. Chat header shows a live agents box; green-pane transcript/status labels use `[QA - model]` / `[HARN - model]` (same 4-letter codes as the agents box) (2026-08-14).
- **TUI navigation while streaming**: `ctrl+1–6` / `alt+1–6` navigate between screens even while an agent is streaming — stream goroutine keeps running. Stream messages (`streamDeltaMsg`, `executeDoneMsg`, `streamCancelDoneMsg`) are always processed regardless of current screen. Destructive actions while streaming (`/hero-new`, `/hero-start`, `/hero-cancel`, `/hero-finish`, `/hero-archive`, `/hero-back`, `ctrl+q`) display a yellow footer confirmation prompt `[y/N]`; `y` cancels the stream and dispatches the action; any other key dismisses. Non-destructive actions remain silently blocked (2026-08-14).
- C3 archived 2026-08-09 (Cursor harness autonomy + TUI conversation).
- ADR-030 (2026-08-08): `hero.json` → `harnesses.<tool>` default model (user picks via TUI `/hero-model` on first use; no install-time slug); TUI freechat without active etapa; Execute passes `--model` kebab slug. **Amended 2026-08-14**: TUI orchestrator Execute uses YAML `agents.orchestration_agent` (freechat + `/hero-new` still use the harness default).
- ADR-024–029 (C3): hyphen slashes, full harness contract, TUI orchestration, hero-cycles/todos, hero-sync pending-doc scan; TUI `/hero-sync`, `/hero-status`, `/hero-archive`, `/hero-resume` aligned to Runtime Execute (2026-08-12).
- ADR-020–023 (C2): slash parity, imported commands, harness warnings, OpenSpec archive coupling.
- ADR-012–019 (C1): SQLite, AI Loop, CLI-as-API, dual UI, 0.9→1.0 breaking upgrade.

## Known Technical Debt

- No GitHub Actions / CI/CD release automation in V1 (ADR-010; deferred to V2 GoReleaser or equivalent).
- GPG-signed release artifacts deferred to V2 (PRD §7; DEPLOY.md).
- Upstream Cursor CLI gaps accepted as limitations: plugin skills, nested skill dirs (ADR-C02); nested Task assistant text is not documented in `stream-json` (TUI attributes best-effort while a Task is open, else prints `result.content`).
- Runtime asset prompts remain concise; metrics are agent-estimated, not API usage.
- Global `--verbose`/`--debug` registered but not yet wired into panic/stack-trace printing paths.
- `update-models` upstream URL assumes `main` branch raw assets on this GitHub repo.
- `.workflow-hero/config/documents.json` omits living PRD/UI index docs (`docs/product/PRD.md`, `docs/product/UI.md`).
- Cursor may still override Task/frontmatter models on some plans (known IDE limits).

## Next Steps

1. Archive C4 with `/hero-archive` when ready (OpenSpec `hero-2-0-multi-harness` first; folder date from store `completed_at`).
2. Archive C5 with `/hero-archive` when ready (OpenSpec `model-properties-tui` first; folder date from store `completed_at`).
3. Keep `.opencode/agents/*.md` frontmatter models/reasoningEffort/thinking in sync with `workflow-config.yml` `agents.*` blocks (last synced 2026-08-18 before C5 Implementation restart).

---

_To be maintained by Hero agents. Update after every cycle; keep this consolidated, not a changelog._
