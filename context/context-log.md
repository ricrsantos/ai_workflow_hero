# Context Log

> Short-term project memory for this repository (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Permanent facts belong in `context/current-state.md`.

## 2026-09-05 — Telegram `/model` uses a numbered remote wizard

**Problem**: Telegram `/model` reused the local slash dispatcher and opened the TUI palette, leaving the remote user unable to choose a harness, model, or reasoning properties.

**Change**: Added address-scoped Telegram selection state in `internal/tui`. It sends numbered harness and model lists, then each selectable C5 property list, validates numeric replies, and atomically saves the resulting free-chat pair/properties without opening the local picker.

**Validation**: `go test ./internal/tui/...` pass.

## 2026-09-05 — Telegram inbound only after TUI restart

**Problem**: Replies reached Telegram, but inbound text still appeared in Chat only after quitting and reopening the TUI. Every TUI start spawned a new daemon that unlinked the live socket, so two processes polled `getUpdates`. The idle process stole updates and queued them; flush happened on the next register.

**Change**: Dial an existing daemon instead of always spawning. `Listen` refuses to steal a live socket. `getUpdates` runs only while a TUI is registered. Last-client shutdown waits 3s so a reconnect reuses the same poller.

**Validation**: `go test ./...` pass.

## 2026-09-05 — Telegram inbound stuck until TUI restart; replies still not sent

**Problem**: Live Telegram messages did not appear in Chat until the TUI was restarted (then the queued question arrived). The harness answer stayed in the TUI and was never sent back. Production `executeDone` is wrapped in `conversationBatchMsg`, which discarded the outbound `tea.Cmd`. Inbound after the first turn depended on re-issuing `waitTelegramMsg`, which did not survive that batch.

**Change**: `conversationBatchMsg` keeps nested cmds (Telegram `outbound` included). Launch relays daemon frames with `tea.Program.Send`. ACK/outbound IPC writes stay in cmds; `ipc.Conn.Send` is mutex-serialized; daemon Bot API send no longer blocks the IPC serve loop.

**Validation**: `go test ./...` pass.

## 2026-09-05 — Telegram Project ID not persisted

**Problem**: Changing Settings Project ID (e.g. `aiwkhero`) only updated in-memory TUI state. Boot always derived the abbreviation from the directory basename (`ai_workflow_hero`), so reopening Settings reset the field.

**Change**: Persist `telegram.project_abbrev` in `.workflow-hero/config/hero.json` on Enter. `startTelegram` loads the saved value when present, otherwise falls back to the directory name. Credentials remain vault-only (ADR-062).

**Validation**: `go test ./...` pass.

## 2026-09-05 — Telegram harness replies stayed in the TUI

**Problem**: Inbound Telegram text ran a harness turn and the transcript showed `→ [Telegram · addr]`, but the final agent output was never sent on IPC `outbound`. Lifecycle `Notifier` only covers cycle/stage/approval/error/final events, not conversation replies.

**Change**: On `executeDone` of a Telegram-originated turn (no remaining sibling Executes), send the harness `Output` (or error text) to the daemon as `outbound`, chunked under the Bot API size limit. Local composer turns are unchanged.

**Validation**: `go test ./...` pass.

## 2026-09-05 — README Telegram setup

**Change**: Added bilingual README sections **Telegram plugin** / **Plugin Telegram** (BotFather, `hero plugin install telegram`, TUI Retry/Pair, addressed messages). Also listed the plugin in Features and the post-install CLI commands.

**Validation**: README EN + PT-BR sections reviewed in place.

## 2026-09-05 — Telegram Settings stuck on Disconnected

**Problem**: Plugin installed but Settings showed `Daemon: Disconnected` with only `| Pair |`, which refused to pair. `startTelegram` wrote Connected/Disconnected into `telegramMsgCh`, but `Init` never issued `waitTelegramMsg`, so the TUI never learned the daemon was up. Pair gated on the stale `connected=false` flag.

**Change**: `Init` batches the telegram listener. The client spawns the daemon at start (with a 2s respawn cooldown, detached session). Disconnected Settings shows `| Retry |` plus recovery copy; Pair appears after Connected.

**Validation**: `go test ./...` pass.

## 2026-09-05 — Settings Pair button showed no pairing UI

**Problem**: Enter on Settings `| Pair |` did not start pairing or show instructions. The handler opened a token form (`pairState=token`) without sending `pair_start`, rendered a few dim lines instead of a full-screen dialog (alt-screen looked empty), and navbar/Tab stole keys before the modal. Disconnected Pair only appended a Chat transcript notice, invisible on Settings.

**Change**: Pair/Replace send `pair_start` immediately and open a centered instruction dialog (UI-C09-001 §2). The masked token field appears only on daemon `missing-token`. Pairing keys are handled before navbar/Tab. Disconnected/success/expiry feedback uses the Settings status bar.

**Validation**: `go test ./...` pass.

## 2026-09-05 — Release Hero v3.0.1

**Change**: Patch release — `hero plugin install telegram` downloads the platform-matched daemon from the matching Hero GitHub Release into `~/.workflow-hero/plugins/telegram/` (no local copy next to `hero` required).

**Validation**: `go test ./...` pass; `./scripts/release.sh`; GitHub Release `v3.0.1`.

## 2026-09-05 — Telegram plugin install downloads from GitHub Releases

**Change**: `hero plugin install telegram` now fetches the platform-matched `hero-telegram-daemon` from the Hero GitHub Release that matches the running binary version (`internal/plugin/release.go`), installs it under `~/.workflow-hero/plugins/telegram/`, and no longer requires a local copy next to the `hero` executable.

**Validation**: `go test ./internal/plugin/...` pass. Released as Hero **v3.0.1**.

## 2026-09-05 — Release Hero v3.0.0 / hero-telegram-daemon 0.1

**Change**: Cut `v3.0.0` — first Hero release shipping the optional Telegram plugin and platform-matched `hero-telegram-daemon` binaries (`scripts/release.sh` builds 4 OS/arch pairs + checksums). Includes C9 conversation service, IPC daemon, TUI Settings Telegram section, log rotation, and doctor/status plugin health.

**Validation**: `go test ./...` pass before tag; `./scripts/release.sh`; GitHub Release `v3.0.0` with 8 binaries + `checksums.txt`.

## 2026-09-05 — Settings TUI redesign

**Problem**: Settings rendered Chat verbosity and Telegram as one flat selectable list, so the plugin heading looked like another verbosity profile.

**Change**: Split the screen into `CHAT VERBOSITY` (navbar-style `>` on the applied profile and a full-width focus bar while navigating) and `TELEGRAM PLUGIN` (status badge, copyable `hero plugin install telegram`, `| Copy command |` or daemon/Project ID + piped action buttons). Focus order is only radios, copy command or Project ID, and Pair/Replace/Clear/Test. Enter applies the focused profile’s value (not the global cursor index). No in-TUI plugin install.

**Validation**: `go test ./...` pass.

## 2026-09-05 — C9 /hero-continue 2: QA closed, Judge started

**Problem**: QA was Escalated 2/2 after a passing TUI `qa_agent` run; `hero stage close` required Running.

**Change**: `hero continue --extra 2` (QA Waiting 2/4). Started QA iter 3, closed with the prior passing report (no re-dispatch), metrics persisted (`gpt-5.6-terra`, 31250/140 tokens, $0.06418, 240000 ms). Auto-advanced; `hero stage start --name judge` (Running 2/3). Did not dispatch stage agents (TUI handoff).

**Validation**: `hero status` — QA Completed 3/4 Auto; Judge Running 2/3.

## 2026-09-05 — TUI focus, resume, Judge loop-back, and transcript attribution

**Change**: Enter on the navbar now transfers focus to the chosen screen. `/hero-resume` performs the deterministic `cycle.Resume` transition asynchronously and immediately enters the existing `/hero-start` bootstrap. Judge prompts now require `hero stage loop-back --from judge` for implementation gaps, reopening completed Implementation and downstream QA before agents rerun. Stage-handoff labels are rendered as `Hero` system messages instead of user input.

**Validation**: focused navbar, resume, stage-handoff, and transcript tests pass; full suite pending.

## 2026-09-05 — Chat footer interruption hint

**Change**: Shortened the fixed Chat footer hint from `enter newline or command` to `enter newline` and added `esc interrupt chat`, making the existing stream-interrupt shortcut visible without increasing the footer footprint materially.

**Validation**: TUI footer and composer hint tests pass.

## 2026-09-05 — C9 Judge-gap completion: conversation boundary and E2E lock

**Change**: Routed every TUI harness Execute through `conversation.Service.SubmitWith` and a per-turn edge dispatcher, preserving Bubble Tea stream rendering while making the service the sole route to the adapter. Expanded the Telegram integration lock to use injected Bot API/vault/IPC for pairing, dual instance suffixes, addressed live routing, offline queue/reconnect, daemon-owned pending cancellation, and outbound notification prefixing.

**Validation**: focused conversation, TUI, and integration tests plus `go test ./...` pass. The `telegram-integration` OpenSpec link remains persisted on cycle C9.

## 2026-09-05 — C9 QA passed on TUI re-run; engine Escalated (2/2)

**Problem**: After Implementation iter 3 closed, the engine escalated QA (`iteration_budget`) instead of `stage start` (QA already used 2/2). The TUI still executed `qa_agent` (`gpt-5.6-terra`). Agent report: `tests_passed: true`, logging pass, lint `go vet` pass; staticcheck/golangci-lint incompatible with Go 1.26. Coverage: engine 78.5%, daemon 57.4%, IPC 64.1%, envhygiene 86.9%.

**Change**: Did not close QA (`hero stage close` requires Running) and did not start Judge. Waiting for `/hero-continue` in the Hero TUI. Estimated metrics (not persisted): model `gpt-5.6-terra`, 31250 in / 140 out tokens, ~$0.0642.

**Validation**: `hero status` — QA Escalated 2/2; Judge Waiting; Implementation Completed 3/4.

## 2026-09-05 — C9 Implementation iter 3: logging fix + Telegram TUI wiring

**Problem**: QA iter 2 failed logging on `LoopBackToImplementation` and Judge left 12 Telegram SDD gaps (Implementation iter 2 returned empty). Backend packages existed (conversation, logrotate, plugin CLI, vault, IPC, daemon) but the TUI and operational paths were not wired.

**Change** (this Implementation pass):
- `Engine.LoopBackToImplementation` now logs every failure path at `error` via a named-return defer and adds `debug` operational logs (request, waiting-shortcut, downstream reset). Default level stays `info`.
- Wired TUI to `conversation.Service`: the model holds `convService` and classifies every composer turn and Telegram inbound through it; the engine publishes `conversation.Event`s through a `Notifier` that the TUI adapter forwards to the daemon outbound path (cycle/stage/approval/error/final only).
- TUI Telegram client (`internal/tui/telegram*.go`): `tea.Cmd`-driven IPC with register/unregister/reconnect + bounded backoff + daemon respawn, Settings Telegram section (installed/version, daemon, configured state, editable abbrev, live suffix), Pair/Replace/Clear/Test actions, keyboard pairing modal with token step + 10-minute code countdown, and transcript `← / → [Telegram · addr]` labels. Added IPC `clear`/`test` frames and daemon handlers.
- Log rotation: TUI slog now writes `.workflow-hero/logs/tui.log` (10 MB × 10) with one-time legacy migration; install/upgrade call `install.MigrateTuiLog`. `.workflow-hero/logs/` added to the managed `.gitignore` block + template.
- Release: `scripts/release.sh` now builds `hero-telegram-daemon` per GOOS/GOARCH with checksums; contract test extended.
- Doctor/status report Telegram plugin health (installed / daemon binary / version match).
- `docs/architecture/architecture-overview.md` updated; integration lock tests added (`internal/integration/telegram_test.go`); `hero cycle openspec-change telegram-integration` persistence verified.

**Validation**: `go build ./...` and `go test ./...` green. Focused TUI Telegram tests, doctor/status/plugin/envhygiene/logrotate/engine/integration tests pass. Did not dispatch stage agents (TUI handoff).

## 2026-09-05 — C9 QA iter 2 failed: loop-back logging coverage

**Problem**: QA (`qa_agent`, `gpt-5.6-terra`) reported `tests_passed: false` / `logging: fail`. `go test ./...`, build, vet, and architecture passed. `LoopBackToImplementation` logs success at info but failure paths return without error-level logs and the changed path has no debug operational logging.

**Change**: Closed QA as Failed with metrics. `hero stage loop-back --from qa` with assignment in `.workflow-hero/cycles/current/qa-gaps.md` (logging fix + leftover Judge gaps from empty Implementation iter 2). Started Implementation (next iteration 3/4). Did not dispatch stage agents (TUI handoff). QA remains 2/2 so the next QA `stage start` may escalate.

**Validation**: `hero status` after close+loop-back+start: Implementation Running; QA Waiting (iteration kept at 2).

## 2026-09-05 — C9 Implementation iter 2 empty (OpenCode restart)

**Problem**: After Judge loop-back, TUI executed `generic_agent` (`opencode-go/deepseek-v4-pro`) for the 12 gaps in `.workflow-hero/cycles/current/judge-gaps.md`. OpenCode serve restarted mid-turn; the agent returned empty output. Working tree has no TUI Telegram wiring (`conversation.Service` unused; no IPC/Settings/pairing/transcript labels; logs/gitignore/release/doctor/e2e lock still missing). Diff remains loop-back engine/docs from the prior orchestrator turn.

**Change**: Closed Implementation without `--failed` (require_human_approval false) from disk artifacts and auto-advanced to QA. Coverage gaps stay for Judge.

**Validation**: `hero status` Implementation Running 2/4 before close; no new `internal/tui` Telegram references.

## 2026-09-05 — C9 Judge loop-back to Implementation

**Problem**: Judge found 12 SDD gaps but Implementation was Completed, so `hero stage start --name implementation` refused. PRD §5.4 requires QA/Judge failure to return to implementation agents.

**Change**: Added `Engine.LoopBackToImplementation` and `hero stage loop-back --from <qa|judge|browser_ui_validation|qa_end_to_end> --reason '...'`. Reopens Implementation and later enabled stages to Waiting, keeps iteration counters, clears StartedAt (timeout clock restarts). Judge report written to `.workflow-hero/cycles/current/judge-gaps.md`.

**Validation**: engine + cycle service tests for loop-back; then `hero stage start --name implementation` for C9 iteration 2.

## 2026-09-05 — C9 Judge: 12 SDD coverage gaps

**Problem**: Judge (`opencode-go/deepseek-v4-pro`) compared `openspec/changes/telegram-integration` (39 tasks) with the tree. Backend packages exist (conversation, logrotate, plugin CLI, vault, IPC, daemon). TUI/operational wiring does not: conversation.Service unused, TUI still owns dispatch, no IPC client, no Settings/pairing/transcript labels, tui.log not migrated, `.gitignore`/release/doctor/docs/context/e2e lock missing. No SDD ambiguity.

**Change**: Closed Judge as Failed with metrics. PRD loop-back is Implementation (`generic_agent`), but `hero stage start --name implementation` refused (`Completed`). Engine has no CLI to reopen a completed stage. Browser UI / E2E remain skipped. Cycle C9 stays active.

**Validation**: `hero status` shows Judge Failed 1/3; Implementation/QA Completed. Did not dispatch stage agents (TUI handoff).

## 2026-09-05 — OpenCode thinking "off" rejected by Console Go

**Problem**: Judge (`opencode-go/deepseek-v4-pro`) failed immediately with TUI `opencode session error: session error`. OpenCode log: `thinking: invalid type: string "off", expected struct ThinkingOptions`. Agent frontmatter and prompt options sent C5 `th=off` as a string; DeepSeek V4 requires `{type: disabled|enabled}`. Nested `session.error` objects were flattened to the generic "session error" text.

**Change**: OpenCode adapter maps thinking to `{type: disabled}` (`off`/`false`) or `{type: enabled}` (`max`/`true`). Agent sync writes the same object into `.opencode/agents` frontmatter. SSE `session.error` unwraps nested `error.message` so the TUI shows the provider text.

**Validation**: `go test ./...` passes, including native payload, agentdef frontmatter, and nested session.error tests.

## 2026-09-05 — C9 QA: flaky tui TempDir cleanup

**Outcome**: Cycle C9 QA (`go test ./...`) failed once in `internal/tui` (`TestConversationCancelDuringStreamWithoutSessionID` left `.workflow-hero` non-empty during `TempDir` cleanup). The isolated test passed 5/5 reruns; git diff was only `.workflow-hero/hero.db`. QA auto-closed (no human approval) and Judge started. Treat as a cleanup race, not an implementation gap from this cycle.

## 2026-09-04 — OpenCode Execute continues after serve restart

**Problem**: During C9 Implementation the OpenCode serve died mid-turn (`go test ./...` tool left `running`, no `session.idle`). Hero restarted serve and reconnected SSE, then waited forever because `GET /session/{id}` has no `status` on OpenCode 1.18.23 and message recovery requires assistant text. Ctrl+C is ignored by design (Esc / Alt+Q).

**Change**: After SSE disconnect, if `opencode serve` generation increased, Execute inspects the last assistant message. A completed turn with text is recovered. A dead/incomplete turn is aborted and continued on the same session with a short continuation prompt (original task is not re-sent). A plain SSE blip without process restart does not re-prompt. Limit: two continues per Execute.

**Validation**: `go test ./...` passes, including continue-after-restart, recover-completed-after-restart, and SSE-blip-does-not-continue tests.

## 2026-09-04 — Cursor TUI login false positive

**Problem**: During C9 Planning, the TUI reported `Cursor Agent CLI authentication required` three times and told the user to run `cursor agent login`. The CLI had already emitted `system/init` with `session_id`, model, and `apiKeySource: "login"` and run for 30s–2.5min. The same session later resumed successfully (also seen 2026-08-20). Cause: `IsAuthFailure` scanned the entire stream-json stdout for `"cursor agent login"`, which appears in docs/tool results; `AuthError.Detail` then used the first stdout line (the init JSON). A related bug treated `NonRetriableError` as retriable because the needle was `retriableerror`.

**Change**: Execute classifies auth failure only when the process failed and no Cursor `session_id` was established. `IsAuthFailure` scans stderr plus non-JSON stdout and dropped the overly broad `"cursor agent login"` / `"unauthorized"` needles. `AuthError.Detail` skips NDJSON lines. `IsRetriableFailure` strips `NonRetriableError` before matching `RetriableError`. Cycle C9 was not cancelled.

**Validation**: `go test ./... -count=1` passes, including stream-json login-phrase success, init-JSON detail skip, authenticated-session exit, and NonRetriableError no-retry tests.

## 2026-08-30 — Release v2.9.2

**Outcome**: Tagged `v2.9.2` (patch bump from `v2.9.1`). Ships Config model catalog picker, welcome dialog surface fill, and Config property/catalog cascade (including Luna `max`).

**Validation**: `go test ./...` green before tag; `scripts/release.sh` artifacts published to GitHub Releases.

## 2026-08-30 — Config model catalog picker

**Problem**: Config model fields cycled one catalog entry per Enter/Space. Harnesses with large catalogs (Cursor/Codex) made choosing a model slow and easy to overshoot.

**Change**: Enter or Space on a model field now opens a slash-overlay-style window listing that harness's models in alphabetical order. Up/Down move the cursor (with a scrolling 8-row window), Enter applies the highlighted model and normalizes thinking/effort/fast, and Escape closes without changing the draft. Tab dismisses the picker and focuses the navbar. Harness and property fields still cycle in place.

**Validation**: `go test ./... -count=1` passes, including picker open/navigate/select/escape, long-catalog scroll, subagent model fields, and harness-field regression coverage.

## 2026-08-29 — Welcome inner black gaps and Config property wheels

**Problem**: The post-`/hero-new` checklist still showed black bars inside the
dialog: nested foreground-only styles reset the surface fill on short wrapped
lines, and the button row's `PlaceHorizontal` leftover cells were unstyled.
Config thinking/effort controls cycled a hardcoded `na/true/false` and
`na/low/medium/high` list, so GPT-5.6 Luna stopped at `xhigh` instead of
catalog `max`.

**Change**: Welcome inner rows now share the surface background and are filled
to the content width before the bordered box is placed. Config property
wheels read C5 snapshot accepted values (plus YAML `na`), normalize
thinking/effort/fast when harness or model changes, and validate models
through the same catalog cascade as the picker. Catalog merge now expands a
partial live/cache effort list when it is missing later rungs such as `max`.

**Validation**: `go test ./... -count=1` passes, including welcome inner-fill
coverage and Luna effort/thinking Config cycle tests.

## 2026-08-29 — TUI welcome backdrop and Config model choices

**Problem**: The post-`/hero-new` guidance dialog left the cells outside its
centered panel unstyled, which showed as black gaps in dark terminals. The
cycle Config screen consulted only the boot-time model rows; boot intentionally
does not launch OpenCode or Codex, so their agent models could appear absent
and could not be changed.

**Change**: The centered welcome dialog now paints all placement whitespace
with Hero's surface background. Config now resolves a model choice through the
same local boot/cache/catalog cascade as the model picker, preserves an
unknown configured value, and starts the enabled-harness C5 refresh
asynchronously after the Config document loads.

**Validation**: Added TUI regression coverage for the full-screen backdrop and
for changing a Codex agent model when boot has only Cursor rows. Focused tests
pass; the full `internal/tui` suite was also run.

## 2026-08-29 — Release v2.9.1

**Outcome**: Tagged `v2.9.1` (patch bump from `v2.9.0`). Ships TUI timer/watchdog fixes, `auto-all` harness permission profile, idea-folder auto-archive on cycle archive, and removes accidental upgrade conflict backup files.

**Validation**: `go test ./...` green before tag; `scripts/release.sh` artifacts published to GitHub Releases.

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

## 2026-09-04 — C09 Telegram remote interface research

**Decision**: Telegram will be an optional official Hero plugin distributed in
the same releases. A local daemon, one per OS user and machine, exclusively
owns Bot API traffic and pushes messages to concurrent TUIs through private
versioned IPC. The TUI and Telegram share a transport-neutral conversation
service; SQLite is durable queue/audit state, never a live-event polling bus.

**Requirements confirmed**: Pair exactly one authorized chat with a
single-use 10-minute code in a Settings modal; keep token and chat id in the
OS credential vault; identify project instances by a user-chosen base
abbreviation plus stable `_2+` suffixes and Free Chats as `free_N`; queue
unavailable-target messages for 24 hours; daemon-owned
`/telegram-cancel-pending` cancels an address's pending queue without touching
cycle execution. Project logs move to `.workflow-hero/logs/tui.log`, retain
10 × 10 MB, and daemon diagnostics use a global rotating user log. Install and
upgrade preserve `.gitignore` while ignoring the project log directory.

**Research artifacts**: PRD-C09-001, ADR-C09-002 (ADR-059–064), UI-C09-001,
plus DEPLOY/TESTING/index/architecture-overview updates and documents registry
entries.

## 2026-09-05 — Telegram instance selection commands

**Decision**: The authorized Telegram chat selects a live *instance* (not only
the project base name), so concurrent project TUIs remain unambiguous. `/list`
returns sorted numbered instance addresses; `/select n` persists the chosen
address in the daemon SQLite store without retaining the credential-vault-only
chat id. Selection is cleared when the authorized chat is replaced or cleared.

**Implementation**: Unprefixed ordinary text and slash commands now route to
the selected live instance. If it disconnects, the daemon reports an actionable
`/list` + `/select` error rather than queuing the turn. Explicit addressed
input remains compatible and keeps its existing offline durable queue behavior.
The daemon replies `OK, Received.` after accepting a live delivery to a TUI.

**Validation**: Added daemon/store/router coverage for deterministic listing,
selection persistence, selected routing, disconnection errors, and live-delivery
confirmation. `go test ./internal/telegram/daemon -count=1`,
`go test ./internal/tui -count=1`, and `go test ./...` pass.

## 2026-09-05 — Telegram status and auto report

**Implementation**: Added TUI-owned Telegram `/status`: it reports `idle`,
active cycle/current-stage data, or `Waiting for harness`, with Session, AI wk,
AI rp, and context-window counters. Telegram turns that start or wait for a
harness turn emit the same immediate status. Settings now persists
`telegram.auto_report_minutes` per project (`0` disabled; `1–300` minutes),
and the existing non-blocking Bubble Tea timer sends periodic status while
Telegram remains connected and paired.

**Validation**: Added focused install/TUI coverage. `go test ./internal/tui
-run 'TestTelegram(StatusText|StatusCommandAndAutoReportSendStatus)' -count=1`
and `go test ./...` pass.

## 2026-08-28 — Discover auto-loads active `docs/idea` files

**Decision**: Research should consider optional design notes under `docs/idea/` at session start. Top-level `archive/` and `tobe/` are excluded; empty folder is fine.

**Implementation**: Added `internal/ideadocs` (`ListActive`, `PromptSection`), TUI injection in `startDiscoverResearchSession`, CLI `hero cycle idea-files` (`--json`), and `discover_agent.md` responsibility to run the command in Cursor IDE. Documented layout in `docs/idea/README.md`.

**Validation**: `go test ./internal/ideadocs/... ./internal/cycle/... ./internal/tui/...` and full `go test ./...`.

## 2026-08-28 — Hide Config after `/hero-archive`

**Implementation**: Added `syncActiveCycleChrome()` to reconcile navbar/palette when the active cycle ends: hides Config, switches label to `alt+1-5`, leaves Config screen for Chat, and clears config draft state. Called on archive success (eager `Status()` sync) and on `refreshDataMsg` when cycle presence drops.

**Validation**: `go test ./internal/tui/...` and full `go test ./...`.
