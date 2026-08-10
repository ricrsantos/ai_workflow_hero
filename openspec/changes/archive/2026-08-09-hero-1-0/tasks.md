# Tasks — hero-1-0

Scope: **native** → all implementation via `generic_agent`.  
Nested generic Task fan-out (when used): model `composer-2.5` (`agents.planning_agent.subagent` / `agents.generic_agent.subagent`).  
Do **not** implement Deferred D1–D13.

**Parallelism legend:** groups marked **PARALLEL** may run as concurrent Task subagents once their dependencies are met. Groups marked **SERIES** must complete in order.

---

## 1. Dependencies and package skeleton — SERIES

- [x] 1.1 Add pure-Go SQLite dependency (`modernc.org/sqlite` or equivalent) and Bubble Tea (+ lipgloss if needed) to `go.mod`; keep huh; run `go mod tidy`
- [x] 1.2 Create package skeletons with README-level package comments only as needed: `internal/store`, `internal/engine`, `internal/cycle`, `internal/harness`, `internal/tui`; wire empty Cobra registration hooks in `cmd/hero` without behavior yet
- [x] 1.3 Bump CLI default version in `cmd/hero/main.go` to `1.0.0`

## 2. SQLite store — SERIES (blocks engine/CLI)

- [x] 2.1 Implement `internal/store` open/migrate for `.workflow-hero/hero.db` with `schema_migrations` and v1 tables (`cycles`, `stages`, `events`, `metrics`, `artifacts`, `conversation`) per `design.md` D2 (ADR-013; spec `sqlite-operational-store`)
- [x] 2.2 Implement repository methods for cycle/stage CRUD, append-only events, metrics upsert, artifact metadata; colocated `store_test.go` using `t.TempDir()` real DB (ADR-009)
- [x] 2.3 Implement one-shot legacy importer: read existing `workflow.md` / `metrics.md` if present → populate store (used by upgrade); test with fixture markdown

## 3. AI Loop engine — SERIES (after §2)

- [x] 3.1 Implement deterministic state machine in `internal/engine` (stage order from config snapshot, Waiting/Running/PendingApproval/Completed/Escalated/Failed, approve/reject/cancel/finish/continue, iteration/timeout) per ADR-012; no LLM calls
- [x] 3.2 Implement cycle lock / busy detection with clear error surfaces; colocated `engine_test.go` table-driven transitions against temp store
- [x] 3.3 Add engine helpers to import `workflow-config.yml` into a cycle config snapshot on `cycle new`

## 4. CLI-as-API commands — SERIES after engine; **PARALLEL** within after 4.1 shared wiring

- [x] 4.1 Wire shared service façade used by Cobra commands (project root discovery, open store, call engine) in `internal/cycle` (or split read vs mutate packages if cleaner)
- [x] 4.2 **PARALLEL with 4.3–4.5:** Implement read commands `hero metrics` and `hero events` (table + `--json`); extend `hero status` to read SQLite (UI-C01-001 §4); colocated tests
- [x] 4.3 **PARALLEL with 4.2, 4.4–4.5:** Implement mutate commands `hero approve|reject|cancel|finish|continue` with non-interactive flags / `--metrics-json` as designed; colocated tests
- [x] 4.4 **PARALLEL with 4.2–4.3, 4.5:** Implement `hero cycle new|archive|resume` (archive date from store `completed_at`); colocated tests
- [x] 4.5 **PARALLEL with 4.2–4.4:** Register commands in `cmd/hero`, update help text; ensure error format matches UI.md §5

## 5. Harness adapter — **PARALLEL with §6** after engine API stable (after §3)

- [x] 5.1 Define `HarnessAdapter` + `DispatchRequest`/`DispatchResult` in `internal/harness` (ADR-016)
- [x] 5.2 Implement Cursor adapter (chat support + best-effort `Dispatch`); integrate with existing `internal/adapters/cursor` paths; unit tests for interface satisfaction and fallback behavior
- [x] 5.3 Implement `hero run` command calling Cursor dispatch; record `harness_invoked` event; test fallback path when dispatch unavailable

## 6. Install / upgrade / doctor — **PARALLEL with §5** after store (§2); finish before Runtime (§7) preferred

- [x] 6.1 Extend `internal/install` to create/migrate `hero.db` on fresh install; update install tests
- [x] 6.2 Extend `internal/upgrade` for 1.0: create/migrate DB, one-shot legacy cycle import, keep checksum-safe asset refresh (DEPLOY §3.1); upgrade tests including 0.9-like fixture
- [x] 6.3 Extend `internal/doctor` to verify DB presence/openability; update doctor tests
- [x] 6.4 Confirm `uninstall` removes `.workflow-hero/` (including DB) while preserving project artifacts; adjust tests if needed

## 7. Runtime asset updates — SERIES after CLI verbs exist (§4); **PARALLEL** file batches inside

- [x] 7.1 Update `orchestration_agent.md` and `workflow-hero/SKILL.md` stage-close sequence to CLI API persistence; remove mandates to write `workflow.md` / `metrics.md` (PRD-C01-001 §5.4)
- [x] 7.2 **PARALLEL batch:** Update `/hero:*` commands (`hero-status`, `hero-approve`, `hero-reject`, `hero-cancel`, `hero-finish`, `hero-continue`, `hero-new`, `hero-archive`, `hero-resume`, and others that touch cycle markdown) to invoke `hero` CLI
- [x] 7.3 Update templates/docs that describe cycle markdown as canonical ops (`assets/templates/*`, `assets/docs/workflow-help.md`, README as needed); stop shipping operational reliance on workflow/metrics markdown
- [x] 7.4 Extend `internal/common/runtime_assets_test.go` (and related) to assert CLI-API stage-close semantics and absence of required `workflow.md`/`metrics.md` write instructions

## 8. Bubble Tea TUI — SERIES after §4 services; can **PARALLEL** screen work after 8.1

- [x] 8.1 Scaffold `hero tui` + Bubble Tea app shell (TTY check, NO_COLOR/non-TTY refusal) in `internal/tui`
- [x] 8.2 **PARALLEL:** Implement Status and Approvals screens (drive approve/reject via engine services)
- [x] 8.3 **PARALLEL with 8.2/8.4:** Implement Artifacts, Costs/metrics, Events screens
- [x] 8.4 **PARALLEL with 8.2/8.3:** Implement command palette + keyboard navigation (Claude Code–inspired patterns, not a clone)
- [x] 8.5 Wire optional dispatch action to harness/`hero run` semantics; add focused TUI tests where deterministic (model/update helpers); smoke-test launch refusal paths

## 9. Integration, docs, archive verify — SERIES (closing)

- [x] 9.1 Add/extend integration tests: install → cycle new → status/metrics/events → approve path → upgrade from 0.9-like tree; run `go test ./...` until green
- [x] 9.2 Verify `docs/idea/archive/v1/` present and fix any stale `docs/idea/v1/` product links (ADR-019); docs-only
- [x] 9.3 Update `context/current-state.md` and append `context/context-log.md` for Hero 1.0 implementation outcome when code lands
- [x] 9.4 Final `go test ./...` green check before marking change ready for QA/Judge

---

## Parallel groups (orchestrator fan-out)

After §1–§3 complete:

| Group | Tasks | Agent |
|---|---|---|
| A | 4.2, 4.3, 4.4, 4.5 (after 4.1) | `generic_agent` × N |
| B | 5.1–5.3 ‖ 6.1–6.4 | `generic_agent` × 2 |
| C | 7.2 file batch (after 7.1) | `generic_agent` nested `composer-2.5` |
| D | 8.2 ‖ 8.3 ‖ 8.4 (after 8.1) | `generic_agent` nested `composer-2.5` |

Hard series spine: **1 → 2 → 3 → 4.1 → (4.x ‖ 5 ‖ 6) → 7 → 8 → 9**.
