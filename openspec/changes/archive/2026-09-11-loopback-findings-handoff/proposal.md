## Why

C13/C14 validation loop-backs could reopen Implementation after OpenSpec `tasks.md` was already fully checked. The scheduler then had an empty assignment while agents reused old `task-*` IDs; ADR-075 correctly rejected those IDs, but the user only saw a generic gate message. Prose gap files were not scheduler input, agents still tried to call loop-back, and Escalated loops had no controlled way to defer selected blockers into durable project ToDos (PRD-C15-001 §1–2).

Cycle C15 replaces that fragile handoff with scheduler-owned findings, atomic failed-stage close, mixed `task-*`/`find-*` assignments, exact diagnostics, Escalated `/hero-add-todo`, Research adoption, and pending-only `/hero-complete-todo`.

## What Changes

- Add schema-v11 findings, occurrences, structured ToDos, adoption history, cycle disposition, and recoverable projection-op records in `hero.db` (PRD-C15-001 §5, §9, §11; ADR-083, ADR-087, ADR-088).
- Make failed validation close, finding persistence, and loop-back one SQLite transaction via `hero stage close --name <source> --failed --findings-json` (PRD-C15-001 §7.1; ADR-084).
- Extend ADR-075 assignments to the ordered union of unchecked owned `task-*` and open/reopened owned `find-*`; apply results only after exact union, ownership, and gate checks (PRD-C15-001 §6.5, §7.2–7.3; ADR-085).
- Fail closed with field-specific diagnostic codes before any mutation (PRD-C15-001 §6, §7.3; ADR-086).
- Add `/hero-add-todo` (Escalated only) and `/hero-complete-todo` (pending only), with idempotent `current-state.md` projection (PRD-C15-001 §8–9; UI-C15-001 §§7,10–11).
- Adopt pending ToDos at Research startup; resolve them only after a validating cycle completion; release them on cancel/emergency finish (PRD-C15-001 §9.3–9.4; ADR-089).
- Surface findings, loop-backs, deferred ToDos, and disposition on existing Status/Events/JSON/Telegram surfaces without a new navbar item (PRD-C15-001 §10; UI-C15-001 §§3–6,12–14; ADR-090).
- Update canonical agent/command assets for Cursor, OpenCode, Codex, and Claude so validators emit JSON and stop (PRD-C15-001 §6.1, §10.4).

## Capabilities

### New Capabilities

- `findings-lifecycle`: stable `find-*` identity, fingerprinting, status transitions, occurrences, and atomic failed-stage handoff.
- `durable-todos`: structured ToDos, adoption/release/resolve, projection reconciliation, Research adoption, and manual pending completion.
- `structured-stage-reports`: typed QA/Judge/Browser UI/E2E/Implementation JSON contracts and field-specific diagnostics.
- `telegram-project-control`: Telegram help, forwarding, and compact status for the C15 commands and additive JSON.

### Modified Capabilities

- `sqlite-operational-store`: forward-only schema v11 tables, constraints, and indexes without rewriting existing rows.
- `runtime-workflow-execution`: transaction-aware loop-back, mixed assignments, Escalated triage, deferred-work disposition, and Research adoption hooks.
- `cli-deterministic-command-suite`: `--findings-json`, `hero add-todo`, `hero complete-todo`, and JSON-safe structured errors.
- `hero-tui`: Status/Events/Chat/palette/research/command dialogs for findings and ToDos (UI-C15-001).
- `asset-bootstrap-and-layout`: canonical four-harness agent/command contracts, workflow-help, and checksum-protected upgrade copies.

## Impact

- Packages: `internal/store`, `internal/engine`, `internal/cycle`, `internal/tui`, `internal/status`, `internal/todos`, `internal/conversation` audit envelope via `cycle.StageAgentAuditBody`, `internal/telegram`, `assets/{cursor,opencode,codex,claude}`.
- Document registry already contains C15 PRD/UI/ADR; implementation updates context files and confirms TESTING/DEPLOY coverage.
- No new navbar screen; no Windows; no live harness accounts; no agent-written gap files as operational truth.
- Scope is native → all implementation tasks owned by `generic_agent`. Apply `go-engineering` and `golang-tui`. Finish with `go test ./...` green.
