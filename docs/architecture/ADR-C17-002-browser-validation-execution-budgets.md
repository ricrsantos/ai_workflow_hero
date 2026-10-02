# ADR-C17-002 — Browser Access, Stage Budgets, and Cycle Screenshots

Date: 2026-09-30. Status: accepted design decisions from Research; implementation pending. PRD: PRD-C17-001-browser-validation-execution-budgets. Existing ADR-C17-001 is preserved. No IDE removal is added in this scope.

## ADR-103: One shared test-access service owns root dotenv credentials and login preparation

Context: Prompt-driven credential reads risk exposing passwords to transcript/tool arguments and cannot prove that a login prepares the same browser used for coverage. Manual and Config editing need one source and conflict handling.

Decision: Introduce a focused `internal/testaccess` vertical slice for direct dotenv parsing, validation, safe write/reload, selected-account reads and sanitized preparation results. Config and deterministic lifecycle operations use this boundary. A deterministic project-specific authentication executor consumes selected credentials privately and a non-secret planned form-login recipe, bound to approved origins. It establishes the isolated in-memory context used by the selected suite/CLI/MCP method. Shared access preparation must not be copied into each adapter. Planning defines the narrow executor/context capability and exact login recipe schema before coding; incompatible methods block rather than passing raw passwords to agents.

The recipe schema is shared, but its application-specific values are produced dynamically during Research/Planning for each consuming project; Hero does not hardcode origins, routes, locators, credentials, or runner commands. For repeatable E2E, prefer an existing Playwright Test suite. For coding-agent browser control, prefer Playwright CLI with the official CLI skill; use the CLI's skills-less mode when the harness cannot load skills. Use MCP for specialized flows requiring persistent browser state or iterative page inspection, or when verified CLI capability is insufficient. The skill supplies CLI guidance and is not a separate executor. The minimum Playwright package baseline is 1.63.0, verified in the reference development environment on 2026-09-30; Planning validates the selected project's actual version and capability, and does not install or upgrade tools during QA. Official guidance: https://playwright.dev/docs/getting-started-cli and https://playwright.dev/docs/getting-started-mcp.

Non-secret managed configuration lives in workflowconfig; test plan describes profile permissions, users, origin and locators/protected target. Secret values exist only in .env.hero and short-lived executor memory. Ordinary environment variables do not constitute a model-isolation boundary. Harnesses with unrestricted filesystem/shell access can technically read local files; they remain trusted and must follow the no-read/no-log credential instructions. Where harness permissions can constrain reads, apply that policy; do not claim 0600 prevents same-user privileged agents from access. Enforce the supported workflow's secret-free prompts, artifacts and diagnostics, with sentinel tests; fail preparation if the chosen integration cannot keep secret values out of agent-visible results.

Consequences: root .env.hero.example is an explicitly approved placeholder exception. Config stale writes fail; parser never evaluates shell code. No auth-state export/reuse. Archive uses exact-path cleanup through this service, not arbitrary recursive deletion. Login recipes/test integrations are Implementation-owned, never created by QA.

## ADR-104: Preparation and coverage are validated contracts; blockers are durable scheduler state

Approved contract clarification (2026-09-30): `planning_agent` writes the shared non-secret recipe, approved method, and coverage contract to `.workflow-hero/cycles/current/browser-plan.json` during Planning for consuming applications with browser validation. Deterministic preparation validates and consumes this artifact before browser execution. Implementation owns the shared schema and integration; application-specific values remain Planning-owned. Config and preparation share the persistent top-level `test_access.enabled` boolean in `workflow-config.yml`, default `false` when absent; disabled required-login preparation blocks with enable instructions. These choices refine ADR-103/104 without moving secret handling or stage-transition ownership.

Context: passed/failed-only reports route absent prerequisites into inappropriate application repair and permit incomplete authenticated coverage to pass.

Decision: Extend `internal/cycle/reports` Browser UI/E2E contracts with blocked outcome, preparation, blockers, coverage and safe evidence references. Each blocker has stable reason, affected coverage/profile IDs, diagnostic uncertainty if relevant, and next action. A planned coverage denominator and mandatory-item accounting are validated before state mutation. Operational stage Blocked/interrupted reason and budget state persist in store/engine, exposed additively through cycle/status and Telegram. Agents report observations; only the scheduler transitions state or accepts findings/tasks.

A mixed validated report transaction preserves genuine C15 findings/occurrences and blocker/coverage data, sets Blocked and schedules no repair until explicit unblock. /hero-continue first rechecks prerequisites; then existing genuine-finding repair and outstanding coverage gates apply. Preflight blocked before validation is iteration-free but spends active time. Preserve finding identity, repro admission, mandatory owner and loop ceilings. Invalid reports continue existing diagnostic/retry behavior within remaining budget, never falsely become prerequisite success.

Consequences: schema migration is additive and transactional; Planning chooses the next free schema version, not a guessed existing version. Update decoders, scheduler handoff, persistence, status/help and all four harness assets together. Environment/tool absence supersedes the former missing-Playwright frontend-failure policy. No general-purpose new QA repair path is introduced.

## ADR-105: A scheduler execution budget owns cancellation independently of health

Context: iteration-boundary checks cannot limit a running Execute; generic activity keeps arriving even without useful work. Parallel agents and human waits make agent-summed durations misleading.

Decision: Engine/store own a durable cumulative active wall-time budget per stage. TUI owns execution generations, independent deadline scheduling and async scoped cancellation/termination. Account elapsed once while any stage-owned work is active; pause only when none is active due to human intervention/approval. Persist consumed time at start/pause/resume/termination and every 5 seconds while active; restart charges the last persisted active checkpoint, marks interrupted and never counts offline time. Recovery precision is bounded by the 5-second checkpoint interval, with fake-clock acceptance tests. Remaining time caps preparation/actions/tests and reconnects; an explicit user budget increase is required when exhausted.

Expiry revokes generation acceptance before cancellation so late completion cannot mutate stages, findings or task checkboxes. Completion and expiry use the same serialized scheduler acceptance boundary, requiring positive remaining time at acceptance. After expiry, scoped cancellation gets a fixed 15-second termination grace, within adapter cancellation capabilities. Nested work must be scoped to its parent; unsupported termination is disclosed and escalated without acceptance of late work.

Consequences: all seven configured stages, partial waves and parallel Implementation use the same timer path. Free chat is excluded. Progress warnings and CheckHealth/watchdog stay passive; transport recovery stays adapter-owned. Budget rows/events contain no credentials. Amend baseline PRD §5.4 explicitly; implementation is not a watchdog modification.

## ADR-106: Cycle-owned screenshots use existing image cards and asynchronous Telegram delivery

Context: Chat is busy during tests; a command requesting a new agent screenshot would be awkward. Existing image cards serve local viewing, but C14 excludes Telegram images and ordinary session assets have a different lifecycle.

Decision: Optional per-stage automatic screen capture writes immutable ready assets under cycle current/screenshots with stable ID and sanitized manifest metadata (cycle, stage, attempt, coverage, user/profile, time, safe relative path, result). Capture/safety belongs to browser execution; retrieval belongs to deterministic TUI control handlers using existing media validation/cards/viewer. Screenshot collection and /hero-screenshot latest/list/id/todos remain usable during streaming without harness dispatch. Request all-images uses a ready-set snapshot and asynchronous bounded batches.

Cycle evidence survives archive and is not owned/deleted by chat-session cleanup. Archive resolves paths safely and retains screenshots without .env.hero. Optional communication captures do not replace mandatory evidence; a safe existing evidence capture can also be registered as a card without duplicate bytes/cards. Never create persisted reusable browser auth state.

Extend the existing addressed Telegram IPC/outbound edge with image-delivery capability, sanitization, path/content validation, bounded transfer and durable sanitized delivery IDs for retry/deduplication. The daemon alone calls Bot API using existing vault credentials. Forward screenshots only under project always_send; sender-origin alone does not bypass that setting. No direct agent-to-Telegram sending. Bot API limits are verified during Planning and handled by ordered batches/files with explicit errors rather than hardcoded assumed limits. Local cards survive disconnected or failed Telegram delivery; optional delivery failure cannot fail application validation.

Consequences: narrowly amends C14's text-only Telegram/output scope for generated cycle screenshots; does not add general image input, approval attachments or terminal preview protocols. Queue jobs carry only validated managed image references and safe captions, not secrets. The screenshot control is read-only and can run while Config/harness turns remain blocked.

## Approved boundary map

```text
workflowconfig + test plan (non-secret) ──→ testaccess ──→ bounded login/browser executor
root .env.hero ── private selected-account read ────────┘             │
                                                                  sanitized results
stage Execute/report ──→ cycle/reports ──→ engine/store ──→ TUI Status/blocked instructions
                           coverage gate       ↑
TUI execution-budget timer ── revoke acceptance + scoped cancel ─────┘
Health/watchdog ── warnings only
browser safe capture ──→ cycle/screenshots ──→ TUI cards/control ──→ Telegram edge (always_send)
```

Planning must provide exact managed keys, report fields, migration/checkpoint/termination constants, login integration capability, and collision-free screenshot shortcut in the SDD. Application-specific execution values are resolved per project at runtime under the shared contract and tool preference above. These are implementation design details constrained by this PRD, not authorization to broaden supported authentication, IDE work or secret exposure.
