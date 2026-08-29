# docs/idea — Design notes

Optional, **non-normative** design notes for upcoming Hero cycles. The Research stage (`discover_agent`) considers files here at session start when present.

## Layout

| Location | Purpose |
|---|---|
| `docs/idea/` (root and subfolders except below) | **Active** ideas — read by discover at Research start |
| `docs/idea/archive/` | Archived ideas from completed cycles (ignored by discover) |
| `docs/idea/tobe/` | Future ideas not yet in scope (ignored by discover) |

## Discover behavior

- **Hero TUI**: active paths are listed in the discover session prompt automatically.
- **Cursor IDE**: `discover_agent` runs `hero cycle idea-files` and reads each returned path before grilling.
- **Empty folder**: Research starts normally with no extra context.
- **Canonical specs**: cycle PRD/ADR/UI always supersede idea notes on conflict (ADR-019).

## Listing active files

```bash
hero cycle idea-files
hero cycle idea-files --json
```
