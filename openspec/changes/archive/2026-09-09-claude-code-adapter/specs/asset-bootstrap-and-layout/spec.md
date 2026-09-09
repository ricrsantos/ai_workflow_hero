## ADDED Requirements

### Requirement: Asset bootstrap SHALL include opt-in Claude projection and catalog inputs

Embedded assets SHALL contain the Claude projection and native model catalog. Fresh install SHALL project Claude only when selected; upgrade SHALL add disabled state without materializing Claude files; and enable flows SHALL update checksums using the existing ownership rules (PRD-C13-001 §2; ADR-073–074).

#### Scenario: Claude-only fresh install
- **WHEN** a user selects Claude as the only harness during install
- **THEN** install succeeds with `.claude/`, `harnesses.claude.enabled=true`, and the installed Claude catalog

#### Scenario: Disabled upgrade remains clean
- **WHEN** a prior project is upgraded without Claude enabled
- **THEN** no `.claude/` or new `CLAUDE.md` is created and the disabled Claude state is available for later explicit enablement

#### Scenario: Projection checksum
- **WHEN** Claude assets are projected or refreshed
- **THEN** each Hero-managed projected asset is represented in checksums and customized user content follows existing conflict handling

