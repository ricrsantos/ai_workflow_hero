# PRD-C17-001 — Browser Validation, Test Access, and Active Execution Budgets

Date: 2026-09-30. Status: Research requirements confirmed by the user; implementation pending. Architecture: ADR-C17-002. UX: UI-C17-001. Input: docs/idea/browser_improvements.md (non-normative).

## 1. Outcome and scope

Hero must not approve a login-only inspection when the plan requires protected application screens, classify missing tools/accounts as code defects, or leave an execution Running beyond its configured budget. The cycle delivers test-user configuration, bounded preparation, traceable authenticated coverage, explicit blocked outcomes, continuous stage budgets, and optional screenshots.

Only execution in Hero TUI is in scope, with Cursor, OpenCode, Codex, and Claude as TUI harnesses. IDE-chat support and removal of IDE-specific assets are excluded from this cycle. Existing ADR-C17-001 remains historical architecture input; it does not authorize additional IDE removal work here. Hero remains Go/Cobra, feature-based vertical slices, on Linux/macOS amd64/arm64. No application code or browser framework is implemented during Research.

The Angular/OpenCode incident is user-reported evidence, not a reproduced defect or proof of a Playwright/model failure. Generic `part type patch` activity is not proof of browser progress or actual file modification.

## 2. Test access and configuration

### FR-01 — Explicit authentication requirements

Research/Planning identify environment, application URL, authentication entry point, permitted authentication destinations, required profiles and permissions, and protected access verification targets. Authentication is `not_needed`, `required`, or `unresolved`; unresolved cannot pass the plan gate. Never infer public access from an empty credential file.

Use login/password form authentication. Unsupported interactive MFA, CAPTCHA, or SSO blocks with a specific next action; no manual OTP/chat collection is added. Validate the selected account/profile against a protected target. Expected denial in a negative authorization test is a passing assertion. A broken login for an independently known-valid account may be an application finding; uncertainty must not be presented as a proven defect.

### FR-02 — Root dotenv file and optional Config editor

The common source is project-root `.env.hero`, separate from application `.env`. A committed root `.env.hero.example` contains placeholders only. The user explicitly approved this exception to the repository's `.env.example`-only policy. Never overwrite an existing application environment file.

The schema uses ordinary dotenv assignments:

```dotenv
HERO_TEST_USERS=operator,administrator
HERO_TEST_USER_OPERATOR_LOGIN=""
HERO_TEST_USER_OPERATOR_PASSWORD=""
HERO_TEST_USER_OPERATOR_PROFILE=operator
HERO_TEST_USER_ADMINISTRATOR_LOGIN=""
HERO_TEST_USER_ADMINISTRATOR_PASSWORD=""
HERO_TEST_USER_ADMINISTRATOR_PROFILE=admin
```

Stable user IDs match `[a-z][a-z0-9_]*`; their uppercase spelling forms the field suffix. Reject duplicate IDs/keys and ambiguous normalization rather than merging them. Profile names describe access roles; multiple users can share a profile. No fixed product maximum is imposed. Planning assigns an exact user ID to each authenticated test.

Parse directly as data, never source the file or expand shell variables/command substitutions. Accept blank lines, comments, unquoted values, literal single-quoted values, and double-quoted values with documented `\n`, `\r`, `\t`, `\"`, and `\\` escapes; dollar signs remain literal. Preserve spaces and `#` inside quoted values. Reject unsupported escapes/malformed quoting with line/field diagnostics without values. For a required account, missing or empty login/password/profile is unusable. Serialization must round-trip spaces, quotes, hashes, backslashes, dollar signs and newlines without evaluation. Preserve comments and unrelated entries where practical.

Config exposes an opt-in Test users editor backed by this file, with scrollable add/edit/remove, stable ID, profile, login, masked password, Save/Discard/Cancel, and reload. The option enables credential setup; it does not waive required authentication. If disabled while a required test needs login, block and instruct the user to enable Test users and configure the required account. Manual file editing remains supported. Distinguish users configured from access verified. Config remains read-only under existing active-execution safety rules.

The persistent opt-in is the top-level `test_access.enabled` boolean in `workflow-config.yml`, default `false` when absent. Config and required-login preparation use the same managed setting; it contains no credentials. This key was confirmed by the user on 2026-09-30.

### FR-03 — Safe local handling

Before writing, add root-anchored `/.env.hero` to `.gitignore` idempotently, preserving existing content. Create/update with mode 0600, atomic replacement and no residual plaintext backups. Validate manual files for parser errors, owner/permissions, Git ignore/tracking and unsafe paths. Already tracked files block use with instructions to remove them from tracking; Hero must not silently stage/untrack them or rewrite history.

Read the latest file before preparation. Detect changes since the Config draft was loaded; refuse stale save and instruct Reload. An attempt uses a consistent credential snapshot; changes affect only the next explicit retry. Never restart login automatically after an edit.

A shared deterministic authentication executor reads only the selected user's credentials and performs the configured login. Agents receive IDs, profile and sanitized outcomes, not credential values in prompts or tool arguments. Bind the login recipe to the planned application/authentication destinations. Do not inject secrets into the frontend server/build, browser bundle, SQLite, workflow YAML/snapshots, conversation logs, metrics, events, Telegram, shell command lines or diagnostics. The runner must establish the same isolated in-memory browser context subsequently used for the test; a separate login in an unrelated browser is not valid preparation. If the selected method cannot provide this boundary, block with configuration instructions.

Saved profiles, storageState import/export, persisted cookies/tokens, cross-stage authentication reuse, OS-vault integration, and credential entry through chat/Telegram are excluded. Each execution authenticates afresh. General-purpose privileged harnesses are trusted software: using a helper does not itself sandbox their filesystem access (ADR-103). No false isolation guarantee is permitted.

### FR-04 — Credential lifecycle

Delete the exact project-root `.env.hero` only on cycle archive, after archive prerequisites succeed; never include it in archives or snapshots. Missing is success. Refuse symlinks/unsafe targets. Failed OpenSpec archive or refused Hero archive must not delete it. Cleanup failure leaves archive pending with an actionable error and safe retry; never report completed archive/removal until cleanup succeeds. Keep the ignore rule. Resume requires reconfiguration.

Finish, cancel, upgrade and uninstall preserve the file. Uninstall informs the user that credentials remain. Use project/cycle ownership checks to reject competing TUI writes/archive operations. Sanitized screenshots, unlike credentials, are retained with archived cycle evidence.

## 3. Preparation and coverage

### FR-05 — Planned execution contract

`planning_agent` creates `.workflow-hero/cycles/current/browser-plan.json` during Planning for consuming applications with browser validation. This shared non-secret artifact contains the login recipe, approved method, and planned coverage alongside the execution contract below. Deterministic preparation validates it before browser execution; Browser UI/E2E agents consume it. Application-specific values remain Planning-owned, and credentials remain in `.env.hero`. The artifact path and producer were confirmed by the user on 2026-09-30.

Planning supplies a non-secret contract referenced through the document registry and TESTING.md: environment/base URL, start/readiness commands, explicit browser method and executable/tool, version compatibility expectations, E2E command where applicable, login recipe, selected users/profiles, fixtures, action/test timeouts, and evidence paths. Do not repeatedly rediscover this contract during QA.

Prefer an existing E2E suite for repeatable E2E. If none exists, Planning explicitly selects exploratory validation with assertions or assigns suite/runner setup to Implementation. Distinguish Playwright Test suite readiness from CLI/skills/MCP browser capability. A missing playwright.config does not disprove CLI/MCP availability; a global package does not establish a project suite. Validate actual launch/navigation in the stage session's harness/permissions/environment. Hero is generic: Planning resolves origins, routes, locators, commands, fixtures, and method dynamically for each consuming application rather than hardcoding application-specific values. For coding-agent browser control, prefer the official Playwright CLI with its official skill; if the harness cannot load skills, use the CLI's documented skills-less mode. Use MCP for specialized loops that need persistent browser state or iterative page introspection, or when verified CLI capability is insufficient. The skill is guidance for the CLI, not a separate browser executor. The minimum Playwright package baseline is 1.63.0, the version available in the reference development environment on 2026-09-30; Planning validates the actual project/harness version and compatibility, and QA does not install or upgrade tools. A missing or below-minimum tool is a prerequisite blocker. QA must not install frameworks, repair dependencies, or create application code. Official selection guidance: https://playwright.dev/docs/getting-started-cli and https://playwright.dev/docs/getting-started-mcp.

Keep current frontend/browser stage guards and `qa_end_to_end.use_playwright`; HTTP mode is explicit and never substitutes for planned browser coverage. Frontend fixtures validate Hero's behavior without enabling these browser stages for Hero's own native-only cycle.

### FR-06 — Bounded preparation

Default preparation ceiling: 120 seconds total per preparation attempt, capped by remaining stage budget. Maximum two attempts per prerequisite within that ceiling. Check service readiness, browser/tool permissions, selected users/protected access and fixtures. Do not retry invalid accounts endlessly or use fixed sleep loops; use bounded locators/assertions and readiness waits. Each action/test has an explicit positive limit bounded by remaining stage time; suite overall timeout complements Hero's scheduler deadline. Timeout values may be configured with validation.

Preparation consumes active time. A prerequisite block before validation starts does not consume a validation iteration. Repeated preflight retries do not reset the stage budget. Once validation has begun, ordinary validation iteration accounting applies. Each explicit `/hero-continue` rechecks prerequisites using current configuration; editing Config alone never resumes.

### FR-07 — Traceable passing gate

Planning provides stable coverage IDs with requirement/acceptance reference, screen or journey, exact user/profile, mandatory/optional flag, expected result and evidence requirements. Scope is the current cycle plus relevant regressions, not every application feature.

Browser UI checks render, CSS/static assets, console, network/API failures for every mandatory screen, including protected screens. Health precedes optional Visual; keep Health desktop width 1280 and reference comparisons 1280/768/375. Missing optional reference PNGs warn without findings or automatic Implementation loops. E2E asserts business outcomes; navigation alone is not completion.

Use isolated contexts per user; parallelize only when accounts/fixtures permit safe concurrency, otherwise run sequentially. Reports account for planned, executed, passed, failed, blocked and optional/skipped items, with known denominators and reasons. No passed stage with a mandatory unexecuted, skipped or blocked item. Mandatory scope changes require explicit user approval and an updated plan.

### FR-08 — Blocked outcomes and clear intervention

Browser UI/E2E typed reports support `passed`, `failed`, `blocked`, validated preparation and coverage metadata. Missing/invalid credentials, insufficient access, disabled required user setup, unavailable browser/tool/service, missing declared suite/fixtures or unsupported interactive login produce prerequisite blockers, not application findings.

Blocked is durable and stops auto-advance/automatic Implementation repair. Every notification includes stage, concrete reason, affected coverage IDs and profile IDs, exact corrective action, and `/hero-continue` for retry. Never say merely “blocked.” Example: “Browser UI blocked: required administrator account is missing. Enable Config → Test users and configure user administrator, or update project-root .env.hero. Screens screen-02 and screen-03 remain unvalidated. Then run /hero-continue.” Credential values never appear.

Mixed reports preserve validated genuine findings under existing C15 identity/repro rules while the stage remains blocked. After explicit unblock, route those findings through existing correction before passing outstanding coverage. Never lose a finding, turn a prerequisite into an owner assignment, or approve incomplete coverage. Optional screenshot delivery failure is a communication warning; failure to produce required planned evidence blocks that coverage item.

## 4. Continuous TUI execution budget

### FR-09 — Active elapsed time across the whole stage

`timeout_minutes` is cumulative active wall time shared by all attempts, partial waves, reconnects and resumptions of that stage. Parallel agents count once by elapsed time, not summed agent durations. Count execution, bounded preparation and scheduler-owned work. Pause when the stage awaits human credentials/questions/permissions/approval and has no work actively progressing; another runnable sibling still consumes wall time. Idle blocked/offline time is excluded. Free chat receives no workflow timeout.

Persist consumed/remaining time and pause/interruption reason at start, pause, resume and termination, and checkpoint active consumption every 5 seconds. Restart/disconnect reconciles the interrupted execution from the last persisted checkpoint, preserves consumption, and requires `/hero-continue`; do not resume automatically, fabricate success or leave it Running indefinitely. No fresh budget on reconnect, command or retry. With zero remaining budget, continuation requires an explicit validated budget increase from the user; the command itself never silently grants time. Display consumed/remaining time and active/waiting/blocked/expired/interrupted state.

### FR-10 — Separate cancellation authority

The TUI scheduler enforces the deadline during active Execute independently of traffic, model cooperation and adapter health. Cover Research, Planning, Implementation (all concurrent agents/nested owned work), QA, Judge, Browser UI, E2E. On expiry: revoke the execution's acceptance authority, initiate scoped controlled cancellation, wait up to 15 seconds for termination, retain partial evidence/metrics, and persist escalation reason `timeout` if work does not terminate in that grace period. No unrelated harness processes may be stopped.

Completion/timeout races are determined by the execution generation and budget state at acceptance. Expired/cancelled/interrupted results cannot close stages, mark tasks/findings accepted or start successors. Existing transport reconnect stays adapter-owned and uses remaining budget. Watchdog, CheckHealth and handleHarnessHealthResult remain observational and MUST NOT cancel/restart Execute. User cancellation stays Ctrl+C, /interrupt and /harness-reset.

### FR-11 — Useful progress

Show preparation phase, current coverage ID, profile ID, completed/required count, remaining time and sanitized tool lifecycle. Generic transport activity is distinct from coverage progress. Warn after 60 seconds without coverage progress or 30 repetitive events without progress; coalesce warnings until progress/phase changes, pause during human waits, and never make warnings corrective actions. OpenCode patch diagnostics use safe event identifiers/counts, not raw patch/file/provider content.

## 5. Screenshots and Telegram

### FR-12 — Optional automatic capture per stage

Browser UI and browser-based E2E each have `screenshots.enabled`, default false, exposed in the respective Config stage section. Enabling captures each tested screen, correlated with coverage/user/attempt, into `.workflow-hero/cycles/current/screenshots/`. Preserve different attempts/roles instead of overwriting. Capture after the screen assertion/check at a stable point; failed-screen capture is useful when safe. The opt-in controls extra communication captures, not mandatory Visual/planned evidence.

Suspend screenshots, traces, video, snapshots and raw login response capture while filling/submitting credentials. Apply sensitive-field/token masks and suppress unsafe capture; do not promise that arbitrary application data can be inferred/redacted automatically. Ordinary test-account application data may be visible. Record an actionable omission reason; mandatory evidence unavailable safely blocks its item.

Use existing text image cards with Enter/o to open the system viewer and save/copy-path actions; a shortcut reaches the screenshot collection while agents run. Do not add terminal inline image protocols. If project Telegram `always_send` is enabled, transmit images as actual images to the paired addressed chat, with safe captions. No global pairing/broadcast expansion or credential entry over Telegram.

### FR-13 — Asynchronous screenshot retrieval

`/hero-screenshot` is a TUI-owned deterministic control, permitted despite the busy composer and available through Telegram. It retrieves generated captures from the active cycle, never invokes a harness, starts a new capture, interrupts a test or waits for the agent to finish:

- No argument: most recent ready screenshot.
- `list`: IDs, screen, profile, stage and attempt; no secrets.
- `<id>`: selected capture.
- `todos`: all ready captures in one request, ordered by creation. `list` and `todos` are reserved selectors.

TUI shows local cards; when `always_send` is true, screenshots also reach Telegram, regardless of request origin. When false, no screenshot forwarding to Telegram is introduced by this command; Telegram text explains the setting and local availability. Selected-project addressing remains mandatory. `todos` uses bounded asynchronous batches, reports progress/partial failures and snapshots the ready set at request time; new captures remain available to subsequent requests. Never truncate silently or require a second command per image. Missing cycle/captures/ID, disconnected Telegram or unreadable image gets a precise explanation. Preserve images with the archived cycle; credential cleanup must not remove them.

## 6. Acceptance and Planning handoff

Acceptance IDs B01–B17 in the idea map to FR-01–FR-11; B18–B21 below cover additions. Planning must retain IDs, concrete verification and canonical owner per task. Apply typed reports, scheduler/storage/config, all four TUI harness prompt projections and help together. Do not stop at prompt wording. Update relevant OpenSpec requirements and install/upgrade templates during Implementation, without IDE removal.

- B01–B04: setup checklist, arbitrary users, dotenv round-trip, atomic/concurrent saves, root ignore/tracked-file diagnostics.
- B05–B07: helper/context boundary, sentinel-secret absence, fresh login, archive safety/retry and preservation on other lifecycle actions.
- B08–B10: actionable block/continue, mixed findings preserved, no login-only pass, optional references remain warnings.
- B11–B12: declared methods, real-session capability, bounded preflight, no QA provisioning or silent HTTP fallback.
- B13–B16: active expiry across all stages/parallel waves; paused/restarted budgets; deterministic race/late-result guards; passive health preserved.
- B17: matching contracts across four TUI harnesses; IDE execution/removal excluded.
- B18: independent screenshots toggles/defaults, per-screen safe captures, stable IDs/cards/viewer and archive retention.
- B19: latest/list/id/todos while streaming, no harness/capture side effects, bounded all-images delivery without omission.
- B20: actual Telegram images only under always_send, selected-chat isolation, resilient partial delivery/status without blocking validation.
- B21: operator instructions show reason, affected coverage, fix location and retry command on TUI/Telegram; secrets never appear.

Use fake clocks/streams, real temp SQLite/files and a local protected fixture with multiple roles, never real credentials or the user's Angular application. Runtime migrations preserve existing state; release gates and planned checks are in TESTING.md and DEPLOY.md. Research did not adopt any ToDos; the authoritative injected list was empty. The pre-document gate was answered “no additions; generate.”

## 7. Technical reference verification

Context7 lookup of official Microsoft Playwright documentation on 2026-09-30 confirms isolated contexts per test and locator-based auto-waiting assertions: https://github.com/microsoft/playwright/blob/main/docs/src/browser-contexts.md and https://github.com/microsoft/playwright/blob/main/docs/src/writing-tests-js.md. Screenshot locator masks are present in the official locator implementation: https://github.com/microsoft/playwright/blob/main/packages/playwright-core/src/client/locator.ts. These capabilities support the chosen isolation/wait/masking contracts; they do not prove a particular installed harness/tool is usable. Planning must verify its actual runner compatibility and authentication-artifact suppression.
