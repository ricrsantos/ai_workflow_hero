# ADR-C14-002 — Multimodal Image Support for Hero TUI

> Cycle C14 decisions. Product: [PRD-C14-001](../product/PRD-C14-001-tui-multimodal-images.md). UI: [UI-C14-001](../product/UI-C14-001-tui-multimodal-images.md).

| # | Decision | Status |
|---|---|---|
| ADR-077 | Common multimodal contract lives in `internal/harness`; promoted to `internal/media` only if justified | Accepted |
| ADR-078 | Immutable asset references in Bubble Tea model — image bytes never stored in model state | Accepted |
| ADR-079 | Explicit capability failure — blocked turn with actionable error when model does not support images | Accepted |
| ADR-080 | Temporary assets stored in `~/.local/share/hero/sessions/<id>/assets/`; session-scoped only | Accepted |
| ADR-081 | Inline image preview (mosaic and terminal protocols) is discontinued in the current TUI | Superseded 2026-09-15 |
| ADR-082 | Any image file written by a tool during a turn is exposed as a model output asset card | Accepted |

---

## ADR-077: Common multimodal contract lives in `internal/harness`

**Context:** The TUI, all adapters, and the conversation engine need shared types for attachments and assets. A premature standalone package introduces coupling; a hasty placement inside a single adapter leaks cross-cutting concern.

**Decision:** `Attachment`, `Asset`, `MediaKind`, and `MediaCapability` are defined in `internal/harness` in the first cycle. If clipboard transport, Telegram, or other non-adapter consumers accumulate enough shared operations to justify cohesion, a small focused `internal/media` package may be extracted with a dedicated ADR. The TUI and adapters must not import each other; both import `internal/harness`.

**Consequences:** Adapters translate the shared contract to native protocol internally. The TUI never builds provider payloads. Cross-package surface area is minimal for the first increment. This extends ADR-031 (adapter vertical slice isolation).

---

## ADR-078: Immutable asset references in Bubble Tea model — image bytes never stored in model state

**Context:** Bubble Tea propagates the model on every `Update` call. Storing multi-megabyte images or base64 strings in the model causes expensive copies, pollutes `View()` hot paths, and inflates memory during scroll and redraw.

**Decision:** Image bytes are materialized exactly once to the session asset directory. The Bubble Tea model carries only an `Attachment` or `Asset` reference (UUID, path, metadata). Downstream file open, clipboard copy, and save operations use the reference through async `tea.Cmd`; the TUI does not decode image pixels.

**Consequences:** The model remains small and allocation-cheap across updates. Async workers own I/O; they may not mutate shared model state (enforced by race tests). This follows the pattern established by the existing async turn-stream handling.

---

## ADR-079: Explicit capability failure — blocked turn with actionable error when model does not support images

**Context:** Silently dropping an attachment or converting it to a fabricated text description misleads the user and corrupts the conversation semantic. The original fail-closed rule also produced false negatives when a harness could carry images but its model/schema discovery was absent or stale.

**Decision:** Before submitting a turn that contains one or more attachments, Hero checks the active harness+model capability. A known unsupported intersection, or an unknown harness transport, blocks the turn with an actionable error identifying the harness and model. If the transport is known to carry images but model-side capability is unknown, Hero admits the request optimistically, sends the unchanged attachments in one logical Execute, and treats an explicit provider rejection as the final result. The TUI does not invoke fallback or automatic retry; existing adapter-level connection recovery remains governed by the adapter contract. Attachment chips remain after either a local block or provider rejection and are cleared only after successful Execute.

**Consequences:** Users receive clear feedback without false-negative blocks caused by stale discovery. No silent data loss or hidden fallback is possible. Transport capabilities remain mandatory, while model discovery/catalog improves diagnostics and can still explicitly block unsupported models. Cursor/Claude file-reference paths remain labeled as degraded where native image interpretation is not proven.

---

## ADR-080: Temporary assets stored in `~/.local/share/hero/sessions/<id>/assets/`; session-scoped only

**Context:** The idea note (§11) specifies a private session directory outside version-controlled paths. Two location candidates were considered: `$XDG_RUNTIME_DIR/hero/...` (may be unavailable or size-limited on some systems) and `~/.local/share/hero/sessions/...` (stable, writable, XDG data home compliant).

**Decision:** Assets are stored at `~/.local/share/hero/sessions/<session-id>/assets/` with directory mode `0700` and file mode `0600`. The session manifest (a small JSON file in the session directory) correlates asset ID, session ID, turn ID, origin, MIME type, and path. Hero cleans up session directories older than 7 days on startup and graceful shutdown. Blobs are never stored in SQLite. Cards are not reconstructed after a TUI restart.

**Consequences:** Storage is predictable and size-bounded. Cleanup is deterministic. The approach is consistent with XDG Base Directory Specification. Future persistence of cards after restart requires a separate ADR and manifest upgrade.

---

## ADR-081: Inline image preview is discontinued in the current TUI

**Context:** The original C14 design listed three visualization tiers: card + external viewer, Unicode mosaic, and optional raster protocols. Inline rendering added layout, terminal-compatibility, and focus complexity without improving the primary image workflow.

**Decision:** The current TUI uses text-only asset cards and composer chips. `Enter/o` opens the materialized image in the system viewer; `c` copies its path; `s` saves/exports it; output cards additionally support `a` to attach, while composer chips support `x/Delete` to remove. No inline pixel, Unicode mosaic, Kitty, Sixel, or iTerm2 rendering is exposed.

All file, clipboard, and external-process operations remain asynchronous from the TUI model's perspective. The card/chip metadata is the accessible representation on every terminal, including SSH and multiplexers.

**Consequences:** The media package has no terminal-preview dependency or image decode in the TUI path. The simpler text surface avoids preview-specific resize and alt-screen behavior while preserving direct access to every materialized image.

---

## ADR-082: Any image file written by a tool during a turn is exposed as a model output asset card

**Context:** A harness may instruct a tool to write an image to disk. The resulting file is semantically a model output (the model requested its creation), but the harness may not explicitly flag it as an asset in the stream. The grilling session required automatic card exposure without adapter-specific opt-in.

**Decision:** All adapters must detect image files written during a turn — via file-system watch, tool-result path extraction, or equivalent mechanism appropriate to the adapter's protocol — and normalize each such file as an `Asset` with `Source: "tool"`, associating it with the turn ID. The asset is materialized to the session directory (deduplication applies), and a card is shown in the transcript after the turn completes.

The planning agent must specify the per-adapter detection mechanism during the Planning stage, since the appropriate approach differs between Codex (structured `imageGeneration`/`imageView` items), OpenCode (tool-result file parts), and file-reference adapters (watch-based heuristic).

**Consequences:** Users always see image outputs regardless of how they were produced. Adapter implementations must not emit duplicate cards for the same file (deduplication by content hash). The detection mechanism must not introduce observable latency to the turn completion path.
