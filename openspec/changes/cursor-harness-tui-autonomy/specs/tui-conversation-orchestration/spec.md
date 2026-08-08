# tui-conversation-orchestration Specification

## Purpose
TUI provides a conversation surface for harness-driven interações within an etapa (ADR-026; UI-C03-001 §3).

## ADDED Requirements

### Requirement: TUI SHALL stream harness output in real time
When an interação uses harness execution with streaming enabled, the TUI SHALL render agent text incrementally as `stream-json` deltas arrive (PRD-C03-001 §4.3).

#### Scenario: Streaming interação
- **WHEN** the user submits input during an interactive etapa
- **THEN** the transcript updates before the harness execution completes

#### Scenario: Execution complete
- **WHEN** the harness signals completion
- **THEN** the TUI re-enables user input

### Requirement: User input SHALL trigger harness Execute
Each conversational interação SHALL invoke `HarnessAdapter.Execute` (or resume) with the assembled prompt; Go SHALL NOT call an LLM API directly (ADR-003, ADR-026).

#### Scenario: Submit message
- **WHEN** the user submits text in the conversation input
- **THEN** the harness receives an Execute request with that text included in the prompt payload

### Requirement: Cancel SHALL interrupt streaming
When the user interrupts during streaming, the TUI SHOULD call harness `Cancel` when supported and show an interrupted state (UI-C03-001 §7).

#### Scenario: Interrupt during stream
- **WHEN** the user sends cancel during active streaming
- **THEN** streaming stops and the UI shows interruption feedback
