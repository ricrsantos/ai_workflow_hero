## Context

See `proposal.md` for motivation. Free Chat today is text-only: `conversation.Input`, `harness.ExecuteRequest`, `harness.ExecutionResult`, and `harness.StreamDelta` carry no image fields. Adapters compose text-only prompts (`codex` turn/start text item; `opencode` text parts; `cursor`/`claude` argv prompt). TUI composer is custom rune input with no chips, no Bubbles filepicker, no bracketed-paste handling, and clipboard helpers that write via OSC 52 / atotto (read path must use native OS APIs). Asset cards and mosaic preview do not exist.

Authoritative requirements: PRD-C14-001 §§1–5, UI-C14-001 §§1–8, ADR-077–082. Idea note yields on conflict.

## Goals / Non-Goals

**Goals:**
- Shared multimodal contract in `internal/harness` (ADR-077) with immutable path/UUID references in the Bubble Tea model (ADR-078).
- Session asset store with validation, dedupe, manifest, retention (ADR-080).
- Fail-closed capability admission (ADR-079).
- Free Chat attachment UX + transcript asset cards + mandatory Unicode mosaic (ADR-081).
- Adapter translation for Codex, OpenCode, Cursor, Claude; tool-written image cards (ADR-082).
- Parallelizable implementation after shared contract; all work owned by `generic_agent`.

**Non-Goals:**
- Images outside Free Chat; Telegram image transport; audio/video/PDF; Windows; SQLite blobs; card reconstruction after TUI restart; Cursor IDE Runtime changes; live harness accounts in tests.
- Auto-enabling Kitty/Sixel/iTerm2; calling image APIs outside harness contracts.

## Decisions

### D1 — Package placement (ADR-077)
Keep `Attachment`, `Asset`, `MediaKind`, `MediaCapability` in `internal/harness`. Do not create `internal/media` in this cycle. Asset store and validators may live as `internal/harness` files or a tightly scoped helper package imported by harness/TUI only — never imported by adapters for protocol logic. Adapters and TUI both import harness; they must not import each other.

### D2 — Keymap (Planning validation)
Existing Free Chat Alt bindings: `alt+1`–`alt+7`, `alt+q`, `alt+m`, `alt+enter`, `alt+r`, `alt+i`, plus screen-specific `alt+s`/`alt+u`/`alt+n`. **`alt+a` and `alt+v` are free.** Keep PRD/UI bindings:
- `Alt+A` / `/attach` → file picker
- `Alt+V` / `/attach-clipboard` → native clipboard image capture
Update footer hints in `internal/tui/screens.go`.

### D3 — Validation limits
Use PRD defaults globally: max 5 attachments/turn, 20 MB/file, 4096×4096, 100 MP total/turn, PNG/JPEG/GIF/WebP via magic bytes + header decode. Adapters may apply stricter discovered native limits at admission time; never looser. Reject decompression bombs by aborting decode when pixel budget exceeded.

### D4 — Capability model
`MediaCapability` flags: `image_input_native`, `image_input_file_reference`, `image_output_native`, `image_output_file`, `supported_image_mime_types`, `max_attachment_bytes`. Sources are independent (adapter transport ∩ model). Unknown ⇒ unsupported. Gate before Execute when attachments present; error names harness+model; chips remain (ADR-079). Fallback chain must not silently strip attachments.

### D5 — Session store layout (ADR-080)
Root: `~/.local/share/hero/sessions/<session-id>/assets/` (respect `XDG_DATA_HOME` when set). Dir `0700`, files `0600`. UUID file names; original name metadata only. SHA-256 dedupe within session. JSON manifest correlates id/session/turn/origin/MIME/path. Cleanup sessions older than retention (default 7d) on TUI startup and graceful shutdown. No git paths; no base64/bytes in logs or context docs.

### D6 — TUI state and async I/O (ADR-078, golang-tui)
Model stores attachment/asset references only. Capture, validate, materialize, mosaic, open, copy-path, save, clipboard-read are `tea.Cmd`. `Update`/`View` never decode images. Race tests ensure workers do not mutate shared model state. Free Chat gate: `m.freeChatMode` (and not research/orchestration live). Image-only turns: allow submit with empty text when chips exist. Part order to harness: images then text.

### D7 — Composer / transcript UX
Chips between text rows and status row in `renderConversationInput`. Error chips for validation failures; external-path warning badge. Asset cards as new transcript branch (not text-delta reuse) with keyboard actions Enter/o/c/a/s. Mosaic expands under card; re-render on resize via async Cmd; degrade when color depth < 256.

### D8 — Adapter translation and tool-written assets (ADR-082)

| Adapter | Input | Output | Tool-written detection |
|---|---|---|---|
| Codex (first) | Ordered `turn/start.input` with `localImage` after schema probe | Normalize `imageGeneration`/`imageView` → Asset; repair via ExecutionResult.Assets | Structured items primary; path extraction from fileChange/tool items secondary |
| OpenCode | Prefer `file:` absolute URI; `data:` only if serve host cannot read file | SSE file/image parts → Asset | `message.part.updated` file/image + tool-result parts; optional file.watcher correlation |
| Cursor | File-reference in prompt composition | Explicit fail if model cannot read; never silent drop | Workspace FS watch during turn + stream-json tool path extraction |
| Claude | Native: `--input-format stream-json` structured stdin after fixture spike; else labeled file-reference degraded path | Fail explicitly if neither path available for active model | Path extraction from tool_result/stream + FS watch while on file-ref path |

Deduplicate cards by content hash. Detection must not add observable latency to turn completion.

### D9 — Claude spike gate
Series task: fake-process/fixture proves `--input-format stream-json` image wire format without a real account. Outcomes:
1. Proven → implement native Claude attachments in this cycle.
2. Not proven → implement degraded file-reference path labeled in UX/diagnostics; native deferred.
Never silent-drop; never require live Anthropic login for tests.

### D10 — Advanced inline preview (phase 4)
Kitty / Sixel / iTerm2 OSC 1337 behind config flag, default off. Card remains primary. Best-effort detect + manual override. Implement after mosaic baseline; may ship disabled stubs if protocol risk threatens schedule, but tasks remain in SDD for coverage.

### D11 — Testing
Fixtures and fake processes only. Tiny image fixtures. Golden/invariant tests for mosaic (no huge ANSI snapshots). `go test ./...` including race where async workers touch shared boundaries. Apply `go-engineering` and `golang-tui` skills.

## Risks / Trade-offs

- [Claude native unavailable] → Degraded file-ref path in-cycle; document limitation in diagnostics / context-log.
- [Codex schema variance] → Probe initialize schema; degrade explicitly per item type.
- [FS watch false positives] → Restrict to image MIME/magic, turn time window, content-hash dedupe.
- [Clipboard/SSH] → `/attach <path>` remains primary remote-friendly path; clipboard may fail with actionable error.
- [Alt-screen + raster protocols] → Keep optional protocols off by default; card+mosaic always work.
- [Layout height] → Chip rows must participate in conversation height accounting or transcript scrolls incorrectly.

## Migration Plan

No user data migration. New session directories appear on first Free Chat attachment. Startup cleanup removes expired session dirs. Document registry gains C14 multimodal PRD/UI/ADR entries. No HeroJSON breaking change required for disabled-by-absence capabilities (unknown = unsupported).

## Open Questions

None blocking Planning. Claude spike outcome selects native vs degraded implementation path inside already-specified tasks.
