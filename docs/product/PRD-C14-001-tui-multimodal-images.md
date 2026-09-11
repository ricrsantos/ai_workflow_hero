# PRD-C14-001 — Multimodal Image Support for Hero TUI

> Cycle C14 requirements. Adds bidirectional image support to the Hero TUI Free Chat. Design input: [idea note](../idea/v3.3_images/tui-imagens-multimodais.md) (non-normative). Architecture decisions: [ADR-C14-002](../architecture/ADR-C14-002-tui-multimodal-images.md). UI spec: [UI-C14-001](UI-C14-001-tui-multimodal-images.md).

## 1. Outcome

The Hero TUI Free Chat supports attaching PNG, JPEG, GIF, and WebP images to a message turn and receiving image assets produced by a harness. Attachments flow through a shared multimodal contract; each harness adapter translates to its native protocol. The TUI remains responsive and terminal-portable at all times. The feature is exclusive to Free Chat — Research and workflow-stage sessions are out of scope.

Target platforms: Linux and macOS on amd64/arm64. Implementation agents must apply the `go-engineering` and `golang-tui` project skills throughout.

## 2. Functional requirements

### 2.1 Common multimodal contract

- Add shared types `Attachment`, `Asset`, `MediaKind`, and `MediaCapability` to `internal/harness` (exact package location per ADR-077).
- Extend `conversation.Input` with `Attachments []Attachment`.
- Extend `harness.ExecuteRequest` with `Attachments []Attachment`.
- Extend `harness.ExecutionResult` with `Assets []Asset`.
- Extend `harness.StreamDelta` with `Asset *Asset` and a new `StreamKindAsset` constant.
- `Asset` carries: `Attachment` embed, `Source` (`user|model|tool`), `SessionID`, `TurnID`, and `Saved bool`.
- `Attachment` carries: `ID` (hero-generated), `Kind`, `Name` (original, metadata only), `MIMEType`, `Path` (materialized local path), `Size`, `Width`, `Height`.
- The `StreamKindAsset` delta and `ExecutionResult.Assets` carry the same asset; adapters use the final result to repair partial stream loss, consistent with the existing text repair pattern.

### 2.2 Temporary asset service

- Store session assets at `~/.local/share/hero/sessions/<session-id>/assets/` with directory mode `0700` and file mode `0600`.
- Directories must be outside any git-tracked path by default.
- Assign a hero-generated UUID identifier; the original file name is stored as metadata only and never used as a write path.
- Apply a SHA-256 content hash for deduplication within a session.
- Maintain a session manifest (JSON file in the session directory) correlating asset ID, session ID, turn ID, origin, MIME type, and path.
- Clean up session directories on TUI startup (remove sessions older than a configurable retention period, default 7 days) and on graceful shutdown.

### 2.3 Validation and security

- Validate image files by reading magic bytes and decoding the image header; do not trust file extension or declared MIME type.
- Enforce per-turn limits: maximum 5 attachments, 20 MB per file, 4096×4096 pixels, 100 MP total per turn. Planning agent may tighten per adapter.
- Resolve all symlinks and canonicalize paths before any read or copy operation.
- If the resolved path escapes the workspace, display a warning in the TUI before proceeding and require the file-permissions policy to allow external paths.
- Reject decompression bombs: abort decoding if the decoded pixel count exceeds the limit.
- Never write base64, image bytes, or sensitive paths to logs, `context-log.md`, `current-state.md`, or git.
- Sanitize the original file name; never use it as a write path.

### 2.4 Capability registry

- Define at minimum these capability flags per harness/model: `image_input_native`, `image_input_file_reference`, `image_output_native`, `image_output_file`, `supported_image_mime_types []string`, `max_attachment_bytes int64`.
- Capabilities have two independent sources: adapter transport and native model (via discovery or catalog).
- Absence of information is not equivalent to support. If capability is unknown, treat as unsupported.
- When a turn is submitted with one or more attachments but the active model lacks `image_input_native` or `image_input_file_reference`, block the turn before sending and display a clear error with the unsupported model/harness identified. Never silently remove the attachment.

### 2.5 Input: attachment UX

- `Alt+A` or `/attach` opens a file picker; `Alt+A` shortcut must be validated against the existing keymap and reassigned if conflicting (planning validates this).
- `Alt+V` or `/attach-clipboard` reads an image from the system clipboard using the native OS clipboard API (not OSC 52).
- When bracketed paste delivers a filesystem path string to the composer, the TUI recognizes the pattern and offers to attach it as an image.
- `/attach <path>` accepts an explicit path, useful in SSH and terminal multiplexer environments.
- After capture and validation, a chip appears in the composer area outside the text input field. The chip displays: file name, MIME type, dimensions, size, and a close button.
- The text cursor does not traverse chip content. Backspace in the text field does not remove an attachment; removal requires explicit focus on the chip and a dedicated key or the close button.
- A turn may consist of images only, without any text from the user.
- Order of parts sent to the harness: images first, then text.

### 2.6 Output: asset cards

- When a harness produces image assets (via stream delta or final result), associate each asset with the turn that produced it.
- Display an asset card in the transcript for each image asset:

```
Image generated
mockup-home.png · PNG · 1536x1024 · 1.8 MB
enter preview · o open · c copy path · a attach · s save
```

- Card actions:
  - **Enter / preview**: display Unicode mosaic preview inline (mandatory, see §2.7).
  - **o**: open the image in the system default viewer (`xdg-open` on Linux, `open` on macOS). Suspend and restore the TUI correctly around the external process.
  - **c**: copy the local asset path to the clipboard.
  - **s**: save/export the image to a user-chosen destination path.
  - **a**: attach the asset as a new attachment to the next composer turn.
- All I/O operations (read, decode, thumbnail generation, file write) must be `tea.Cmd`; they must not block `Update()` or execute in `View()`.

### 2.7 Unicode mosaic preview

- Implement a Unicode block-character + ANSI color mosaic renderer for image preview.
- The renderer must operate outside `View()` as an async `tea.Cmd`.
- The mosaic must respect the available pane width and height, degrade to the card on terminals without 256+ color support, and not produce layout-breaking output on resize.
- Use golden-file or invariant tests; avoid large ANSI sequence snapshots.

### 2.8 Advanced inline preview (optional, phase 4)

- Add opt-in Kitty Graphics Protocol, Sixel, and iTerm2 OSC 1337 inline image rendering.
- Detection is best-effort; manual override is allowed via a config flag.
- The card remains mandatory even when inline rendering is active.
- Disable by default; never enable automatically without user or configuration opt-in.

### 2.9 Adapter: Codex (phase 2, first adapter)

- Detect `localImage` and `image` input capability and `imageView`/`imageGeneration` output items from the installed App Server schema.
- Input: convert each validated local attachment to a `localImage` entry in the ordered `turn/start.input` list.
- Output: normalize `imageGeneration` and `imageView` items to `Asset`, preserving `savedPath`, status, revised prompt, and safe error when available.
- Do not assume every installed Codex version exposes the same schema items; probe and degrade explicitly.

### 2.10 Adapter: OpenCode (phase 2, second adapter)

- Input: prefer `file:` absolute URI for materialized attachments; use `data:` URI only when the file is not accessible at the OpenCode serve host.
- Output: extend the SSE normalizer to preserve file/image parts and tool-result image assets.
- Validate OpenCode's own provider/model limits before or during prompt admission.

### 2.11 Adapter: Claude (phase 3)

- Native attachment path: implement via `--input-format stream-json` structured stdin. Develop a fixture/fake-process spike to verify the wire format before altering the adapter. The spike must be reproducible without a real account.
- Temporary path: reference the materialized local file and add an explicit instruction for the harness to read it. This path is labeled as a degraded fallback, not native attachment. The planning agent decides whether the temporary path is in scope for this cycle.
- Fail explicitly if the native path is not available for the active model.

### 2.12 Adapter: Cursor (phase 2, file-reference fallback)

- Reference the materialized local file path in the prompt composition.
- If the active model/harness cannot read the image, fail explicitly with a diagnostic before the turn is sent. Never remove the attachment silently.

### 2.13 Tool-generated asset detection

- Any image file written to disk during a turn (by a tool call or file-write operation emitted by the harness) is treated as a model output asset and exposed as an asset card in the transcript.
- This detection applies to all adapters.
- The planning agent defines the per-adapter mechanism (file-system watch, path extraction from tool results, etc.).

### 2.14 Card persistence

- Asset cards exist only during the active TUI session. They are not reconstructed after a TUI restart.
- The session manifest (§2.2) is retained on disk for the configured retention period; assets referenced by the manifest remain accessible by path after restart but no card is automatically shown.

## 3. Constraints and quality

- Apply the `go-engineering` and `golang-tui` project skills during implementation, code review, and refactoring.
- Preserve the vertical-slice feature architecture: multimodal contract lives in `internal/harness` or `internal/media` (if justified); attachment validation, asset storage, and preview rendering do not leak into adapters; adapter protocol translation does not leak into TUI or core.
- All async I/O (capture, validation, thumbnail, file operations) runs as `tea.Cmd`; `Update()` and `View()` remain non-blocking.
- `go test ./...` must pass before any task is considered complete.
- No real harness account, API key, or real image generation service is required by tests. Use fixtures and fake processes.
- Race tests must verify that async workers do not mutate shared TUI model state.

## 4. Out of scope

- Image support outside Free Chat (Research, planning, implementation, QA agents, or workflow-stage sessions).
- Audio, video, PDF, or arbitrary binary attachments.
- Calling image generation APIs outside the harness contract.
- Automatic persistence of attachments or assets to git or cycle documents.
- Image transport over Telegram in this cycle.
- Attaching images to `/hero-*` control commands.
- Windows support.
- Session card reconstruction after TUI restart.
- A second authentication layer, billing, or provider-specific session management.
- Claude native adapter if the fixture spike is not reproducible without a real account (falls back to file-reference degraded path or deferred to a sub-cycle).

## 5. Acceptance criteria

1. `Alt+A` opens a file picker in Free Chat; the user selects a PNG; a chip appears in the composer; on send, the attachment reaches Codex natively and the response is displayed normally.
2. A Codex image generation result appears as an asset card with a Unicode mosaic preview in the transcript.
3. `Alt+V` reads an image from the system clipboard; a chip appears in the composer.
4. When the active model lacks image capability, submitting a turn with an attachment blocks with a clear error identifying the model and harness; no attachment is silently dropped.
5. An image file written by a tool during a turn appears as an asset card.
6. All adapter fixture tests pass without a real account or API call.
7. `go test ./...` passes with no race conditions.
