# cli-deterministic-command-suite Specification

## MODIFIED Requirements

### Requirement: Hero CLI SHALL expose deterministic plugin commands

The CLI SHALL provide `hero plugin install|uninstall|list` for official plugins without LLM reasoning. Telegram install SHALL verify platform artifacts and record plugin metadata under the user Hero state directory (ADR-003; ADR-059).

#### Scenario: Plugin list is JSON/table friendly
- **WHEN** the user runs `hero plugin list`
- **THEN** output includes installed plugins with version and daemon path without secrets

#### Scenario: Uninstall removes plugin artifacts
- **WHEN** `hero plugin uninstall telegram` succeeds
- **THEN** plugin metadata and copied daemon binary are removed while unrelated Hero install state remains
