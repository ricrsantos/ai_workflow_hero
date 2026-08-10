# ADR-C03-001 — Cursor Harness Autonomy & TUI Orchestration

> Cycle C3 ADRs. Index: [ADR.md](ADR.md). Product: [PRD-C03-001](../product/PRD-C03-001-cursor-harness-tui-autonomy.md).

| # | Title | Status |
|---|---|---|
| [ADR-024](#adr-024-hero-slash-vocabulary-uses-hyphen-not-colon) | Hero slash vocabulary uses hyphen, not colon | Accepted |
| [ADR-025](#adr-025-full-harnessadapter-contract-in-v1) | Full HarnessAdapter contract in V1 | Accepted |
| [ADR-026](#adr-026-tui-orchestrates-via-harness-not-chat) | TUI orchestrates via harness, not chat | Accepted |
| [ADR-027](#adr-027-tui-harness-selection-at-boot) | TUI harness selection at boot | Accepted |
| [ADR-028](#adr-028-hero-cycles-and-hero-todos-runtime-commands) | `hero-cycles` and `hero-todos` Runtime commands | Accepted |
| [ADR-029](#adr-029-hero-sync-scans-product-and-architecture-pending-items) | `hero-sync` scans product and architecture pending items | Accepted |
| [ADR-030](#adr-030-harness-default-model-in-herojson-tui-freechat-without-cycle) | Harness default model in hero.json; TUI freechat without cycle | Accepted |

**Amends:** [ADR-015](ADR-C01-001-hero-1-0.md#adr-015-dual-entry-ui-chat-and-tui-parity) (TUI gains autonomous harness path), [ADR-016](ADR-C01-001-hero-1-0.md#adr-016-harness-adapter-interface-cursor-only-impl) (full interface + CLI execution), [ADR-020](ADR-C02-001-slash-parity-harness-archive.md#adr-020-user-facing-vocabulary-is-hero-slash-commands) (hyphen vocabulary), [ADR-026](#adr-026-tui-orchestrates-via-harness-not-chat) (freechat without active etapa).

---

## ADR-024: Hero slash vocabulary uses hyphen, not colon

**Context**: ADR-020 (C2) standardized `/hero:*` (colon) in user-facing copy. Cursor command files are named `hero-start.md`, which surfaces in the IDE as `/hero-start` (hyphen). C2 TUI labels used `/hero:approve`, diverging from actual chat behavior.

**Decision**: Canonical user-facing Hero slash commands use **hyphen**: `/hero-start`, `/hero-sync`, `/hero-approve`, etc. Runtime asset headers, TUI labels, README, and workflow-help must use hyphen form. CLI verbs (`hero approve`) unchanged.

**Consequences**:
- ADR-020 is amended; C2 docs referencing `/hero:*` as canonical are superseded for display vocabulary.
- Asset tests should ban colon-form in user-facing TUI/palette strings (except historical changelog references).

---

## ADR-025: Full HarnessAdapter contract in V1

**Context**: C1 shipped `HarnessAdapter` with only `Dispatch`; `Pusher` was nil, so execution always fell back to chat. Design docs `06_harness_adapter.md` / `07_cursor_adapter.md` specify `IsAvailable`, session management, `Execute`, `Cancel`, `Status`, and JSON/stream parsing via Cursor Agent CLI.

**Decision**: Expand `internal/harness` to the full contract. Cursor adapter implements CLI discovery (`cursor-agent` / `cursor agent`), `cursor agent login` validation, `--print`, `--output-format json|stream-json`, `--resume`. Workflow Engine and TUI call the interface only — never `exec` Cursor CLI outside `internal/adapters/cursor`.

**Consequences**:
- `Dispatch` becomes a thin wrapper or alias over `Execute` until call sites migrate.
- Tests use injectable process runners; no live LLM in unit tests.
- ADR-016 “chat path remains reliable baseline” still holds for dual-entry, but TUI/`hero run` must succeed when CLI is available.

---

## ADR-026: TUI orchestrates via harness, not chat

**Context**: Users expect Hero TUI to run the full cycle without an open Cursor chat. Interactive Research (grill-me) requires multiple **interações** (conversational rounds) within one **etapa**.

**Decision**:
1. Go owns etapa state machine (SQLite, `cycle.Service`).
2. Each harness **interação** (user message or agent prompt) invokes `HarnessAdapter.Execute` / `ResumeSession`.
3. TUI renders streaming output (`stream-json`) in a conversation UI.
4. Runtime agent markdown (`discover_agent.md`, etc.) remains the prompt source; Go does not perform LLM reasoning (ADR-003).

**Consequences**:
- New TUI conversation surface (screen or split panel).
- Session IDs stored per etapa or per cycle in SQLite (SDD defines schema).
- Dual-entry: Cursor chat orchestration unchanged for users who prefer chat.

---

## ADR-027: TUI harness selection at boot

**Context**: TUI must not depend on `hero doctor`. Projects may lack `cli.tools` or need harness confirmation.

**Decision**: On TUI launch when no harness is configured in `hero.json` → `cli.tools`:
1. Auto-detect harness markers.
2. Present supported harnesses (V1: **cursor** only).
3. On user selection, run `IsAvailable`; on failure show error + remediation (e.g. `cursor agent login`); **exit TUI**.
4. Persist choice to `cli.tools`.

**Consequences**:
- `hero install --tools cursor` pre-populates `cli.tools`; boot prompt skipped when already set.
- Doctor may duplicate checks for diagnostics but is not on the TUI critical path.

---

## ADR-028: `hero-cycles` and `hero-todos` Runtime commands

**Context**: Users need cycle history with metrics and a backlog view without parsing SQLite or folders manually.

**Decision**:
- Add Runtime assets `hero-cycles.md` and `hero-todos.md` (and TUI palette entries `/hero-cycles`, `/hero-todos`).
- **`hero-cycles`**: aggregate SQLite cycles + metrics; include archive directories under `.workflow-hero/cycles/archive/` (parse legacy `metrics.md` when no DB row).
- **`hero-todos`**: read-only display of `context/current-state.md` pending sections; always notify that `/hero-sync` may be needed after doc changes.
- No new deterministic CLI verbs required (orchestrator may use `hero metrics`, `hero status`, store queries internally).

**Consequences**:
- `documents.json` and AGENTS.md doc map updated at implementation.
- Formatting rules for cycle list output in UI spec.

---

## ADR-029: `hero-sync` scans product and architecture pending items

**Context**: `/hero-todos` reads only `current-state.md`. Pending work is also documented in cycle PRDs and ADRs under `docs/product/` and `docs/architecture/`.

**Decision**: Extend `/hero-sync` orchestration: after context_agent codebase scan, analyze product/architecture index and cycle documents for explicit pending/deferred/out-of-scope-for-later items; merge into `context/current-state.md` (SDD defines sections and dedup rules). `/hero-todos` does not trigger sync automatically.

**Consequences**:
- `hero-sync` prompt asset updated; context_agent or orchestrator sub-step reads doc pointers.
- Users run `/hero-sync` then `/hero-todos` when docs change.

---

## ADR-030: Harness default model in hero.json; TUI freechat without cycle

**Context**: TUI Chat previously required an active etapa. Users also need ad-hoc harness conversations (and a CLI `--model`) when no cycle is open. Per-agent models in `workflow-config.yml` only exist inside a cycle.

**Decision**:
1. Persist per-harness defaults under `.workflow-hero/config/hero.json` → `harnesses.<tool>` with `model` and `enable_fast_model` (Cursor V1 default: `composer-2.5`, `enable_fast_model: false`).
2. Resolve a kebab CLI slug (same rules as ADR-005: fast → `<id>-fast`) and pass it as Cursor Agent CLI `--model` on `Execute`.
3. TUI Chat MAY submit interações with no active etapa (**freechat**): `StageName` empty, session id kept in TUI memory only (resume within the TUI process; not written to SQLite stages).
4. With an active etapa, Chat still binds to the etapa session in SQLite; TUI Execute still uses the harness default model (cycle agent models remain Runtime/Task only).

**Consequences**:
- `hero install` / `hero upgrade` write or merge `harnesses.cursor` defaults.
- Editing the model is via `hero.json` (no TUI editor in V1).
- Freechat sessions do not survive TUI restart.

---

## Amendment notes

- **ADR-003**: CLI still never reasons; harness executes AI. Orchestration prompts are assembled by Runtime/TUI glue in Go from markdown templates + state — not open-ended LLM in Go.
- **ADR-015**: Dual-entry retained; TUI gains full autonomous path.
- **ADR-020**: Superseded for hyphen vs colon in user-facing strings.
