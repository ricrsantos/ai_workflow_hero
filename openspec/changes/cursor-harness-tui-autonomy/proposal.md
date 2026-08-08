## Why

Hero V1 shipped a minimal `HarnessAdapter` (`Dispatch` only, `Pusher == nil`) that always falls back to Cursor chat, contradicting the V1 architecture (`06_harness_adapter.md`, `07_cursor_adapter.md`). Users expect the Hero TUI to run the full development cycle—including interactive Research (grill-me)—autonomously via **Cursor Agent CLI**, not an open IDE chat panel (PRD-C03-001 §1; ADR-025–026).

Cycle C3 corrects the Cursor adapter, delivers TUI conversation orchestration with streaming, fixes slash vocabulary to **`/hero-<name>`** (hyphen, amending ADR-020), and adds `/hero-cycles`, `/hero-todos`, and extended `/hero-sync` (PRD-C03-001 §4; ADR-024, ADR-028–029).

## What Changes

- **BREAKING (user-facing copy):** Canonical slash commands use **hyphen** (`/hero-start`, not `/hero:start`) across Runtime assets, TUI, README, workflow-help (ADR-024).
- Expand **`HarnessAdapter`** to full contract: `IsAvailable`, `CreateSession`, `ResumeSession`, `Execute`, `Cancel`, `Status`; Cursor adapter runs `cursor agent` / `cursor-agent` with JSON and `stream-json` output (ADR-025).
- **TUI autonomous orchestration:** conversation UI, streaming agent output, harness **interação** per conversational round within an etapa; Go owns etapa state only (ADR-026).
- **TUI harness boot:** prompt when `cli.tools` empty; validate harness; persist to `cli.tools`; abort on failure with remediation (e.g. `cursor agent login`) (ADR-027).
- **TUI palette:** all `hero-*.md` commands as `/hero-<name>` actions; not only approve/reject subset (UI-C03-001 §4).
- **New Runtime commands:** `hero-cycles.md`, `hero-todos.md` (+ TUI palette entries) (ADR-028).
- **Extended `hero-sync`:** scan `docs/product/` and `docs/architecture/` for pending items → merge into `context/current-state.md` (ADR-029).
- **Dual-entry preserved:** chat path remains supported with capability parity (ADR-015 amended).

### In Scope

- Full Cursor CLI harness execution; TUI conversation + boot; hyphen slash parity; hero-cycles/todos; hero-sync doc scan; complementary doctor CLI/login checks (not TUI prerequisite).

### Out of Scope

- Non-Cursor harness implementations (D1).
- IDE chat injection.
- Go-side LLM reasoning (ADR-003).
- Windows CLI target.
- Daemon / background agent (ADR-014).

### CLI vs Runtime Classification (ADR-003)

- **CLI:** `internal/harness`, `internal/adapters/cursor`, `internal/tui`, `internal/cycle` (cycles list query), `internal/doctor` (optional CLI checks), SQLite session metadata if added.
- **Runtime:** new/updated `hero-*.md`, orchestration/skill copy, hyphen vocabulary, hero-sync prompt extension.

## Capabilities

### New Capabilities

- `cursor-cli-harness-execution`: Full HarnessAdapter + Cursor Agent CLI process runner, session resume, JSON/stream-json parsing, login/version errors (ADR-025; PRD-C03-001 §4.1–4.2).
- `tui-conversation-orchestration`: Conversation screen, streaming transcript, user input for interações, harness Execute/Resume per interação (ADR-026; UI-C03-001 §3).
- `tui-harness-boot`: Harness selection prompt, validation, `cli.tools` persistence, abort UX (ADR-027; UI-C03-001 §2).
- `hero-cycles-command`: List cycles from SQLite + archive folders with per-etapa metrics (ADR-028; PRD-C03-001 §4.6).
- `hero-todos-command`: Display `current-state.md` pending items + sync notice (ADR-028; PRD-C03-001 §4.7).
- `hero-sync-pending-docs`: Extend sync to merge pending items from product/architecture docs into `current-state.md` (ADR-029; PRD-C03-001 §4.8).

### Modified Capabilities

- `harness-adapter`: Requirements change from best-effort `Dispatch` + chat fallback to mandatory CLI execution path when available; full interface surface (ADR-025).
- `hero-tui`: Hyphen labels; all Hero slash commands; conversation screen; harness boot (supersedes colon labels from C2 spec) (ADR-024, ADR-026–027; UI-C03-001).
- `runtime-workflow-execution`: Hyphen vocabulary; new slash commands; dual-entry guidance (ADR-024, ADR-028).
- `sqlite-operational-store`: Optional harness session id storage per cycle/etapa for `--resume` (design detail).

## Impact

- Packages: `internal/harness`, `internal/adapters/cursor`, `internal/tui`, `internal/cycle`, `internal/store`, `internal/doctor`, `assets/cursor/commands/`, orchestration skill.
- External: requires `cursor agent` / `cursor-agent` on PATH; `cursor agent login` for authenticated use.
- Tests: injectable process runner, stream parser fixtures, TUI boot/conversation tests, cycles/todos formatting, sync doc scan fixtures; `go test ./...`.
- Implementation agent: **generic_agent** (`scope.native: true`).
- OpenSpec change: `cursor-harness-tui-autonomy` (separate from `hero-1-0`, `browser-ui-validation`).
