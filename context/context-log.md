# Context Log

> Short-term project memory for this repository (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Permanent facts belong in `context/current-state.md`.

## 2026-08-29 — Archive active idea notes with cycles

**Change**: `hero cycle archive` now moves every direct file and subfolder under
`docs/idea/` into `docs/idea/archive/`, preserving relative structure. The root
`README.md`, `tobe/`, and the existing `archive/` directory remain untouched.
The archive preflights destination collisions and rolls back idea moves when a
later Hero cycle filesystem step fails.

**Validation**: `go test ./internal/cycle -count=1` passes. The full
`go test ./... -count=1` suite passes all affected packages and is limited by
the two known restricted-sandbox OpenCode serve-spawn tests that cannot expose
a listening URL.

## 2026-08-29 — TUI Chat verbosity Settings

**Change**: Added the persistent Settings screen to the navbar. It offers Compact, Standard, Detailed, and Debug profiles; the default/legacy value is Debug, preserving existing transcript output. The setting is saved as `chat_verbosity` in `hero.json`. Settings is the final normal navigation item and moves immediately before the conditional Config item. Shortcut range labels now follow the visible nav list (`alt+1-6` normally; `alt+1-7` with Config; `alt+1-2` for free chat).

**Behaviour**: Profiles filter transcript detail only: Compact shows agent text; Standard adds tools/Task lifecycle; Detailed adds thinking, activities, and warnings; Debug shows all currently emitted rows. Permission/question gates, session failure handling, live-agent state, and warning status remain active regardless of the selected profile.

**Validation**: Focused install/TUI Settings/navigation tests pass. Full `go test ./... -count=1 -p 1` passes all other packages; the two pre-existing restricted-sandbox OpenCode serve-spawn tests still fail because they cannot expose a listening URL.

## 2026-08-29 — Unified Harness Manager permissions

**Change**: Collapsed the former `/harness` → enabled-Harness list → individual permission profile screens into one interactive Harness Manager. Each Harness has two indented, checkbox-style permission rows: `Ask every time` and `Automatic in project`. Space toggles the focused Harness or permission row, while Enter saves the full draft. A disabled Harness leaves its permission rows visible but muted and non-interactive, preserving the stored profile. On save, no marked permission falls back to `Ask every time`; if both are marked, `Automatic in project` takes precedence. Permission changes still restart long-lived OpenCode/Codex servers so their native settings are reloaded.

**Validation**: Focused Harness Manager tests, `go test ./internal/tui -count=1`, and `go test ./... -count=1` pass with the isolated Go cache.

## 2026-08-29 — TUI post-cycle welcome dialog

**Change**: After `/hero-new` successfully finishes `PrepareCycle`, the TUI now immediately refreshes active-cycle chrome and opens a clean, centered English guidance dialog. It explains Harness authentication, Skills parity, idea notes in `docs/idea/`, and cycle configuration through `workflow-config.yml` or Config. `Go to Config` opens the existing editable Config screen; `Close`/Esc returns to Chat. Tab or left/right switches the selected action. The dialog is transient and is shown once per successful new cycle within that TUI process; it is not persisted or shown after restart. Small terminals receive a concise resize fallback.

**Validation**: `go test ./internal/tui -run 'TestCycleWelcome' -count=1`, `go test ./internal/tui -count=1`, and `go test ./... -count=1` pass with an isolated Go cache because the restricted environment cannot write the default compiler cache.

## 2026-08-28 — Auto-ignore TUI slog log

**Problem**: `hero` redirects slog to `.workflow-hero/tui.log`, but install/upgrade
gitignore hygiene only ensured secrets patterns and skipped projects that already
had a Hero block or an existing `.env` ignore.

**Change**: `assets/templates/gitignore-secrets` now includes
`.workflow-hero/tui.log`. `internal/common/envhygiene` patches existing Hero
blocks (insert before `# END Hero secrets hygiene`) or appends a small runtime
block when needed. `hero tui` / default `hero` entry also runs env hygiene on
boot so older projects pick up the ignore without reinstalling.

**Validation**: `go test ./...` passes.

## 2026-08-28 — Accumulated TUI token usage

**Problem**: TUI Chat replaced the session context-token counter with the
latest completed Execute's usage. Cycle-stage attribution also relied on
mutable global speaker state, which could misattribute parallel stage-agent
results.

**Change**: Completed Execute usage now accumulates `input+output` for the
current Chat session. `/new-chat` and successful `/harness-reset` clear the
counter, and invalidated late completions cannot re-add usage to that new
counter. Each tagged Execute captures its stage, agent, model, and prompt for
usage fallback and cycle metrics attribution. OpenCode step usage is summed
within one Execute, while Codex app-server v2 cumulative snapshots are
normalized to their `last` turn before the TUI/cycle accumulator consumes them.
Existing cycle metrics aggregation remains additive by cycle, stage, and
agent; a late result from a reset session still records consumed cycle usage.
Nested Runtime Task usage remains represented by the parent agent's Metrics
Procedure estimate.

**Validation**: Focused TUI, Codex adapter, and harness tests pass with a
writable temporary Go build cache.

## 2026-08-28 — Codex turn callback isolation

**Change**: Codex app-server exposes one notification/request callback pair
per JSON-RPC connection. The adapter now serializes turns on the same
connection, while retaining concurrency across different harness adapters,
so parallel stage executions cannot replace one another's event and usage
routing. Queued turns honor cancellation.

**Validation**: Codex adapter and affected TUI/cycle/engine tests pass. The
unfiltered repository suite still reaches all packages and fails only the two
previously documented restricted-sandbox OpenCode serve-spawn tests, which
cannot expose a listening URL in this environment.

## 2026-08-28 — Alt-oriented TUI and navbar focus navigation

**Change**: Removed TUI Ctrl aliases and standardized modified shortcuts on Alt. Tab/Shift+Tab now switch shell focus between screen content and the visible navbar; the navbar keeps a wrapping luminous Up/Down cursor separate from the `>` active-screen marker, and Enter activates the highlighted screen. Chat Build/Plan moved from Tab to Alt+M. Config now uses Alt+S, Alt+Enter, and Alt+R; redundant Ctrl+P/N navigation aliases were removed in favor of arrow keys.

**Validation**: Added behavioral tests for focus transfer, luminous selection, marker stability, Enter activation, wrapping, hidden-navbar behavior, dirty Config leave protection, edit commit on Tab, Alt bindings, and the ignored legacy control quit key. `go test ./internal/tui -count=1` passes. The repository suite passes when skipping the two pre-existing restricted-sandbox OpenCode serve-spawn tests; an unfiltered `go test ./...` fails only those same documented cases because they cannot expose a listening URL in this environment.

## 2026-08-28 — AI working and response-gap timers

**Change**: Renamed the execution timer label to `AI wk` and added `AI rp`
directly below it. `AI rp` is transient TUI state: it is zero and stopped at
boot, starts when the first harness response is placed in Chat, and restarts on
every subsequent harness response. It continues after Execute completion so a
growing value exposes an absent response. Session metadata and local watchdog
alerts do not reset it.

**Validation**: Focused AI response-timer/sidebar-layout tests and
`go test ./internal/tui -count=1` pass. `go test ./... -count=1 -p 1` passes
every other package; the known restricted-sandbox OpenCode spawn tests
(`TestDiscoverModelPropertiesNormalized` and
`TestIsManagedOpenCodeServeDetectsSpawnedServe`) cannot expose a listening URL.

## 2026-08-28 — Sidebar timer value alignment

**Problem**: The `Session` and `AI` labels were aligned, but `AI` reserved one
column less before its `HH:MM:SS` value.

**Change**: Both timer rows now use the same fixed label field, and the layout
test asserts that the two counter values start in the same rendered column.

**Validation**: `go test ./internal/tui -count=1` passes. `go test ./... -count=1 -p 1`
passes all other packages; the known restricted-sandbox OpenCode spawn tests
(`TestDiscoverModelPropertiesNormalized` and
`TestIsManagedOpenCodeServeDetectsSpawnedServe`) cannot expose a listening URL.

## 2026-08-28 — Chat Session starts before cycle restore

**Problem**: An ordinary first Chat prompt left `Session` at zero after opening
the project TUI when SQLite already contained an active cycle. The timer only
considered whether a cycle row existed, not whether this TUI had restored a
cycle session.

**Change**: Ordinary Chat now starts the process-local Session timer unless
`/hero-start` or `/hero-resume` has restored the cycle timer (or `/hero-new`
is creating one). The first prompt is covered by a regression test.

**Validation**: `go test ./internal/tui -count=1` passes. `go test ./... -count=1 -p 1`
passes all other packages; the known restricted-sandbox OpenCode spawn tests
(`TestDiscoverModelPropertiesNormalized` and
`TestIsManagedOpenCodeServeDetectsSpawnedServe`) cannot expose a listening URL.

## 2026-08-28 — TUI timer label

**Change**: Renamed the blue navbar timer label from `Sessão` to `Session`.
The existing lifecycle remains unchanged: zero at TUI boot, first free-chat
prompt, and cycle transitions according to ADR-058.

**Validation**: `go test ./internal/tui` passes. The full suite passes outside
the two pre-existing restricted-sandbox OpenCode spawn cases documented in the
previous entry.

## 2026-08-28 — TUI navbar hint and Session timer lifecycle

**Problem**: The `alt+1-6` hint was attached to the navigation rows, and a fresh TUI restored the persisted cycle Session timer before the user explicitly resumed/started a cycle. Free-chat prompts also reset the Session timer on every turn.

**Change**: Anchored the shortcut hint to the last row of the navbar navigation area, immediately above the timer divider, with responsive clipping for short terminals. TUI startup now begins Session at zero; `/hero-start` and `/hero-resume` explicitly request persisted cycle recovery, while `/hero-new` starts a zeroed timer. Free chat starts at the first prompt and keeps the same process-local timer across later prompts. AI timing remains per Execute.

**Validation**: Focused layout/timer tests and the complete `internal/tui` suite pass with `GOCACHE=/tmp/hero-go-cache CC=/usr/bin/gcc CXX=/usr/bin/g++`. Full `go test ./... -count=1 -p 1` passes all other packages; the two known restricted-sandbox OpenCode serve-spawn tests still fail because their simulated process exits before exposing a listening URL.

## 2026-08-27 — TUI test helper and conditional navigation

**Outcome**: Test-only models skip the 30-second health probe, and command draining stops after the first business message. Esc cancels active Executes/preflight; the protected quit binding was later standardized on Alt+Q. Sidebar numbering follows visible screens (`alt+1-5`, or `alt+1-6` with Config).

**Validation**: TUI and focused navigation/cancellation tests passed.

## 2026-08-27 — `hero chat` OpenCode workspace routing

**Outcome**: Free chat stores configuration under `~/.workflow-hero/` but executes in cwd. All OpenCode session-scoped calls now use the same `directory` query as event subscription/recovery, preventing hangs and `context canceled` failures.

**Validation**: Adapter and full-suite checks passed apart from the two known restricted-sandbox OpenCode serve-spawn tests.

## 2026-08-26 — Chat composer and harness wait UX

**Outcome**: Enter inserts ordinary newlines while recognized slash commands execute; Alt+Enter submits ordinary prompts. Composer caret movement follows visual lines, and watchdog alerts are suppressed during permission/question waits.

**Validation**: TUI and repository tests passed except the known restricted-sandbox OpenCode serve-spawn cases.

## 2026-08-25 — C7 Config and C8 TUI-direct execution

**Outcome**: Added the active-cycle Config form with managed YAML saves/retry and TUI-direct named stage Executes after orchestrator handoff. Parallel Implementation agents and nested Task labels are represented in Chat.

**Validation**: Feature and repository tests passed during the implementation cycle.

## 2026-08-28 — Per-harness project-local approval profiles

**Decision**: User confirmed a simple profile per enabled harness: `Ask every time` is the default for new and legacy configuration; `Automatic in project` is opt-in. The automatic preset must not become unrestricted yolo: network, MCP, shell, and external-directory access stay in the native approval path.

**Implementation**: Added `harness.PermissionProfile` to normalized Execute requests and persisted `harnesses.<id>.permission_profile` in `hero.json`. `/hero-harness` now continues from enable/disable selection into an enabled-harness profile manager. OpenCode starts its managed server with an inline process-only permission override, Codex retains `on-request` and auto-approves only workspace-confined file changes, and Cursor uses sandboxed `--auto-review` without auto-approving MCPs. Existing absent values read as `ask` without forced migration writes.

**Validation**: `go test ./...` passes with an isolated Go cache and writable temporary OpenCode data directory (the execution sandbox makes the default Go/ccache and OpenCode user-data locations read-only).

## 2026-08-29 — Release v2.9.0

**Outcome**: Tagged `v2.9.0` (minor bump from `v2.8.0`). Ships TUI settings screen, checklist window, harness config adjustments, auto-hide Config after archive, TUI label/status-bar polish, and `docs/idea` folder support.

**Validation**: `go test ./...` green before tag; `scripts/release.sh` artifacts published to GitHub Releases.

## 2026-08-29 — AI rp tracks harness responsiveness independently of transcript detail

**Decision**: `AI rp` measures elapsed time without response content from the harness, rather than time since the last response visible in the Chat detail profile. Hidden thinking, tool, activity, or warning response content therefore resets it.

**Implementation**: Removed the transcript-verbosity condition from TUI response-timer resets and added coverage for Compact mode hiding thinking.

## 2026-08-29 — Harness Manager visual grouping and unrestricted profile

**Decision**: User authorized an unrestricted per-harness `auto-all` profile. It is labeled `Auto approve every time (Yolo)` and may approve shell, network, MCP, and external-path operations through the native harness mechanisms.

**Implementation**: `/harness` now groups each harness's three exclusive approval choices under `Permissions:` with blank separation between harnesses. Cursor maps `auto-all` to force/MCP approval with sandbox disabled, OpenCode to `permission: allow`, and Codex to no-approval danger-full-access plus automatic replies to residual requests.

## 2026-08-29 — Pause watchdog during interactive harness callbacks

**Decision**: A permission or question callback is an expected harness pause. Its user-wait duration must not count toward the harness inactivity timeout.

**Implementation**: Added watchdog pause/resume accounting around interactive callback lifecycle, preserving the active time before the callback and resuming it after the response.

## 2026-08-28 — Release v2.8.0

**Outcome**: Tagged `v2.8.0` (minor bump from `v2.7.0`). Ships C7 TUI cycle configuration, C8 TUI-direct stage Execute, shared Session/AI timers, OpenCode question mapping and hang workarounds, Codex stream/permission improvements, and per-harness project-local approval profiles (`ask` / `auto-project`).

**Validation**: `go test ./...` green before tag; `scripts/release.sh` artifacts published to GitHub Releases.

## 2026-08-28 — Discover auto-loads active `docs/idea` files

**Decision**: Research should consider optional design notes under `docs/idea/` at session start. Top-level `archive/` and `tobe/` are excluded; empty folder is fine.

**Implementation**: Added `internal/ideadocs` (`ListActive`, `PromptSection`), TUI injection in `startDiscoverResearchSession`, CLI `hero cycle idea-files` (`--json`), and `discover_agent.md` responsibility to run the command in Cursor IDE. Documented layout in `docs/idea/README.md`.

**Validation**: `go test ./internal/ideadocs/... ./internal/cycle/... ./internal/tui/...` and full `go test ./...`.

## 2026-08-28 — Hide Config after `/hero-archive`

**Implementation**: Added `syncActiveCycleChrome()` to reconcile navbar/palette when the active cycle ends: hides Config, switches label to `alt+1-5`, leaves Config screen for Chat, and clears config draft state. Called on archive success (eager `Status()` sync) and on `refreshDataMsg` when cycle presence drops.

**Validation**: `go test ./internal/tui/...` and full `go test ./...`.
