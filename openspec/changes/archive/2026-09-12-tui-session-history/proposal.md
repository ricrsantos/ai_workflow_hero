## Why

Hero keeps Free Chat transcripts and session IDs only for the TUI process lifetime. Orchestration and stage rows store harness-native IDs, but there is no user-facing inventory, no complete transcript restoration, and no rename/archive/delete lifecycle. A TUI restart therefore breaks the expectation that a conversation can be found and continued (PRD-C16-001 §1–2).

## What Changes

- Add schema-v12 `sessions`, `session_events`, `session_assets`, `session_leases`, and recoverable `session_delete_ops` in `hero.db` without rewriting existing v11 rows or the cycle-audit `conversation` table (PRD-C16-001 §3.3, §4 FR-01–FR-02, FR-07, FR-10, FR-12; ADR-091–092, ADR-094, ADR-096, ADR-098).
- Create one durable Hero session on the first accepted Free Chat, orchestration, Research, or named stage-agent turn, including a distinct row per parallel stage agent (PRD-C16-001 §3.1–3.2; FR-01, FR-13).
- Persist every Chat-visible event incrementally with deterministic order and Telegram origin; restore the full locally available transcript after restart (PRD-C16-001 §3.3; FR-02).
- Add a `History` navbar screen after `Chat` (available without an active cycle) with active/archived views, name search, rename, archive/restore, and confirmed permanent delete (PRD-C16-001 §3.4, §3.7; UI-C16-001 §§1–7; FR-03, FR-04, FR-10).
- Resume the original harness/model/property snapshot under an exclusive database lease; offer an explicit context fork when exact native resume is impossible; never mutate completed stage lifecycle by opening a conversation (PRD-C16-001 §3.5; FR-05–FR-09; ADR-094–095).
- Keep managed C14 asset copies until explicit session deletion; never unlink original user files; amend age-only 7-day cleanup for registered durable sessions (PRD-C16-001 §3.3, §3.7; ADR-096).
- Migrate legacy orchestration/stage bindings idempotently without inventing transcript content; remote history import only after confirmation through an optional adapter capability (PRD-C16-001 §3.6; FR-11–FR-12; ADR-097–098).
- Keep all persistence/filesystem/adapter I/O out of Bubble Tea `Update`/`View` (PRD-C16-001 FR-14; ADR-093).

## Capabilities

### New Capabilities

- `durable-session-history`: Hero session identity, create-on-first-turn, titles, ordered events, list/search/rename/archive/delete, paging, and legacy labeling.
- `session-continuation`: exclusive leases, exact native resume, interrupted recovery, historical-continuation mode, and explicit context fork.

### Modified Capabilities

- `sqlite-operational-store`: forward-only schema v12 tables, constraints, and indexes without rewriting existing rows or dropping the audit `conversation` table.
- `hero-tui`: History screen, navbar order/shortcuts, Chat restore, `/new-chat`, dialogs, and reconstruction of durable asset cards.
- `harness-adapter`: optional narrow remote-history and native-deletion capabilities; adapters expose only truthful support.
- `session-asset-store`: durable sessions retain managed copies until delete; startup cleanup removes only orphans and non-durable legacy temp sessions.
- `runtime-workflow-execution`: resuming a completed stage conversation MUST NOT reopen the stage or mutate scheduler state.
- `telegram-project-control`: Telegram-originated turns share the Hero session and persist origin on restored events.

## Impact

- Packages: `internal/store`, `internal/conversation`, `internal/tui`, `internal/harness`, `internal/adapters/{cursor,opencode,codex,claude}`, `internal/media`, `internal/telegram`, `internal/cycle` (compatibility projections only).
- Document registry already contains C16 PRD/UI/ADR; implementation updates context files and confirms TESTING/DEPLOY/architecture-overview coverage.
- No global cross-project history; no import of Cursor IDE / other harness UI catalogs; no full-text search; no LLM titles; no encryption; no Windows.
- Scope is native → all implementation tasks owned by `generic_agent`. Apply `go-engineering` and `golang-tui`. Finish with `go test ./...` green.
