# UI-C16-001 — TUI Session History

> Cycle C16 terminal UX. Extends [UI-C03-001](UI-C03-001-tui-harness-autonomy.md), [UI-C08-001](UI-C08-001-tui-stage-execute.md), [UI-C09-001](UI-C09-001-telegram-integration.md), and [UI-C14-001](UI-C14-001-tui-multimodal-images.md). Product: [PRD-C16-001](PRD-C16-001-tui-session-history.md).

## 1. Navigation

Project TUI navigation becomes:

```text
Chat | History | Status | Artifacts | Costs | Events | Settings | Config*
```

`Config` remains conditional on an active cycle. Visible numeric shortcuts always match visible order:

- project without active cycle: `alt+1` Chat through `alt+7` Settings;
- project with active cycle: `alt+1` Chat through `alt+8` Config;
- standalone Free Chat: `Chat | History | Settings`, `alt+1-3`.

History is available without an active cycle. Existing navbar focus rules remain: `Esc` focuses the navbar, Up/Down moves its cursor, Enter opens the item, and Tab/Shift+Tab switches shell focus.

## 2. Main screen

Wide layout:

```text
History                                      Active  Archived
Search by name: deployment_

› API deployment review       Stage · cursor/composer-2.5   11 Sep 21:42
  C16 · Research · DISC        Research · codex/gpt-5.6-sol  11 Sep 20:18
  Image exploration            Free Chat · codex/gpt-5.6    10 Sep 09:04
│
│ Name          API deployment review
│ Type          Stage
│ Cycle         C15
│ Stage/Agent   Implementation / GEN
│ Harness       cursor
│ Model         composer-2.5
│ Created       11 Sep 2026 20:54
│ Last activity 11 Sep 2026 21:42
│ State         Active
│ Transcript    Available locally

↑↓ navigate · ←→ active/archived · / search · enter open · r rename · a archive · d delete
```

- The list is the primary focus. Selection updates the detail panel without I/O in `View`.
- Active rows sort by `last_activity_at DESC`, then stable session ID.
- Search performs case-insensitive name matching within the selected Active/Archived view. `Esc` first clears/exits search; a second `Esc` focuses the navbar.
- Left/Right switches Active/Archived. Opening an archived row restores it before Chat opens.
- `Enter` opens the selected session in Chat at the newest transcript position.
- `r` opens inline/modal rename, `a` archives or restores according to the current view, and `d` opens permanent-delete confirmation.
- The fixed footer shows only actions that are valid for the selected row and current execution state.

## 3. Responsive behavior

- Width and height come from `tea.WindowSizeMsg`; ANSI-aware widths and actual style frames determine truncation.
- On medium width, the detail panel moves below the list.
- On narrow or short terminals, History shows the list alone. A details action opens a full-content detail state and `Esc` returns to the list.
- Long names are truncated with an ellipsis without splitting grapheme/rune sequences. Harness/model and timestamps are progressively hidden before the session name.
- Loading, empty, error, and confirmation states remain usable at minimum supported dimensions. If no meaningful layout fits, use the existing centered `window too small` behavior.

## 4. States and copy

Empty active view:

```text
No saved sessions yet.
→ Send a message in Chat to create one.
```

Empty archived view:

```text
No archived sessions.
```

Busy ownership:

```text
⚠ Session is open in another Hero TUI.
→ Retry after that instance releases it.
```

Legacy entry:

```text
Transcript  Local transcript unavailable
→ Native resume may still be available.
```

Interrupted entry:

```text
⚠ This session was interrupted during a response.
→ Checking the harness execution before continuing…
```

Native resume unavailable:

```text
✗ The original <harness>/<model> session cannot be resumed.
→ Create a new session using the available local transcript as context?
```

Remote import:

```text
Import transcript from <harness>?
The imported conversation will be stored locally in this project.

Import  Cancel
```

All asynchronous loads and mutations display a non-blocking spinner and disable duplicate submission until their result message returns.

## 5. Rename

- The current name is prefilled and selected for editing.
- Trim surrounding whitespace; reject an empty result with an inline error.
- Duplicate names are accepted.
- `Enter` saves, `Esc` cancels, and failed persistence retains the edit buffer.
- Rename is allowed in Active and Archived views, including for the currently open idle session.

## 6. Archive and restore

- `a` on Active asks for a lightweight confirmation only when the selected session is currently open; otherwise it archives immediately and shows `✓ Session archived.`
- `a` on Archived restores without destructive confirmation and shows `✓ Session restored.`
- `Enter` on Archived performs restore then opens Chat as one user action.
- Archiving the current idle session opens a new empty Chat surface after persistence succeeds.
- Archive/restore is disabled while an Execute or unsafe preflight is active.

## 7. Permanent deletion

```text
Delete “API deployment review” permanently?

This removes the local transcript and Hero-managed asset copies.
Original files are never deleted. Provider-side data may remain if the
harness cannot delete it.

Delete permanently  Cancel
```

- Default focus is `Cancel`.
- The confirmation names the session and cannot be submitted twice.
- On local success: remove the row immediately and show `✓ Session deleted.`
- If remote deletion is unsupported or fails, local success remains and an amber warning states that provider-side data may remain.
- If local deletion fails, keep the row and all local assets, show an actionable error, and do not request remote deletion.
- Deleting the current idle session opens a new empty Chat surface.
- Delete is disabled during an Execute or unsafe preflight.

## 8. Chat restoration

- Chat header shows the saved session name plus its existing cycle/freechat and harness information.
- The transcript restores all locally persisted visible events with their original actor styling, agent labels, Telegram origin, attachment cards, and interruption markers.
- `/new-chat` leaves the restored session in History and opens an unpersisted blank surface.
- Resuming an archived session restores it automatically.
- A completed stage conversation displays a muted note: `Historical continuation · workflow stage remains completed.`
- If original native continuation fails, the explicit fork dialog identifies the unavailable harness/model. A successful fork creates a new History row and leaves the original unchanged.
- If the harness reports a still-running turn after restart, the composer stays disabled while Hero reconnects or offers cancellation.

## 9. Input and focus rules

- Use centralized `key.Binding` definitions and `key.Matches`; no raw-key branch may duplicate a global binding without an explicit screen rule.
- In History content focus, `/` is search rather than the command palette. The footer makes this exception visible. After search is closed, the command palette remains reachable from the navbar or another screen.
- Modal dialogs own keyboard focus. Background list/navigation keys do not leak through.
- All database, filesystem, adapter, remote import/delete, and transcript paging work runs through `tea.Cmd`; `Update` and `View` never block.

## 10. Acceptance scenarios

1. Create a Free Chat, exchange text/image turns, exit, reopen Hero, find it by name, and continue the same native session.
2. Open History with no cycle, search, rename, archive, restore, and open a session using only documented keys.
3. View separate History rows for parallel stage agents and continue a completed one without changing stage status.
4. Receive a Telegram message and verify the same session row/activity timestamp and restored origin marker.
5. Simulate a crash mid-stream and verify durable partial events, interrupted state, native status check, and composer gating.
6. Attempt the same session from a second TUI and receive a busy message; expire the abandoned lease and recover.
7. Delete a session with copied and original assets; only Hero-managed copies disappear.
8. Resize through wide, stacked, and narrow layouts without clipped actions or stale selection.

