## Why

Real frontend cycles showed CSS/asset load failures that Hero's existing QA and QA End-to-End stages missed. Users need an explicit browser-instrumented validation stage (render, console, network/CSS health, optional visual comparison) before business E2E journeys run.

## What Changes

- Add Runtime stage **Browser UI Validation** with agent `browser_ui_agent`, inserted in the canonical flow as: Configuration → Research → Planning → Implementation → QA → Judge → **Browser UI Validation** → QA End-to-End.
- Stage always runs **Browser Health** when enabled (Playwright required): open app, render check, console errors, failed network requests (CSS/JS/images/fonts/APIs), CSS load verification.
- Optional **Visual Validation** (config toggle): agent vision judgment against user PNGs under `docs/ui/visual_reference` (default); missing PNG → warn and continue; no `screens.yml` / no extra user config beyond `workflow-config.yml`.
- Extend `workflow-config.yml` template with `stages.browser_ui_validation` (standard stage fields + `visual_validation`) and `agents.browser_ui_agent`.
- Update orchestration/commands/skills/docs (README + `workflow-help.md`), metrics/workflow templates, ADR/PRD stage lists, and Runtime contract tests.
- Bump CLI default version to **0.6.0**.

### In Scope

- Cursor Runtime assets + templates + docs + contract tests (ADR-003 Runtime).
- Config gates: `enabled: true` requires `scope.frontend: true` (validated at `/hero:start`); Playwright usability checked at stage execution.
- Failure routing: Health failure skips Visual; loops to `frontend_agent` (or `backend_agent` when failure is clearly an API); Visual failure loops to `frontend_agent`.
- Cycle artifacts under `.workflow-hero/cycles/current/browser-ui/`.
- Fixed viewports in agent prompts: 1280 / 768 / 375.

### Out of Scope

- Pixel-diff / deterministic screenshot tooling.
- `base_url`, `start_command`, or `screens.yml` in config (agent discovers from project artifacts, same spirit as E2E).
- Changing QA End-to-End Playwright journey semantics (`use_playwright` remains for business flows).
- New IDE harnesses or other V2 stages (PRD §2.3).

### CLI vs Runtime Classification (ADR-003)

- **Runtime (primary):** new agent, stage orchestration, config semantics, docs for end users.
- **CLI (narrow):** version bump to `0.6.0`; install/upgrade continue to materialize updated embedded assets with no new command.
- No new `internal/<feature>/` package required beyond existing asset embed + Runtime semantic tests.

## Capabilities

### New Capabilities

- None (behavior extends existing Runtime/bootstrap capabilities).

### Modified Capabilities

- `runtime-workflow-execution`: Insert Browser UI Validation into stage order; define Health/Visual semantics, Playwright dependency, approval/iteration loops, failure routing, and config gates.
- `asset-bootstrap-and-layout`: Ship `browser_ui_agent` asset, updated `workflow-config.yml` / metrics / workflow templates, bilingual help/README, and contract assertions for the new stage/agent.

## Impact

- Runtime files under `assets/cursor/` (agents, commands, skills, orchestration).
- Templates: `assets/templates/workflow-config.yml`, `workflow.md`, `metrics.md`, `AGENTS.md`.
- Docs: `assets/docs/workflow-help.md`, root `README.md`, `docs/product/PRD.md`, `docs/architecture/ADR.md`.
- Tests: `internal/common/runtime_assets_test.go` (and related inventory/semantic tests).
- Version: `cmd/hero/main.go` → `0.6.0`.
- Context compression files updated when implementation lands.
