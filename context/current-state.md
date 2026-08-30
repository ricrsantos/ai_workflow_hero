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
| **Phase** | Hero **2.9.1** released (tag `v2.9.1`). Patch: TUI timer/watchdog fixes, `auto-all` harness permission profile, idea-folder auto-archive on cycle archive. |

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

- **Feature Based + Vertical Slice**: `cmd/hero` + `internal/<feature>/` (`install`, `upgrade`, `uninstall`, `doctor`, `status`, `variables`, `update_models`, `cycle`, `store`, `engine`, `tui`, `harness`, `harnessmgr`, `todos`, `workflowconfig`, `ideadocs`) + `internal/adapters/cursor/` + `internal/adapters/opencode/` + `internal/adapters/codex/` + `internal/common/` (includes `assetconflict` for upgrade conflict backup/replace).
- **Strict CLI vs Runtime**: CLI is deterministic; orchestration lives in embedded `assets/cursor/`.
- **Simple templating**: `internal/common/template` — `{{path.key}}` only (ADR-006).
- **Assets**: embedded via `assets.FS`; install copies into `.cursor/` (when Cursor enabled), `.opencode/` (when OpenCode enabled), `.codex/` (when Codex enabled — full agents/commands/skills mirroring OpenCode; no AGENTS.md / no default config template), and `.workflow-hero/`. Codex `SKILL.md` files include required YAML frontmatter (`name`, `description`) for Codex skill discovery. This repository also carries a project-local `.codex/config.toml` for its own trusted development workspace.
- **Multi-harness (C4/C5)**: `hero.json` → `harnesses.<id>.enabled`, `permission_profile` (`ask` default, workspace-scoped `auto-project`, or unrestricted `auto-all`), `freechat_default {harness, model}`, C5 `model_properties`, and persisted `chat_verbosity`; `workflow-config.yml` requires `harness` on every agent + `fallback_model`; `internal/harnessmgr` registry + fallback chain (ADR-033); SQLite schema **v8** (C6 registry/index plus `cycles.session_duration_seconds` for the TUI Session timer).

## Implemented Features

- CLI commands: `hero install` (interactive harness multi-select; **`--tools` removed** — explicit error on install/upgrade), `upgrade`, `uninstall`, `doctor`, `version`, `variables`, `update-models`, `status`, `help`, plus Hero operational API (`metrics`, `events`, `approve`, `reject`, `cancel`, `finish`, `continue`, `stage`, `cycle`, `run`, `tui`, **`chat`**) (plus global `--verbose`/`--debug`). Cycle API includes `hero cycle openspec-change` / `--clear`, `hero cycle idea-files` (`--json`), and `hero cycle archive --force|--skip-openspec|--openspec-change`; cycle archive also moves active direct entries from `docs/idea/` to `docs/idea/archive/`, preserving `docs/idea/README.md`, `docs/idea/tobe/`, and existing archive entries.
- **`hero chat`**: free-chat-only TUI (no project install/git). Config in `~/.workflow-hero/`; Execute cwd; navbar exposes Chat and Settings; palette filter excludes `/hero*` and Go to. Global TUI palette: `/model`, `/harness`, `/hero-refresh` (before Go to).
- **Bubble Tea TUI** (`hero tui`): Left nav sidebar with live **agents** block (named 4-letter codes plus nested generic `TASK` chips), a bottom blue `Session`/`AI wk`/`AI rp` timer subdivision, and Chat/Status/Artifacts/Costs/Events/Settings, with Config appended only for an active cycle. Settings persists the Chat transcript detail profile (`compact`, `standard`, `detailed`, `debug`) in `hero.json`; absent/invalid configuration normalizes to Debug to preserve historical output. It filters transcript details but never permission/question gates, session failures, live-agent bookkeeping, warning status, or AI-rp responsiveness tracking. Orchestrator Execute uses `agents.orchestration_agent`. `/hero-start` tells ORCH to `hero stage start` then STOP; `/hero-approve` does the same for the next Waiting stage (no second `/hero-start`). The TUI Executes named stage agents on their YAML harness+model pair (C8; Research still uses dedicated `discover_agent`). At Research start, `internal/ideadocs` scans `docs/idea/` (excluding top-level `archive/` and `tobe/`) and injects active file paths into the discover prompt when present. Implementation may run BACK/FRNT/GEN concurrently; nested Task fan-out stays in the parent harness. Chat multiplexes tagged Executes, shows `[LABEL - model · harness]` including Task start lines, keeps `Waiting for harness…` while any Execute is live, and Esc cancels all in-flight Executes; Alt+Q is the protected quit shortcut while an agent is running. TUI combinations are Alt-oriented with no Ctrl aliases. Numbered navigation follows the visible list: `alt+1-6` without Config and `alt+1-7` with Config; free chat exposes Chat/Settings as `alt+1-2`. Tab switches focus between screen content and the navbar; the navbar has a wrapping luminous Up/Down cursor, Enter activation, and an independent `>` active-screen marker. `/hero-start` validates/syncs YAML asynchronously; OpenCode/Codex Prepare resets serve/app-server. Control slashes stay on orchestrator. Composer switches Build/Plan with Alt+M; auto-follow scroll; context bar accumulates per-turn `result.usage` for the current Chat session and resets on `/new-chat` or successful `/harness-reset`. Session starts at zero on TUI boot; `/hero-start` and `/hero-resume` explicitly restore persisted cycle seconds, while `/hero-new` starts at zero. A free-chat Session starts on the first ordinary prompt whenever this TUI has not restored a cycle session, including when a cycle already exists, and continues across prompts until a cycle starts or the TUI exits. `AI wk` measures each live Execute; process-local `AI rp` starts when a harness response is received, resets on every later response-content event even if the detail profile hides it, and keeps growing while no new response arrives.
- **Chat composer interaction**: Enter executes a recognized slash command (including command arguments) and inserts a newline for ordinary/unknown text; Alt+Enter submits ordinary prompts and stays idle on recognized slash commands. ↑/↓ move the caret between explicit and soft-wrapped visual lines while retaining the preferred column, then scroll the transcript at the composer boundaries. A new ordinary turn clears stale status-bar errors/warnings; successful ordinary turns leave the footer ready. Harness health probes/alerts are suppressed while a permission or question callback is waiting for a TUI response.
- **SQLite operational store** (schema **v8**: `harness_serve_registry`, `stages.harness_id`, plus `cycles.session_duration_seconds`) + workflow engine + CLI-as-API cycle service (`hero cycle new` = prepare active cycle with deferred title/objective; `hero cycle sync-config` syncs meta **and still-open stage budgets** from YAML before `/hero-start`) with OpenSpec-coupled archive (`openspec` resolved via PATH plus nvm/fnm/volta/user bins; skip OpenSpec CLI when the linked change dir is already archived). Stage close/approve merge prefers TUI-accumulated harness tokens over agent `--metrics-json` estimates.
- **HarnessAdapter (full)**: Shared `StreamDelta` + TUI watchdog (Cursor 5m / OpenCode+Codex **3m**). `HealthFailed` auto-cancels the stream; `HealthSuspected` is warn-only. Stall clock ignores `file.watcher` / LSP / `session.status` running and pauses while a permission or question callback awaits the user, excluding that wait from inactivity. **Cursor** NDJSON (incl. `result.usage`); **OpenCode** SSE + serve lifecycle + SSE reconnect + session idle/gone probe + `ResumeSession` + early `session.bound` persist + `Cancel("")` cancels in-flight `runCtx` + interactive `question.asked` (composer answers) + `PrepareHeroStart` + `info.tokens`/`step-finish` → `Usage` (all steps in one Execute are summed); **Codex** (`internal/adapters/codex`) Hero-managed `codex app-server` stdio JSON-RPC — lazy start, SQLite registry (no URL), stream/permission/auth/`ListModels`/C5 map, orphan reap, `ResetAppServer` + `PrepareHeroStart`, and v2 cumulative usage snapshots normalized to their `last` turn. One Codex adapter serializes its turns because the underlying connection has one callback pair; different harness adapters remain concurrent. `/harness-reset` on OpenCode keeps `harnessSessionID` (process restart, not a new conversation). TUI Research persist binds `harnessSessionHarnessID` (and stage harness id) on `discover_agent` so mixed orch/discover pairs (e.g. Cursor + Codex) resume the same thread; `/hero-start` Prepare uses the registry singleton (not a throwaway `NewAdapter`). Codex `thread/resume` failure on an unloaded id starts a new thread (warn); `StopAppServer` clears in-memory session maps. Codex notify delivery is queued off the stdout `readLoop` (avoids stdio pipe deadlock under TUI backpressure); a per-Execute ordered/coalescing TUI relay prevents the callback from waiting on the chat channel and is stopped on cancellation. Transcript-critical methods (`item/agentMessage/delta`, `item/completed`, `turn/completed`, `thread/tokenUsage/updated`, …) remain ordered. Codex final output is reconstructed from authoritative `item/completed` snapshots in wire order, so missing middle spans in live deltas are repaired at `executeDone`; an unambiguous single-item `lastAgentMessage` is the fallback. TUI applies auto-follow once per event batch. Agent text uses raw deltas + newline between successive `agentMessage` items; `executeDone` reconciles the parent transcript from the canonical `result.Output`; reasoning → thinking on completed only; noisy activities / unrecognized warnings only with `hero --debug`. Cycle metrics add actual per-turn TUI usage to the active stage+agent row; nested Runtime Task usage remains part of the parent agent's reported Metrics Procedure estimate. Green response pane shows `↑/↓ more` when scrolled and `…` when a row is hard-clipped. `harnessmgr` resolves cursor+opencode+codex; fallback chain ADR-033; no cross-harness session resume.
- Model catalogs: `assets/models/*.yml` pricing + C5 `properties`; OpenCode 27 models; Cursor includes `auto`; Codex-native ids without invented ChatGPT USD rates.
- Install: interactive harness picker (≥1 required; Cursor / OpenCode / **Codex**); conditional projections; 2.4.x → 2.5.0 never auto-provisions `.codex/` (ADR-048).
- Doctor / status / variables: table + `--json`; warn-only Cursor/OpenCode/Codex CLI checks when enabled; `.codex/` supported marker (C6).
- Upgrade: checksum conflict backup/replace; env hygiene (`.env.example`, `.gitignore` secrets block, and `.workflow-hero/tui.log`); refreshes `docs/workflow-help.md` when not customized.
- Uninstall: Hero-owned `.codex/` / `.opencode/` trees only; preserves user `config.toml`, `AGENTS.md`, `context/`, `docs/`, `openspec/`.
- `update-models`: upstream YAML fetch + conflict backup; updates `checksums.json`.
- **`/hero-harness`** / **`/hero-model`**: Codex enable→`.codex/` projection; model step lists native ids (Codex may start app-server); C5 property submenu; Chat `[LABEL - model · harness]` / `Build · model · harness` follow the **active execute pair** (`runtimeHarnessID` + `runtimeModelSlug` from agent YAML / `ResolveExecutePair`), not a cross-mix with freechat; UI-C06-001 §6 goldens.
- Embedded Runtime: Cursor + `assets/opencode/` + `assets/codex/` (no AGENTS.md / no Codex config template).
- C5 model properties: `internal/harness` + `internal/modelprops`; catalogs carry `properties` for Cursor base + OpenCode 27 + Codex ids.
- `scripts/release.sh` + `build_dev.sh` + contract tests; latest release **2.9.1**; integration tests include C6 Codex path.
- Test strategy in [docs/testing/TESTING.md](docs/testing/TESTING.md); bilingual README.

## Pending Features

- **Windows CLI** — out of scope for Hero 2.0; planned for a future major (PRD §7; DEPLOY.md).
- **CI/CD release automation and GPG-signed artifacts** — no GitHub Actions / GoReleaser pipeline in 2.0; manual `scripts/release.sh` only (ADR-010; PRD §7).
- **Additional harness adapters** — Claude Code, VS Code, and other IDEs remain deferred (Codex shipped in C6 / 2.5.0).
- **C6 QA/Judge** — next after Implementation close; Browser UI / E2E skipped by scope.
- **Post-1.0 deferred D2–D13** — e.g. external integrations, notification manager, daemon/RPC `hero serve`, full event bus (PRD-C01-001 §4).

## Recent Decisions

- **2026-08-30 — Config model picker**: Enter (or Space) on a Config model field opens a slash-menu-style catalog window for that agent's harness, listed alphabetically. Arrow keys move the selection, Enter applies it, and Escape cancels without changing the draft. Harness and property fields still cycle in place.

- **2026-08-29 — Welcome inner fill and Config property catalogs**: The post-`/hero-new` welcome dialog paints both the full-screen backdrop and every inner row (including wrapped copy and the button row) with the Hero surface color. Config thinking/effort wheels now use the same C5 snapshot as `/hero-model` (catalog/cache, including Luna `max`), and model validation uses the catalog cascade rather than boot-only rows.

- **2026-08-29 — Cycle archive preserves idea workspace boundaries**: `hero cycle archive` moves active files and subfolders directly under `docs/idea/` into `docs/idea/archive/`, preserving their relative structure. The root `README.md`, `tobe/`, and the existing `archive/` directory are never moved; destination collisions fail before any active idea entry is changed.

- **2026-08-29 — TUI Chat verbosity Settings**: Added a Settings sidebar screen with four curated profiles: Compact, Standard, Detailed, and Debug. `chat_verbosity` persists in `hero.json`; missing and unrecognized values resolve to Debug so established projects retain their prior transcript. Settings is last without an active cycle and immediately precedes Config while Config is visible. `alt+1-6` and `alt+1-7` reflect those two layouts. Compact renders agent text only; Standard adds tools/Task lifecycle; Detailed adds thinking, activities, and warnings; Debug preserves all transcript rows. Permission/question gates, session failures, activity bookkeeping, and warning status remain safety-visible.

- **2026-08-29 — Unified Harness Manager permissions**: `/harness` presents each Harness and its indented `Permissions:` choices in one screen: `Ask every time`, `Auto approve in project`, and `Auto approve every time (Yolo)`. Disabled Harnesses keep their choices visible but inert and retain their persisted profile. Space chooses one focused profile; Enter persists the topology and enabled-Harness profiles together. No selection (or an invalid choice) resolves conservatively to `Ask every time`. The former two-step permission picker was removed.

- **2026-08-29 — Post-`/hero-new` setup guidance**: After the TUI successfully prepares a new active cycle, it presents a transient, keyboard-accessible English welcome dialog with configuration and close actions. It appears once for each newly created cycle in the current TUI session, does not reappear on TUI restart, and enters Config immediately when selected. Config remains conditionally visible through the existing active-cycle navigation rules.

- **2026-08-28 — Discover auto-loads active `docs/idea` files**: `internal/ideadocs` scans `docs/idea/` excluding top-level `archive/` and `tobe/`; TUI injects paths into the discover session prompt; Cursor IDE uses `hero cycle idea-files` per `discover_agent.md`. Idea notes are non-normative (ADR-019).

- **2026-08-28 — Shared TUI timers (ADR-058)**: Replaced the `/hero-start`-owned footer elapsed value with one-second `internal/tui/timers.go` ticks. The blue bottom-navbar `Session` timer starts at zero on TUI boot, restores persisted cycle seconds only for explicit `/hero-start` or `/hero-resume`, starts new cycles at zero, stops at cycle completion, and resets at archive or when entering free chat; free-chat Session starts on its first ordinary prompt whenever this TUI has not restored a cycle session, continues across prompts, and is process-local. `AI wk` starts per Execute and stops at the last response/cancel. Process-local `AI rp` begins only with the first harness response displayed in Chat, restarts on every displayed harness response, and grows until the next one. The counter values share a fixed sidebar column and format as continuous `HH:MM:SS`; pending `/hero-new` state ignores stale previous-cycle refreshes and clears on cancellation.

- **2026-08-27 — TUI interruption and conditional navigation**: Esc cancels active Harness Executes/preflight; Alt+Q is the protected quit shortcut. Sidebar order is Chat, Status, Artifacts, Costs, Events, Config; the visible shortcut label is `alt+1-5` without an active cycle and `alt+1-6` with Config, and hidden `alt+6` is ignored. The 2026-08-28 focus update removed the former Ctrl aliases.
- **2026-08-27 — `hero chat` OpenCode execute hang**: Free chat stores config under `~/.workflow-hero/` but Execute uses cwd; OpenCode session APIs (`/session`, `prompt_async`, `/message`, `/abort`, resume) now pass the same `directory` query as SSE subscribe/recovery. Fixes infinite wait / `context canceled` when OpenCode is selected in `hero chat`.
- **2026-08-27 — Chat slash command Enter behavior**: Recognized slash commands in the Chat composer execute with Enter again, including control commands and arguments; ordinary text keeps Enter for newlines, while Alt+Enter remains the prompt-send key.
- **2026-08-26 — Codex stream integrity under TUI backpressure**: A per-Execute relay now decouples adapter callbacks from Bubble Tea rendering, preserves delta order, coalesces adjacent text/thinking deltas, and releases on cancel. Codex rebuilds `ExecutionResult.Output` from authoritative completed agent-message items, repairing losses inside a sentence rather than only missing suffixes. Chat auto-follow now runs once per coalesced batch.
- **2026-08-26 — Codex project execution defaults**: Added root `.codex/config.toml` with `approval_policy = "never"`, `sandbox_mode = "workspace-write"`, and workspace network access enabled. The Codex adapter canonicalizes the project path and starts `codex app-server` with an explicit trusted-project `-c` override plus the same approval/sandbox defaults. `thread/start.sandbox` uses SandboxMode kebab-case (`workspace-write`); `turn/start.sandboxPolicy.type` uses SandboxPolicy camelCase (`workspaceWrite`). Project trust is not stored in the local TOML because Codex skips project-scoped `.codex/` layers for untrusted projects.
- **2026-08-26 — C7 Config form UX**: Config now renders its editable value and caret inline (with cursor movement/deletion), uses the Chat composer blue for active labels and gray for inactive controls, limits focus highlighting to labels, and groups controls by Identity, Scope, stage, and Shared/Advanced. A dirty leave replaces the long form with a visible Save/Discard/Cancel confirmation (`enter` saves), preventing the captured confirmation keys from looking like a frozen TUI.
- **2026-08-26 — OpenCode hang workarounds**: Overnight spinner with a live SSE socket was not a transport crash. Execute now completes when `GET /session/{id}` is idle while `/event` stays open; session 404 fails the turn; `HealthFailed` auto-cancels; stall ignores filesystem/LSP noise; session id is persisted on `session.bound`; `Cancel("")` cancels in-flight `runCtx`; OpenCode `ResumeSession` + `/harness-reset` keeps the id; serve is not bound to Execute `runCtx`. Local `opencode-go` `chunkTimeout` 180000. Did not migrate to `opencode run`.
- **2026-08-25 — C8 TUI-direct stage Execute**: Named stage agents run as TUI `HarnessAdapter.Execute` on their YAML pair after ORCH `hero stage start` then STOP (ADR-054). Nested Task stays in the parent harness; generic Tasks chip `TASK` (ADR-056). Implementation scope agents (BACK/FRNT/GEN) run concurrent tagged Executes (ADR-055/057). Cursor IDE Runtime unchanged.
- **2026-08-25 — C7 Implementation: TUI cycle configuration**: Added conditional Config navigation and asynchronous YAML-backed Config state. Managed Save now merges against the latest YAML while retaining unmanaged nodes, validates before atomic replacement, keeps invalid drafts visible with field-level errors, syncs the cycle, and offers an explicit stage-specific Failed→Waiting retry that preserves events and metrics. Config warns for enabled PATH/auth-unavailable harnesses, hides `browser_ui_agent` unless frontend Browser UI Validation is active, and flags missing capability metadata for every visible agent/fallback pair.
- **2026-08-25 — C7 Planning: TUI cycle configuration SDD**: OpenSpec change `tui-cycle-config` created (36 tasks, native/`generic_agent`). Idea-file reload/merge/cancel dialog superseded by ADR-050 latest-file managed merge. `openspec/config.yml` context regenerated from `documents.json` including C07 docs.
- **2026-08-25 — `/hero-start` after `/hero-resume`**: Execute done/error/cancel clears `actionBusy` for the current status label (not only `/hero-start`), so the palette no longer treats a finished resume as still running.
- **2026-08-25 — C7 Research: TUI cycle configuration**: Requirements confirmed for a Config screen backed by the active YAML. The TUI wins only on managed fields during parallel edits; completed stages are read-only; failed stages support explicit stage-specific retry after a saved config change, with fresh attempt counters and preserved history/metrics. Proposed specs are registered as PRD/UI/ADR-C07-001; implementation remains pending approval and Planning.
- **2026-08-25 — `/hero-start` / `/hero-resume` chat wait**: Preflight shows `Preparing /hero-start…` in the transcript; `/hero-start` clears leftover chat; Execute follows the bottom instead of jumping to offset 0; `/hero-resume` keeps history and shows `Waiting for harness…` while the shared TUI timers continue.
- **2026-08-25 — Mixed-harness Research resume**: Discover persist now records `harnessSessionHarnessID`; `/hero-start` Prepare resets the registry Codex/OpenCode adapter; Codex `thread/resume` of an unloaded id starts a new thread.
- **2026-08-24 — Chat wait spinner**: Shown at the transcript end for the whole Execute, not only while the green agent bubble is empty (thinking/tools no longer hide it).
- **2026-08-24 — TUI configuration screen idea**: Consensus for a Config sidebar screen available only with an active cycle. It is editable whenever no agent is executing, persists to `workflow-config.yml` using round-trip-safe YAML changes, and syncs the active cycle's still-open stages. Cursor IDE YAML + `/hero-start` workflow remains unchanged.
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

1. C7/C8 QA and Judge; archive OpenSpec changes when approved.
2. C6 archive (`codex-adapter`) when ready.

---

_To be maintained by Hero agents. Update after every cycle; keep this consolidated, not a changelog._
