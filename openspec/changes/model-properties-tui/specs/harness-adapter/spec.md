# harness-adapter Specification

## Purpose

Extend the C4 adapter boundary with normalized model-property transport while keeping provider protocol details inside Cursor/OpenCode adapters (PRD-C05-001 §4.2/§4.5; ADR-038/041).

## MODIFIED Requirements

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
