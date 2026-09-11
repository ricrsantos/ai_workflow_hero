# C14 Judge loop-back — implementation gaps (iteration 1)

Source: Judge (`judge_agent`, `cursor-grok-4.6-high`) on `openspec/changes/tui-multimodal-images`.
`all_requirements_met: false`. No SDD ambiguity. Re-run `generic_agent` (native scope).

SDD: `openspec/changes/tui-multimodal-images/` (proposal, design, 7 specs, 16 task groups).
Authoritative product docs: `docs/product/PRD-C14-001-tui-multimodal-images.md`,
`docs/architecture/ADR-C14-002-tui-multimodal-images.md`,
`docs/product/UI-C14-001-tui-multimodal-images.md`.

Do not reimplement landed contract/store/adapter translation helpers. Keep and wire them.

## What already landed (keep)

- Shared `harness.Attachment` / `Asset` / `MediaKind` / `MediaCapability`, `StreamKindAsset`, `conversation.Input.Attachments`, image-only submit, `RepairAssets` / content-hash merge.
- Session store under `XDG_DATA_HOME`/`~/.local/share/hero/sessions/<id>/assets/` with UUID filenames, `0700`/`0600`, SHA-256 dedupe, JSON manifest, magic-byte/header limits, symlink canonicalize, external-path warning, retention cleanup on TUI start/shutdown.
- `internal/media` capability registry + Admit helpers with tests (unknown = unsupported). Production TUI does **not** use them yet.
- Free Chat-only Alt+A/Alt+V, `/attach`, `/attach-clipboard`, `/attach <path>`, Bubbles picker, native clipboard PNG read (not OSC 52), `msg.Paste` path offer, chips outside the text field, independent dismiss, image-only Alt+Enter send, chip height in composer chrome.
- Transcript asset cards with Enter/o/c/a/s, `tea.ExecProcess` open, save dialog with overwrite confirm, `[tool wrote]` source label, session-ephemeral cards (new `mediaSessionID` per TUI process; no manifest rebuild).
- Async Unicode mosaic as `tea.Cmd` + low-color copy; mosaic is not decoded in `View()`.
- Adapter translation helpers: Codex `localImage` + `imageGeneration`/`imageView`; OpenCode `file:`/`data:`; Cursor file-reference; Claude spike then labeled degraded file-reference; tool-written detection + hash dedupe.
- Optional Kitty/Sixel/iTerm2 library (`internal/media/protocol_preview.go`) default-off with fixture tests. TUI does not call it (acceptable under design D10 stubs, unless preview wiring is touched).
- Docs/registry indexes, TESTING.md C14 note, architecture overview multimodal diagram.

## Required work

1. **task-03.1 / task-03.2 / task-05.1 / media-capability** — Wire the capability registry into the live Free Chat execute path.
   - Construct and keep a `media.Registry` on the TUI model (today `mediaRegistry` stays nil; `Admit` is skipped).
   - Register adapter transport flags and model discovery/catalog facts (unknown remains unsupported).
   - Call Admit **before** Execute when attachments are present; keep chips; name harness+model; do not invoke fallback or strip attachments.
   - Populate each adapter's `MediaCapability` from the admitted intersection. Today the field is never assigned, so OpenCode/Cursor/Claude attachment Execute always fail-closed even for capable models, and Codex skips model-side intersection (schema probe only).

2. **task-11.2 / unicode-mosaic-preview** — On `tea.WindowSizeMsg`, if a mosaic is expanded, re-issue `media.MosaicCmd` with the new pane width/height. Do not block `Update`/`View`. Truncating existing ANSI lines is not a re-render.

3. **hero-tui progress states** — While `assetMosaicPending` is true, the focused/expanded card must show a non-blocking spinner (UI-C14-001 §6; spec scenario "Mosaic spinner is async"). Clipboard/validation spinner chips already exist.

## Unmet spec requirements

- media-capability: capabilities SHALL combine adapter transport ∩ model sources; unsupported attachment turns SHALL fail closed before Execute with an actionable harness+model error and chips retained.
- harness-adapter: adapters SHALL translate admitted attachments without silent loss. Zero `Adapter.MediaCapability` makes OpenCode/Cursor/Claude refuse every image turn in production.
- unicode-mosaic-preview: mosaic SHALL re-render on resize asynchronously.
- hero-tui: mosaic-in-progress SHALL show a spinner without blocking keys.

Tests: fixtures/fakes only; no live provider accounts; no image bytes/base64 in logs or context docs.
