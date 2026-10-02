# Proposal: Browser Validation, Test Access, and Active Execution Budgets

Slug: `browser-validation-execution-budgets`. Date: 2026-09-30.
Status: Implementation in progress; release acceptance remains pending.
PRD: `docs/product/PRD-C17-001-browser-validation-execution-budgets.md` (FR-01–FR-13, B01–B21).
ADR: `docs/architecture/ADR-C17-002-browser-validation-execution-budgets.md` (ADR-103–106).
UX: `docs/product/UI-C17-001-browser-validation-execution-budgets.md` (§§1–6).
User guide: `docs/testing/browser-validation-user-guide.md`. Input (non-normative): `docs/idea/browser_improvements.md`.
Release gates: `docs/testing/TESTING.md` (§C17), `docs/deployment/DEPLOY.md` (§C17).

## Why

A user running Hero against an Angular application reported two inadequate outcomes (PRD-C17-001 §1; user-reported evidence, not a reproduced defect): Browser UI approved the login screen while most functionality stayed behind authentication, and QA E2E stayed Running ~29 minutes despite a 15-minute timeout. Read-only source inspection confirmed the structural causes: Browser Health opens the app without requiring authenticated screens (`assets/opencode/agents/browser_ui_agent.md`); Browser UI/E2E reports accept only passed/failed (`internal/cycle/reports`); missing Playwright is instructed to become a frontend failure; stage budgets are checked between iterations only (`internal/engine/engine.go`); Execute carries no stage-derived deadline (`internal/tui/conversation.go`); and unknown OpenCode part types render as generic activity (`internal/adapters/opencode/events.go`). The existing timeout semantics are `docs/product/PRD.md` §5.4; this cycle explicitly amends them (ADR-105 consequence).

## What Changes

- One shared test-access service owns root dotenv credentials and login preparation: direct dotenv parse/validate/serialize for project-root `.env.hero` with arbitrary indexed users, masked opt-in Config Test users editor with stale-draft rejection, idempotent root `.gitignore`, 0600 atomic writes, and a deterministic executor that privately consumes the selected account, binds a non-secret form-login recipe to approved origins, verifies protected access in the same isolated in-memory context used for tests, and returns only sanitized outcomes (PRD FR-01–FR-04; ADR-103).
- Preparation and coverage become validated contracts with durable blocked outcomes: Browser UI/E2E typed reports gain `blocked`, preparation, blocker, coverage, and safe-evidence metadata; the scheduler persists Blocked/interrupted state, stops auto-advance and automatic repair, preserves mixed C15 findings, and `/hero-continue` rechecks prerequisites before repair/coverage gates (PRD FR-05–FR-08; ADR-104).
- Continuous active execution budgets bound every stage: `timeout_minutes` becomes cumulative active wall time across attempts, waves, reconnects, and resumptions; the TUI scheduler enforces an independent deadline with scoped cancellation and bounded termination; expiry revokes acceptance authority first so late results cannot mutate stages; watchdog/CheckHealth stay passive (PRD FR-09–FR-11; ADR-105).
- Optional per-stage screenshots use existing image cards and asynchronous Telegram delivery: `screenshots.enabled` per Browser UI and browser E2E stage (default false), immutable ready assets under `current/screenshots` with stable IDs and sanitized manifests, login-secret suppression, TUI collection plus `/hero-screenshot` latest/list/id/todos during streaming, and actual Telegram images only under project `always_send` (PRD FR-12–FR-13; ADR-106).
- Credential lifecycle: archive removes only the exact project-root `.env.hero` after archive prerequisites succeed (missing is success; failures leave archive pending with safe retry); finish/cancel/upgrade/uninstall preserve it; uninstall discloses retention; screenshots survive archive (PRD FR-04).
- All four TUI harness projections (Cursor, OpenCode, Codex, Claude) plus help text change together; no IDE-chat execution and no IDE-asset removal are added (ADR-C17-001 stays historical; PRD §1, §6).

## Out of scope

Saved browser profiles, storageState import/export, persisted cookies/tokens, cross-stage auth reuse, OS-vault integration, credential entry via chat/Telegram, interactive MFA/CAPTCHA/SSO support, new session CRUD CLI verbs, terminal inline image protocols, general image input beyond cycle screenshots, a Hero-wide pinned Playwright version, and any IDE removal work.

## Capabilities

### New Capabilities

- `test-access-dotenv`: root `.env.hero` parse/validate/serialize round-trip, arbitrary indexed users, diagnostics without values.
- `stage-execution-budget`: cumulative active timers, checkpoints, pause semantics, generations, independent expiry with scoped cancellation.
- `cycle-screenshots`: per-stage toggles, immutable ready assets with stable IDs, sanitized manifests, safe capture rules.

### Modified Capabilities

- `browser-test-access`: executor boundary, same-context login, fresh-every-execution, blocked preparation, archive cleanup.
- `structured-stage-reports`: `blocked` outcome, preparation/blocker/coverage/evidence metadata, mixed C15 preservation.
- `runtime-workflow-execution`: durable Blocked state, `/hero-continue` recheck, no auto-advance while blocked, PRD §5.4 amendment.
- `sqlite-operational-store`: forward-only v16 budget/blocker/screenshot migration from v15, followed by v17 coverage traceability and approved-plan digest migration from v16.
- `hero-tui`: Test users editor, per-stage screenshot toggles, blocked/budget/progress rendering, passive warnings, screenshot collection, `/hero-screenshot`.
- `telegram-project-control`: image-delivery edge under `always_send`, addressed delivery, bounded batches, mismatch warnings.
- `cli-deterministic-command-suite`: `/hero-screenshot` control semantics, archive cleanup messaging, uninstall retention disclosure.
- `asset-bootstrap-and-layout`: committed root `.env.hero.example` placeholders (approved policy exception), root ignore handling, no credential values on upgrade.
- `harness-adapter`: unchanged passive health contract restated; transport recovery stays adapter-owned within remaining budget.
