## MODIFIED Requirements

### Requirement: Deduplication and manifest SHALL be session-scoped
Hero SHALL SHA-256 dedupe within a session and maintain a JSON manifest correlating asset id, session id, turn id, origin, MIME type, and path. Session directories SHALL use the Hero session ID once a durable session exists. Sessions registered in the durable store SHALL retain their private directory and managed copies until permanent session deletion; age alone MUST NOT delete them. Startup cleanup MAY remove only directories with no matching `sessions` row (orphans) and non-durable legacy temporary sessions older than seven days. Original user source paths SHALL never be unlinked. Cards for durable sessions SHALL be reconstructable from `session_assets` metadata after restart (PRD-C14-001 §§2.2,2.14; PRD-C16-001 §3.3, §3.7; ADR-080 as amended by ADR-096).

#### Scenario: Same bytes reuse path
- **WHEN** two attachments in one session share content hash
- **THEN** the store reuses the materialized blob and distinct metadata ids still correlate in the manifest

#### Scenario: Expired sessions are removed
- **WHEN** startup finds an unregistered session directory older than retention (no matching `sessions` row)
- **THEN** that directory is deleted and registered durable sessions remain

#### Scenario: Registered durable sessions survive age cleanup
- **WHEN** startup cleanup runs and a registered session directory is older than seven days
- **THEN** that directory is retained

#### Scenario: Orphans and unregistered temp dirs still expire
- **WHEN** startup finds a session directory with no `sessions` row and directory mtime older than retention
- **THEN** that directory is deleted

#### Scenario: Original files survive session delete
- **WHEN** a session with a managed copy and an `external_source` path is permanently deleted
- **THEN** only the managed copy is removed and the original path remains
