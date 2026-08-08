# UI-C02-001 — TUI Slash Command Parity & Imported Commands

> Cycle C2 UI requirements for Hero TUI command naming and imported Cursor commands. Extends [UI-C01-001-hero-tui.md](UI-C01-001-hero-tui.md) and [UI.md](UI.md). Product: [PRD-C02-001](PRD-C02-001-slash-parity-tui-harness.md).

## 1. Scope

- **In**: TUI palette/footer/empty-state labels aligned to `/hero:*`; listing and invoking non-Hero Cursor custom commands via markdown expansion; archive failure UX for force path; doctor warning copy for harness detection.
- **Out**: Redesigning TUI screen layout; full Claude Code clone; IDE chat injection UI; skill palette entries.

## 2. Hero command labels (mandatory)

Map Hero TUI actions to slash labels (hints may stay short English/PT per chat language rules — labels themselves are the slash token):

| Action | TUI label |
|---|---|
| Approve pending stage | `/hero:approve` |
| Reject pending stage | `/hero:reject` |
| Cancel cycle | `/hero:cancel` |
| Finish cycle | `/hero:finish` |
| Archive (when exposed) | `/hero:archive` |
| Resume (when exposed) | `/hero:resume` |
| Status screen | `/hero:status` (or “Go: Status” may remain for screen jumps; prefer slash when the item is an action) |
| Dispatch / run harness | Keep as harness dispatch affordance; do not invent a non-existent `/hero:run` slash unless Runtime adds it — if shown, hint “harness dispatch” |
| Help (when exposed) | `/hero:help` |

Screen navigation entries (`Go: Approvals`, …) may remain as navigation, not slash actions.

Empty cycle hint must mention `/hero:new` (and may mention CLI only as secondary).

## 3. Imported commands section

- Palette groups or prefixes imported commands (e.g. under “Harness commands” / filterable `/`).
- Source paths shown in hint or detail: project vs user (`~/.cursor/commands`).
- Selecting an imported command shows a brief progress line (`→ Running /opsx:archive via markdown expansion…`) then adapter result.
- On dispatch failure: `⚠` / `✗` with message to run the same command in Cursor chat.

## 4. Archive force UX

When OpenSpec archive fails during `/hero:archive` (chat or TUI-triggered CLI):

```text
✗ OpenSpec archive failed: <reason>
→ Options: retry | force Hero archive (--force) and archive OpenSpec manually
  Manual: openspec archive <name> -y
```

TUI may present confirm for force; CLI documents `--force` / `--skip-openspec`.

## 5. Doctor / install / sync copy

Harness detection warnings follow UI.md icon rules:

```text
⚠ Detected .claude/ but cli.tools does not include it (unsupported in this Hero version — not installed).
→ Supported today: cursor. See docs for multi-harness roadmap (D1).
```

## 6. Success Criteria

- Palette Hero actions are recognizable as `/hero:*` without reading docs.
- Imported commands appear with slash-style names and fail gracefully when the adapter cannot dispatch.
- Archive failure messaging always includes the manual OpenSpec command.
