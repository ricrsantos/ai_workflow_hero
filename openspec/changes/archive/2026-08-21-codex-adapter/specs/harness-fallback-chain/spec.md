# harness-fallback-chain Specification

## MODIFIED Requirements

### Requirement: Fallback chain SHALL treat Codex as a first-class harness pair
When an agent specifies `harness: codex` with a native model, resolution SHALL try that pair first, then `fallback_model`, then hard stop — without inventing a third harness (ADR-033; ADR-048; PRD-C06-001 §4.5).

#### Scenario: Codex agent pair tried first
- **WHEN** `planning_agent` uses `harness: codex` and model `gpt-5.4`
- **THEN** Execute attempts Codex with `gpt-5.4` before fallback_model

#### Scenario: Codex unavailable warns then fallback
- **WHEN** the Codex pair is unavailable but fallback_model is available
- **THEN** Chat warns about Codex unavailability and attempts the fallback pair once

#### Scenario: No third harness invented
- **WHEN** both agent pair and fallback_model fail
- **THEN** execution stops with `/hero-continue` guidance and does not silently pick Cursor or OpenCode beyond the configured fallback pair
