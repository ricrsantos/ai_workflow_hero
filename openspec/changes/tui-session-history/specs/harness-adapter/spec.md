## ADDED Requirements

### Requirement: Remote session history and native deletion SHALL be optional capabilities
Hero SHALL NOT require every adapter to implement remote history reading or native session deletion. Optional capability interfaces SHALL be discovered explicitly. Remote history import SHALL require user confirmation, normalize provider events, and be idempotent via `provider_event_id`. Unsupported or failed remote-history reads SHALL leave local state unchanged and return an actionable message. Permanent deletion SHALL complete the authoritative local purge first and only then best-effort native deletion using the captured harness/native reference. Unsupported or failed remote deletion SHALL warn and MUST NOT resurrect local data. Adapter calls SHALL honor context cancellation and bounded timeouts. Session identifiers and provider payloads MUST NOT be written to diagnostic logs (PRD-C16-001 §3.6–3.7, §4 FR-10, FR-11; ADR-097; UI-C16-001 §§4,7).

#### Scenario: Unsupported import leaves local state unchanged
- **WHEN** the owning adapter does not support remote history and the user requests import
- **THEN** no `session_events` are inserted and Chat explains that import is unavailable

#### Scenario: Confirmed import is idempotent
- **WHEN** the user confirms import and the same provider event IDs are read twice
- **THEN** each provider event appears once in `session_events`

#### Scenario: Native delete is best-effort after local purge
- **WHEN** local deletion succeeded and the adapter does not implement native delete
- **THEN** local data stays deleted and the UI warns that provider-side data may remain

#### Scenario: C16 adapters do not invent delete support
- **WHEN** Cursor, Codex, or Claude adapters are queried for native deletion
- **THEN** the capability is reported unsupported

### Requirement: Exact resume SHALL keep using existing adapter session IDs
Adapters SHALL continue to accept `ExecuteRequest.SessionID` for native resume. Cross-harness resume remains forbidden. C16 MUST NOT add silent new-session fallback when the stored native ID is rejected for exact resume; the conversation service SHALL surface incompatibility for the fork flow instead (PRD-C16-001 §3.5; ADR-095).

#### Scenario: Rejected native ID does not silently recreate
- **WHEN** exact resume of the stored native ID fails
- **THEN** the adapter/service returns a resume-unavailable error rather than silently creating a replacement native session used as the original
