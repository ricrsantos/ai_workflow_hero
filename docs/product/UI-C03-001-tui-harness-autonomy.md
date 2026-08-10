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

New screen (or primary mode when an etapa requires interações):

| Region | Content |
|---|---|
| Header | With etapa: Cycle C{N}, etapa name, iteration, harness session. Without etapa (**freechat**): `Free chat · harness <tool>` |
| Transcript | User, **thinking** (muted), **tool activity**, and agent answer; all stream as `stream-json` events arrive |

| Input | OpenCode-style boxed prompt with colored accent bar; status line shows **Build** or **Plan**, model slug, and harness name; Enter submits **interação**; Esc clears input; **Tab** toggles Build ↔ Plan (Plan → Cursor Agent CLI `--mode plan`) |
| Footer | Hints: `tab mode`, `/hero-model`, `alt+1–6` screens, `ctrl+q` quit |

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
- Forward `thinking` deltas (muted italic `Thinking:`) and `tool_call` started events (`→ Read path`) in addition to assistant text. Thinking/tools stay in the transcript after completion; the agent bubble is replaced with canonical `result.Output`.
- Buffer partial lines; update transcript incrementally (lipgloss-safe wrapping).
- On Ctrl+C during stream: call harness `Cancel` if supported; show `Interrupted`.

## 8. Success Criteria

- User never needs open Cursor chat to complete Research grilling in TUI (CLI logged in).
- All `hero-*.md` commands appear in palette with `/hero-<name>` labels.
- Boot validation failure never enters main TUI loop.
- `/hero-cycles` and `/hero-todos` render in TUI without chat fallback when harness available for display-only commands (todos may be read-only local file — no harness required for todos-only; cycles may use SQLite CLI only).
