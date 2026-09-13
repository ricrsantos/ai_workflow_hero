# session-asset-store Specification

## Purpose
TBD - created by archiving change tui-multimodal-images. Update Purpose after archive.

## Requirements

### Requirement: Session assets SHALL materialize under XDG data home with secure modes

Hero SHALL store session assets at `~/.local/share/hero/sessions/<session-id>/assets/` (honoring `XDG_DATA_HOME`), using directory mode `0700` and file mode `0600`, outside git-tracked paths by default (PRD-C14-001 §2.2; ADR-080).

#### Scenario: Materialize creates private paths
- **WHEN** an attachment is accepted
- **THEN** bytes are written under the session assets directory with `0700`/`0600` modes and a hero-generated UUID filename

#### Scenario: Original name is metadata only
- **WHEN** a user attaches `../evil.png` or a path with separators
- **THEN** the write path uses the UUID only and the original name is stored as sanitized metadata

### Requirement: Validation SHALL trust magic bytes and enforce limits

Hero SHALL validate images by magic bytes and header decode, not extension or declared MIME. Limits: max 5 attachments/turn, 20 MB/file, 4096×4096, 100 MP total/turn. Decoding SHALL abort when pixel budget is exceeded. Symlinks SHALL be resolved and canonicalized before read/copy (PRD-C14-001 §2.3).

#### Scenario: Spoofed extension is rejected
- **WHEN** a `.png` file contains non-image bytes
- **THEN** validation fails with an unsupported-format error chip and the file is not materialized as a sendable attachment

#### Scenario: Decompression bomb aborts
- **WHEN** decoding would exceed the pixel budget
- **THEN** validation aborts without completing the decode and returns a size/pixels error

#### Scenario: External path warns
- **WHEN** the resolved path escapes the workspace
- **THEN** the TUI shows an external-path warning and continues only if file-permissions policy allows external paths

### Requirement: Deduplication and manifest SHALL be session-scoped
Hero SHALL SHA-256 dedupe within a session and maintain a JSON manifest correlating asset id, session id, turn id, origin, MIME type, and path. Session directories SHALL use the Hero session ID once a durable session exists. Sessions registered in the durable store SHALL retain their private directory and managed copies until permanent session deletion; age alone MUST NOT delete them. Startup cleanup MAY remove only directories with no matching `sessions` row (orphans) and non-durable legacy temporary sessions older than seven days. Original user source paths SHALL never be unlinked. Cards for durable sessions SHALL be reconstructable from `session_assets` metadata after restart (PRD-C14-001 §§2.2,2.14; PRD-C16-001 §3.3, §3.7; ADR-080 as amended by ADR-096).

#### Scenario: Same bytes reuse path
- **WHEN** two attachments in one session share content hash
- **THEN** the store reuses the materialized blob and distinct metadata ids still correlate in the manifest

#### Scenario: Expired sessions are removed
- **WHEN** startup finds an unregistered session directory older than retention (no matching `sessions` row)
- **THEN** that directory is deleted and registered durable sessions remain

#### Scenario: Registered durable sessions survive age cleanup
- **WHEN** startup cleanup runs and a registered session directory is older than seven days
- **THEN** that directory is retained

#### Scenario: Orphans and unregistered temp dirs still expire
- **WHEN** startup finds a session directory with no `sessions` row and directory mtime older than retention
- **THEN** that directory is deleted

#### Scenario: Original files survive session delete
- **WHEN** a session with a managed copy and an `external_source` path is permanently deleted
- **THEN** only the managed copy is removed and the original path remains

### Requirement: Sensitive image data SHALL NOT leak into logs or docs

Hero SHALL NEVER write base64, image bytes, or sensitive attachment paths to logs, `context/context-log.md`, `context/current-state.md`, or git (PRD-C14-001 §2.3).

#### Scenario: Error logs omit bytes
- **WHEN** validation or adapter translation fails
- **THEN** diagnostics include safe identifiers/MIME/size only, not file bytes or base64
