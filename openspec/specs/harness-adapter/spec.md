# harness-adapter Specification

## Purpose
TBD - created by archiving change slash-parity-tui-harness. Update Purpose after archive.

## Requirements

### Requirement: Dispatch SHALL accept expanded command markdown as prompt
`HarnessAdapter.Dispatch` SHALL accept a `Prompt` that may be the full body of a Cursor custom command markdown file (after optional frontmatter strip). Cursor adapter behavior remains best-effort; IDE chat injection remains out of scope (ADR-016 as amended by ADR-021; PRD-C02-001 §5.2).

#### Scenario: Dispatch with markdown body
- **WHEN** a caller dispatches with `Prompt` set to a command markdown body and `ProjectDir` set
- **THEN** the adapter uses that prompt text for the dispatch attempt (or returns a non-silent unavailable result)

#### Scenario: Unavailable dispatch returns actionable message
- **WHEN** dispatch cannot run
- **THEN** `Dispatched` is false and `Message` tells the user to run the command in Cursor chat

### Requirement: Harness execution SHALL accept normalized model properties without changing model listing compatibility

`HarnessAdapter.Execute` SHALL continue to accept the existing prompt/session/model fields, and its normalized request SHALL additionally carry string-valued properties keyed by C5 `fs`, `th`, and `ef`. `ListModels` SHALL remain a separate optional contract. The TUI SHALL never pass Cursor/OpenCode-native option names or response objects through the shared boundary (PRD-C05-001 §4.2.1–3; §4.5.1/4; ADR-038/041).

#### Scenario: Existing model lister remains compatible
- **WHEN** an adapter implements `ListModels` but does not implement capability discovery
- **THEN** model listing and ordinary execution remain available, with normalized properties coming from the fallback resolver

#### Scenario: Normalized values reach Execute
- **WHEN** Chat selects `fs=true`, `th=max`, and `ef=high`
- **THEN** the adapter receives those normalized string keys in the execution request without provider-specific fields in the TUI

### Requirement: Each adapter SHALL own native property mapping

Cursor SHALL preserve its established native model/slug composition where applicable, including existing workflow model behavior. OpenCode SHALL serialize normalized C5 values using the native HTTP request keys and value formats supported by its adapter. Native parsing, mapping, and capability API response handling SHALL not be implemented in shared TUI or workflow code (PRD-C05-001 §4.5.2–4; ADR-038/041).

#### Scenario: Cursor keeps native slug behavior
- **WHEN** a Cursor execution receives a normalized fast/thinking/effort selection
- **THEN** the Cursor adapter composes or preserves the established native model slug and does not route the request through OpenCode code

#### Scenario: OpenCode builds a native payload
- **WHEN** OpenCode executes `opencode-go/deepseek-v4-pro` with normalized C5 values
- **THEN** only the OpenCode adapter converts those values into the native HTTP payload, while the shared request remains normalized

### Requirement: Property rejection SHALL be explicit and non-retriable

If a harness rejects a selected property, the adapter SHALL return an error identifying the rejected normalized property and model where available. The TUI/adapter layer SHALL not strip the property, silently retry without it, or convert the rejection into an unrelated pair fallback (PRD-C05-001 §4.5.5; §2 goal 7; UI-C05-001 §5; ADR-041).

#### Scenario: OpenCode rejects an effort value
- **WHEN** the OpenCode API rejects `ef=high` for the selected model
- **THEN** execution fails with a property-aware error naming `ef` and does not retry with `ef` removed

#### Scenario: Cursor rejection is surfaced
- **WHEN** Cursor rejects a composed property/model slug
- **THEN** Chat shows the explicit execution error and the saved user choice remains unchanged for the next correction

### Requirement: Harness adapters SHALL normalize stream events without silent loss

During `Execute` with `Stream: true`, adapters SHALL map harness-native events to normalized `StreamDelta` kinds (`text`, `thinking`, `tool`, `warning`, `permission`, `activity`, `session`). Unrecognized event types SHALL emit `StreamKindWarning` with harness name, event type, session id, and truncated payload. Adapters SHALL NOT ignore parseable events silently (V2.1.1 event-stream design).

#### Scenario: Unknown OpenCode SSE event
- **WHEN** OpenCode `/event` delivers a type not in the versioned handler list
- **THEN** the adapter emits `StreamKindWarning` and continues streaming

#### Scenario: Unknown Cursor stream-json line
- **WHEN** Cursor NDJSON includes a `type` not handled by the parser
- **THEN** the adapter emits `StreamKindWarning` and continues parsing

### Requirement: OpenCode adapter SHALL handle documented SSE event families

The OpenCode adapter SHALL consume `/event` SSE and handle message, tool, permission, session, file, LSP, todo, shell, TUI, and server connection events documented for OpenCode serve. Tool events SHALL map to `StreamKindTool` with `started`/`completed` phases. Message reasoning parts SHALL map to `StreamKindThinking`. Its long-lived `serve` child SHALL inherit the owning TUI's private lifecycle-event endpoint when one is configured, so CLI-as-API commands launched by the harness can publish cycle transitions back to that TUI.

#### Scenario: Tool before and after
- **WHEN** `tool.execute.before` and `tool.execute.after` arrive for a session
- **THEN** Chat receives tool started and completed deltas

#### Scenario: Session error ends execution
- **WHEN** `session.error` arrives for the active session
- **THEN** Execute returns an error and Chat shows the failure

### Requirement: Harness permission prompts SHALL block until user response

When a harness emits a permission request (e.g. OpenCode `permission.asked`), the adapter SHALL invoke `ExecuteRequest.OnPermissionRequest` and block until the callback returns. When the callback is nil, the adapter SHALL emit a warning and fail explicitly rather than hang silently. The TUI SHALL prompt with `Harness permission: … Allow? [y/N]` distinct from Hero stage approval and, when Telegram is paired, SHALL forward the request ID and accept only a correlated `/hero-permission <id> allow|deny` response.

#### Scenario: OpenCode permission approved
- **WHEN** the user approves a harness permission in Chat
- **THEN** the adapter replies via `POST /permission/{requestID}/reply` with `reply: once` and execution continues

#### Scenario: Permission without handler
- **WHEN** `permission.asked` arrives and `OnPermissionRequest` is nil
- **THEN** Execute fails with an explicit permission error after emitting a warning

#### Scenario: Telegram answers a native permission
- **WHEN** OpenCode emits a permission request while a paired Telegram TUI is executing
- **THEN** the TUI forwards the request ID, and `/hero-permission <id> allow` or `/hero-permission <id> deny` releases only that blocked callback

### Requirement: Harness question prompts SHALL block until user response

When a harness emits a question request (e.g. OpenCode `question.asked` / `question.v2.asked`), the adapter SHALL invoke `ExecuteRequest.OnQuestionRequest` and block until the callback returns. When the callback is nil, the adapter SHALL emit a warning and fail explicitly rather than hang silently. The TUI SHALL show formatted questions in Chat and accept answers via the composer (Enter submits, Esc rejects).

#### Scenario: OpenCode question answered
- **WHEN** the user answers all questions in Chat during an OpenCode Execute
- **THEN** the adapter replies via `POST /question/{requestID}/reply` with `answers` per question and execution continues

#### Scenario: OpenCode question rejected
- **WHEN** the user rejects a harness question in Chat (Esc)
- **THEN** the adapter calls `POST /question/{requestID}/reject` and execution continues or fails per harness behavior

#### Scenario: Question without handler
- **WHEN** `question.asked` arrives and `OnQuestionRequest` is nil
- **THEN** Execute fails with an explicit question error after emitting a warning

### Requirement: Cursor stream termination SHALL surface process failure

When Cursor `stream-json` ends without a `result` event and the CLI exits non-zero with empty assistant output, the adapter SHALL return an error including stderr/exit code and emit a failed session delta when streaming.

#### Scenario: Process exit without result
- **WHEN** the Cursor CLI exits with a non-zero code before emitting `result`
- **THEN** Execute fails with stderr detail and does not report success

### Requirement: Harness adapters SHALL expose runtime health probes

Adapters that support TUI Execute SHALL implement `harness.HealthChecker` with `CheckHealth(ctx, sessionID)`. Health probes SHALL be read-only: they MUST NOT start, adopt, stop, or reset harness processes. Cursor probes in-flight CLI process state; OpenCode probes managed serve process, `GET /global/health` (fallback: `/config/providers` liveness), and a read-only session existence GET when a session id is known. Inconclusive session probes (timeout, non-404 errors) MUST NOT set `SessionAlive` to false.

#### Scenario: OpenCode server health
- **WHEN** TUI requests health during an OpenCode Execute
- **THEN** the adapter reports `ServerAlive` from `/global/health` or the documented liveness fallback without spawning or resetting serve

#### Scenario: Cursor process health
- **WHEN** TUI requests health during a Cursor Execute
- **THEN** the adapter reports `ProcessAlive` from the in-flight CLI track

### Requirement: TUI SHALL not block indefinitely on harness Execute

During TUI streaming Execute, Hero SHALL run a generic watchdog (`internal/harness/watchdog.go`) that combines adapter health probes with stream activity timestamps. Probe interval SHALL be 30s; OpenCode stall timeout SHALL be 6m (Cursor stall remains 5m). When the stream delivers substantive activity within the probe interval, the TUI SHALL treat the harness as healthy and MAY skip that interval's `CheckHealth`. On `degraded`, `suspected_hang`, or `failed`, the TUI SHALL surface warnings or error messages only — it MUST NOT cancel Execute, restart the harness, or take other corrective actions from the health path. This requirement applies to Hero TUI only — not Cursor IDE chat Runtime.

#### Scenario: Stalled harness during Execute
- **WHEN** the harness process is alive but no substantive stream activity occurs for the configured stall timeout
- **THEN** the TUI shows a stall warning and does not cancel or reset the harness automatically

#### Scenario: Active stream skips health probe
- **WHEN** substantive stream activity occurred within the last probe interval
- **THEN** the TUI skips `CheckHealth` for that tick and keeps watchdog status healthy

### Requirement: TUI SHALL warn on successful Execute with empty output

When TUI Execute completes without error and the agent transcript has no substantive text or tool output, the TUI SHALL insert a `convRoleWarning` message and set a footer warning. Adapters MAY also return an explicit error for hard empty Cursor stream-json results.

#### Scenario: Empty harness success
- **WHEN** Execute returns success with empty `Output` and no streamed agent content
- **THEN** Chat shows a warning that the harness returned an empty response

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

### Requirement: Remote session history and native deletion SHALL be optional capabilities
Hero SHALL NOT require every adapter to implement remote history reading or native session deletion. Optional capability interfaces SHALL be discovered explicitly. Remote history import SHALL require user confirmation, normalize provider events, and be idempotent via `provider_event_id`. Unsupported or failed remote-history reads SHALL leave local state unchanged and return an actionable message. Permanent deletion SHALL complete the authoritative local purge first and only then best-effort native deletion using the captured harness/native reference. Unsupported or failed remote deletion SHALL warn and MUST NOT resurrect local data. Adapter calls SHALL honor context cancellation and bounded timeouts. Session identifiers and provider payloads MUST NOT be written to diagnostic logs (PRD-C16-001 §3.6–3.7, §4 FR-10, FR-11; ADR-097; UI-C16-001 §§4,7).

#### Scenario: Unsupported import leaves local state unchanged
- **WHEN** the owning adapter does not support remote history and the user requests import
- **THEN** no `session_events` are inserted and Chat explains that import is unavailable

#### Scenario: Confirmed import is idempotent
- **WHEN** the user confirms import and the same provider event IDs are read twice
- **THEN** each provider event appears once in `session_events`

#### Scenario: Native delete is best-effort after local purge
- **WHEN** local deletion succeeded and the adapter does not implement native delete
- **THEN** local data stays deleted and the UI warns that provider-side data may remain

#### Scenario: C16 adapters do not invent delete support
- **WHEN** Cursor, Codex, or Claude adapters are queried for native deletion
- **THEN** the capability is reported unsupported

### Requirement: Exact resume SHALL keep using existing adapter session IDs
Adapters SHALL continue to accept `ExecuteRequest.SessionID` for native resume. Cross-harness resume remains forbidden. C16 MUST NOT add silent new-session fallback when the stored native ID is rejected for exact resume; the conversation service SHALL surface incompatibility for the fork flow instead (PRD-C16-001 §3.5; ADR-095).

#### Scenario: Rejected native ID does not silently recreate
- **WHEN** exact resume of the stored native ID fails
- **THEN** the adapter/service returns a resume-unavailable error rather than silently creating a replacement native session used as the original
