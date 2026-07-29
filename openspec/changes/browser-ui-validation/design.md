## Context

Hero V1 Runtime validates frontend work via `qa_agent` (tests/lint/build/logging) and optional Playwright journeys in `end2end_qa_agent`. Neither reliably catches CSS/asset load failures or visual regressions against user references. Grilling (2026-07-28) locked a new stage **Browser UI Validation** between Judge and QA End-to-End, driven by `browser_ui_agent`, with Browser Health always-on when the stage is enabled and Visual Validation optional.

This change is primarily **Runtime** (ADR-003): new agent prompt, orchestration/commands/skills, `workflow-config.yml` schema (ADR Appendix), docs. CLI impact is asset embed + version `0.6.0`. Extends ADR-005 (Task isolation + model resolution), ADR-008 (fallback), ADR-011 (one-file-per-agent).

## Goals / Non-Goals

**Goals:**

- Insert stage order: QA → Judge → Browser UI Validation → QA End-to-End.
- Ship `browser_ui_agent` with Health (Playwright) + optional Visual (agent vision vs PNGs).
- Config only via `workflow-config.yml` (standard stage fields + `visual_validation`); no URL/startup/screens manifests.
- Document usage in `workflow-help.md` and bilingual README; bump to `0.6.0`.
- Contract tests assert inventory, stage string, and key config/orchestration rules.

**Non-Goals:**

- Pixel-diff tooling; `screens.yml`; `base_url` / `start_command` fields.
- Merging Browser UI Validation into E2E or removing `use_playwright` journeys.
- Auto-installing Playwright into consumer projects.
- New IDE adapters.

## Decisions

### D1 — Stage placement after Judge
**Choice:** `… → QA → Judge → Browser UI Validation → QA End-to-End`.  
**Why:** Judge remains SDD coverage; browser health is runtime UI quality; E2E stays business journeys.  
**Alt:** Between QA and Judge — rejected (mixes SDD check with browser checks).

### D2 — Responsibility split vs E2E Playwright
**Choice:** Browser UI Validation = health + optional visual; E2E `use_playwright` = user journeys only.  
**Why:** Avoid collapsing two concerns; CSS miss can fail before journey scripts.  
**Alt:** Force E2E to HTTP-only when Browser UI Validation is on — rejected (journeys still need browser).

### D3 — Config schema (ADR Appendix `workflow-config.yml`)
**Choice:**

```yaml
stages:
  browser_ui_validation:
    enabled: false
    purpose: …
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: true
    visual_validation:
      enabled: false
      reference_dir: docs/ui/visual_reference
agents:
  browser_ui_agent:
    model: …
    reasoning_effort: …
    enable_fast_model: …
    thinking: …
```

Health has no toggle. Defaults: stage off, visual off.  
**Alt:** User-configured `base_url`/`start_command` — rejected (E2E/frontend already discover from project).

### D4 — Discovery of app URL and screens
**Choice:** Agent discovers how to open the app from project artifacts (TESTING.md, package scripts, current-state, implementation docs) like E2E. Screen candidates from cycle docs/routes; match `<screen-id>.png` under `reference_dir`.  
**Why:** Zero config beyond YAML. Missing PNG → warn + continue; empty/missing dir → one warning, skip Visual block.

### D5 — Visual method
**Choice:** Agent vision judgment; Playwright only navigates/captures. Viewports fixed in agent prompt: 1280 / 768 / 375. Health uses desktop only.  
**Alt:** Pixel-diff — deferred.

### D6 — Artifacts
**Choice:** `.workflow-hero/cycles/current/browser-ui/` with `health-report.md`, `screenshots/`, optional `visual-report.md`. Never overwrite user refs. Archive with the cycle.

### D7 — Failure routing
**Choice:** Health fail → skip Visual → loop `frontend_agent`, or `backend_agent` if classified as API failure. Visual fail → `frontend_agent`. Missing PNG ≠ fail. Consumes stage iterations like QA.

### D8 — Gates
**Choice:** `/hero:start` blocks `browser_ui_validation.enabled: true` without `scope.frontend: true`. Playwright absence detected at execution → Health failure → frontend loop.  
**Alt:** Playwright probe at start — rejected (false positives / env noise).

### D9 — Version
**Choice:** SemVer bump `0.5.2` → `0.6.0` (new stage/agent feature).

## Risks / Trade-offs

- [Agent vision flaky vs pixel-diff] → Mitigation: structured reports, human approval default `true`, warn-not-fail on missing refs.
- [Playwright not installed in consumer project] → Mitigation: clear Health failure + frontend fix loop; document prerequisite in help/README.
- [Overlap with E2E Playwright cost/time] → Mitigation: stage default off; Health is cheaper than full journeys; users enable when frontend in scope.
- [API vs asset misclassification] → Mitigation: require explicit failure class in agent JSON report; orchestrator routes on that field.
- [Upgrade overwrites customized workflow-config] → Mitigation: existing checksum non-overwrite; template changes apply on fresh install / non-customized upgrade paths only.

## Migration Plan

1. Implement Runtime assets + templates + docs + tests; bump version.
2. Consumers: `hero upgrade` picks up new agent/docs; existing cycles keep old config until next `/hero:new` or manual YAML merge — document new keys in help.
3. Rollback: revert release / reinstall previous Hero version; stage simply absent if assets old.

## Open Questions

- None from grilling; implementation may refine agent output JSON field names while preserving required semantics.
