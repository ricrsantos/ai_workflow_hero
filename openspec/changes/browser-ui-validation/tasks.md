## 1. Product docs and ADR schemas

- [x] 1.1 Update PRD stage flow and agent table to include Browser UI Validation / `browser_ui_agent` (order after Judge, before QA End-to-End)
- [x] 1.2 Update ADR Appendix `workflow-config.yml` schema + workflow rules for `browser_ui_validation` / `browser_ui_agent`; note stage-order change in relevant ADR/Runtime sections
- [x] 1.3 Record grilling decisions already captured in `design.md` into `context/context-log.md` when implementation starts (keep `current-state` pending until code lands)

## 2. Templates (series — shared schema)

- [x] 2.1 Extend `assets/templates/workflow-config.yml` with `stages.browser_ui_validation` (defaults off) + `visual_validation` + `agents.browser_ui_agent` + workflow_rules gate for `scope.frontend`
- [x] 2.2 Add Browser UI Validation rows to `assets/templates/workflow.md` and `assets/templates/metrics.md`
- [x] 2.3 Update stage-flow mention in `assets/templates/AGENTS.md`

## 3. Runtime agent and orchestration (can parallelize file edits after 2.1)

- [x] 3.1 Create `assets/cursor/agents/browser_ui_agent.md` (Health + Visual, Playwright, viewports, artifacts path, failure classification, metrics JSON, `model: inherit`)
- [x] 3.2 Update `orchestration_agent.md` for new stage order, gates, dispatch, Health→skip Visual, failure routing front/back
- [x] 3.3 Update Runtime commands/skills that hardcode stage order (`hero-start`, `hero-approve`, `hero-help`, `hero-new` if needed, `workflow-hero/SKILL.md`, other agents' Stage Flow lines)
- [x] 3.4 Keep `end2end_qa_agent` Playwright journey semantics distinct; only refresh Stage Flow string

## 4. End-user documentation

- [x] 4.1 Document Browser UI Validation in `assets/docs/workflow-help.md` (EN + PT-BR): order, Health vs Visual, PNG refs, config keys, Playwright prerequisite, artifacts, failure loops
- [x] 4.2 Document the same in bilingual `README.md`

## 5. Version and contract tests

- [x] 5.1 Bump CLI default version in `cmd/hero/main.go` to `0.6.0`
- [x] 5.2 Extend `internal/common/runtime_assets_test.go` (and related inventory tests) for `browser_ui_agent`, stage-order string, config keys, visual_validation defaults, scope gate rule, artifact path mentions
- [x] 5.3 Run `go test ./...` and fix until green

## 6. Context compression close-out

- [x] 6.1 Update `context/current-state.md` (features, version 0.6.0, pending/next steps) and append implementation outcome to `context/context-log.md`
