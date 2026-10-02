# Implementation Tasks: Browser Validation, Test Access, and Active Execution Budgets

Every task has an executable verification criterion. Artifacts stay English.

Lint/static baseline rule (user-approved 2026-10-01; replaces the former one-wave exception and applies to every remaining wave and to task-19): the `staticcheck ./...` and `golangci-lint run` diagnostics recorded in `verification-wave-1-lint-baseline.txt` pre-date C17, are out of scope for this change, and are tracked as Known Technical Debt in `context/current-state.md`. Lint/static acceptance is exactly two commands (amended 2026-10-01, design.md D9): `golangci-lint run --new-from-rev=a74db0f --max-issues-per-linter 0 --max-same-issues 0 ./...` exits 0 (`a74db0f` = pre-C17 base commit), and `staticcheck ./...` reports no `file|message` key absent from the baseline's staticcheck section. Pre-existing diagnostics in files C17 touched are out of scope. Verified 2026-10-01: both commands pass. Baseline diagnostics never set `tests_passed` or `acceptance_gates.required_tests_passed` to false. All assigned tests, builds, `go vet`, and race checks remain mandatory.
Report semantics: `tests_passed` and `acceptance_gates.required_tests_passed` cover `go test ./...` plus the Verify commands of the IDs in `tasks_completed`. Unfinished IDs belong in `tasks_remaining` and do not make either flag false.

`[SERIES]` = ordering constraint. `[PARALLEL]` = independent `generic_agent` fan-out after prerequisites.
Scope: **native** → all tasks use `[agent:generic_agent]`. Apply `go-engineering` and `golang-tui` skills.
Schema: current v15 verified in `internal/store/migrate.go`; next free version is **v16** (ADR-104).
Acceptance IDs B01–B21 map in traceability.md. Never use real credentials or the user's Angular app; use fake clocks/streams, real temp SQLite/files, and a local form-login fixture with operator/admin roles.

## Execution order and fan-out

```text
PARALLEL: task-01 dotenv parser | task-04 schema v16 migration
  -> PARALLEL after 01: task-02 safe store | task-03 executor boundary
  -> PARALLEL after 01+04: task-05 blocked reports | task-07 budget timer | task-17 workflowconfig+templates
  -> task-06 scheduler blocked state (after 04+05)
  -> PARALLEL after 03+05: task-09 bounded preparation | task-10 coverage gate (10 also after 06)
  -> task-11 screenshot capture (after 04; PARALLEL with 06/07)
  -> PARALLEL after 02+17: task-14 Config UI | task-15 archive lifecycle (15 also after 04)
  -> PARALLEL after 11: task-12 screenshot control/UI | task-13 Telegram image edge
  -> task-08 TUI budget/blocked/progress display (after 06+07)
  -> task-16 four-harness projections + help (after 05+09)
  -> task-18 traceability + docs/context (after 16; PARALLEL-safe with 12/13)
  -> task-19 final verification (after all) [SERIES]
```

Service/store/UI work must not precede its contract task. Lifecycle rollout is not complete until v15→v16, lease-style budget checkpoint, and archive/delete tests pass (TESTING.md §C17).

## Approved contract clarifications (2026-09-30)

The user accepted both previously pending recommendations and authorized further Implementation. Apply these contracts to the remaining assignments; keep all existing owners, task IDs, dependencies, and completion gates.

- task-14: persist the top-level `test_access.enabled` boolean in `workflow-config.yml`, default `false` when absent. Config and execution use the same setting; required-login preparation blocks with enable instructions while disabled. Implement the managed config/template support and atomic YAML persistence needed by this editor. Verify persistence across reload/restart, the absent-key default, invalid-type rejection, unrelated-key preservation, and the disabled-required-login gate in addition to the existing editor criteria.
- task-03/task-09/task-10: the shared non-secret recipe, approved execution method, and coverage denominator live in `.workflow-hero/cycles/current/browser-plan.json`. Implement the shared schema, loader, and validation before production preparation/execution. The plan's application-specific values are resolved dynamically by Planning; credential values remain in `.env.hero` and private executor memory.
- task-09 preparation questions answered (user-approved 2026-10-01, design.md D10 decisions a–c): project-relative fixture paths are checked by the Preparer, label-only fixtures by the stage session; planned HTTP/API E2E skips browser admission with screenshots disabled; pre-dispatch preparation is local only and login/protected-role verification stays in the dispatched session. No adapter API. Do not ask these again.
- task-09/task-11/task-13 (user-approved 2026-10-01): the production call sites are fixed in design.md D10. Hero does not own a browser process; the browser runs inside the stage agent's harness. Wire the existing `Preparer`, `screenshots.Service`, and Telegram delivery into the TUI scheduler at the D10 call sites; do not invent another browser runtime.
- task-16/task-18: `planning_agent` produces this file during Planning in each consuming application with browser validation; Browser UI/E2E preparation and agents consume it. Update all four harness producer/consumer instructions and document the path in the planned execution-contract references. C17 acceptance continues to use the synthetic local operator/admin fixture.

## 1. Dotenv parser — [SERIES foundation; PARALLEL with task-04]

- [x] 1.1 [task-01] [agent:generic_agent] Implement `internal/testaccess` dotenv parse/validate/serialize: blank lines, comments, unquoted, literal single-quoted, double-quoted with `\n \r \t \" \\` only, literal `$`, spaces/`#` preserved in quotes; reject unsupported escapes/malformed quoting with line/field diagnostics that never include values; user IDs `[a-z][a-z0-9_]*` with uppercase suffix, duplicate-ID/key and ambiguous-normalization rejection; required-account usability (non-empty login/password/profile); complex-value round-trip without evaluation (PRD FR-02; B01–B04). Verify: `go test ./internal/testaccess/ -run 'TestDotenv|TestUserID|TestRoundTrip' -v`.

## 2. Safe credential store — [PARALLEL after task-01]

- [x] 2.1 [task-02] [agent:generic_agent] Implement `internal/testaccess` safe store: idempotent root-anchored `/.env.hero` gitignore insertion preserving content; 0600 atomic replace with no residual backups; loaded-revision stale-draft refusal with Reload instruction; per-attempt credential snapshots; manual-file validation (parser errors, owner/mode, git ignore/tracked status with block-and-instruct on tracked files, symlink/unsafe-path refusal); competing-TUI ownership checks (PRD FR-02–FR-03; B04–B05). Verify: `go test ./internal/testaccess/ -run 'TestSafeStore|TestStale|TestTracked|TestUnsafe' -v`.

## 3. Authentication executor boundary — [PARALLEL after task-01]

- [x] 3.1 [task-03] [agent:generic_agent] Implement the deterministic executor interface and shared login-recipe schema; resolve application-specific origins, routes, locators, protected targets, expected roles, users, and fixtures dynamically during Planning instead of hardcoding them in Hero. Hero's tests use only a synthetic local protected fixture with operator/admin roles. Implement private selected-account reads, the same isolated in-memory context for login and tests, fresh login every execution (no state export/reuse), protected-role verification with expected-denial pass and invalid-account block, sanitized results only (IDs/profile/outcomes), unsupported MFA/CAPTCHA/SSO block, and no-sandboxing disclosure for privileged harnesses. Preserve the tool preference in FR-05: an existing Playwright Test suite for repeatable E2E; otherwise Playwright CLI with its official skill, skills-less CLI if skills cannot load, and MCP only for persistent/iterative inspection or verified CLI capability gaps. Require Playwright >=1.63.0 and block missing/below-minimum tools without QA installation (PRD FR-01, FR-03, FR-05; ADR-103; B05–B07, B11–B12). Verify: `go test ./internal/testaccess/ -run 'TestExecutor|TestSentinel|TestFreshLogin|TestExpectedDenial' -v` with sentinel passwords/tokens asserted absent from prompts, SQLite, YAML, logs, events, and captures.

## 4. Schema v16 migration — [SERIES foundation; PARALLEL with task-01]

- [x] 4.1 [task-04] [agent:generic_agent] Add transactional additive v16 migration in `internal/store/migrate.go` (stage budget ledger, blocker/coverage records, screenshot manifests per design D3); never recreate `hero.db`; add a real-SQLite v15 fixture proving cycles, stages, events, metrics, audit conversation, findings, ToDos, and session-history rows intact with empty new tables (PRD §6; ADR-104–106). Verify: `go test ./internal/store/ -run 'TestMigrateV15ToV16|TestV15FixtureIntact' -v`.

## 5. Blocked report contracts — [PARALLEL after task-01+task-04]

- [x] 5.1 [task-05] [agent:generic_agent] Extend `internal/cycle/reports` Browser UI/E2E typed contracts with `blocked`, validated preparation, blockers (stable reason, affected coverage/profile IDs, uncertainty, next action), coverage denominator with mandatory accounting, and safe evidence references; mixed validated reports preserve genuine C15 findings/occurrences without scheduling repair; invalid reports keep diagnostic/retry behavior (PRD FR-07–FR-08; ADR-104; B08–B10). Verify: `go test ./internal/cycle/... -run 'TestBlockedReport|TestMixedFindings|TestCoverageDenominator' -v`.

## 6. Scheduler blocked state — [SERIES after task-04+task-05]

- [x] 6.1 [task-06] [agent:generic_agent] Persist operational Blocked/interrupted reason and budget state in `engine/store`; stop auto-advance and automatic Implementation repair while blocked; `/hero-continue` rechecks prerequisites from current configuration before genuine-finding repair and outstanding coverage gates; expose additively via cycle/status and Telegram; scheduler-only transitions (PRD FR-08; ADR-104; B08). Verify: `go test ./internal/engine/ ./internal/cycle/ -run 'TestBlockedState|TestHeroContinueRecheck|TestNoAutoAdvance' -v`.

## 7. Active execution budget — [PARALLEL after task-04; SERIES before task-08]

- [x] 7.1 [task-07] [agent:generic_agent] Implement durable cumulative active wall-time budgets per stage in `engine/store` plus TUI generations, independent deadline scheduling, and async scoped cancellation with a fixed 15-second termination grace: elapsed-once for parallel agents, pause only on human-only waits with no active sibling, persist start/pause/resume/termination and every 5-second active checkpoint (fake-clock), restart charging last active checkpoint with interrupted state and `/hero-continue`, no offline counting, no fresh budget on reconnect/retry, zero-balance explicit-increase gate, generation revocation before cancellation, serialized acceptance requiring positive remaining time, late/expired/cancelled result rejection, passive watchdog/CheckHealth untouched (PRD FR-09–FR-11; ADR-105; B13–B16; amends PRD §5.4). Verify: `go test ./internal/engine/ ./internal/store/ ./internal/tui/ -run 'TestActiveBudget|TestExpiry|TestRace|TestPause|TestPassiveHealth' -v` covering all seven stages, concurrent/nested/partial work, and repetitive-stream expiry.

## 8. Bounded preparation — [PARALLEL after task-03+task-05]

- [x] 8.1 [task-09] [agent:generic_agent] Implement bounded preparation: 120s total per attempt capped by remaining stage budget, max two attempts per prerequisite, readiness/browser-permission/selected-user/fixture checks with bounded locators (no fixed sleep loops), and real selected-method admission in the stage session. For repeatable E2E prefer an existing Playwright Test suite; for coding-agent browser control prefer CLI with the official skill, then skills-less CLI, then MCP only for persistent/iterative inspection needs or a verified CLI capability gap. Verify Playwright >=1.63.0 and actual harness/tool capability; missing/below-minimum tools block with setup instructions. No QA provisioning, silent HTTP fallback, or automatic latest install; preparation spends active time without consuming a validation iteration (PRD FR-05–FR-06; B11–B12). Verify: `go test ./internal/... -run 'TestPreparation|TestMethodAdmission|TestNoProvisioning' -v` with fake clocks.

## 9. Coverage gate — [SERIES after task-05+task-06; PARALLEL with task-09]

- [x] 9.1 [task-10] [agent:generic_agent] Implement the traceable passing gate: stable coverage IDs (requirement/acceptance, screen/journey, user/profile, mandatory flag, expected result, evidence), per-mandatory-screen render/CSS/console/network checks, Health-before-Visual with 1280 desktop and 1280/768/375 references, missing optional references as warnings only, E2E business-outcome assertions (navigation alone insufficient), isolated contexts per user with safe-concurrency rule, planned/executed/passed/failed/blocked/skipped accounting, no pass with mandatory items pending, mandatory scope changes requiring approval + plan update (PRD FR-07; B08–B10). Verify: `go test ./internal/cycle/... -run 'TestCoverageGate|TestMandatoryAccounting|TestOptionalReference' -v`.

## 10. Screenshot capture — [PARALLEL after task-04]

- [x] 10.1 [task-11] [agent:generic_agent] Implement safe capture: per-stage `screenshots.enabled` gating (default false; HTTP-only E2E cannot enable), immutable ready assets under `current/screenshots` with stable IDs and sanitized manifests (cycle/stage/attempt/coverage/user/time/path/result), no partial-write visibility, no attempt/role overwrites, capture suspension during credential fill/submit with masks and omission reasons, mandatory-evidence failure blocking its item vs optional-delivery warning, archive retention (PRD FR-12; ADR-106; B18, B20). Verify: `go test ./internal/... -run 'TestScreenshotCapture|TestManifest|TestSecretSuppression|TestReadySet' -v` with real temp files.

## 11. Screenshot control and collection UI — [PARALLEL after task-11]

- [x] 11.1 [task-12] [agent:generic_agent] Implement TUI-owned `/hero-screenshot` (latest/list/id/todos; reserved selectors; distinct no-capture vs unknown-ID errors) plus the screenshot collection (scrolling cards with ID/coverage/stage/attempt/user/timestamp/result, Enter/o viewer, copy-path/Save, Open-all with bounded launches): async during streaming, no harness dispatch or new capture, ready-set snapshot, bounded batches with progress/partial-failure IDs, collision-free shortcut chosen from the keyboard guide and listed in `/help` and the footer (UI §§4–5; B18–B19). Verify: `go test ./internal/tui/ -run 'TestHeroScreenshot|TestCollection|TestTodosBatch' -v`.

## 12. Telegram image delivery — [PARALLEL after task-11]

- [x] 12.1 [task-13] [agent:generic_agent] Extend the addressed Telegram IPC/outbound edge with image delivery: actual images only under project `always_send` with paired/connected addressed chat (disabled explains TUI availability + setting), validated managed references with safe captions, bounded ordered batches with retry/dedup IDs, daemon version-mismatch/disconnect/partial-failure warnings that retain local cards, no agent-to-Telegram sending, Bot API size/group limits verified (never hardcoded) (PRD FR-12–FR-13; ADR-106; UI §5; B20). Verify: `go test ./internal/telegram/ ./internal/tui/ -run 'TestImageDelivery|TestAlwaysSend|TestPartialBatch' -v` with injected transport (no live Telegram).

## 13. Config editors — [PARALLEL after task-02+task-17]

- [x] 13.1 [task-14] [agent:generic_agent] Implement Config Test users editor (opt-in toggle, scrollable add/edit/remove, stable ID/profile/login/masked password, no reveal/copy/export, Save validates field names and atomically updates `.env.hero`, Discard/Cancel, unrelated-entry preservation, external-change Reload gate, read-only under execution guards, disabled-with-required-login block) and per-stage Screenshots Off/On toggles reusing the Telegram Always-send setting (PRD FR-02, FR-12; UI §1; B01–B03). Verify: `go test ./internal/tui/ -run 'TestTestUsersEditor|TestStaleReload|TestScreenshotToggle' -v`.

## 14. Credential lifecycle — [PARALLEL after task-02+task-04]

- [x] 14.1 [task-15] [agent:generic_agent] Implement archive exact-path `.env.hero` removal only after archive prerequisites succeed (missing is success; symlink/unsafe refusal; failed OpenSpec/refused archive keeps the file; cleanup failure leaves archive pending with actionable retry; retry safety; screenshots retained; ignore rule kept), preservation on finish/cancel/upgrade/uninstall with uninstall disclosure, project/cycle ownership checks, resume reconfiguration (PRD FR-04; B05, B07). Verify: `go test ./internal/cycle/ ./internal/store/ -run 'TestArchiveCleanup|TestArchivePending|TestLifecyclePreserve' -v`.

## 15. Workflowconfig and templates — [PARALLEL after task-01+task-04]

- [x] 15.1 [task-17] [agent:generic_agent] Add managed keys (`screenshots.enabled` per Browser UI and browser E2E stage, default false; timeout/budget semantics; checkpoint/termination constants from design D5) with Config validation and atomic YAML write, plus install/upgrade templates provisioning root `.env.hero.example` placeholders only (never overwriting customized examples or application `.env`, never creating values, never copying `.env.hero` into checksums/backups/snapshots/artifacts) (PRD FR-02, FR-09, FR-12; DEPLOY §C17; B18). Verify: `go test ./internal/workflowconfig/ ./internal/install/ ./internal/upgrade/ -run 'TestManagedKeys|TestEnvHeroExample|TestNoCredentialLeak' -v` plus golden template tests.

## 16. TUI budget/blocked/progress display — [SERIES after task-06+task-07]

- [x] 16.1 [task-08] [agent:generic_agent] Render durable blocked state (stage, reason, affected coverage/profile IDs, corrective steps, remaining budget, `/hero-continue`; PT-BR example per UI §2), active elapsed/remaining, preparation phase, current coverage/profile IDs, completed/required counts, sanitized tool lifecycle, one-shot coalesced warnings (60s no progress / 30 repetitive events; suspended during human waits; never corrective), interrupted/expired states with preserved evidence, no secret values in any surface (UI §§2–3; B21). Verify: `go test ./internal/tui/ -run 'TestBlockedDisplay|TestBudgetDisplay|TestProgressWarning' -v`.

## 17. Four-harness projections and help — [SERIES after task-05+task-09]

- [x] 17.1 [task-16] [agent:generic_agent] Apply typed-report/blocked, preparation/method, budget, and screenshot contracts to all four TUI harness projections (Cursor, OpenCode, Codex, Claude) plus help text together; replace the missing-Playwright frontend-failure instruction with environment/tool-absence escalation; keep HTTP mode explicit; no IDE execution or removal content (PRD §6; B17). Verify: `go test ./... -run 'TestHarnessParity|TestProjectionMatch' -v` proving four projections match.

## 18. Traceability, docs, and context — [PARALLEL-safe after task-16]

- [x] 18.1 [task-18] [agent:generic_agent] Write `traceability.md` (B01–B21 to FR/tasks), update `docs/testing/TESTING.md` C17 gates from planned to implemented where true, and append `context/context-log.md`; keep artifacts English (PRD §6). `context/current-state.md` is not part of this task: the orchestrator updates it in the Stage Close / `/hero-finish` sequence, and implementation agents must not edit it (C15 report contract). Verify: registry/path/whitespace consistency checks pass with no Go changes in this task.

## 19. Final verification — [SERIES after all]

- [x] 19.1 [task-19] [agent:generic_agent] Run `go test ./...`, `go test -race ./...`, `go vet ./...`, both builds, and the two lint/static commands of the baseline rule at the top of this file; verify four-harness parity, safe archive, active cancellation/race coverage, and every task's owner marker and acceptance criterion; confirm no `.env.hero` or credential values in repo, archives, or snapshots (TESTING.md §C17; DEPLOY §C17). Verify: full suite output attached to the completion report.
