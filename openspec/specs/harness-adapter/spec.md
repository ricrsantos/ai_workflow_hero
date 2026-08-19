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

The OpenCode adapter SHALL consume `/event` SSE and handle message, tool, permission, session, file, LSP, todo, shell, TUI, and server connection events documented for OpenCode serve. Tool events SHALL map to `StreamKindTool` with `started`/`completed` phases. Message reasoning parts SHALL map to `StreamKindThinking`.

#### Scenario: Tool before and after
- **WHEN** `tool.execute.before` and `tool.execute.after` arrive for a session
- **THEN** Chat receives tool started and completed deltas

#### Scenario: Session error ends execution
- **WHEN** `session.error` arrives for the active session
- **THEN** Execute returns an error and Chat shows the failure

### Requirement: Harness permission prompts SHALL block until user response

When a harness emits a permission request (e.g. OpenCode `permission.asked`), the adapter SHALL invoke `ExecuteRequest.OnPermissionRequest` and block until the callback returns. When the callback is nil, the adapter SHALL emit a warning and fail explicitly rather than hang silently. The TUI SHALL prompt with `Harness permission: … Allow? [y/N]` distinct from Hero stage approval.

#### Scenario: OpenCode permission approved
- **WHEN** the user approves a harness permission in Chat
- **THEN** the adapter replies via `POST /permission/{requestID}/reply` with `reply: once` and execution continues

#### Scenario: Permission without handler
- **WHEN** `permission.asked` arrives and `OnPermissionRequest` is nil
- **THEN** Execute fails with an explicit permission error after emitting a warning

### Requirement: Cursor stream termination SHALL surface process failure

When Cursor `stream-json` ends without a `result` event and the CLI exits non-zero with empty assistant output, the adapter SHALL return an error including stderr/exit code and emit a failed session delta when streaming.

#### Scenario: Process exit without result
- **WHEN** the Cursor CLI exits with a non-zero code before emitting `result`
- **THEN** Execute fails with stderr detail and does not report success
