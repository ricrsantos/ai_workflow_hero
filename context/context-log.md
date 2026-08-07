# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

---

## 2026-08-07 — Release 0.9.0 (nested subagent model config)

**Problem:** Ship minor release for nested `agents.<name>.subagent` model configuration.

**Decision:** SemVer minor bump `0.8.0` → `0.9.0` (user-facing Runtime config/behavior change).

**Outcome:** Version default in `cmd/hero/main.go` set to `0.9.0`; tag/push/`release.sh`/GitHub Release follow on this commit.

---

## 2026-08-07 — Nested subagent model configuration

**Problem:** Agents hesitated to fan out nested Tasks because children reused the parent’s (often expensive) model; users need a cheaper per-agent nested model.

**Decision:** Nest `subagent` under each `agents.<name>` in `workflow-config.yml` (`same_of_agent` + model fields). Orchestrator→agent uses top-level; nested generic Task uses `agents.<name>.subagent`; named Hero agents (e.g. `context_agent`) keep their own block. Defaults: `same_of_agent: false`, `composer-2.5`. Missing/`same_of_agent: true` → parent model (backward compatible).

**Outcome:** Template, Model Resolution (orchestration + impl/planning agents), ADR-005/PRD/help/hero-new, and asset tests updated; `go test ./...` green.

---

## 2026-08-04 — Publish GitHub Release v0.8.0

**Problem:** Tag `v0.8.0` and local `dist/` existed, but the GitHub Release had never been created (DEPLOY §4.2 steps 4–5).

**Investigation:** `go test ./...` green; tag already on origin at `24f3f18`; rebuilt artifacts via `./scripts/release.sh` on the tagged commit.

**Outcome:** Published https://github.com/ricrsantos/ai_workflow_hero/releases/tag/v0.8.0 with 4 platform binaries + `checksums.txt` and release notes. Intermediate tags `v0.6.0`–`v0.7.0` remain unpublished on GitHub.

---

## 2026-07-29 — Tag and build release v0.8.0

**Problem:** Cut minor release for chat language preference + `fallback_model` reorder.

**Outcome:** Tagged `v0.8.0` on `24f3f18`, pushed to origin; `./scripts/release.sh` produced `dist/hero_v0.8.0_{linux,darwin}_{amd64,arm64}` + `checksums.txt`. GitHub Release later published 2026-08-04.

---

## 2026-07-29 — Release 0.8.0 (chat language + fallback_model reorder)

**Problem:** Ship minor release for `workflow_config.user_preferred_language` and `fallback_model` section reorder.

**Decision:** SemVer minor bump `0.7.0` → `0.8.0` (user-facing Runtime config/behavior change).

**Outcome:** Version default in `cmd/hero/main.go` set to `0.8.0`; tag/push/`release.sh` follow on this commit.

---

## 2026-07-29 — workflow_config.user_preferred_language + fallback_model reorder

**Problem:** Users need a default chat language for agents; `fallback_model` was placed awkwardly before `scope`.

**Decision:** Add `workflow_config.user_preferred_language` (default `EN`) before `scope`; move `fallback_model` after `agents` / before `workflow_rules`. Agents must chat in the preferred language unless the user asks otherwise; cycle artifacts stay English. Import `workflow_config` on `/hero:new` with the other durable sections.

**Outcome:** Template, Runtime agents/commands/skill, PRD/ADR/help/README/design notes, and contract tests updated.

---

## 2026-07-29 — Tag and build release v0.7.0

**Problem:** Cut minor release for `/hero:new` rename + previous-cycle config import.

**Outcome:** Tagged `v0.7.0` on `ade35c0`, pushed to origin; `./scripts/release.sh` produced `dist/hero_v0.7.0_{linux,darwin}_{amd64,arm64}` + `checksums.txt`. GitHub Release upload still pending (DEPLOY §4.2 step 4).

---

## 2026-07-29 — Release 0.7.0 (`/hero:new` + previous-cycle import)

**Problem:** Ship minor release for Runtime command rename and previous-cycle config import fix.

**Decision:** SemVer minor bump `0.6.1` → `0.7.0` (user-facing Runtime behavior change).

**Outcome:** Version default in `cmd/hero/main.go` set to `0.7.0`; tag/push/`release.sh` follow on this commit.

---

## 2026-07-29 — Fix previous-cycle workflow-config import on `/hero:new`

**Problem:** `/hero:new` only copied the blank template into `workflow-config.yml`, so models/stages from the previous cycle were lost.

**Decision:** Mandatory **Previous Cycle Config Import**: deep-merge previous `workflow_config` + `fallback_model` + `stages` + `agents` onto the installed template; always reset `title` / `objective` / `scope`. Documented in PRD §5.5, ADR workflow_rules, workflow-help, design decision #39 (no ask — always import).

**Outcome:** Updated `hero-new.md`, orchestration/skill/help assets, docs, contract test; `go test ./...` green. Consumers need `hero upgrade` to pick up the fixed command.

---

## 2026-07-29 — Rename `/hero:init` → `/hero:new`

**Problem:** Runtime command for starting a new cycle should be named `/hero:new` (asset `hero-new.md`) instead of `/hero:init`.

**Decision:** Rename across assets, Go inventory/tests/CLI messages, README, PRD, ADR, workflow-help, design notes, and related prose (“init chat history” → “`/hero:new` chat history”).

**Outcome:** `hero-init.md` → `hero-new.md`; all `/hero:init` / `hero-init` references updated; `go test ./...` green.

---

## 2026-07-28 — Tag and build release v0.6.1

**Problem:** Cut patch release for updated Moonshot/Zhipu model pricing.

**Outcome:** Tagged `v0.6.1` on `96ee544`, pushed to origin; `./scripts/release.sh` produced `dist/hero_v0.6.1_{linux,darwin}_{amd64,arm64}` + `checksums.txt`. GitHub Release upload still pending (DEPLOY §4.2 step 4).

---

## 2026-07-28 — Add Kimi K3 / K2.7 Code / GLM 5.2 pricing

**Problem:** Hero `models/*.yml` still had placeholder Moonshot/Zhipu ids (`moonshot-v1-*`, `glm-4*`) instead of current Cursor catalog models.

**Decision:** Replace with Cursor docs rates — `kimi-k2.7-code` ($0.95/$0.19/$4), `kimi-k3` + `kimi-k3-max` ($3/$0.30/$15), `glm-5.2` + `glm-5.2-high` ($1.40/$0.26/$4.40). Effort variants mirror `cursor-grok-4.5-high` pattern for metrics slug lookup.

**Outcome:** `assets/models/moonshot.yml` and `zhipu.yml` updated; CLI default version bumped to `0.6.1`; `go test ./...` green.

---

## 2026-07-28 — Apply Browser UI Validation (complete)

**Problem:** Implement OpenSpec change `browser-ui-validation`.

**Outcome:** Added `browser_ui_agent`, stage order Judge → Browser UI Validation → QA End-to-End, `workflow-config.yml` keys (`browser_ui_validation` + `visual_validation`, defaults off), orchestration/commands/docs/README, ADR/PRD updates, inventory + contract tests, version `0.6.0`. `go test ./...` green. Ready to archive.

---

## 2026-07-28 — Propose Browser UI Validation (OpenSpec)

**Problem:** Frontend cycles missed CSS/asset failures; need a Playwright-based Browser UI Validation stage before E2E.

**Decision (grilling):** Flow `QA → Judge → Browser UI Validation → QA End-to-End`; Health always-on when stage enabled; Visual optional (agent vision + PNGs); no URL/screens.yml config; failure routing front/back; artifacts under `.workflow-hero/cycles/current/browser-ui/`; OpenSpec change + bump `0.6.0`.

**Outcome:** Change proposed then applied (see entry above).

---

## 2026-07-28 — Confirm V1 OpenSpec archive; start post-V1 grilling

**Problem:** User asked to archive `v1-ai-workflow-hero` before grilling post-V1 priorities; `current-state` still listed archive as pending.

**Investigation:** `openspec list` empty; change already at `openspec/changes/archive/2026-07-21-v1-ai-workflow-hero/`.

**Decision:** No move needed. Update `current-state` / this log; proceed to grill post-V1 priorities (PT-BR).

**Outcome:** Context files corrected; grilling led to Browser UI Validation change.

---

## 2026-07-28 — Bump version to 0.5.2

**Problem:** Ship Runtime fixes under a new patch version.

**Decision:** Increment SemVer patch `0.5.1` → `0.5.2` in `cmd/hero/main.go`.

**Outcome:** Version bumped to `0.5.2`; later superseded by `0.6.0` for Browser UI Validation.

---

## 2026-07-28 — Runtime UX fixes (links, archive date, kebab models, clean session)

**Problem:** Init/metrics file links, archive folder dates, Task model brackets, and start-in-same-chat context waste.

**Decision:** Clickable markdown links; archive uses `workflow.md` Completed; kebab Task Model Resolution + `cursor-grok-4.5` pricing; Clean Session Handoff after init.

**Outcome:** Runtime + ADR + tests updated under `0.5.2` line.

---
