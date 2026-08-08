# ADR-C01-001 — Hero 1.0 Architecture Decisions

> Cycle C1 ADRs. Amendments and new decisions for harness orchestration, SQLite state, dual UI, and CLI-as-API. Index: [ADR.md](ADR.md). Idea drafts at `docs/idea/archive/v1/` are non-normative.

| # | Title | Status |
|---|---|---|
| [ADR-012](#adr-012-go-owns-deterministic-ai-loop-state-machine) | Go owns deterministic AI Loop state machine | Accepted |
| [ADR-013](#adr-013-sqlite-as-sole-hero-operational-store) | SQLite as sole Hero operational store | Accepted |
| [ADR-014](#adr-014-cli-as-api-no-daemon-in-10) | CLI as API; no daemon in 1.0 | Accepted |
| [ADR-015](#adr-015-dual-entry-ui-chat-and-tui-parity) | Dual entry UI: chat and TUI parity | Accepted |
| [ADR-016](#adr-016-harness-adapter-interface-cursor-only-impl) | HarnessAdapter interface; Cursor-only impl | Accepted |
| [ADR-017](#adr-017-bubble-tea-tui-claude-code-inspired) | Bubble Tea TUI; Claude Code inspired | Accepted |
| [ADR-018](#adr-018-breaking-major-upgrade-from-09x) | Breaking major upgrade from 0.9.x | Accepted |
| [ADR-019](#adr-019-archive-idea-v1-cycle-docs-canonical) | Archive idea v1; cycle docs canonical | Accepted |

**Amends:** [ADR-003](ADR.md#adr-003-cli-vs-runtime-separation) — clarified below (CLI still never reasons; CLI now owns cycle **state transitions**).

---

## Amendment to ADR-003 (CLI vs Runtime)

**Prior emphasis**: All development-cycle orchestration lived in IDE chat Runtime assets; CLI was admin-only.

**1.0 clarification**:

- **Still true**: the CLI never performs LLM reasoning.
- **New**: the Go binary owns the **deterministic** AI Loop (stage machine, gates, events, metrics persistence) and exposes it via CLI commands. Chat Runtime and TUI are **UIs** that call that API and delegate stage *reasoning* to harness agents.
- Commands that only mutate/query Hero operational state are CLI (deterministic). Commands that require agent reasoning remain harness/Runtime responsibilities (e.g. grilling content, implementing code), orchestrated *through* the state machine.

---

## ADR-012: Go owns deterministic AI Loop state machine

**Context**: Chat-only orchestration made Hero state fragile, hard to share across UIs, and coupled to markdown agents edit by hand.

**Decision**: Implement the AI Loop / Workflow Engine in Go as a deterministic state machine (stages, approvals, iterations, timeouts, locks). Harness agents execute stage work; Go records outcomes and advances when rules allow.

**Consequences**:
- State transitions are unit-testable without LLMs.
- Chat and TUI cannot diverge on “source of truth” for stage status.
- Runtime markdown prompts must be updated to call CLI instead of rewriting cycle markdown.

---

## ADR-013: SQLite as sole Hero operational store

**Context**: Cycle markdown (`workflow.md`, `metrics.md`) burns tokens when agents scan the tree and is a poor sync surface for dual UI.

**Decision**: Persist Hero-exclusive operational data **only** in SQLite under `.workflow-hero/` (exact schema in SDD). Do **not** emit cycle markdown projections in 1.0. Project artifacts (`context/*.md`, `docs/`, `openspec/`, code) remain files; Hero must not kidnap the project — context files stay usable by non-Hero agents. Users query ops via CLI/TUI.

**Consequences**:
- Lower token use; clearer Hero vs project boundary.
- Humans/agents that previously read `workflow.md` must use `hero status` / `hero metrics` / `hero events`.
- Optional future projections are explicitly out of scope (deferred D9).

---

## ADR-014: CLI as API; no daemon in 1.0

**Context**: Dual UI needs a shared API; a long-running daemon adds ops complexity.

**Decision**: Every UI and agent interacts with the state machine through `hero` subcommands (read/write SQLite). No `hero serve` / local RPC in 1.0.

**Consequences**:
- Simple deployment (single binary).
- Slightly higher process-start overhead per call; acceptable for 1.0.
- Daemon deferred (D7).

---

## ADR-015: Dual entry UI — chat and TUI parity

**Context**: Users want today’s Cursor chat experience and a first-class TUI without losing either.

**Decision**: Both Cursor chat and Hero TUI are first-class entry points over the same core. Either can run the full cycle (including approvals and stage progression). TUI is not a read-only dashboard.

**Consequences**:
- Feature parity checklist required in QA for chat vs TUI flows.
- Slash commands remain; TUI exclusivity deferred (D5).

---

## ADR-016: HarnessAdapter interface; Cursor-only implementation

**Context**: 1.0 must be an orchestrator of harnesses architecturally without shipping every harness.

**Decision**: Define a stable `HarnessAdapter` interface in Go. Ship a Cursor implementation that supports: (1) chat-driven execution (current UX), (2) TUI/`hero run` dispatch into Cursor when available. Other harnesses post-1.0 (D1).

**Consequences**:
- Future harnesses plug in without rewriting the state machine.
- Cursor API/IDE limits may constrain push dispatch; chat path remains the reliable baseline.

---

## ADR-017: Bubble Tea TUI; Claude Code inspired

**Context**: Need a usable terminal UX without building the full archived TUI vision.

**Decision**: Build 1.0 TUI with Charm **Bubble Tea**, reusing **huh** where prompts fit. Minimum screens: cycle status, approvals, artifacts, costs/metrics, recent events, basic command palette. Use Claude Code’s TUI as **UX inspiration** (interaction patterns), not a clone. Full multi-panel richness deferred (D10).

**Consequences**:
- Fits existing Charm ecosystem (huh already in use).
- Scope stays deliverable for 1.0.

---

## ADR-018: Breaking major upgrade from 0.9.x

**Context**: SQLite-canonical ops and CLI state machine are incompatible with treating 0.9.x cycle markdown as live truth.

**Decision**: Ship as SemVer **1.0.0** (major). `hero upgrade` creates/migrates SQLite and refreshes Runtime assets to CLI-API orchestration. Document migration. Do **not** maintain long-lived soft dual-mode for markdown cycles in 1.0 (D11).

**Consequences**:
- Clear break; simpler runtime.
- Users must upgrade deliberately; migration notes required in DEPLOY/help.

---

## ADR-019: Archive idea v1; cycle docs canonical

**Context**: `docs/idea/v1/` was generated without full knowledge of current Hero and must not compete with ADR/PRD.

**Decision**: Move to `docs/idea/archive/v1/`. Canonical product/architecture for 1.0 are cycle docs (this file, PRD-C01-001, UI-C01-001) plus updated indexes. Do not rewrite idea in-place as product spec (D12).

**Consequences**:
- Inspiration preserved; no dual source of truth.
