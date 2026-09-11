# UI-C14-001 — Multimodal Image Support TUI UX

> Cycle C14 UI specification. Product: [PRD-C14-001](PRD-C14-001-tui-multimodal-images.md). Architecture: [ADR-C14-002](../architecture/ADR-C14-002-tui-multimodal-images.md).

## 1. Scope

Defines terminal UX for image attachment input and image asset output in the Hero TUI Free Chat. Applies to Linux and macOS. Does not apply to Research or workflow-stage sessions.

---

## 2. Composer — attachment chips

### 2.1 Layout

The composer area renders attachment chips below the text input line and above the status line:

```
│ Compare this mockup with the current implementation.
│ [image] checkout.png · PNG · 1440×900 · 820 KB  [x]
│ Free Chat · claude-sonnet-4-6 · claude
```

- Each chip occupies its own line. Multiple chips stack vertically.
- Chip content: `[image]` icon tag, original file name (truncated to terminal width − margin), MIME type, dimensions, file size, and a `[x]` close indicator.
- The text input cursor does not traverse chip regions.

### 2.2 Attachment interaction

| Action | Behavior |
|---|---|
| `Alt+A` or `/attach` | Open file picker |
| `Alt+V` or `/attach-clipboard` | Capture image from system clipboard |
| `/attach <path>` | Attach a file by explicit path |
| Bracketed paste of a filesystem path | TUI offers to attach it as an image (confirmation prompt inline) |
| Backspace in text field | Does not remove a chip |
| Focus chip + `x` or `Delete` | Remove that attachment |
| `Enter` (in text field, with chips) | Send turn; images sent first, then text |
| `Enter` (chips only, no text) | Send image-only turn |

### 2.3 External path warning

When a resolved path is outside the workspace, the chip displays a warning badge before the file name:

```
│ [image] ⚠ external: /home/user/screenshots/design.png · PNG · 1920×1080 · 1.2 MB  [x]
```

The warning does not block the turn unless the file-permissions policy explicitly denies external paths.

### 2.4 Capability error

When the active model lacks image input capability, submitting a turn with attachments renders an inline error instead of sending:

```
✗ Model claude-haiku-4-5 (claude) does not support image input.
  Remove the attachment or switch to a capable model.
```

Chips remain in the composer. No attachment is removed automatically.

### 2.5 Validation errors

Per-file validation failures appear as error chips in place of the attachment chip:

```
│ [error] design.pdf — unsupported format (PNG, JPEG, GIF, WebP only)
│ [error] screenshot.png — file too large (24 MB, limit 20 MB)
```

Error chips cannot be sent; they must be removed individually.

---

## 3. Transcript — asset cards

### 3.1 Basic card layout

After a turn that produces an image asset, the card appears in the transcript attached to the model turn:

```
┌─ Image  ──────────────────────────────────────────┐
│ mockup-home.png · PNG · 1536×1024 · 1.8 MB        │
│ [enter] preview  [o] open  [c] copy path           │
│ [a] attach  [s] save                               │
└────────────────────────────────────────────────────┘
```

- The card is keyboard-navigable from the transcript. Focus moves to it with standard scroll/cursor keys.
- `Source` label variants: "Image generated" (model/tool), "Image received" (user attachment echo, if shown).

### 3.2 Unicode mosaic preview

Pressing `Enter` on a focused card (or the `preview` action) expands an inline Unicode mosaic below the card:

```
┌─ Image  ──────────────────────────────────────────┐
│ mockup-home.png · PNG · 1536×1024 · 1.8 MB        │
│ [enter] close  [o] open  [c] copy path             │
│ [a] attach  [s] save                               │
├────────────────────────────────────────────────────┤
│ ██████████████████████████████████████████████████ │
│ ██▓▓░░░░▓███▓░░░░░░░░░░░░░░▓████████▓░░░░░███████ │
│ ██████████████████████████████████████████████████ │
│ (mosaic — 80×24 cells, truncated to pane width)    │
└────────────────────────────────────────────────────┘
```

- Mosaic dimensions adapt to the current terminal width and the available pane height.
- On terminals without 256+ color support, the `preview` action shows: `Preview unavailable (terminal color depth insufficient). Use [o] to open in viewer.`
- The expanded mosaic state persists while the card remains in view; closing collapses it.

### 3.3 Advanced inline preview (optional, phase 4)

When Kitty, Sixel, or iTerm2 support is enabled and detected, `preview` renders pixels instead of a mosaic. The card action bar remains visible and navigable. The inline image is cleared on scroll or resize; the card reverts to mosaic (or card-only) state automatically.

### 3.4 Tool-generated asset cards

Image files written by a tool during a turn appear as asset cards at the end of the model turn, after any text output:

```
[tool wrote] design-export.png · PNG · 2048×1536 · 3.4 MB
[enter] preview  [o] open  [c] copy path  [a] attach  [s] save
```

Multiple tool-written images appear as separate cards in creation order.

---

## 4. File picker

- The file picker uses the existing Bubbles `filepicker` component (or equivalent) and follows the project's theming and keymap conventions.
- The picker filters to: `*.png`, `*.jpg`, `*.jpeg`, `*.gif`, `*.webp` (case-insensitive).
- Confirming a file triggers async validation. A spinner chip appears in the composer while validation runs; it is replaced by the attachment chip on success or an error chip on failure.
- Cancel (Escape) returns focus to the text input without changes.

---

## 5. Save dialog

Pressing `s` on a focused asset card opens an inline path input:

```
Save to: ~/Downloads/mockup-home.png  [enter confirm]  [esc cancel]
```

- Pre-populated with `~/<download-dir>/<original-name>`.
- Overwrite warning if the destination path already exists.
- On success: card footer updates to `Saved to ~/Downloads/mockup-home.png`.
- On failure: inline error below the input.

---

## 6. Status and progress indicators

| State | Indicator |
|---|---|
| Clipboard capture in progress | Spinner on the attachment area |
| File validation in progress | Spinner chip in composer |
| Mosaic rendering | Spinner in card preview area |
| External viewer launching | One-line status: `Opening mockup-home.png…` |
| Save in progress | Spinner on save dialog |

All indicators are non-blocking: the TUI remains responsive to keyboard input during async operations.

---

## 7. Keyboard summary

| Key | Context | Action |
|---|---|---|
| `Alt+A` | Composer (Free Chat) | Open file picker |
| `Alt+V` | Composer (Free Chat) | Attach from clipboard |
| `Enter` | Composer | Send turn |
| `Delete` / `x` | Focused chip | Remove attachment |
| `Enter` | Focused asset card | Toggle mosaic preview |
| `o` | Focused asset card | Open in system viewer |
| `c` | Focused asset card | Copy path to clipboard |
| `a` | Focused asset card | Attach to next turn |
| `s` | Focused asset card | Open save dialog |
| `Esc` | File picker / save dialog | Cancel |

Key assignments for `Alt+A` and `Alt+V` must be validated against the existing keymap during Planning. If either conflicts with an existing binding, the planning agent proposes an alternative that does not break the current keymap.

---

## 8. Accessibility and terminal compatibility

- All image content remains accessible via the text card (file name, dimensions, size, actions) regardless of terminal capability.
- Color degradation: mosaic degrades gracefully when the terminal lacks 256+ color support.
- SSH and terminal multiplexer environments: mosaic is the primary preview (pixel protocols are disabled by default); `open` action and `copy path` remain functional.
- No UI element depends on mouse input; full keyboard navigation is mandatory.
- Resize: the mosaic (if expanded) is re-rendered on `tea.WindowSizeMsg`; re-render is async and non-blocking.
