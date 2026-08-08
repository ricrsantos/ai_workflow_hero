# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

---

## 2026-08-08 — README updated for C2 TUI docs

**Problem:** README lacked “no active cycle” flow and C2 TUI/archive details after slash-parity cycle.

**Outcome:** EN+PT-BR README: dual-entry lede, C2 feature bullets, TUI no-cycle → `/hero:new`, slash palette + imported commands, finish/archive wording, internal/ layout packages.

---

## 2026-08-08 — Cycle C2 archived

**Outcome:** `/hero:archive` ran `openspec archive slash-parity-tui-harness -y` (specs merged), then Hero archive to `.workflow-hero/cycles/archive/C2-2026-08-08-corre-o-dos-comandos-do-hero-ap-s-cria-o/`. OpenSpec folder: `openspec/changes/archive/2026-08-08-slash-parity-tui-harness`.

---

## 2026-08-08 — Cycle C2 finished

**Outcome:** Research→Planning→Implementation→QA→Judge complete for `slash-parity-tui-harness`. `hero finish` recorded `completed_at` 2026-08-08. Totals ~160354 tokens / ~$0.8482 (project grand ~541k / ~$3.02). Next: `/hero:archive`.

---

## 2026-08-07 — Cycle C2 Implementation: slash-parity-tui-harness

**Problem:** Deliver OpenSpec change `slash-parity-tui-harness` (native): slash-first UX, TUI command import, harness marker warnings, OpenSpec-coupled archive.

**Investigation:** Nested generic fan-out (2A–2E) landed most slices; parent fixed cycle `resolveArchiveCycle` pointer bug and closed §3 (tests, context, `hero cycle openspec-change`).

**Decision / Outcome:** Schema v2 `openspec_change`; `hero cycle openspec-change` + archive `--force`/`--skip-openspec`/`--openspec-change`; Runtime slash-first assets; doctor/install harness warn-only; Cursor `DiscoverCommands` + Dispatch prompt fallback; TUI `/hero:*` palette + imported commands. `go test ./...` green. Ready for QA/Judge.

---

## 2026-08-07 — Cycle C2 Research: slash parity + harness commands + archive

**Problem:** After 1.0, user-facing copy and TUI labels drifted from 0.9 `/hero:*` names; users want TUI to run other Cursor commands, harness detection suggestions, and OpenSpec archive coupled to `/hero:archive`.

**Decision:** Slash-first vocabulary (ADR-020); non-Hero commands via `.md` expansion from `.cursor/commands` + `~/.cursor/commands` (ADR-021); detect markers vs `cli.tools` on install/sync/doctor, warn-only for unsupported (ADR-022); `openspec archive -y` before Hero archive with `--force` escape (ADR-023).

**Outcome:** Docs PRD/ADR/UI C02 registered; Research closed (~35k/8.5k tokens, ~$0.23). Next: Planning SDD.

---

## 2026-08-07 — Default `hero` TUI + auto SQLite ensure

**Problem:** Users should not run a separate command to create `hero.db`; `hero` with no args should open the TUI; uninstalled projects must be blocked with install guidance.

**Decision:** `EnsureOperationalStore` on OpenService (create/migrate + one-shot legacy import); doctor auto-creates DB; root command `RunE` → TUI (`hero tui` remains alias); `ErrNotInstalled` with install suggestion.

**Outcome:** README EN/PT-BR **Hero TUI** section; `go test ./...` green.

---

## 2026-08-07 — Cycle C1 completed (Hero 1.0)

**Outcome:** Research→Planning→Implementation→QA→Judge→E2E complete. OpenSpec `hero-1-0` delivered (SQLite, AI Loop, CLI-as-API, TUI, Cursor adapter, upgrade path). Grand total ~381k tokens / ~$2.17. Completed date 2026-08-07. Next: `/opsx:archive`, `/hero:archive`, optional `v1.0.0` release.

---

## 2026-08-07 — Judge fix: engine timeout + discover_agent metrics

**Problem:** `timeout_minutes` stored but never compared to elapsed wall-clock; `discover_agent.md` still told orchestrator to fill `metrics.md`.

**Decision:** Check timeout between iterations in `StartStage`/`EscalateIfExhausted` (preserve first `StartedAt`; clear on `Continue`); update discover_agent Metrics to CLI/`--metrics-json`/SQLite; ban fill-metrics.md wording in asset test.

**Outcome:** Colocated engine timeout tests; assets + `.cursor` discover_agent synced; `go test ./...` green.

---

## 2026-08-07 — QA fix: workflow-config CLI persistence rule

**Problem:** QA found `workflow_rules` still said `Update workflow.md after completing each stage`; asset test did not cover `templates/workflow-config.yml`.

**Decision:** Replace with hero CLI / `--metrics-json` SQLite guidance; extend `TestRuntimeAssets_CLIAPIStageClose` to ban backtick and plain `Update workflow.md` / `Update metrics.md`.

**Outcome:** Template + current-cycle config updated; `go test ./...` green.

---

## 2026-08-07 — Hero 1.0 implementation (OpenSpec hero-1-0)

**Problem:** Ship Hero 1.0 harness orchestrator: Go AI Loop, SQLite ops store, CLI-as-API, Cursor adapter, Bubble Tea TUI, Runtime CLI persistence.

**Decision:** Vertical slices `store`/`engine`/`cycle`/`harness`/`tui`; pure-Go SQLite; Runtime slash commands shell out to `hero`; TUI calls services in-process; deferred D1–D13 excluded; stage close via `hero stage start|close` + `approve`/`finish` with `--metrics-json`.

**Outcome:** All 34 tasks in `openspec/changes/hero-1-0/tasks.md` complete; `go test ./...` green; CLI default `1.0.0`. Ready for QA/Judge.

---

## 2026-08-07 — Hero 1.0 TUI (OpenSpec §8)

**Problem:** Implement Bubble Tea TUI with parity to CLI-as-API for cycle control and inspection.

**Decision:** `internal/tui` Bubble Tea app calling `cycle.Service` in-process; refuse when `NO_COLOR` or stdout not TTY (`clierr`); lipgloss colors per UI.md.

**Outcome:** Tasks 8.1–8.5 complete — screens (status, approvals, artifacts, costs, events), command palette, dispatch via `Service.Run`, `Service.Artifacts()` helper, unit tests.

---

## 2026-08-07 — Cycle C1 Implementation completed (hero-1-0)

**Outcome:** 34/34 OpenSpec tasks via generic_agent; SQLite store, AI Loop engine, CLI-as-API, Cursor harness, TUI, Runtime CLI persistence; version `1.0.0`; `go test ./...` green. Advance to QA.

---

## 2026-08-07 — Cycle C1 Planning completed (OpenSpec hero-1-0)

**Outcome:** SDD at `openspec/changes/hero-1-0/` (proposal, design, 8 specs, 34 tasks). Deferred D1–D13 excluded. Auto-advance to Implementation (`generic_agent`).

---

## 2026-08-07 — Cycle C1 Research completed (Hero 1.0 specs)

**Outcome:** PRD/ADR/UI C01-001 written; DEPLOY §3.1 upgrade notes; `docs/idea/v1` → `docs/idea/archive/v1`; deferred D1–D13 in research-checkpoint.

---

## 2026-08-07 — Cycle C1 Research paused (Hero 1.0 grilling)

**Problem:** Start C1 for Hero 1.0 (TUI + harness orchestrator + AI Loop); refine `docs/idea/v1/` against current Hero.

**Decisions so far:** Dual UI (chat + TUI) on one core; SQLite = Hero ops store; project `context/*.md` stay first-class (no project kidnap); Cursor-only + `HarnessAdapter`; Go owns state machine, harness executes stages (hybrid C); 1.0 package = core + usable TUI (integrations/multi-harness/rich notifications out).

**Outcome:** Research paused after Q6. Checkpoint: `.workflow-hero/cycles/current/research-checkpoint.md`. Resume grilling at Pergunta 7 (chat↔Go bridge, etc.). Docs not generated yet.

---

## 2026-08-07 — Tag, build, and publish release v0.9.0

**Problem:** Cut and publish minor release for nested subagent model config.

**Outcome:** Tagged `v0.9.0` on `cc4a8b5`, pushed to origin; `./scripts/release.sh` produced `dist/hero_v0.9.0_{linux,darwin}_{amd64,arm64}` + `checksums.txt`; published https://github.com/ricrsantos/ai_workflow_hero/releases/tag/v0.9.0 with 4 binaries + checksums.

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
