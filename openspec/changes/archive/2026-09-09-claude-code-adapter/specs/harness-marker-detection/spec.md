## ADDED Requirements

### Requirement: Harness marker detection SHALL distinguish configured Claude from an unknown marker

Detection SHALL treat `.claude/` as a supported marker only when Hero state explicitly enables Claude; an unconfigured `.claude/` remains a warn-only divergence and SHALL not be auto-enabled or projected over. Existing Cursor, OpenCode, Codex, and unsupported-marker behavior SHALL remain unchanged.

#### Scenario: Enabled Claude marker
- **WHEN** `.claude/` exists and `harnesses.claude.enabled=true`
- **THEN** install/Doctor do not report Claude as an unsupported marker

#### Scenario: Unconfigured Claude marker
- **WHEN** `.claude/` exists but Hero has no enabled Claude state
- **THEN** Doctor warns that the marker is not managed by the current Hero configuration and does not change `hero.json`

