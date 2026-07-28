# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

---

## 2026-07-28 — Bump version to 0.5.2

**Problem:** Ship Runtime fixes (clean session handoff, kebab Task models, archive Completed date, clickable file links) under a new patch version.

**Decision:** Increment SemVer patch `0.5.1` → `0.5.2` in `cmd/hero/main.go`.

**Outcome:** Version bumped; commit and push requested.

---

## 2026-07-28 — Clickable links for config review and metrics

**Problem:** During init, users had to hunt for `workflow-config.yml`; metrics summaries in chat did not point to the details file.

**Decision:** Runtime prompts must emit Cursor-clickable markdown links: `[path](path)` for `.workflow-hero/cycles/current/workflow-config.yml` on init review, and for `.workflow-hero/cycles/current/metrics.md` (plus `metrics-summary.md` on finish) after every metrics summary.

**Outcome:** Updated hero-init, orchestration Metrics Procedure/output, hero-approve, hero-finish, workflow-hero skill; contract assertions in runtime asset tests.

---

## 2026-07-28 — Archive folder date = cycle Completed

**Problem:** `/hero:archive` produced wrong/future dates in folder names (e.g. `C1-2026-07-28-…` when the cycle finished on the 26th). `hero-archive.md` only said `YYYY-MM-DD` with no source; agents invented “today” from chat/model context.

**Decision:** (1) `workflow.md` header gains **Status**, **Started**, **Completed**. (2) `/hero:init` sets Started via `date +%Y-%m-%d`. (3) `/hero:finish` writes Completed the same way. (4) `/hero:archive` MUST use Completed from workflow.md as the folder date; shell `date` only as fallback for mid-progress or legacy missing Completed. Never hallucinate dates.

**Outcome:** Runtime + ADR + tests updated; `go test ./...` green.

---

## 2026-07-28 — cursor-grok-4.5 pricing + kebab Task Model Resolution

**Problem:** Workflow feedback: “O slug com brackets não é aceito nesta versão do Task — usando `cursor-grok-4.5-high` (equivale a effort=high).” Hero Model Resolution built `id[fast=…,effort=…]` (ADR-005), but Cursor Task only accepts kebab allow-list slugs. Also missing Cursor Grok pricing entry.

**Decision:** (1) Add `cursor-grok-4.5` and `cursor-grok-4.5-high` to `assets/models/cursor.yml` with the same rates as `xai.yml` → `grok-4.5`. (2) Change Model Resolution to kebab suffixes (`-fast`, `-<effort>`, `-thinking`); forbid brackets. (3) Metrics lookup strips known suffixes when needed. (4) Template/ADR default backend model → `cursor-grok-4.5`. Update ADR-005.

**Outcome:** Runtime + ADR + tests updated; `go test ./...` green.

---

## 2026-07-28 — Clean Session Handoff after configuration

**Problem:** Running `/hero:start` in the same chat as `/hero:init` wastes the orchestrator context window on configuration grilling/Q&A before Research → Implementation.

**Decision:** Soft UX guidance only (Cursor cannot hard-gate a new chat). After `/hero:init`, instruct the user to open a new empty chat, select the IDE agent/model to use as Hero orchestrator / grill-me, then run `/hero:start`. `/hero:start` bootstraps only from disk files. Documented in `hero-init.md`, `hero-start.md`, `orchestration_agent.md`, workflow-hero skill, `workflow-help.md`, README, and a Runtime asset contract test.

**Outcome:** Runtime assets + docs updated; `go test ./...` green.

---

## 2026-07-25 — Release script fix, bump to 0.5.1

**Problem:** `scripts/release.sh` fell back to `dev` when no git tag existed; tag `v0.5.0` was created before the script fix was committed.

**Decision:** Harden `scripts/release.sh` (exact tag on HEAD, strip `v` for ldflags, artifact names use full tag). Bump CLI default `0.5.0` → `0.5.1`.

**Outcome:** Changes committed; tagged `v0.5.1`, `./scripts/release.sh` built artifacts, tag pushed, GitHub Release published at https://github.com/ricrsantos/ai_workflow_hero/releases/tag/v0.5.1. Follow-up: `release.sh` now clears `dist/` before build and runs `chmod +x` on each binary; added `scripts/build_dev.sh` for untagged dev builds (`<latest-tag>_<short-commit>`).

---

## 2026-07-25 — First release tag v0.5.0 + release.sh hardening

**Problem:** `hero version` showed `dev` when the binary was built without a git tag (README/release fallback). No release tags existed yet.

**Decision:** Update `scripts/release.sh` to require an exact tag on the current commit (no `dev` fallback), strip the leading `v` from the tag for `-X main.version=...` (CLI shows `0.5.0`), and name artifacts with the full tag (`hero_v0.5.0_<os>_<arch>`). Tagged `v0.5.0`, ran `./scripts/release.sh`, pushed tag to origin.

**Outcome:** `dist/hero_v0.5.0_*` binaries + `checksums.txt` built; `hero version 0.5.0` on release binary. Next: GitHub Release upload.

---

## 2026-07-25 — User guide + README capability sweep (v0.5.0)

**Problem:** README under-documented capabilities (architecture via ADR, logging standard, secrets hygiene, Playwright, parallelism). No installed end-user guide; install success did not point users to documentation.

**Decision:** (1) Extend README Features/Recursos + Quick Start + Documentation rows without changing section structure. (2) Add bilingual `assets/docs/workflow-help.md`, install/upgrade copy to `.workflow-hero/docs/workflow-help.md`, doctor check for the file. (3) After `hero install`, print `→ Full user guide: .workflow-hero/docs/workflow-help.md`. (4) Mention guide from `/hero:help` and workflow-hero skill. Updated UI.md install ceremony example. Bumped CLI default version `0.4.0` → `0.5.0`.

**Outcome:** `go test ./...` green.

---

## 2026-07-25 — Runtime logging standard for implementation + QA

**Problem:** Consumer projects had no Hero Runtime guidance requiring application logs during Implementation, and QA did not check for logging.

**Decision:** Add a **Logging** contract to `backend_agent`, `frontend_agent`, and `generic_agent`: levels `error` / `info` / `debug` only; default level `info`; reuse project logger when present; never log secrets. `qa_agent` must verify logging on new/changed paths and report failures in the structured QA output (`logging` field). Contract covered by `TestRuntimeAssets_LoggingStandard`.

**Outcome:** Runtime agent assets updated; `go test ./...` green. (This is Runtime guidance for *consumer* code — not a Hero CLI `logger` package.)

---

## 2026-07-24 — Soft secrets hygiene (env example + doctor warns)

**Problem:** Hero had no guidance or checks to keep secrets out of git in consumer projects.

**Decision:** Soft enforcement only (no commit hooks): (1) templates `env.example` + `gitignore-secrets`; (2) `install`/`upgrade` call `envhygiene.EnsureProjectRoot`; (3) `hero doctor` warn-only checks; (4) AGENTS.md + Runtime agents instruct never commit secrets. CLI default version `0.4.0`.

**Outcome:** Package `internal/common/envhygiene`; tests green.

---

## 2026-07-24 — `use_playwright` + `fallback_model` rename

**Problem:** Playwright e2e lacked explicit opt-in; `generic_model` was confusing and incomplete vs per-agent options.

**Decision:** `stages.qa_end_to_end.use_playwright` (default `false`, requires `scope.frontend`). Renamed `generic_model` → `fallback_model` block; upgrade migrates legacy configs.

**Outcome:** `go test ./...` green.

---

## 2026-07-24 — AGENTS.md consumer template aligned with Hero repo standard

**Problem:** `/hero:sync` AGENTS.md lacked context compression, Reference Lookup Order, Ambiguity, and Testing sections.

**Decision:** Expanded `assets/templates/AGENTS.md`; `context_agent` / `hero-sync` use it as structural base.

**Outcome:** `TestAssets_AgentTemplateSections` added; tests green.

---

_Older grilling / V1 implementation session entries consolidated into `context/current-state.md` and archived OpenSpec change `v1-ai-workflow-hero`._
