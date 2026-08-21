# model-property-discovery Specification

## MODIFIED Requirements

### Requirement: Model property refresh SHALL include Codex when enabled
When `/hero-model` opens, background capability refresh SHALL query enabled harness adapters including Codex, without starting Codex app-server at mere TUI boot (PRD-C06-001 §4.3; C5 analog).

#### Scenario: Codex refresh on model picker
- **WHEN** Codex is enabled and the user opens `/hero-model`
- **THEN** refresh includes Codex adapter discovery or ListModels-derived capabilities

#### Scenario: Disabled Codex skipped
- **WHEN** Codex is not enabled
- **THEN** refresh does not start Codex app-server solely for metadata
