# ADR-C14-002 — Multimodal Image Support for Hero TUI

> Cycle C14 decisions. Product: [PRD-C14-001](../product/PRD-C14-001-tui-multimodal-images.md). UI: [UI-C14-001](../product/UI-C14-001-tui-multimodal-images.md).

| # | Decision | Status |
|---|---|---|
| ADR-077 | Common multimodal contract lives in `internal/harness`; promoted to `internal/media` only if justified | Accepted |
| ADR-078 | Immutable asset references in Bubble Tea model — image bytes never stored in model state | Accepted |
| ADR-079 | Explicit capability failure — blocked turn with actionable error when model does not support images | Accepted |
| ADR-080 | Temporary assets stored in `~/.local/share/hero/sessions/<id>/assets/`; session-scoped only | Accepted |
| ADR-081 | Unicode mosaic is the mandatory preview baseline; Kitty/Sixel/iTerm2 are optional and off by default | Accepted |
| ADR-082 | Any image file written by a tool during a turn is exposed as a model output asset card | Accepted |

---

## ADR-077: Common multimodal contract lives in `internal/harness`

**Context:** The TUI, all adapters, and the conversation engine need shared types for attachments and assets. A premature standalone package introduces coupling; a hasty placement inside a single adapter leaks cross-cutting concern.

**Decision:** `Attachment`, `Asset`, `MediaKind`, and `MediaCapability` are defined in `internal/harness` in the first cycle. If clipboard transport, Telegram, or other non-adapter consumers accumulate enough shared operations to justify cohesion, a small focused `internal/media` package may be extracted with a dedicated ADR. The TUI and adapters must not import each other; both import `internal/harness`.

**Consequences:** Adapters translate the shared contract to native protocol internally. The TUI never builds provider payloads. Cross-package surface area is minimal for the first increment. This extends ADR-031 (adapter vertical slice isolation).

---

## ADR-078: Immutable asset references in Bubble Tea model — image bytes never stored in model state

**Context:** Bubble Tea propagates the model on every `Update` call. Storing multi-megabyte images or base64 strings in the model causes expensive copies, pollutes `View()` hot paths, and inflates memory during scroll and redraw.

**Decision:** Image bytes are materialized exactly once to the session asset directory. The Bubble Tea model carries only an `Attachment` or `Asset` reference (UUID, path, metadata). All downstream operations (thumbnail generation, clipboard copy, file open, mosaic render) use the reference to read from disk via async `tea.Cmd`.

**Consequences:** The model remains small and allocation-cheap across updates. Async workers own I/O; they may not mutate shared model state (enforced by race tests). This follows the pattern established by the existing async turn-stream handling.

---

## ADR-079: Explicit capability failure — blocked turn with actionable error when model does not support images

**Context:** Silently dropping an attachment or converting it to a fabricated text description misleads the user and corrupts the conversation semantic. The idea note (§4.3) requires closed failure.

**Decision:** Before submitting a turn that contains one or more attachments, Hero checks the active harness+model capability. If neither `image_input_native` nor `image_input_file_reference` is available (or if the capability is unknown), the turn is blocked and an actionable error is displayed identifying the harness and model. The attachment chips remain in the composer; the user may remove them or switch to a capable model. The fallback chain is not invoked silently; any fallback path must be documented explicitly in a future PRD.

**Consequences:** Users receive clear feedback. No silent data loss. The capability registry must be populated for every supported harness/model combination and conservatively default to "unsupported" for unknowns.

---

## ADR-080: Temporary assets stored in `~/.local/share/hero/sessions/<id>/assets/`; session-scoped only

**Context:** The idea note (§11) specifies a private session directory outside version-controlled paths. Two location candidates were considered: `$XDG_RUNTIME_DIR/hero/...` (may be unavailable or size-limited on some systems) and `~/.local/share/hero/sessions/...` (stable, writable, XDG data home compliant).

**Decision:** Assets are stored at `~/.local/share/hero/sessions/<session-id>/assets/` with directory mode `0700` and file mode `0600`. The session manifest (a small JSON file in the session directory) correlates asset ID, session ID, turn ID, origin, MIME type, and path. Hero cleans up session directories older than 7 days on startup and graceful shutdown. Blobs are never stored in SQLite. Cards are not reconstructed after a TUI restart.

**Consequences:** Storage is predictable and size-bounded. Cleanup is deterministic. The approach is consistent with XDG Base Directory Specification. Future persistence of cards after restart requires a separate ADR and manifest upgrade.

---

## ADR-081: Unicode mosaic is the mandatory preview baseline; Kitty/Sixel/iTerm2 are optional and off by default

**Context:** The original idea note (§8) listed three visualization tiers: card + external viewer (mandatory base), Unicode mosaic (universal), and raster protocols (optional). The grilling session elevated Unicode mosaic to mandatory from phase 1.

**Decision:** Every image asset card in the transcript must offer a Unicode block-character + ANSI-color mosaic preview. The mosaic is computed asynchronously as a `tea.Cmd` and is never calculated inside `View()`. It degrades to the card-only display on terminals without 256+ color support. The card (with text actions) remains present whether or not the mosaic is rendered.

Kitty Graphics Protocol, Sixel, and iTerm2 OSC 1337 inline rendering are implemented in phase 4 but are disabled by default. Enabling them requires explicit user or configuration opt-in. Protocol detection is best-effort; a config flag allows manual override. These protocols interact adversarially with alt-screen, resize, and scroll; the card must remain the primary navigation surface even when inline rendering is active.

**Consequences:** Every terminal environment, including SSH and multiplexers, gets a functional and usable preview. Raster protocols add zero regression risk to the base path. The mosaic renderer must have golden-file or invariant tests; large ANSI sequence snapshots are prohibited.

---

## ADR-082: Any image file written by a tool during a turn is exposed as a model output asset card

**Context:** A harness may instruct a tool to write an image to disk. The resulting file is semantically a model output (the model requested its creation), but the harness may not explicitly flag it as an asset in the stream. The grilling session required automatic card exposure without adapter-specific opt-in.

**Decision:** All adapters must detect image files written during a turn — via file-system watch, tool-result path extraction, or equivalent mechanism appropriate to the adapter's protocol — and normalize each such file as an `Asset` with `Source: "tool"`, associating it with the turn ID. The asset is materialized to the session directory (deduplication applies), and a card is shown in the transcript after the turn completes.

The planning agent must specify the per-adapter detection mechanism during the Planning stage, since the appropriate approach differs between Codex (structured `imageGeneration`/`imageView` items), OpenCode (tool-result file parts), and file-reference adapters (watch-based heuristic).

**Consequences:** Users always see image outputs regardless of how they were produced. Adapter implementations must not emit duplicate cards for the same file (deduplication by content hash). The detection mechanism must not introduce observable latency to the turn completion path.
