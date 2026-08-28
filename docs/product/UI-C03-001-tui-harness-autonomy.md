# UI-C03-001 — TUI Harness Autonomy & Conversation UX

> Cycle C3 UI requirements for autonomous TUI orchestration, harness boot, streaming conversation, and slash hyphen parity. Extends [UI-C01-001](UI-C01-001-hero-tui.md), [UI-C02-001](UI-C02-001-tui-slash-command-parity.md). Product: [PRD-C03-001](PRD-C03-001-cursor-harness-tui-autonomy.md).

## 1. Scope

- **In**: Harness selection at boot; conversation screen with streaming; full `/hero-<name>` palette; hyphen labels (amends UI-C02); `/hero-cycles` and `/hero-todos` TUI invocation; harness error UX.
- **Out**: Multi-harness picker beyond Cursor (V1); IDE chat panel integration; full multi-panel Claude Code clone (D10).

## 2. Harness boot flow

When `cli.tools` is empty or missing:

```text
→ No harness configured for this project.
? Select harness:
  › cursor (detected: .cursor/)
  [Only supported harness in V1]
```

After selection, validation runs:

**Success:**

```text
✓ Cursor harness ready (cursor-agent v2026.07.23)
```

**Failure (example — not logged in):**

```text
✗ Cursor harness unavailable: authentication required
→ Run: cursor agent login
→ Then start Hero again: hero
```

TUI **exits** with non-zero status on validation failure.

## 3. Conversation screen

The project navigation sidebar order is **Chat | Status | Artifacts | Costs | Events**, with **Config** appended only while an active cycle exists. Shortcuts follow the visible order (`alt+1` / `ctrl+1` = Chat through `alt+5` / `ctrl+5` = Events; `alt+6` / `ctrl+6` = Config when visible). The navbar footer shows only `alt+1-5` or `alt+1-6` accordingly. TUI boot opens **Chat** (composer focused).

New screen (or primary mode when an etapa requires interações):

| Region | Content |
|---|---|
| Header | Left: with etapa Cycle C{N}, etapa name, iteration, harness session; without etapa (**freechat**): `Chat · harness <tool>`. Right: live **agents** box (`agents: N` plus 4-letter labels such as `ORCH \| BACK`; `HARN` for harness-native / freechat / unknown Task). |
| Transcript | Linear, borderless scrollable chat of the **full session** (all user + thinking/tool/agent turns). Each message has a thin vertical `│` accent bar in the actor color (user: violet; agent/response: green; thinking/tool: muted; warnings: amber) and a Hero origin label: `You` for user turns, `[LABEL - {model} · {harness}]` for agent blocks (same 4-letter labels as the agents box). Blank line between messages. Streams as `stream-json` events arrive; while waiting, shows the live speaker header plus a spinner. Subagent blocks keep blank separation when multiple agents speak. Task start/complete updates the agents box; nested Task text is shown when the CLI forwards it, otherwise Task `result.content` is printed in the subagent block. |
| Scroll hint | Directly under the transcript: property status labels and context bar (`█` filled / `░` empty) plus compact `used/max` (`180k/256k`) when the speaking model's `context_window` is in `models/*.yml`. Fill uses green below ~80%, yellow below ~95%, red at or above ~95% (UI.md §2.1). Omitted when the slug has no window. Used tokens come from the last harness `result.usage` (input+output); `/new-chat` resets to 0. Keyboard scroll hints stay in the footer (`↑↓ scroll`). |
| Input | OpenCode-style boxed prompt with colored solid accent bar; status line shows **Build** or **Plan**, model slug, and harness name. For ordinary text, **Enter** inserts a newline without sending; **Alt+Enter** submits **interação** (does not conflict with **Alt+1–5** screen jumps). A recognized slash command is executed with **Enter**; **Alt+Enter** remains reserved for ordinary prompt submission. **Alt+y** copies the latest user prompt; **Alt+r** copies the latest agent turn; **Alt+i** copies the composer (OSC 52 + native clipboard). Esc clears input (or dismisses the slash overlay first). **`/` stays in the composer** and opens a filtered autocomplete overlay of the full palette (including `Go to - *`, Refresh, Quit, `/hero-*`, and imported commands). **Tab** on **`/hero-approve` `/hero-reject` `/hero-cancel` `/hero-continue` `/hero-finish` `/hero-back`** **inserts** the token; **Enter** executes it. **Tab** on **every other item** runs the same action as the full-screen palette (navigate / Execute immediately); **Enter** preserves that command execution behavior. **Tab** toggles Build ↔ Plan only when the overlay is closed (Plan → Cursor Agent CLI `--mode plan`). `/` on other screens still opens the full command palette. With a live `/hero-start` orchestrator session, `/hero-approve` (and `/hero-reject` `/hero-cancel` `/hero-finish` `/hero-continue` `/hero-back`) are sent as **follow-ups** to that session — they must not fail on SQLite `PendingApproval` (the waiting agent persists via CLI). ↑↓ move the composer caret between visual lines and scroll the unified transcript at the boundaries. |
| Footer | Fixed hints: `tab mode · / commands · enter newline or command · alt+enter send · alt+r/i copy · ↑↓ scroll · alt+q quit` |

The left navbar ends with a blue timer subdivision below the navigation
shortcut hint:

```text
 Sessão 00:00:00
 AI     00:00:00
```

At TUI boot, `Sessão` starts at `00:00:00`; an existing persisted cycle is not
restored automatically. `/hero-new` starts a new cycle timer at zero, while
`/hero-start` and `/hero-resume` explicitly restore the accumulated cycle
seconds and continue from that baseline. The timer stops when the cycle
reaches a terminal state and archive resets its display. Without an active
cycle, free chat starts at zero with the first submitted prompt and continues
across later prompts until the TUI exits or a cycle is started. `AI` starts
with each Execute and stops when the response (including concurrent Executes)
completes or is cancelled. Both values use `HH:MM:SS` with continuous hours
and one-second resolution.

Chat **works without an active cycle/etapa** using `hero.json` → `harnesses.<tool>` defaults (Cursor: `composer-2.5`, `enable_fast_model: false`). Freechat session ids stay in TUI memory for the process lifetime.

At TUI boot, after harness validation, Hero calls the harness model catalog (`agent models` / `--list-models` for Cursor). Models are available via `/hero-model` in the palette; selection updates the Chat screen and persists to `hero.json` → `harnesses.<tool>.model`. If listing fails, boot continues with the configured slug (no hard fail).

During harness execution:

```text
→ Agent responding…
```

On completion, input re-enables. Errors show harness message verbatim + remediation line.

## 4. Hero slash labels (hyphen — mandatory)

Replaces UI-C02 colon labels. All Hero palette action labels:

| Action | TUI label |
|---|---|
| New cycle | `/hero-new` |
| Start workflow | `/hero-start` |
| Sync project | `/hero-sync` |
| Status | `/hero-status` |
| Approve | `/hero-approve` |
| Reject | `/hero-reject` |
| Cancel | `/hero-cancel` |
| Finish | `/hero-finish` |
| Archive | `/hero-archive` |
| Resume | `/hero-resume` |
| Continue (escalation) | `/hero-continue` |
| Back to planning | `/hero-back` |
| Help | `/hero-help` |
| List cycles | `/hero-cycles` |
| Show todos | `/hero-todos` |
| Select chat model | `/hero-model` |

Screen navigation (`Go: Status`, …) may remain for non-slash jumps.

Empty-cycle hint: run `/hero-new`, then `/hero-start` in a new context (or start from TUI conversation when harness ready).

Imported non-Hero commands: `/<stem>` unchanged from UI-C02.

## 5. `/hero-cycles` output (TUI + chat)

Formatted table or stacked sections per cycle:

```text
→ Cycles (3 total)

C3 — Correção de distorções na implementação da V1 [active]
  Research     running   in: 12k  out: 3k   ~$0.05   8m
  …
  Total: 15k tokens  ~$0.05

C2 — slash-parity-tui-harness [archived 2026-08-08]
  …
```

Include archive-only cycles from `.workflow-hero/cycles/archive/` when absent from SQLite.

## 6. `/hero-todos` output

```text
→ Pending items (from context/current-state.md)

• Tag/publish GitHub Release v1.0.0 when ready
• Post-1.0 deferred D1–D13
…

⚠ If docs/product or docs/architecture changed, run /hero-sync then /hero-todos to refresh.
```

## 7. Streaming rules

- Use harness `stream-json` with `--stream-partial-output` when TUI conversation is active.
- Pipe CLI stdout into the NDJSON parser **while the process runs** so transcript deltas appear before exit (not only after buffered `Run`).
- Forward `thinking` deltas (muted italic `Thinking:`), `tool_call` started events (`→ Read path`), and Task lifecycle (live agents box + labeled subagent blocks). Thinking/tools stay in the transcript after completion. The parent agent bubble is replaced with canonical `result.Output` **only when the turn has no subagent blocks** (replacing would wipe labels).
- Attribute assistant/tool events to an in-flight Task when the CLI tags them (`parent_tool_call_id`) or when exactly one Task is open. If no nested text arrived, print Task `result.content` in that subagent block.
- Buffer partial lines; update transcript incrementally (lipgloss-safe wrapping).
- On Esc during stream: call harness `Cancel` if supported; show `Interrupted`.

When Chat is idle, Esc retains its input-clear or overlay-dismiss behavior; while a Harness Execute or `/hero-start` preflight is active, Esc takes priority as the interruption key.

## 8. Approvals and Artifacts screens

**Approvals** lists the current pending stage (if any) and a chronological **History** of `requested` / `approved` / `rejected` / `escalated` / `continued` events with local timestamps. Empty active cycle: `No approval activity for cycle CN.` Keys `a` `r` `c` `f` unchanged.

**Artifacts** lists files generated for the active cycle: `.workflow-hero/cycles/current/`, the linked OpenSpec change directory, `documents.json` rows for this cycle, and cycle-tagged files under `docs/product|architecture|testing|deployment|ui`. Store metadata is merged (no duplicate paths). Columns: time, kind, label, path.

Status, Approvals, Artifacts, Costs, and Events clip to the content viewport and scroll with ↑↓ / PgUp / PgDn. Switching to those screens refreshes from SQLite + disk.

## 9. Success Criteria

- User never needs open Cursor chat to complete Research grilling in TUI (CLI logged in).
- All `hero-*.md` commands appear in palette with `/hero-<name>` labels.
- Boot validation failure never enters main TUI loop.
- `/hero-cycles` and `/hero-todos` render in TUI without chat fallback when harness available for display-only commands (todos may be read-only local file — no harness required for todos-only; cycles may use SQLite CLI only).
