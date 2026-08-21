# cli-deterministic-command-suite Specification

## MODIFIED Requirements

### Requirement: hero doctor SHALL warn when Codex enabled but CLI missing
When `harnesses.codex.enabled` is true and `codex` is not on PATH, `hero doctor` SHALL emit a warn-only line naming `codex-cli` without failing the overall doctor run (UI-C06-001 §8; PRD-C06-001 §4.3).

#### Scenario: Warn-only codex-cli
- **WHEN** doctor runs with Codex enabled and no `codex` binary
- **THEN** output includes `⚠ codex-cli` guidance that Codex will be unavailable until installed

#### Scenario: No warn when Codex disabled
- **WHEN** Codex is not enabled
- **THEN** doctor does not emit the codex-cli warning
