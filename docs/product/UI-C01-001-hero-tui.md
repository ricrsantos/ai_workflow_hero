# UI-C01-001 — Hero 1.0 TUI & CLI Query UX

> Cycle C1 UI requirements for the Hero TUI and CLI-as-API query surfaces. Extends (does not replace) terminal conventions in [UI.md](UI.md). Inspiration: Claude Code TUI patterns; archived idea `docs/idea/archive/v1/12_hero_tui.md` is non-normative richness.

## 1. Scope

- **In**: Bubble Tea TUI (minimal screens), CLI human/JSON query commands for cycle ops stored in SQLite, shared semantic icons with chat Runtime.
- **Out**: Full multi-panel TUI (deferred D10); daemon UI; web UI; making TUI the only entry (deferred D5).

## 2. Entry Points

| Entry | How | Parity |
|---|---|---|
| Cursor chat | `/hero:*` commands; agents call `hero` for state | Same cycle experience as 0.9.x, SQLite-backed |
| Hero TUI | `hero tui` (final verb in SDD) | Can drive full cycle: progress, approve, dispatch, inspect |

User chooses either; both share the Go state machine + SQLite.

## 3. TUI Requirements

### 3.1 Stack

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the interactive app.
- [huh](https://github.com/charmbracelet/huh) for discrete prompts/forms where appropriate.
- Follow existing color/icon semantics from [UI.md](UI.md) (§2).

### 3.2 UX inspiration (Claude Code)

Adopt patterns, not branding or a clone:

- Keyboard-first navigation and a compact **Commands** menu opened with `/` (Claude Code–style; avoids Cursor/VS Code `Ctrl+P`).
- Clear focus on “what is happening now” (current stage / pending approval).
- Low-chrome layout; avoid dashboard clutter.
- Streaming/progress feel for long operations where the adapter reports status.

### 3.3 Minimum screens (1.0)

1. **Cycle status** — title, objective summary, stage table (status, iteration, approval).
2. **Approvals** — pending stage summary; approve / reject / cancel / finish actions (mapped to CLI API).
3. **Artifacts** — list metadata from store + paths to project files.
4. **Costs / metrics** — per-stage and totals (from SQLite).
5. **Events** — recent append-only event log.
6. **Commands menu** (`/`) — jump to screens / common actions (slash-style, like Claude Code; avoids IDE `Ctrl+P` conflicts).

### 3.4 Non-goals for 1.0 TUI

- Multi-split IDE-like panes as default.
- Embedded full markdown editors for PRD/SDD (open paths in external editor / IDE).
- Notification center for external integrations.

## 4. CLI Query & Control API (UX)

Deterministic commands (names finalized in SDD), all supporting human table default and `--json` where they are read commands (per UI.md):

| Concern | Behavior |
|---|---|
| Status | Show stage machine state from SQLite (replaces reading `workflow.md`). |
| Metrics | Show token/cost estimates (replaces reading `metrics.md`). |
| Events | Show recent or filtered event log. |
| Advance / approve / reject / cancel / finish | Mutate state machine per workflow rules. |
| TUI | Launch Bubble Tea app. |
| Run / dispatch | Optional push into Cursor adapter when driving from TUI. |

Errors use the `✗ Error: <message>.` convention (UI.md §5).

## 5. Chat Runtime UX

- Keep slash-command discoverability and stage summary style (icons, metrics block).
- Stage close must **not** instruct agents to write `workflow.md` / `metrics.md`; instruct CLI persistence instead.
- Metrics summary in chat remains mandatory; point users to `hero metrics` for full details (and project summary as designed).

## 6. Accessibility & Degradation

- `NO_COLOR` / non-TTY: TUI should refuse or degrade gracefully; CLI tables fall back to plain text per UI.md.
- Lock file / busy cycle: clear error if another session holds the cycle.

## 7. Success Criteria

- New user can approve a stage and see updated status in TUI without opening markdown cycle files.
- Chat user can finish a stage without creating `workflow.md`.
- `--json` status/metrics suitable for scripting.
