# Browser Validation, Test Credentials, and Stage Execution Budgets

> Active idea for a future Hero development cycle. Recorded 2026-09-30.
> This document captures the user's agreed direction and implementation requirements. It does not authorize implementation or replace approved PRD/UI/ADR/OpenSpec documents. Research must resolve the explicitly listed design questions before Planning.

## 1. Problem and evidence

A user running Hero against an Angular application reported two inadequate validation outcomes:

- Browser UI approved the login screen while most application functionality remained behind authentication.
- QA End-to-End remained Running for approximately 29 minutes despite a configured 15-minute timeout. The user's local investigation reported one OpenCode run with MiMo V2.6 Pro performing 195 loop/process/stream steps before cancellation. Hero reportedly retained Running afterward.
- The E2E agent initially searched for TESTING.md and a Playwright/E2E harness, found neither, and continued inspecting the application before repeated `part type patch` activity appeared.

The OpenCode log findings above are user-supplied incident evidence, not independently reproduced in this repository. They do not establish a Playwright failure, a model defect, or repeated actual file edits.

Read-only inspection of Hero source confirmed:

| Current behavior | Source | Consequence |
|---|---|---|
| Browser Health opens the app without requiring an explicit set of authenticated screens | `assets/opencode/agents/browser_ui_agent.md` | Login-only validation can satisfy the current instructions |
| Browser UI and E2E validation reports accept passed/failed | `internal/cycle/reports/validation.go`, `browser_ui.go`, `e2e.go` | Missing prerequisites have no dedicated blocked outcome |
| Missing Playwright is instructed to become a frontend failure | Browser UI and orchestration agent assets | Environment problems can trigger inappropriate Implementation loop-back |
| Stage budgets are checked in StartStage between iterations | `internal/engine/engine.go` | A single Execute can exceed timeout_minutes |
| Implementation also checks timeout between productive partial waves | `internal/tui/stage_handoff.go` | This checkpoint still cannot bound an active wave |
| Execute uses a cancellable context without a stage-derived deadline | `internal/tui/conversation.go` | No general hard stage limit is enforced during Execute |
| Stage progress scheduling returns while executions/handoff are active | `internal/tui/stage_progress.go` | Periodic progress scheduling is not continuous budget enforcement |
| Unknown message part types emit generic activity text | `internal/adapters/opencode/events.go` | `part type patch` does not identify a browser command or test progress |

The existing timeout behavior is documented in `docs/product/PRD.md` §5.4. The proposed execution limit therefore requires an explicit product and architecture amendment, rather than being described as an already-supported behavior.

## 2. Agreed scope and non-goals

The cycle must address credentials, preparation, coverage, blocked reporting, efficient Playwright execution, and timeout enforcement together. Apply the browser contracts across Cursor, OpenCode, Codex, and Claude projections; do not fix only OpenCode prompts.

Agreed requirements:

1. Credentials are the default mechanism for authenticated browser tests.
2. Store test credentials in a project-root `.env.hero`, separate from the application's `.env`.
3. Users can edit `.env.hero` manually or through Hero's Config screen.
4. Support an arbitrary number of test users and distinct access profiles.
5. Automatically add `.env.hero` to the project's `.gitignore`.
6. Delete `.env.hero` when the cycle is archived, and never include its contents in the archive.
7. Prioritize test-access setup in the checklist after `/hero-new`.
8. Browser UI and E2E explicitly block when required test users are missing or unusable.
9. Make the actual Playwright execution method explicit and verify that it is usable.
10. Require evidence for the planned screen/journey coverage before approval.
11. Enforce execution budgets for all configured stages in Hero TUI, including active executions.

Out of scope for this first cycle: saved browser profiles, storageState import/export, persisted authentication tokens/cookies, cross-stage session reuse, OS credential-vault integration, and collecting passwords/OTP/tokens through agent chat or Telegram. Each browser stage execution authenticates afresh; ordinary in-memory authentication during that execution is necessary and allowed.

## 3. Test access contract

### 3.1 Non-secret configuration

Research/Planning must identify the test environment, URL, authentication entry point, required profiles, and screen/journey assignments. Store these non-secret requirements in the test plan and managed configuration, not in credential values.

Authentication requirement must be explicit: not needed, required, or unresolved. An unresolved requirement must be resolved before approving the test plan; lack of configured accounts must not silently mean authentication is unnecessary.

Profiles describe permissions, not just display names. Credentials present in a file do not prove a login works or that the account has the necessary role. Verify access to a protected target after login. Keep the incident's Cognito/`own` claim as application-specific context, not a universal Hero requirement.

### 3.2 File format

Proposed syntax to ratify during Research:

```dotenv
HERO_TEST_USERS=operator,administrator

HERO_TEST_USER_OPERATOR_LOGIN=
HERO_TEST_USER_OPERATOR_PASSWORD=
HERO_TEST_USER_OPERATOR_PROFILE=operator

HERO_TEST_USER_ADMINISTRATOR_LOGIN=
HERO_TEST_USER_ADMINISTRATOR_PASSWORD=
HERO_TEST_USER_ADMINISTRATOR_PROFILE=admin
```

This example contains no real credentials. It illustrates an explicit user index and multiple entries; names are not yet an approved schema. Define identifier normalization, collision handling, duplicate rejection, quoting/escaping, empty versus missing values, and parser diagnostics. Passwords containing spaces, `#`, quotes, backslashes, dollar signs, or newlines must round-trip according to the chosen format. Parse data directly: never source the file as shell code or interpolate it into shell commands.

Do not impose a fixed product maximum of two or three users. The UI must handle a scrollable collection and stable identifiers. Multiple users may share a profile; each planned test selects a particular user or an unambiguous profile assignment.

Provide a committed placeholder template through a filename approved in Research. Do not commit `.env.hero`; the current repository rule permits only `.env.example` among committed `.env` files, so a new example filename requires an explicit policy amendment. Never merge actual credentials into the application `.env`.

### 3.3 Secure read/write and execution

- `.env.hero` is the common source for manual edits and Config. Read the latest file before preparation.
- Restrict file access to the current user (0600 on supported Unix platforms); use atomic replacement and avoid residual plaintext backup/temp files.
- Preserve unrelated/manual entries and comments where practical. Detect concurrent edits; never silently replace a newer password or discard unknown entries.
- Add the root-anchored `/.env.hero` ignore rule idempotently before writing credentials. Preserve existing .gitignore contents.
- Check for an already Git-tracked `.env.hero`. Explain that ignore rules do not untrack files and require the problem to be resolved before normal secret handling. Do not silently rewrite Git history or stage changes.
- Validate manually created files for format, permissions, and ignore/tracking state before use; provide actionable diagnostics with field names, never values.
- A deterministic authentication helper/runner reads only the selected user's credentials and performs login. Agents receive profile identifiers and sanitized preparation results, not secret values in prompts/tool arguments.
- Do not claim that environment variables alone hide secrets from a model. Specify the helper boundary, tool capabilities, redaction, and supported harness permissions concretely.
- Keep credentials out of SQLite, workflow YAML snapshots, session transcripts, event payloads, debug logs, Telegram, shell command lines, and rendered error messages.
- Do not pass test credentials into the frontend development server/build environment or Angular browser bundle. Limit injection to the authentication helper/test process.
- Browser snapshots, traces, screenshots, and videos can contain sensitive data. Define capture suppression/redaction during login and ensure credential fields, tokens, and raw login responses do not become ordinary shareable artifacts.
- Do not implement a general-purpose arbitrary-site credential injector. Bind preparation to the configured test environment/authentication destinations.

### 3.4 Archive lifecycle

Treat these credentials as belonging to the active cycle even though the file is project-root scoped.

- Successful archive removes the exact project-root `.env.hero`; missing file is an idempotent success.
- Never copy credentials into the archived cycle, context files, config snapshot, metrics, findings, or idea archive.
- Keep the ignore rule after deletion to protect subsequent cycles.
- Confirm removal without displaying usernames/passwords. Resume of an archived cycle requires users to configure credentials again.
- Do not delete credentials if archive was refused or prerequisite OpenSpec archive failed before Hero commits its own archive operation.
- Define ordering/recovery for partial archive failures. A cleanup error must be visible and must not be reported as successful credential removal; retry must be safe.
- Refuse unsafe path/symlink targets instead of following them to delete an unrelated file.
- Specify the effect of finish, cancel, upgrade, uninstall, and failed archive during Research. The user's deletion requirement is archive; do not silently expand deletion to other actions.

## 4. Config and checklist UX

Add a Config section named Test users, available for the active cycle, with add/edit/remove operations, stable user identifiers, profile selection/entry, login, and masked password input. Do not display secret values in summaries or reveal them through ordinary copy/export features. Provide Save/Discard/Cancel behavior and explicit feedback for save/parse errors. Distinguish configured from access verified.

Editing while tests are executing must follow existing Config safety rules. Preparation should use a consistent credential snapshot for its attempt. Manual changes must be reloadable and usable on a subsequent explicit retry. Do not automatically retry login endlessly when the file changes.

The `/hero-new` checklist must prioritize this item for browser-enabled frontend cycles:

> Does this application require authentication for the planned tests? Configure every required test user in Config → Test users, or edit the project-root `.env.hero`. Include the access profiles required by Browser UI and E2E. Hero deletes this file when the cycle is archived.

Show not needed, setup pending, or users configured; actual authentication is verified before testing. Explain the file path and both options without exposing its contents. Cursor IDE Runtime must communicate the manual option and the TUI Config option accurately, without assuming that Config exists inside the IDE chat itself.

Before either browser stage, identify missing/unverified access and the affected required screens/journeys. Provide the same sanitized notification through the supported status/chat surfaces. Credential entry remains local, not Telegram/chat.

## 5. Bounded preparation and Playwright capability

Resolve preparation from an explicit test execution contract, with TESTING.md and the document registry as references. Do not repeatedly rediscover commands from the whole repository during QA.

The contract must include application URL/environment, start command if needed, readiness check, browser method, configured executable/tool, version expectations, E2E command where applicable, authentication preparation, required profiles, and evidence output paths. Non-secret configuration keys and precedence are to be specified in Research.

Distinguish:

| Mode | Purpose | Readiness proof |
|---|---|---|
| Playwright CLI/skills or MCP browser tools | Browser UI instrumentation and explicitly planned exploratory E2E | The selected stage session can actually launch and navigate a browser |
| Playwright Test project suite | Repeatable E2E with assertions | Declared dependencies, usable browser, config/suite and exact test command |

A missing playwright.config file does not prove MCP/CLI browser automation is unavailable. An installed global/extraneous npm package does not prove a repeatable project suite exists. Checking readiness must use the stage agent's actual harness, tool permissions, and environment, not another agent's successful session.

Prefer an existing project E2E suite. If none exists, Planning must explicitly choose exploratory validation or assign runner/test setup to Implementation. QA agents must not install frameworks, repair dependencies, create application code, or endlessly search for a runner. Temporary validation artifacts and browser state in memory are allowed; persisted reusable auth state is out of scope.

Keep the existing frontend/use_playwright gates and explicit HTTP mode. Never silently replace browser validation with HTTP requests or bypass guards/mock login and claim authenticated browser coverage.

Preparation must be bounded, with a short configurable ceiling (two minutes is a proposal, not an agreed fixed value), limited attempts, and a structured result. Verify URL/service readiness, tool/browser capability, required data/fixtures, credentials, and permissions. For unsupported MFA/SSO, return an actionable blocked result; a manual-login or OTP mechanism requires additional user-approved scope.

For efficient execution: use targeted locators/assertions and Playwright waiting mechanisms, avoid fixed sleep loops and excessive full-page dumps, capture screenshots when visual evidence is required, record concise per-screen/per-flow results, and use isolated browser contexts so users/profiles do not contaminate each other. Use sequential execution when accounts/fixtures cannot safely support concurrency.

Evaluate Playwright CLI+skills for token-efficient agent navigation; retain MCP where persistent in-memory exploration and introspection are useful. Verify current capabilities/version compatibility before choosing a default. No universal migration from MCP is agreed.

## 6. Coverage and passing gates

Planning must produce a traceable matrix with stable item ID, requirement/acceptance reference, screen or journey, required user/profile, priority/mandatory flag, expected outcome, and evidence requirements. Scope is the current cycle plus relevant regressions, not an automatic full audit of every application feature.

- Browser UI checks rendering, CSS/static assets, console, failed network requests and, when configured, responsive visual comparison for all required matrix screens, including protected screens. It does not replace business journeys.
- Preserve existing Health-before-Visual semantics and default viewports (Health desktop 1280; reference comparisons 1280/768/375). Missing optional reference PNGs remain warnings and must not cause an Implementation loop.
- E2E checks business outcomes with assertions; navigating to a page alone is not journey completion.
- Fresh login is required for each execution. Verify the intended account/profile before protected tests; test login itself explicitly where required by acceptance criteria.
- Reports account for planned, executed, passed, failed, blocked, and explicitly optional/skipped items. Include evidence and reasons; do not present a percentage without a known denominator.
- No passed stage while any mandatory coverage item is unexecuted or blocked. Login-only success cannot substitute for required authenticated coverage.
- Optional skips need a reason and must remain visible. Changes to mandatory scope require user approval and a revised plan, not an agent's unilateral downgrade.

## 7. Blocked contract and scheduler behavior

Extend Browser UI and E2E typed reports with `blocked` and coverage/preparation metadata. Update all projected prompts, decoders, validators, scheduler handoffs, persistence/status views, and IDE orchestration instructions together. Adding the word only to a prompt is insufficient.

Separate application defects from inability to validate:

- Missing/invalid account, unavailable tool/browser, missing runner for selected suite mode, unavailable test service, or missing required fixtures → blocked prerequisite with a specific next action.
- A proven application regression, including broken authentication for a known-valid account → failed finding under the existing ownership/repro contract.
- Expected access denial for a negative authorization test → passed assertion, not an unusable user.
- Unknown cause → report diagnostic uncertainty without asserting a frontend/backend defect.

Blocked must stop auto-advance and automatic implementation repair of environmental problems. Preserve evidence and previous outcomes, list affected item IDs and profiles, and notify the user directly. Example:

> Browser UI blocked: profile operator could not access the required screens. Update `.env.hero` or Config → Test users, then retry. Five screens remain unvalidated.

Define explicit retry semantics and whether pre-dispatch preparation consumes an iteration during Research. Waiting for missing user input must not consume active execution budget. Do not silently restart after editing Config.

For reports containing both proven defects and blocked items, define precedence and handoff so genuine findings are preserved without treating missing access as a code defect or approving incomplete coverage. Respect C15 finding identity, repro policies, and loop ceilings.

## 8. Continuous execution budgets for all stages

The TUI scheduler must enforce a stage budget independently of adapter health, stream traffic, model cooperation, and iteration boundaries. Cover Research, Planning, Implementation, QA, Judge, Browser UI, and QA End-to-End when enabled, including concurrent Implementation agents and nested work owned by the execution.

Specify semantics before implementation:

- Reconcile the existing first-start cumulative stage budget with active execution limits. Preserve a bounded budget across productive waves/retries; do not grant an unlimited new budget for each tool call or stream reconnect.
- Display elapsed/remaining time and clear waiting/blocked/expired states.
- Pause active-budget accounting while awaiting human credentials, permission/question responses, or stage approval. Resume without granting a fresh full budget; define persistence/restart behavior.
- Each browser action/test/preparation has its own bounded timeout within the remaining stage budget. Playwright Test's overall timeout complements Hero's deadline.
- On expiry, the scheduler initiates controlled cancellation of executions belonging to that stage, waits for bounded termination, records elapsed metrics/evidence and timeout reason, and persists an intervention/escalation outcome.
- Protect against late completion: expired or cancelled executions cannot close the stage, mark tasks done, mutate findings acceptance, or start the next stage. Handle completion-vs-timeout races deterministically.
- A disconnected or cancelled harness must not leave the stage indefinitely Running. Reconcile outcomes without fabricating test success. Recovery must respect the stage's remaining budget.
- Free chat is not a workflow stage; do not introduce a workflow timeout into free chat accidentally.

**Inviolable boundary:** `CheckHealth`, watchdog evaluation, and `handleHarnessHealthResult` remain observational. They must never cancel/restart Execute. Deadline enforcement belongs to a separate scheduler execution-budget path and requires an approved PRD/ADR amendment. Adapter transport reconnect remains adapter-owned; user cancellation remains supported.

IDE Runtime cannot assume TUI timers apply to Task executions. Research must specify actual harness/IDE cancellation capabilities and bounded execution strategy; unsupported hard enforcement must be disclosed, never falsely claimed as parity.

## 9. Observability and repeated events

Report current preparation phase, screen/journey ID, selected profile identifier, completion count, remaining budget, and sanitized tool lifecycle. Separate transport activity from useful test progress.

For OpenCode, improve `part type patch` diagnostics with safe identifiers/metadata and counts sufficient to distinguish duplicate events from distinct steps. Never include raw patches, credentials, file contents, or provider payloads simply to diagnose repetition. A patch part is not automatically a patch command or browser action.

Warn when many repetitive events occur without coverage progress. Define thresholds during Research. This is diagnostic only: health/progress warnings cannot become hidden corrective actions. The execution deadline remains effective even while generic activity continues arriving.

Record sanitized termination reasons (completed, user cancelled, timeout, prerequisite blocked, harness error), stage/iteration association and available evidence. Do not blame the model or Playwright without tool/transcript evidence.

## 10. Implementation map and documentation

Inspect these existing boundaries before introducing packages:

- `internal/tui/config_screen.go`: users form, secret editing and safe save/reload.
- `internal/workflowconfig/`: non-secret test configuration and validation.
- `internal/cycle/service.go`, archive lifecycle and tests: exact credential cleanup.
- `internal/install/`, upgrade and doctor: applicable ignore/template/readiness lifecycle.
- `internal/cycle/reports/`: blocked/coverage report contracts.
- `internal/engine/`, `internal/store/`: stage outcomes, budget persistence and events.
- `internal/tui/timers.go`, `stage_progress.go`, `stage_handoff.go`, `conversation.go`: execution budgets, cancellation ownership and late-result guards.
- `internal/harness/`, individual adapters: capabilities/cancellation/transport boundaries.
- `assets/{cursor,opencode,codex,claude}/agents/` and commands: matching browser/E2E/orchestrator/discover/planning instructions and `/hero-new` checklist.
- `assets/templates/`: test configuration/templates and user guidance.

Keep the CLI deterministic; requirement discovery and interpretation remain Runtime work. Do not duplicate all secret handling in each adapter. Define an approved boundary for the authentication helper before implementation. New structure requires an approved ADR and an updated architecture overview.

Research must create/update cycle PRD, UI and ADR, relevant living OpenSpec specs, user help, testing/deployment guidance and the document registry. Amend the existing timeout definition and missing-Playwright failure policy explicitly. Update context files after implementation. This idea does not itself modify canonical architecture.

## 11. Acceptance and verification matrix

| ID | Required acceptance evidence |
|---|---|
| B01 | Checklist after hero-new prioritizes authentication setup, both configuration options, and archive deletion policy |
| B02 | Config supports multiple users, duplicate validation, masked secrets, safe save/discard/reload, and manual file compatibility |
| B03 | Complex credential strings round-trip without shell evaluation or disclosure |
| B04 | Ignore rule is added once; existing content is preserved; tracked/unsafe credential files are diagnosed |
| B05 | File/helper permissions and redaction prevent secrets in YAML, SQLite, transcript, logs, commands and browser artifacts |
| B06 | Archive deletes credentials and never archives them; missing file, failed OpenSpec archive, cleanup failure/retry, unsafe paths and resume are covered |
| B07 | Each browser stage logs in afresh using the required profile; no saved session is imported/exported or reused across executions |
| B08 | Missing/invalid user and insufficient access yield explicit blocked status, affected coverage and a next action without automatic repair loop |
| B09 | Genuine auth regressions and expected denial tests are classified correctly; mixed blocked/failed reports have defined behavior |
| B10 | Login-only validation cannot pass a plan requiring protected screens/journeys; optional missing references remain warnings |
| B11 | MCP/CLI availability is distinguished from suite availability; missing TESTING/runner produces bounded preparation diagnostics |
| B12 | Actual harness capability/permissions are checked; no silent HTTP fallback or QA framework installation occurs |
| B13 | Every enabled TUI stage expires during an active execution even when repetitive events continue |
| B14 | Partial waves, concurrent agents, nested work, permission waits, reconnect/restart and persisted remaining budgets obey the approved timing contract |
| B15 | Expiry/cancel races and late completions cannot mutate stage completion or acceptance; Running is reconciled after termination |
| B16 | Health remains passive; existing transport recovery and user cancellation behavior remain intact |
| B17 | All harness projections and IDE guidance reflect the same supported contracts and disclose enforcement limitations |

Use deterministic fake clocks, fake harness streams and temporary project directories for Hero behavior tests. Browser smoke/integration tests should use a local fixture application with protected pages and multiple roles, not real provider credentials or the user's Angular environment. Verify secret absence in generated artifacts and failure diagnostics with synthetic sentinel values. After code changes, `go test ./...` must pass; complete other checks required by project testing docs. This documentation-only task does not require running the Go suite.

## 12. Research decisions still required

The following details were not finalized in the conversation; resolve them in the future cycle rather than presenting example choices as approved contracts:

1. Exact `.env.hero` schema, placeholder template location, parser/escaping and Config concurrent-edit semantics.
2. Multiple-user profile selection, test-data ownership and safe concurrency rules.
3. Authentication helper ownership and capability/secret isolation for each harness; behavior for unsupported MFA/SSO.
4. Explicit Playwright mode/config keys, default choice, version policy, provisioning ownership and preparation/action/test attempt limits.
5. Blocked persistence/state model, retry/iteration semantics and mixed defect/blocker precedence.
6. Timeout semantics (cumulative active budget versus separate limits), pause accounting, restart/reconnect recovery, and supported IDE enforcement.
7. Archive ordering and cleanup recovery, plus credential policy for finish/cancel/upgrade/uninstall and multi-TUI concurrency.
8. Safe artifact capture/redaction/retention and repetitive-event warning thresholds.

## References

- [Hero PRD](../product/PRD.md), particularly browser roles and §5.4 timeout semantics.
- [TUI stage execution ADR](../architecture/ADR-C08-001-tui-stage-execute.md).
- [Finding handoff PRD](../product/PRD-C15-001-loopback-findings-handoff.md) and [ADR](../architecture/ADR-C15-001-loopback-findings-handoff.md).
- [Harness adapter specification](../../openspec/specs/harness-adapter/spec.md), especially passive health and question/permission handling.
- [Playwright authentication](https://playwright.dev/docs/auth): context isolation and sensitive authentication state; persistent-state reuse is deliberately excluded here.
- [Playwright MCP](https://github.com/microsoft/playwright-mcp) and [CLI](https://github.com/microsoft/playwright-cli): execution alternatives; verify current capabilities during Research.
- [Playwright timeouts](https://playwright.dev/docs/test-timeouts): action/test and total-run limits.
