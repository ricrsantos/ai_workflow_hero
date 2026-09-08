# Implementation Tasks: Claude Code Adapter for Hero TUI

Every task below has an executable test or verification criterion. All task
artifacts and code remain in English. `[SERIES]` means the listed items have an
ordering constraint; `[PARALLEL]` means the listed items may be delegated to
independent implementation agents once the stated prerequisite is complete.

## Execution order and fan-out

```text
task-01 protocol gate
  -> task-02 shared state/contract
  -> task-03 adapter core
  -> task-04 permission bridge + task-05 lifecycle/health
  -> task-06 projection + task-07 model catalog
  -> task-08 install/upgrade/uninstall
  -> task-09 TUI runtime
  -> task-10 diagnostics/docs + task-11 Telegram
  -> task-12 mixed-harness integration
  -> task-13 regression/golden suite
  -> task-14 final verification and context landing
```

The following fan-out is intentional:

- After task-01: task-02, task-03.1, task-06.1, and task-07.1 can proceed in
  parallel.
- After task-03.1: task-03.2 and task-03.3 can proceed in parallel; task-03.4
  follows the decoder contract.
- After the protocol and adapter event contracts: task-04.1/4.2 and
  task-05.1/5.2 can proceed in parallel; their callback/watchdog integration
  follows those contracts.
- After task-02 and task-01: task-06 and task-07 can proceed in parallel.
- After task-08/09 and the bridge callback contract: task-10 and task-11 can
  proceed in parallel.
- After task-12/13: task-14 is series-only.

## 1. Protocol compatibility gate — [SERIES]

- [x] 1.1 [task-01.1-protocol-fixture] Add an injected fake Claude executable/process launcher that records argv, working directory, environment, process-group operations, and version/flag probes; cover the minimum version `2.1.261` and required stream-json flags with deterministic fixtures.
- [x] 1.2 [task-01.2-stream-fixture] Freeze representative NDJSON fixtures for system/init session identity, partial text/thinking, tools, subagents, retries, hooks/plugins, usage, final result, malformed/unknown events, resume, auth failure, and stderr; assert line-by-line fixture decoding.
- [x] 1.3 [task-01.3-permission-fixture] Freeze the `permission-prompt-tool` stdio request/decision/termination fixture, one-time token rules, and compatibility outcome; add a test that unsupported ask-mode behavior is reported as an explicit error rather than downgraded.

## 2. Shared harness state and contracts — [PARALLEL after task-01]

- [x] 2.1 [task-02.1-state] Add `claude` to supported harness IDs, workflow/config validation, HeroJSON state, selection/migration defaults, and free-chat/last-harness behavior while keeping existing installations unchanged; test disabled-by-default upgrades and invalid IDs.
- [x] 2.2 [task-02.2-registry] Register the Claude adapter in the harness registry and explicit fallback/model resolution paths; keep Cursor/OpenCode/Codex fallback and session identity isolated; test lazy local catalog discovery without starting a Claude process.
- [x] 2.3 [task-02.3-health-contract] Extend shared health/watchdog handling for Claude's five-minute stall timeout, thirty-second probes, normalized activity/progress, permission/question pauses, and process/session status; test that non-rendered activity still prevents false stalls.

## 3. Claude adapter execution core — [SERIES internally; PARALLEL package after task-01]

- [x] 3.1 [task-03.1-adapter-contract] Create `internal/adapters/claude` with injected process, clock, random-token, filesystem, and executable-lookup dependencies; implement the shared lifecycle contract without starting a process from availability, create-session, or resume validation; add contract tests.
- [x] 3.2 [task-03.2-process-command] Implement version/flag availability checks, command construction for `claude -p --output-format stream-json --verbose --include-partial-messages --model`, working-directory and permission mapping, stderr redaction, auth/version/actionable errors, and context-aware process-group supervision; test argv and failure classification.
- [ ] 3.3 [task-03.3-ndjson-normalizer] Implement incremental NDJSON decoding and normalization into existing text/thinking/tool/warning/activity/session/permission/question metadata, including subagents, retry, hook, plugin, and bounded redacted unknown-event warnings; test all task-01 fixtures.
- [ ] 3.4 [task-03.4-result-assembly] Implement final-result repair, output/session/usage precedence, duration and stream completion, malformed terminal payload errors, native model/effective property reporting, and `Dispatch` delegation; test partial-stream exits and normal completion.

## 4. Permission bridge and decision routing — [PARALLEL after task-01.3 and task-03.1/3.3; 4.3 SERIES]

- [ ] 4.1 [task-04.1-bridge] Implement the execution-scoped stdio permission bridge with random one-time token validation, request-shape validation, permission-only forwarding, bounded cleanup, and no credential/plugin/MCP exposure; test token replay, malformed requests, and bridge shutdown.
- [ ] 4.2 [task-04.2-profiles] Map ask, auto-project constrained edits, and auto-all profiles to the proven Claude flags/transport; return an actionable fail-closed unsupported-ask error and never silently approve; test profile-specific command construction.
- [ ] 4.3 [task-04.3-callbacks] Connect bridge decisions and Claude questions to the existing TUI callback contract, including pause/resume watchdog state and cancellation while waiting; test approval, rejection, timeout, and cancellation paths.

## 5. Session lifecycle, cancellation, and health — [PARALLEL after task-03; 5.3 SERIES with task-04]

- [ ] 5.1 [task-05.1-session-binding] Emit and persist the first native session ID before completion, validate same-harness/stage resume, use `--resume <id>` only for the bound session, and avoid resending the prompt after a resume failure; test cross-harness rejection and interrupted-turn recovery.
- [ ] 5.2 [task-05.2-cancel-cleanup] Implement idempotent SIGINT-to-process-group cancellation, bounded wait, kill fallback, bridge cleanup, abnormal-exit cleanup, and no orphan child processes; test escalation and repeated cancellation with the fake launcher.
- [ ] 5.3 [task-05.3-watchdog-health] Wire normalized activity, progress, session, retry, and pause events into health snapshots and watchdog probes; test five-minute stall behavior, thirty-second probe scheduling, permission pauses, and process/session termination states.

## 6. Claude projection and CLAUDE.md preservation — [PARALLEL package after task-01 and task-02.1; 6.3 SERIES]

- [ ] 6.1 [task-06.1-assets] Add embedded Claude agents, commands, and workflow-hero/grilling skills plus owned paths/checksum groups; test asset lookup, projection paths, conflict reporting, and removal of only Hero-owned files.
- [ ] 6.2 [task-06.2-marked-block] Implement the marked root `CLAUDE.md` block importing `@AGENTS.md`, with explicit insert/update/leave-unchanged decisions, atomic writes, duplicate/malformed marker handling, user-text preservation, and managed-field-only updates; test file modes and symlink/error behavior.
- [ ] 6.3 [task-06.3-projection-lifecycle] Integrate enable/disable projection decisions, disabled-upgrade no-file behavior, uninstall preservation, and upgrade conflict behavior; test against temporary projects containing user `.claude` files and unmarked `CLAUDE.md` text.

## 7. Claude model catalog and properties — [PARALLEL package after task-01 and task-02.1; 7.2/7.3 SERIES]

- [ ] 7.1 [task-07.1-catalog] Add `assets/models/claude.yml` with aliases `sonnet`, `opus`, `haiku`, and `fable`, full native IDs, official dated suffixes/metadata, and unknown pricing where appropriate; test schema and duplicate/unsupported metadata rejection.
- [ ] 7.2 [task-07.2-properties] Add provider-namespaced `claude` resolution, C5 role/alias precedence, effort-to-`--effort`, thinking/fast `na` handling unless proven by the spike, system/init injection, and effective verbosity; test collisions with the Anthropic catalog and native argv output.
- [ ] 7.3 [task-07.3-discovery] Integrate local catalog discovery into model pickers, boot, fallback, and Telegram without launching Claude or requiring credentials; test provider labels, aliases, full IDs, and unsupported-model diagnostics.

## 8. Install, upgrade, and uninstall integration — [SERIES after task-02 and task-06]

- [ ] 8.1 [task-08.1-install] Wire Claude selection, HeroJSON serialization, asset copying, checksums, default-disabled state, and `hero harness enable/disable` behavior through the existing install service; test a clean temporary installation.
- [ ] 8.2 [task-08.2-upgrade-uninstall] Wire upgrade and uninstall to preserve disabled projections, user-owned `.claude` files, and unmarked `CLAUDE.md` content while removing only owned assets; test enabled, disabled, conflicted, and partially missing states.
- [ ] 8.3 [task-08.3-install-integration] Add integration coverage using the real embedded filesystem and temporary projects for install, enable, upgrade, disable, uninstall, and re-enable; assert checksums, marker choices, and unchanged legacy harness state.

## 9. TUI runtime integration — [SERIES after tasks 3, 5, 6, 7, and 8]

- [ ] 9.1 [task-09.1-boot-picker] Add Claude to boot availability diagnostics, harness enable/disable picker, explicit fourth-harness labels, and model picker while excluding it from persistent-server reset/reap flows; test mixed enabled-harness selections.
- [ ] 9.2 [task-09.2-conversation] Connect normalized Claude deltas, early session persistence, permission/question callbacks, cancellation, health updates, usage, and final errors to the existing Bubble Tea conversation flow; test that Update remains non-blocking and session IDs persist before completion.
- [ ] 9.3 [task-09.3-routing-properties] Integrate Claude C5 model labels, native property display, explicit same-harness resume, fallback behavior, config/agent selection, and no Cursor Runtime reuse; test routing and label formatting for sonnet/opus/haiku/fable.
- [ ] 9.4 [task-09.4-diagnostics-ui] Add actionable PATH/version/flag/auth/ask-transport errors, progress and permission-wait rendering, pause/resume behavior, and warnings for Claude loading project instructions/hooks/plugins/MCP; test view-model messages and cancellation affordances.
- [ ] 9.5 [task-09.5-tui-regression] Extend Bubble Tea tests for narrow widths, ANSI-aware labels, boot failure, no reset target, permission decisions, unknown-event warnings, and four-harness coexistence; verify all I/O is issued through commands/callbacks.

## 10. CLI diagnostics and documentation — [PARALLEL after tasks 2, 6, 7, and 8]

- [ ] 10.1 [task-10.1-doctor-status] Add Claude-aware Doctor and Status output for enabled state, projection, PATH/version/required flags, active process/session/health, permission pause, and actionable remediation; test deterministic output with fake probes.
- [ ] 10.2 [task-10.2-cli-docs] Update deterministic CLI help, harness/model listings, detection behavior, deployment/testing guidance, and document registries for the C13 adapter while preserving the CLI/runtime boundary; test generated/help snapshots or equivalent command behavior.
- [ ] 10.3 [task-10.3-cli-regression] Add CLI integration tests for detect, install, enable/disable, doctor, status, model listing, unsupported ask mode, and upgrade-with-disabled-Claude; assert no live process or credential access.

## 11. Telegram decision integration — [PARALLEL after tasks 4 and 9]

- [ ] 11.1 [task-11.1-telegram-routing] Route Claude permission/question callbacks, labels, wait states, rejection, timeout, and cancellation through the existing Telegram integration without storing credentials or bypassing the permission profile.
- [ ] 11.2 [task-11.2-telegram-tests] Test Telegram approval/rejection/timeout/cancel behavior with the fake bridge and verify callback cleanup, session binding, and no cross-harness dispatch.

## 12. Mixed-harness acceptance integration — [SERIES after tasks 9, 10, and 11]

- [ ] 12.1 [task-12.1-four-harness] Exercise install, boot, model resolution, one execution, cancellation, and fallback with Cursor, OpenCode, Codex, and Claude enabled together; assert no adapter receives another harness's session or process signals.
- [ ] 12.2 [task-12.2-acceptance] Run the C13 acceptance scenarios for protocol fixtures, ask bridge, asset/CLAUDE.md preservation, catalog/C5/cost/fallback, Doctor/Status/Telegram, watchdog, and cancel; record each result in test output.

## 13. Regression and golden coverage — [PARALLEL after the affected implementation groups]

- [ ] 13.1 [task-13.1-golden-events] Add/update golden tests for normalized text/thinking/tool/session/activity/warning/permission/question events, retries/subagents/hooks/plugins, usage, and final-result repair.
- [ ] 13.2 [task-13.2-fault-injection] Add fault-injection tests for malformed NDJSON, unknown events, version/flag mismatch, auth failure, bridge replay, resume failure, process-group cancellation, kill escalation, watchdog stalls, and projection conflicts.
- [ ] 13.3 [task-13.3-cross-package] Run package-level and integration regressions for harness manager, store session binding, install/upgrade/uninstall, model properties, TUI, Doctor, Status, and Telegram; ensure existing Cursor/OpenCode/Codex behavior is unchanged.

## 14. Final verification and context landing — [SERIES]

- [ ] 14.1 [task-14.1-traceability] Review implementation against the C13 PRD/ADR/UI and every delta in this change, update affected architecture/document indexes, and record final decisions and test evidence in `context/current-state.md` and `context/context-log.md`.
- [ ] 14.2 [task-14.2-full-verification] Run `openspec validate claude-code-adapter --strict`, `go test ./...`, documented build/static checks, and the C13 acceptance suite; resolve all failures before handoff.
