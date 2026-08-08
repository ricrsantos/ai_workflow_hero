## Why

Hero 0.9.x keeps cycle operational state in markdown (`workflow.md` / `metrics.md`) edited by chat agents, which burns tokens, cannot sync chat and TUI, and cannot host a deterministic AI Loop. Cycle C1 (PRD-C01-001) ships Hero **1.0** as a harness orchestrator: Go owns the state machine and SQLite store; Cursor chat and a Bubble Tea TUI are dual entry UIs over the same CLI-as-API core (ADR-012–019).

## What Changes

- **BREAKING**: SemVer **1.0.0** — SQLite becomes the sole Hero operational store; cycle markdown (`workflow.md` / `metrics.md`) ceases to be canonical (ADR-013, ADR-018; DEPLOY §3.1).
- Add a deterministic **AI Loop / Workflow Engine** in Go (stage advance, gates, iterations, timeouts, locks) — no LLM reasoning in CLI (ADR-012; amends ADR-003).
- Add **SQLite** under `.workflow-hero/` for cycle/stage status, events, metrics/costs, operational conversation history, and artifact **metadata** (project files stay on disk).
- Expand **CLI as API**: query/control subcommands (`status`, `metrics`, `events`, advance/approve/reject/cancel/finish, cycle lifecycle, `tui`, optional `run`/dispatch) invoked by Runtime slash commands and TUI — no daemon (ADR-014).
- Add **HarnessAdapter** interface + **Cursor** implementation; chat path preserved; TUI/`hero run` can dispatch when available (ADR-016). Other harnesses deferred (D1).
- Add minimal **Bubble Tea** TUI (`hero tui`): cycle status, approvals, artifacts, costs, events, command palette — Claude Code–inspired patterns, not a clone (ADR-017; UI-C01-001).
- Update **Runtime assets** so stage close persists via CLI API and **never** instructs agents to write `workflow.md` / `metrics.md` (PRD-C01-001 §5.4).
- Extend **`hero upgrade`** to create/migrate SQLite and refresh Runtime assets for CLI-API orchestration (DEPLOY §3.1).
- Verify idea archive path `docs/idea/archive/v1/` (ADR-019 already executed in Research); docs-only touch-ups if needed.

### In Scope

- Core + usable TUI; dual UI parity (chat **or** TUI drive the full cycle).
- Cursor-only concrete harness behind stable adapter interface.
- Config files remain user-editable; imported into store as needed.
- Project vs Hero separation: do not kidnap `context/*.md`, `docs/`, `openspec/`, code.

### Out of Scope (Deferred D1–D13 — do not implement)

D1 multi-harness · D2 external integrations · D3 rich notifications · D4 Browser Visual Validation expansion · D5 TUI-only primary UI · D6 full idea roadmap · D7 daemon/`hero serve` · D8 full event bus · D9 cycle markdown projections · D10 full rich TUI · D11 soft dual-mode 0.9.x · D12 idea-as-living-spec · D13 chat-only or push-only without parity.

### CLI vs Runtime Classification (ADR-003 as amended)

- **CLI (primary)**: state machine, SQLite store, CLI-as-API commands, upgrade migration, HarnessAdapter + Cursor plumbing, Bubble Tea TUI rendering — new/extended `internal/<feature>/` packages.
- **Runtime**: slash commands, orchestration/skills/agents updated to call CLI for operational state; stage **reasoning** stays in harness agents.

## Capabilities

### New Capabilities

- `ai-loop-state-machine`: Deterministic Go workflow engine — stages, approvals, iterations, timeouts, locks; unit-testable without LLMs (ADR-012; PRD-C01-001 §3, §5.4).
- `sqlite-operational-store`: SQLite sole store for Hero ops; schema for cycle/stage/events/metrics/artifact metadata; no cycle markdown projections (ADR-013).
- `cli-as-api`: Subcommands for status/metrics/events and state mutations; human table + `--json` for reads; UIs invoke CLI only (ADR-014; UI-C01-001 §4).
- `harness-adapter`: Stable `HarnessAdapter` interface + Cursor implementation (chat-driven + optional TUI/`hero run` dispatch) (ADR-016).
- `hero-tui`: Bubble Tea (+ huh) minimal screens and palette; dual-entry parity with chat (ADR-015, ADR-017; UI-C01-001 §3).

### Modified Capabilities

- `cli-deterministic-command-suite`: Add 1.0 CLI verbs; extend `upgrade`/`status`/`doctor` for SQLite; bump default version to 1.0.0; preserve checksum-safe upgrade (DEPLOY §3.1, §7).
- `runtime-workflow-execution`: Stage close sequence and slash commands use CLI API; remove requirements to update `workflow.md`/`metrics.md`; keep Browser UI Validation and other 0.9 Runtime stage semantics unless superseded.
- `asset-bootstrap-and-layout`: Install creates SQLite store; stop treating cycle markdown templates as canonical ops; Runtime asset inventory updates for CLI-API prompts; doctor checks store presence.

## Impact

- New packages (suggested): `internal/store/`, `internal/engine/` (or `workflow`), `internal/cycle/` (CLI API), `internal/tui/`, `internal/harness/` (+ Cursor under `internal/adapters/cursor/` or harness package).
- Modified: `internal/upgrade/`, `internal/status/`, `internal/doctor/`, `internal/install/`, `cmd/hero/`, `internal/common/runtime_assets_test.go`.
- Dependencies: SQLite driver (e.g. modernc.org/sqlite or equivalent pure-Go), charmbracelet/bubbletea (+ lipgloss as needed); huh already present.
- Runtime: `assets/cursor/commands/*`, `orchestration_agent`, `workflow-hero` skill, templates that reference cycle markdown.
- Docs: help/README references to `hero metrics`/`events`; context compression after implementation.
- Version: CLI default → `1.0.0`.
- Implementation agent: **generic_agent** (workflow scope `native: true`).
