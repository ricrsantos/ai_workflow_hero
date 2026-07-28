# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

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
