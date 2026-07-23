# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

---

## 2026-07-20 — Grilling session: full V1 design definition

**Problem:** `docs/idea/ai_workflow_hero.md` contained a broad initial design for Hero with many open questions (approval loops, model fallback, scope handling, document numbering, CLI vs. Runtime boundaries, testing strategy, release process, terminal UX, etc.).

**Investigation:** Ran an extensive `/grill-me` session covering every open question in the source document, plus three additional recommendations from the user: clean subagent sessions (Task tool, file pointers only), Go testing principles (real dependencies, colocated tests), and Feature Based Architecture for the CLI.

**Decision:** Resolved 70+ individual decisions (see the "Decisões da Sessão de Grilling" sections at the end of `docs/idea/ai_workflow_hero.md` for the full log), covering: approval/control loop semantics (`/hero:continue`, `/hero:back`, `/hero:resume`), implementation orchestration (Task tool, OpenSpec integration), the new `generic_agent` and extended `scope` (native/script/infrastructure), the 3-level model fallback chain, document numbering (`[CATEGORY]-C[XX]-[seq]-[slug].md`), stage-dependency validation, `/hero:sync` promoted to V1, Git as a mandatory prerequisite, concurrency locking, metrics structure, and CLI terminal UX (colors/icons, `--json`, survey prompts, error format).

**Outcome:** All decisions incorporated into `docs/idea/ai_workflow_hero.md`. No code written — this was a pure design/planning session.

**Next step:** Consolidate all decisions into formal documents (`AGENTS.md`, PRD, UI spec, ADRs, DEPLOY.md) so another agent can act on them without re-reading the full grilling transcript.

---

## 2026-07-20 — Documentation creation (AGENTS.md, PRD, UI, ADR, DEPLOY)

**Problem:** Needed formal, cross-referenced project documents capturing all grilling decisions, so any agent (not just the one that ran the session) can understand and act on them.

**Investigation:** Reviewed the full grilling decisions log and mapped each decision to the document(s) it belongs in (product requirement, architecture rationale, UX spec, or deployment process).

**Decision:** Created 5 documents: `AGENTS.md` (permanent prompt), `docs/product/PRD.md`, `docs/product/UI.md`, `docs/architecture/ADR.md` (10 ADRs), `docs/deployment/DEPLOY.md`. Used the `ADR-NNN-title` convention inside a single indexed `ADR.md` file rather than splitting into separate files per ADR, since the set was small enough (10) to stay readable in one document with anchors.

**Outcome:** All 5 documents created and committed (`61924f8`). Found and fixed an inconsistency: `AGENTS.md`/`PRD.md` referenced `ADR-004` for "CLI vs. Runtime separation" but the actual ADR index had this as `ADR-003` (`ADR-004` was "Git as a mandatory prerequisite"). Fixed both references.

**Next step:** Add context compression files (`context/current-state.md`, `context/context-log.md`) and strengthen `AGENTS.md` with explicit ambiguity-questioning policy, documentation map (including context files), reference lookup order, and a concrete test-loop procedure.

---

## 2026-07-20 — Context files + AGENTS.md hardening

**Problem:** `AGENTS.md` was missing: (1) an explicit instruction to question ambiguous/missing information rather than assume, (2) pointers to context compression files, (3) a concrete reference lookup order (project docs → Context7 → web), and (4) a concrete run-tests-until-green loop. The project itself also lacked its own `context/` files.

**Investigation:** The user's requested test loop used `npm test`, but this repository's stack is Go (Cobra CLI), with no Node/npm involved. Asked the user to confirm the correct test command via `AskQuestion` rather than silently substituting one — per the same ambiguity policy being added to `AGENTS.md`.

**Decision:** User confirmed `go test ./...` as the equivalent command. Updated `AGENTS.md` Testing section to use it. Created `context/current-state.md` and `context/context-log.md` for this repository (describing Hero's own codebase state, not an end-user project managed by Hero).

**Outcome:** `AGENTS.md` trimmed and reorganized to stay within the 700-word target while adding all requested sections. Two new context files created.

**Next step:** Extend `docs/architecture/ADR.md`, `docs/product/PRD.md`, and `docs/deployment/DEPLOY.md` with the full data-file schemas (`hero.json`, `project.json`, `documents.json`, `models/*.yml`) and testing-requirement cross-references, per user request to make the documents fully self-sufficient with code snapshots.

---

## 2026-07-20 — OpenSpec proposal created for `v1-ai-workflow-hero`

**Problem:** The project had complete V1 docs but no implementation plan artifacts in OpenSpec for turning the approved scope into executable work.

**Investigation:** Reviewed authoritative sources (`AGENTS.md`, `PRD.md`, `UI.md`, `ADR.md`, `DEPLOY.md`, `context/current-state.md`, `context/context-log.md`) and generated a constraints-first synthesis to avoid introducing requirements outside documented scope.

**Decision:** Created change `v1-ai-workflow-hero` with all planning artifacts required before apply: `proposal.md`, `design.md`, `tasks.md`, and capability specs under `openspec/changes/v1-ai-workflow-hero/specs/`:
- `cli-deterministic-command-suite`
- `runtime-workflow-execution`
- `asset-bootstrap-and-layout`

The proposal explicitly separates CLI vs Runtime responsibilities (ADR-003), keeps V1 in-scope/out-of-scope boundaries from PRD §2.2/§2.3/§7, and defines an ordered implementation checklist with test gates (`go test ./...`).

**Outcome:** Planning is apply-ready for implementation kickoff through `/opsx:apply`.

**Next step:** Execute `/opsx:apply` and begin Task Group 1 (scaffold + command wiring), then progress through the install-first dependency chain.

---

## 2026-07-20 — Apply `v1-ai-workflow-hero` (full V1 CLI + Runtime assets)

**Problem:** Docs and OpenSpec planning were complete, but the repo had no Go implementation.

**Investigation:** Confirmed two gaps with the user before scaffolding: Go module path and interactive prompt library. Used Context7 for Cobra/huh; parallel subagents for repo/spec exploration, asset extraction, and CLI implementation.

**Decision:**
- Module: `github.com/ricrsantos/ai_workflow_hero` (git remote).
- Prompts: `charmbracelet/huh`.
- Layout: ADR-002 vertical slices; assets via `embed.FS` in `assets/`; checksum-protected upgrade; Hero-owned uninstall scope; Runtime semantics encoded in embedded markdown (not in CLI Go code).

**Outcome:** All 42 OpenSpec tasks completed. Packages under `cmd/hero`, `internal/*`, `assets/`, `scripts/release.sh` landed. `go test ./...` green (unit, golden/template, inventory/runtime-asset, integration, release-script contract). No V1 out-of-scope items introduced (no Windows release targets, no CI release automation, no `hero sync` CLI command).

**Refactor / rationale:** Shared `clierr` + `output` enforce UI §2/§5; install-first lifecycle; injectable HTTP for `update-models` tests.

**Next step:** Archive the change (`/opsx:archive`), then cut the first tagged release with `scripts/release.sh`.

---

## 2026-07-20 — V1 Go CLI full implementation

**Problem:** OpenSpec change `v1-ai-workflow-hero` had complete planning artifacts (proposal, design, specs, tasks.md) but no Go source code existed.

**Investigation:** Reviewed PRD, UI spec, ADR, DEPLOY, and all three capability specs. Identified all required packages, commands, asset files, and tests per tasks 1.1–5.7, 4.x, 3.x, 2.x, 7.1–7.4.

**Decision:** Implemented the complete V1 CLI in one session:
- `assets/` package with `embed.FS` for all 13 commands, 10 agents, 2 skills, 6 templates, 7 model pricing files, 1 config file
- `internal/common/` with clierr (UI §5 error format), output (✓/⚠/→/table, NO_COLOR/TTY), template ({{path.key}} only, ADR-006)
- `internal/adapters/cursor/` with all Cursor path constants and ADR-011 inventory lists
- All 7 feature packages: install, upgrade, uninstall, doctor, status, variables, update_models
- `cmd/hero/main.go` with root command and all subcommands registered
- `scripts/release.sh` for cross-compilation (4 targets) + checksums
- Full test suite: unit, integration, asset inventory, runtime-semantics keyword checks

**Outcome:** `go test ./...` — 12 packages pass, 0 failures. All tasks 1.1–5.7 and 7.1–7.4 marked complete. Context files updated.

**Rationale:** Single-pass implementation possible because all architecture decisions were pre-documented (10 ADRs). Feature-based structure kept each package independently testable.

**Next step:** Archive `v1-ai-workflow-hero` with `/opsx:archive`, then tag and release.

---

## 2026-07-20 — Bilingual README

**Problem:** Project needed public-facing documentation in the style of [screenshot_hero README](https://github.com/ricrsantos/screenshot_hero/blob/main/README.md).

**Decision:** Single `README.md` with EN + PT-BR sections (language switcher + anchors), covering install, quick start, Runtime commands, build/test, structure, and docs map — aligned with implemented V1.

**Outcome:** `README.md` created; `context/current-state.md` updated.

**Next step:** Archive OpenSpec change; first tagged release.

---

## 2026-07-21 — Fix metrics accounting (Runtime prompts)

**Problem:** End-user cycle left `metrics.md` Input/Output/Cost as `—`. Agents reported “metrics not computed” because Task returns no API usage and prompts only said “update metrics.md”.

**Investigation:** Confirmed design (PRD §5.10): agent estimates chars÷~4 × `models/*.yml`. CLI never computes cost. Root cause: orchestration stubs had no executable procedure; subagent outputs lacked char estimates; `hero-approve` only mentioned iteration count.

**Decision:** Keep estimation in Runtime (ADR-003). Added Metrics Procedure to `orchestration_agent`, mirrored in `hero-finish`/`hero-approve`, required `metrics.{model,input_chars,output_chars}` on Task agents, stage-close pointers in skill/commands, explicit cost formula in `metrics.md` template Notes. Strengthened `TestRuntimeAssets_Metrics`.

**Outcome:** Runtime assets instruct compute-and-write; tests assert procedure + contracts. Existing installs need `hero upgrade` to pick up assets.

**Follow-up:** Chat line originally showed only tokens+cost. Expanded output format to require Input/Output/Total tokens, Duration, and Cost in chat on every stage close (wall-clock duration recorded by orchestrator).

**Next step:** Archive OpenSpec change; first tagged release; verify metrics fill after upgrade on a short cycle.

---

## 2026-07-21 — workflow-config agents section formatting

**Problem:** `agents` in `workflow-config.yml` used YAML flow-style one-liners (`{ model: ..., ... }`), hard to read/edit and inconsistent with `stages`.

**Decision:** Expand to block style (same pattern as `stages`); sync ADR schema example. No field/semantics change.

**Outcome:** Template + ADR updated. Existing cycle configs remain valid YAML.

**Next step:** Archive OpenSpec change; first tagged release.

---

## 2026-07-21 — Polish `hero install` CLI UX

**Problem:** Interactive install looked ugly (default huh left border, required summary, progress line) vs idea-doc/mock (`🚀 Hero Project Setup`, `> ` prompts, optional summary).

**Decision:** Minimal huh theme (no border), setup header, `Prompt("> ")`, optional summary via `Flags().Changed("summary")`, remove mid-flow progress line; document ceremony in UI.md §4.

**Outcome:** Install UX matches mock; `go test ./...` green.

**Next step:** Archive OpenSpec change; first tagged release.

---

## 2026-07-21 — Encourage Task parallelism in implementation

**Problem:** Design called for parallel/series SDD marking and “use subagents when possible,” but Runtime prompts for planning/orchestration/impl agents did not encode fan-out.

**Decision:** Enrich `planning_agent` (parallel_groups), `orchestration_agent` (Implementation Parallelism), and backend/frontend/generic (nested Task + context_agent). No CLI change.

**Outcome:** Assets encode parallel dispatch and nested fan-out rules; runtime asset tests assert keywords.

**Next step:** Archive OpenSpec change; first tagged release.

---

## 2026-07-21 — Research Pre-document gate

**Problem:** After grill-me, discover_agent jumped straight to document generation with no chance for the user to add last-minute project info.

**Decision:** Mandatory Pre-document gate in `discover_agent` + grilling skill: ask before generating docs; evaluate and incorporate any additions; output fields `pre_document_additions` / `additions_summary`.

**Outcome:** Research flow is grill → summarize → ask → (optional evaluate) → generate docs.

**Next step:** Archive OpenSpec change; first tagged release.

---

## 2026-07-23 — Subagent models from workflow-config.yml

**Problem:** After `hero install` in a consumer Cursor project, subagents ran on the orchestrator session model. Agent UI showed “Inherit from parent”; chat history confirmed the parent model. Root cause: (1) agent `.md` files lacked Cursor YAML frontmatter (default `inherit`); (2) Runtime prompts never required passing Task `model` from `workflow-config.yml`.

**Investigation:** Cursor docs require frontmatter `model` (default inherit). Hero design (ADR-005 / idea §16) says pass model via Task from per-cycle YAML. PRD: switching models = edit YAML only — so frontmatter must stay `inherit`, not bake static model ids.

**Decision:** Add frontmatter (`name`, `description`, `model: inherit`) to all 10 agents. Add mandatory **Model Resolution** procedure in `orchestration_agent` + `hero-start`/`sync`/`back`/`init`: always set Task `model` from `agents.<name>` with `[fast=…]` / `effort=` brackets; nested fan-out reuses that resolved id. Extend ADR-005/008 notes. Runtime asset tests for frontmatter + Model Resolution keywords.

**Outcome:** `go test ./...` green. Consumer projects need `hero upgrade` to pick up assets. Agent editor may still show Inherit; execution must pass Task `model`. Cursor plan limits may still override models (documented debt).

**Next step:** Archive OpenSpec change; first tagged release; validate Task `model` on a real consumer cycle after upgrade.

---

_To be maintained by agents. Prune entries older than the last 3–5 sessions once they no longer inform current work._
