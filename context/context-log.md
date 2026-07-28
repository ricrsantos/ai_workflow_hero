# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

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
