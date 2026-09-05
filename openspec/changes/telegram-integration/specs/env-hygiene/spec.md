# env-hygiene Specification

## MODIFIED Requirements

### Requirement: Managed gitignore SHALL ignore project log directories

Install and upgrade SHALL extend Hero's managed `.gitignore` block to include `.workflow-hero/logs/` while preserving all user-owned content and existing rules. The legacy single-file `.workflow-hero/tui.log` rule MAY remain for backward compatibility but logs SHALL primarily live under `logs/` (PRD-C09-001 §3.5; ADR-064).

#### Scenario: User gitignore lines are preserved
- **WHEN** upgrade rewrites the managed block
- **THEN** pre-existing user `.gitignore` entries outside the managed block remain byte-identical

#### Scenario: Logs directory is ignored
- **WHEN** install completes on a fresh project
- **THEN** `.workflow-hero/logs/` appears in the managed ignore block
