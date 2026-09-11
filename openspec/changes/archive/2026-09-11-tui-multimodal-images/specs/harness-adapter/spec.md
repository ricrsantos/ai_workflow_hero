## ADDED Requirements

### Requirement: Adapters SHALL translate shared attachments without silent loss

Every Free Chat adapter (Codex, OpenCode, Cursor, Claude) SHALL accept `ExecuteRequest.Attachments` and translate them to the harness-native input form after capability admission. If translation or native support is unavailable, the adapter SHALL fail with an actionable diagnostic naming harness and model. Adapters SHALL NEVER silently drop attachments (PRD-C14-001 §§2.9–2.12; ADR-079).

#### Scenario: Codex native localImage
- **WHEN** Codex schema probing exposes `localImage` and attachments are present
- **THEN** each attachment is sent as a `localImage` entry in ordered `turn/start.input` before text

#### Scenario: OpenCode prefers file URI
- **WHEN** OpenCode can read the materialized absolute path
- **THEN** the prompt parts use a `file:` URI rather than inlining `data:` bytes

#### Scenario: Cursor file-reference failure is explicit
- **WHEN** Cursor cannot read an attached image for the active model
- **THEN** Execute fails before/at admission with a diagnostic and the composer chips remain

#### Scenario: Claude degraded path is labeled
- **WHEN** the Claude native stream-json input spike is not proven and file-reference fallback is used
- **THEN** diagnostics/UX label the path as degraded file-reference rather than native attachment

### Requirement: Adapters SHALL normalize image outputs to Asset

Adapters SHALL map native image outputs to `Asset` values, emit `StreamKindAsset` when streamed, and include the same assets in `ExecutionResult.Assets` for repair. Schema variance SHALL be probed and degraded explicitly (PRD-C14-001 §§2.1,2.9–2.10).

#### Scenario: Codex imageGeneration becomes asset
- **WHEN** Codex emits `imageGeneration` or `imageView` items with a saved path
- **THEN** the adapter emits a normalized Asset and a stream asset delta when available

#### Scenario: OpenCode SSE image parts become assets
- **WHEN** OpenCode SSE includes file/image parts or tool-result image assets
- **THEN** those parts are normalized to Asset with safe metadata and session materialization

### Requirement: Tool-written images SHALL become tool-sourced assets

During a turn, adapters SHALL detect image files written by tools and expose them as `Source: tool` assets without duplicate cards for the same content hash. Detection mechanisms: Codex structured items plus path extraction; OpenCode SSE/tool-result parts; Cursor/Claude file-reference adapters via turn-scoped workspace watch and tool-result path extraction (PRD-C14-001 §2.13; ADR-082).

#### Scenario: Tool-written PNG appears once
- **WHEN** a tool writes `design-export.png` during a turn and the same bytes are also mentioned in a tool result
- **THEN** Free Chat shows a single tool-sourced asset card after the turn’s text output

#### Scenario: Detection does not stall completion
- **WHEN** tool-written detection runs
- **THEN** turn completion is not observably delayed by unbounded watching
