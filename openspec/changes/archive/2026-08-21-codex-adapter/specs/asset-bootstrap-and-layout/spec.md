# asset-bootstrap-and-layout Specification

## MODIFIED Requirements

### Requirement: Embedded assets SHALL include assets/codex for projection
The binary SHALL embed `assets/codex/` with the same agent/command/skill families as OpenCode projection, sourced from Hero assets (ADR-046; PRD-C06-001 §4.7).

#### Scenario: Codex assets embedded
- **WHEN** `go test` or install reads embedded FS
- **THEN** `assets/codex/` paths are present in the embedded bundle

### Requirement: Install and upgrade SHALL project Codex only when enabled
Install, upgrade, and `/hero-harness` enable SHALL write `.codex/` only when `harnesses.codex.enabled` is true. Upgrade from 2.4.x SHALL NOT auto-enable or auto-project Codex (ADR-048).

#### Scenario: Upgrade leaves Codex disabled
- **WHEN** a 2.4.x project upgrades to 2.5.0 without selecting Codex
- **THEN** `.codex/` is not created and `harnesses.codex.enabled` remains false

#### Scenario: Checksum rules apply to Hero-managed .codex files
- **WHEN** upgrade refreshes Hero-managed `.codex/` files
- **THEN** customized files use conflict backup before replacement, matching `.opencode/` behavior
