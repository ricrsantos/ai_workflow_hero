## Why

Hero TUI Free Chat is text-only today: users cannot attach screenshots or mockups to a turn, and harness-produced images never surface as first-class transcript assets. Cycle C14 adds a shared multimodal contract, session-scoped secure asset storage, capability-gated admission, Free Chat attachment UX, Unicode mosaic preview, and per-adapter protocol translation so Codex, OpenCode, Cursor, and Claude can send and receive images without leaking provider payloads into the TUI (PRD-C14-001 §1; ADR-077–082).

## What Changes

- Add shared `Attachment`, `Asset`, `MediaKind`, and `MediaCapability` types in `internal/harness` and extend `conversation.Input`, `harness.ExecuteRequest`, `harness.ExecutionResult`, and `harness.StreamDelta` (`StreamKindAsset`) so attachments and assets flow through the normalized boundary (PRD-C14-001 §2.1; ADR-077).
- Add a session asset service under `~/.local/share/hero/sessions/<session-id>/assets/` with UUID paths, SHA-256 dedupe, JSON manifest, 0700/0600 modes, retention cleanup (default 7 days), magic-byte validation, symlink canonicalization, and external-path warnings (PRD-C14-001 §§2.2–2.3; ADR-078/080).
- Add a media capability registry combining adapter transport flags and model discovery; unknown means unsupported; turns with attachments fail closed with an actionable error naming harness+model (PRD-C14-001 §2.4; ADR-079).
- Extend Free Chat composer UX only: `Alt+A` / `/attach` file picker, `Alt+V` / `/attach-clipboard` native clipboard capture, `/attach <path>`, bracketed-paste path offer, chips outside the text field, image-only turns, images-then-text ordering (PRD-C14-001 §2.5; UI-C14-001 §§2,4,7).
- Render transcript asset cards with async Unicode mosaic preview and actions open/copy/save/attach; optional Kitty/Sixel/iTerm2 remain off by default (PRD-C14-001 §§2.6–2.8; ADR-081; UI-C14-001 §§3,5–6,8).
- Translate the shared contract in adapters: Codex native `localImage` + `imageGeneration`/`imageView`; OpenCode `file:`/`data:` parts + SSE image parts; Cursor file-reference with explicit failure; Claude native `--input-format stream-json` after a fixture spike, else labeled file-reference degraded path (PRD-C14-001 §§2.9–2.12).
- Detect tool-written image files during a turn and expose them as `Source: tool` asset cards with content-hash dedupe (PRD-C14-001 §2.13; ADR-082).
- Keep cards session-ephemeral (no reconstruct after TUI restart); never log base64/bytes/sensitive paths; no Telegram image transport; no workflow-stage or Research image support (PRD-C14-001 §§2.14,4).

## Capabilities

### New Capabilities

- `multimodal-contract`: shared Attachment/Asset/MediaKind/MediaCapability types and request/result/stream extensions.
- `session-asset-store`: materialization, validation, dedupe, manifest, retention, and secure path handling.
- `media-capability`: harness/model image capability registry and fail-closed turn admission.
- `unicode-mosaic-preview`: mandatory async Unicode block+ANSI mosaic renderer and degradation rules.
- `advanced-inline-preview`: optional Kitty/Sixel/iTerm2 inline rendering (disabled by default).

### Modified Capabilities

- `harness-adapter`: adapters accept attachments, emit assets/stream deltas, probe/degrade image schema, and detect tool-written images without silent attachment loss.
- `hero-tui`: Free Chat attachment chips, slash/key bindings, file picker, clipboard capture, capability/validation errors, asset cards, and card actions.

## Impact

- Packages: `internal/harness`, `internal/conversation`, `internal/tui`, `internal/adapters/{codex,opencode,cursor,claude}`; optional small helpers colocated with harness for clipboard/FS watch injection.
- Document registry: register C14 PRD/UI/ADR multimodal docs in `.workflow-hero/config/documents.json`; update indexes (PRD.md, UI.md, ADR.md), architecture overview, testing notes, context files.
- No SQLite blob storage; no new daemon; no Cursor IDE Runtime change; no Telegram multimodal; tests use fixtures/fake processes and tiny PNG/JPEG fixtures only.
- Scope is native → all implementation tasks owned by `generic_agent`. Apply `go-engineering` and `golang-tui` skills; finish with `go test ./...` green.
